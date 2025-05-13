package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type XrayConfig struct {
	Inbounds  []Inbound  `json:"inbounds"`
	Outbounds []Outbound `json:"outbounds"`
}

type Inbound struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Settings struct {
		Auth string `json:"auth"`
	} `json:"settings"`
}

type Outbound struct {
	Protocol string `json:"protocol"`
	Settings struct {
		Vnext []struct {
			Address string `json:"address"`
			Port    int    `json:"port"`
			Users   []struct {
				ID         string `json:"id"`
				Encryption string `json:"encryption"`
				Flow       string `json:"flow,omitempty"`
			} `json:"users"`
		} `json:"vnext"`
	} `json:"settings"`
	StreamSettings struct {
		Security        string `json:"security"`
		RealitySettings struct {
			ServerName string `json:"serverName"`
			PublicKey  string `json:"publicKey"`
			ShortId    string `json:"shortId"`
		} `json:"realitySettings"`
	} `json:"streamSettings"`
}

var currentCmd *exec.Cmd

func main() {
	myApp := app.New()
	myWindow := myApp.NewWindow("Swift-Xray")
	myWindow.Resize(fyne.NewSize(600, 400))

	// Сохранённые ссылки
	var links []string
	loadSavedLinks(&links)

	// Элементы интерфейса
	linkEntry := widget.NewEntry()
	linkEntry.SetPlaceHolder("Введите VLESS ссылку...")

	logOutput := widget.NewMultiLineEntry()
	logOutput.SetMinRowsVisible(10)
	logOutput.Wrapping = fyne.TextWrapBreak
	logOutput.Disable()

	// Dropdown для выбора конфигурации
	confDropdown := widget.NewSelect(links, func(selected string) {
		linkEntry.SetText(selected)
	})

	// Сохранение и запуск
	btn := widget.NewButton("Сохранить и запустить", func() {
		link := strings.TrimSpace(linkEntry.Text)
		if !strings.HasPrefix(link, "vless://") {
			dialog.ShowError(fmt.Errorf("Неверный формат ссылки"), myWindow)
			return
		}

		conf := parseVLESS(link)
		writeConfig(conf)

		// Сохраняем ссылку
		links = append(links, link)
		writeLinks(links)

		// Запускаем xray
		go func() {
			err := runXray(func(output string) {
				logOutput.SetText(logOutput.Text + output)
			})
			if err != nil {
				dialog.ShowError(err, myWindow)
			}
		}()
	})

	// Остановка xray
	stopBtn := widget.NewButton("Остановить", func() {
		if currentCmd != nil && currentCmd.Process != nil {
			currentCmd.Process.Kill()
			dialog.ShowInformation("Остановка", "Сервер остановлен", myWindow)
		}
	})

	// Размещение элементов
	myWindow.SetContent(container.NewVBox(
		widget.NewLabel("VLESS ссылка:"),
		linkEntry,
		btn,
		stopBtn,
		widget.NewLabel("Выбор конфигурации:"),
		confDropdown,
		widget.NewLabel("Логи xray.exe:"),
		logOutput,
	))

	myWindow.ShowAndRun()
}

