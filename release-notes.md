# Ясный v0.42

Первый официальный релиз языка программирования Ясный на русском языке.

## Возможности

- 🇷🇺 Полностью на русском языке
- 📦 Модульная система (импорт/экспорт)
- 🔧 62 встроенные функции
- ⚡ Современные возможности: деструктуризация, spread, optional chaining, pattern matching
- 📝 List comprehension
- 🌐 HTTP сервер и клиент
- 📄 Работа с JSON и файлами
- 🎨 VS Code расширение с автодополнением

## Установка

### Linux
```bash
wget https://github.com/warcorprp-web/yasny-lang/releases/download/v0.42/yasny-0.42-linux-amd64.tar.gz
tar -xzf yasny-0.42-linux-amd64.tar.gz
sudo ./install.sh
```

### macOS (Intel)
```bash
curl -L https://github.com/warcorprp-web/yasny-lang/releases/download/v0.42/yasny-0.42-macos-intel.dmg.tar.gz -o yasny.tar.gz
tar -xzf yasny.tar.gz
./install.command
```

### Windows
1. Скачайте `yasny-0.42-windows-amd64.zip`
2. Распакуйте
3. Запустите `install.bat` от имени администратора

### VS Code расширение
```bash
code --install-extension yasny-lang-0.42.0.vsix
```

## Быстрый старт

Создайте файл `hello.ya`:
```yasny
вывод("Привет, мир!")
```

Запустите:
```bash
yasny hello.ya
```

## Документация

- [Полное руководство](https://github.com/warcorprp-web/yasny-lang/blob/main/РУКОВОДСТВО.md)
- [README](https://github.com/warcorprp-web/yasny-lang/blob/main/README.md)

## Что нового

- Первый релиз
- Расширение файлов: `.ya`
- Команда запуска: `yasny`
- Готовые установщики для всех платформ
