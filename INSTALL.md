# 📥 Установка языка "Простой"

## Windows 🪟

### Вариант 1: Инсталлятор (рекомендуется)
1. Скачайте `prostoy-setup-0.40-windows.exe`
2. Запустите установщик
3. Следуйте инструкциям мастера установки
4. ✅ Готово! Команда `prostoy` доступна в командной строке

### Вариант 2: Portable версия
1. Скачайте `prostoy-0.40-windows-amd64.zip`
2. Распакуйте в любую папку
3. Добавьте папку в PATH или запускайте через полный путь

---

## macOS 🍎

### Вариант 1: DMG образ (рекомендуется)
1. Скачайте `prostoy-0.40-macos.dmg`
2. Откройте DMG файл
3. Перетащите **Простой.app** в папку **Applications**
4. Запустите **Установить в PATH/install.command** для доступа из терминала
5. ✅ Готово! Команда `prostoy` доступна в Terminal

### Вариант 2: Homebrew (скоро)
```bash
brew install prostoy
```

### Вариант 3: Ручная установка
```bash
# Скачайте архив
curl -L https://github.com/user/prostoy-lang/releases/latest/download/prostoy-0.40-macos-universal.tar.gz -o prostoy.tar.gz

# Распакуйте
tar -xzf prostoy.tar.gz

# Установите (выберите версию для вашей архитектуры)
sudo cp prostoy-macos-arm64 /usr/local/bin/prostoy  # Apple Silicon (M1/M2/M3)
# или
sudo cp prostoy-macos-amd64 /usr/local/bin/prostoy  # Intel

# Дайте права на выполнение
sudo chmod +x /usr/local/bin/prostoy
```

---

## Linux 🐧

### Ubuntu/Debian

#### Вариант 1: DEB пакет (рекомендуется)
```bash
# Скачайте пакет
wget https://github.com/user/prostoy-lang/releases/latest/download/prostoy_0.40_amd64.deb

# Установите
sudo apt install ./prostoy_0.40_amd64.deb

# Или через dpkg
sudo dpkg -i prostoy_0.40_amd64.deb
```

#### Вариант 2: Из архива
```bash
# Скачайте
wget https://github.com/user/prostoy-lang/releases/latest/download/prostoy-0.40-linux-amd64.tar.gz

# Распакуйте
tar -xzf prostoy-0.40-linux-amd64.tar.gz

# Установите
sudo cp prostoy-linux-amd64 /usr/local/bin/prostoy
sudo chmod +x /usr/local/bin/prostoy
```

### Fedora/RHEL/CentOS

```bash
# Скачайте RPM (скоро)
# sudo rpm -i prostoy-0.40-1.x86_64.rpm

# Пока используйте архив
wget https://github.com/user/prostoy-lang/releases/latest/download/prostoy-0.40-linux-amd64.tar.gz
tar -xzf prostoy-0.40-linux-amd64.tar.gz
sudo cp prostoy-linux-amd64 /usr/local/bin/prostoy
sudo chmod +x /usr/local/bin/prostoy
```

### Arch Linux

```bash
# AUR пакет (скоро)
# yay -S prostoy

# Пока используйте архив
wget https://github.com/user/prostoy-lang/releases/latest/download/prostoy-0.40-linux-amd64.tar.gz
tar -xzf prostoy-0.40-linux-amd64.tar.gz
sudo cp prostoy-linux-amd64 /usr/local/bin/prostoy
sudo chmod +x /usr/local/bin/prostoy
```

---

## Проверка установки

После установки проверьте:

```bash
prostoy --version
```

Запустите REPL:

```bash
prostoy
```

Запустите программу:

```bash
prostoy программа.pr
```

---

## Сборка из исходников

Требуется Go 1.21+

```bash
# Клонируйте репозиторий
git clone https://github.com/user/prostoy-lang
cd prostoy-lang

# Соберите
go build -o prostoy main.go

# Установите
sudo cp prostoy /usr/local/bin/
```

---

## Удаление

### Windows
Используйте "Установка и удаление программ" или запустите uninstall.exe из папки установки

### macOS
```bash
sudo rm /usr/local/bin/prostoy
rm -rf /Applications/Простой.app
```

### Linux (DEB)
```bash
sudo apt remove prostoy
```

### Linux (ручная установка)
```bash
sudo rm /usr/local/bin/prostoy
```

---

## Поддержка

- 📚 [Документация](РУКОВОДСТВО.md)
- 🐛 [Сообщить об ошибке](https://github.com/user/prostoy-lang/issues)
- 💬 [Telegram чат](https://t.me/prostoy_lang)
- 📧 Email: team@prostoy-lang.org