// Функция парсинга VLESS ссылки
func parseVLESS(link string) XrayConfig {
	link = strings.TrimPrefix(link, "vless://")
	parts := strings.SplitN(link, "#", 2)
	raw := parts[0]

	u, _ := url.Parse("vless://" + raw)
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "443"
	}
	query := u.Query()
	uuid := u.User.Username()
	sni := query.Get("sni")
	publicKey := query.Get("pbk")
	flow := query.Get("flow")
	shortId := query.Get("sid")

	if publicKey == "" || shortId == "" {
		log.Fatalf("Ошибка: отсутствует pbk или sid в ссылке")
	}

	xconf := XrayConfig{
		Inbounds: []Inbound{
			{
				Port:     10809,
				Protocol: "http",
			},
		},
		Outbounds: []Outbound{
			{
				Protocol: "vless",
			},
		},
	}
	xconf.Inbounds[0].Settings.Auth = "noauth"

	out := &xconf.Outbounds[0]
	pint, _ := parsePort(port)

	vnext := struct {
		Address string `json:"address"`
		Port    int    `json:"port"`
		Users   []struct {
			ID         string `json:"id"`
			Encryption string `json:"encryption"`
			Flow       string `json:"flow,omitempty"`
		} `json:"users"`
	}{
		Address: host,
		Port:    pint,
	}

	user := struct {
		ID         string `json:"id"`
		Encryption string `json:"encryption"`
		Flow       string `json:"flow,omitempty"`
	}{
		ID:         uuid,
		Encryption: "none",
		Flow:       flow,
	}
	vnext.Users = append(vnext.Users, user)

	out.Settings.Vnext = append(out.Settings.Vnext, vnext)
	out.StreamSettings.Security = "reality"
	out.StreamSettings.RealitySettings.ServerName = sni
	out.StreamSettings.RealitySettings.PublicKey = publicKey
	out.StreamSettings.RealitySettings.ShortId = shortId

	return xconf
}

// Парсинг порта
func parsePort(p string) (int, error) {
	var i int
	_, err := fmt.Sscanf(p, "%d", &i)
	return i, err
}

// Запись конфигурации в файл config.json
func writeConfig(conf XrayConfig) {
	file, err := os.Create("bin/config.json")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	enc.Encode(conf)
}

// Запуск xray.exe
func runXray(logFn func(string)) error {
	cmd := exec.Command("bin/xray.exe", "-config", "config.json")
	currentCmd = cmd // Сохраняем текущую команду

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("не удалось запустить xray.exe: %v", err)
	}

	go copyOutput(stdout, "STDOUT", logFn)
	go copyOutput(stderr, "STDERR", logFn)

	return cmd.Wait()
}

// Функция для копирования вывода
func copyOutput(pipe io.Reader, prefix string, logFn func(string)) {
	buf := make([]byte, 1024)
	for {
		n, err := pipe.Read(buf)
		if err != nil {
			break
		}
		if n > 0 {
			logFn(fmt.Sprintf("[%s] %s", prefix, string(buf[:n])))
		}
	}
}

// Загрузка сохранённых ссылок
// Загрузка сохранённых ссылок
// Загрузка сохранённых ссылок
func loadSavedLinks(links *[]string) {
	file, err := os.Open("configs/links.json")
	if err != nil {
		if os.IsNotExist(err) {
			// Если файл не существует, просто возвращаем
			return
		}
		log.Fatalf("Ошибка при открытии файла links.json: %v", err)
	}
	defer file.Close()

	// Проверяем, не пустой ли файл
	fileInfo, err := file.Stat()
	if err != nil {
		log.Fatalf("Ошибка при получении информации о файле: %v", err)
	}

	// Если файл пустой, просто возвращаем
	if fileInfo.Size() == 0 {
		log.Println("Файл links.json пуст.")
		return
	}

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(links); err != nil {
		// Логируем ошибку, если файл имеет неправильный формат JSON
		log.Printf("Ошибка при чтении links.json: %v", err)
		// Инициализируем пустой список, если не удалось загрузить данные
		*links = []string{}
	}
}

// Запись ссылок в файл
// Запись ссылок в файл
func writeLinks(links []string) {
	// Check for duplicates before writing the file
	file, err := os.Create("configs/links.json")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	// Avoid writing the link if it's already present in the file
	uniqueLinks := make(map[string]bool)
	for _, link := range links {
		if !uniqueLinks[link] {
			uniqueLinks[link] = true
		}
	}

	// Write unique links
	uniqueLinksList := []string{}
	for link := range uniqueLinks {
		uniqueLinksList = append(uniqueLinksList, link)
	}

	err = encoder.Encode(uniqueLinksList)
	if err != nil {
		log.Fatal(err)
	}
}
