#!/bin/bash

VERSION="0.43"

echo "Создаю установщик для Windows..."

# Создаю структуру
mkdir -p windows-installer
cp dist/yasny-windows-amd64.exe windows-installer/yasny.exe

# Создаю install.bat
cat > windows-installer/install.bat << 'EOF'
@echo off
echo Установка Ясный v0.43...
echo.

set INSTALL_DIR=%ProgramFiles%\Yasny

echo Создание папки %INSTALL_DIR%...
mkdir "%INSTALL_DIR%" 2>nul

echo Копирование файлов...
copy yasny.exe "%INSTALL_DIR%\yasny.exe"

echo Добавление в PATH...
setx PATH "%PATH%;%INSTALL_DIR%" /M

echo.
echo ✅ Установка завершена!
echo Перезапустите командную строку и запустите: yasny
echo.
pause
EOF

# Создаю README
cat > windows-installer/README.txt << 'EOF'
Установка Ясный на Windows

Быстрая установка:
  1. Запустите install.bat от имени администратора
  2. Перезапустите командную строку
  3. Готово!

Ручная установка:
  1. Скопируйте yasny.exe в C:\Program Files\Yasny\
  2. Добавьте C:\Program Files\Yasny в PATH

Проверка:
  yasny

Документация:
  https://github.com/warcorprp-web/yasny-lang
EOF

# Создаю архив
cd windows-installer
zip -q ../yasny-${VERSION}-windows-amd64.zip *
cd ..
rm -rf windows-installer

echo "✅ Готово: yasny-${VERSION}-windows-amd64.zip"
ls -lh yasny-${VERSION}-windows-amd64.zip
