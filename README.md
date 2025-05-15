# swift-xray

`swift-xray` — это простая утилита на Go, которая подключается по VLESS-ссылке и поднимает локальный прокси на `127.0.0.1:10809`, используя [Xray-core (XTLS/Xray-core)](https://github.com/XTLS/Xray-core) в качестве ядра.

---

## 🧰 Возможности

- Подключение по VLESS-ссылке
- Автоматическая генерация конфигурации для Xray
- Запуск прокси-сервера на `127.0.0.1:10809`
- Кроссплатформенность (Windows / Linux )

---

## ⚙️ Установка

1. Убедитесь, что установлен Go (версия 1.18+).
2. Склонируйте репозиторий:

```bash
git clone https://github.com/swiftmessage/Swift-Xray.git
cd swift-xray
```

## Сборка проекта Windows

Установка зависимостей

```bash
go mod tidy
```

## Установка ядра Xray

```bash
https://github.com/XTLS/Xray-core/releases/tag/v25.4.30
```

Выбирите подходящий release под Windows и распакуйте содержимое в папку bin

## Сборка exe

```bash
go build -o swift-xray.exe main.go
```

## Запуск

```bash
sudo ./swift-xray.exe

```
