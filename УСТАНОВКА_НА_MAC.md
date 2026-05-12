# 🍎 Установка "Простой" на macOS

## Шаг 1: Установка интерпретатора

### Вариант A: Скопировать готовый бинарник

```bash
# Скачай файл prostoy-macos-arm64 (для M1/M2/M3) 
# или prostoy-macos-amd64 (для Intel)

# Установи
sudo cp prostoy-macos-arm64 /usr/local/bin/prostoy
sudo chmod +x /usr/local/bin/prostoy

# Проверь
prostoy --version
```

### Вариант B: Собрать из исходников

```bash
# Клонируй репозиторий
git clone https://github.com/user/prostoy-lang
cd prostoy-lang

# Собери
go build -o prostoy main.go

# Установи
sudo cp prostoy /usr/local/bin/
```

## Шаг 2: Проверка

```bash
# Запусти REPL
prostoy

# Создай тестовый файл
echo 'вывод("Привет, мир!")' > test.pr
prostoy test.pr
```

## Шаг 3: Установка расширения для VSCodium

```bash
cd vscode-extension

# Установи зависимости
npm install

# Собери расширение
npm run compile
npm run package

# Установи в VSCodium
codium --install-extension prostoy-lang-0.40.0.vsix
```

Готово! Теперь открой любой `.pr` файл в VSCodium.
