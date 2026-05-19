#!/bin/bash
# Главный скрипт сборки всех дистрибутивов Ясный

set -e

VERSION="0.50"

echo "╔════════════════════════════════════════╗"
echo "║  Сборка дистрибутивов Ясный v${VERSION}        ║"
echo "╚════════════════════════════════════════╝"
echo ""

# Создаём директории
mkdir -p dist installers build

# 1. Компилируем бинарники для всех платформ
echo "📦 Компиляция бинарников..."
./build.sh

# 2. Создаём архивы для ручной установки
echo ""
echo "📦 Создание архивов..."

cd installers
rm -f *.tar.gz *.zip *.deb SHA256SUMS

# --- Linux amd64 ---
mkdir -p tmp-linux-amd64
cp ../dist/yasny-linux-amd64 tmp-linux-amd64/yasny
chmod +x tmp-linux-amd64/yasny
cp ../РУКОВОДСТВО.md ../README.md tmp-linux-amd64/
cat > tmp-linux-amd64/install.sh << 'EOF'
#!/bin/bash
echo "Установка Ясный v0.50..."
if [ "$EUID" -ne 0 ]; then
    echo "Запустите с sudo: sudo ./install.sh"
    exit 1
fi
cp yasny /usr/local/bin/yasny
chmod +x /usr/local/bin/yasny
echo "✅ Установка завершена! Запустите: yasny"
EOF
chmod +x tmp-linux-amd64/install.sh
tar czf "yasny-${VERSION}-linux-amd64.tar.gz" -C tmp-linux-amd64 .
rm -rf tmp-linux-amd64
echo "  ✓ yasny-${VERSION}-linux-amd64.tar.gz"

# --- Linux arm64 ---
mkdir -p tmp-linux-arm64
cp ../dist/yasny-linux-arm64 tmp-linux-arm64/yasny
chmod +x tmp-linux-arm64/yasny
cp ../РУКОВОДСТВО.md ../README.md tmp-linux-arm64/
cat > tmp-linux-arm64/install.sh << 'EOF'
#!/bin/bash
echo "Установка Ясный v0.50..."
if [ "$EUID" -ne 0 ]; then echo "Запустите с sudo"; exit 1; fi
cp yasny /usr/local/bin/yasny
chmod +x /usr/local/bin/yasny
echo "✅ Готово!"
EOF
chmod +x tmp-linux-arm64/install.sh
tar czf "yasny-${VERSION}-linux-arm64.tar.gz" -C tmp-linux-arm64 .
rm -rf tmp-linux-arm64
echo "  ✓ yasny-${VERSION}-linux-arm64.tar.gz"

# --- macOS Intel (amd64) ---
mkdir -p tmp-macos-intel
cp ../dist/yasny-macos-amd64 tmp-macos-intel/yasny
chmod +x tmp-macos-intel/yasny
cp ../РУКОВОДСТВО.md ../README.md tmp-macos-intel/
cat > tmp-macos-intel/install.command << 'EOF'
#!/bin/bash
cd "$(dirname "$0")"
echo "Установка Ясный v0.50..."
sudo cp yasny /usr/local/bin/yasny
sudo chmod +x /usr/local/bin/yasny
echo "✅ Готово! Запустите: yasny"
read -p "Нажмите Enter для выхода..."
EOF
chmod +x tmp-macos-intel/install.command
tar czf "yasny-${VERSION}-macos-intel.tar.gz" -C tmp-macos-intel .
rm -rf tmp-macos-intel
echo "  ✓ yasny-${VERSION}-macos-intel.tar.gz"

# --- macOS Apple Silicon (arm64) ---
mkdir -p tmp-macos-arm64
cp ../dist/yasny-macos-arm64 tmp-macos-arm64/yasny
chmod +x tmp-macos-arm64/yasny
cp ../РУКОВОДСТВО.md ../README.md tmp-macos-arm64/
cat > tmp-macos-arm64/install.command << 'EOF'
#!/bin/bash
cd "$(dirname "$0")"
echo "Установка Ясный v0.50..."
sudo cp yasny /usr/local/bin/yasny
sudo chmod +x /usr/local/bin/yasny
echo "✅ Готово!"
read -p "Нажмите Enter..."
EOF
chmod +x tmp-macos-arm64/install.command
tar czf "yasny-${VERSION}-macos-arm64.tar.gz" -C tmp-macos-arm64 .
rm -rf tmp-macos-arm64
echo "  ✓ yasny-${VERSION}-macos-arm64.tar.gz"

# --- Windows ---
mkdir -p tmp-windows
cp ../dist/yasny-windows-amd64.exe tmp-windows/yasny.exe
cp ../РУКОВОДСТВО.md ../README.md tmp-windows/
cat > tmp-windows/install.bat << 'EOF'
@echo off
echo Установка Ясный v0.50...
echo.
set INSTALL_DIR=%ProgramFiles%\Yasny
mkdir "%INSTALL_DIR%" 2>nul
copy /Y yasny.exe "%INSTALL_DIR%\yasny.exe"
setx PATH "%PATH%;%INSTALL_DIR%" /M
echo.
echo Установка завершена!
echo Перезапустите cmd и запустите: yasny
pause
EOF
zip -j -q "yasny-${VERSION}-windows-amd64.zip" tmp-windows/*
rm -rf tmp-windows
echo "  ✓ yasny-${VERSION}-windows-amd64.zip"

# --- VS Code расширение ---
if [ -f ../yasny-lang/vscode-extension/yasny-lang-0.50.0.vsix ] || [ -f ../vscode-extension/yasny-lang-0.50.0.vsix ]; then
    cp ../vscode-extension/yasny-lang-*.vsix . 2>/dev/null && echo "  ✓ VS Code .vsix"
fi

# --- Контрольные суммы ---
echo ""
echo "🔐 Создание контрольных сумм..."
sha256sum *.tar.gz *.zip 2>/dev/null > SHA256SUMS
cat SHA256SUMS

cd ..

echo ""
echo "╔════════════════════════════════════════╗"
echo "║       Сборка завершена! ✅             ║"
echo "╚════════════════════════════════════════╝"
echo ""
echo "Файлы в installers/:"
ls -lh installers/
