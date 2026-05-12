#!/bin/bash
# Главный скрипт сборки всех дистрибутивов

set -e

VERSION="0.40"

echo "╔════════════════════════════════════════╗"
echo "║  Сборка дистрибутивов Простой v${VERSION} ║"
echo "╚════════════════════════════════════════╝"
echo ""

# Создаем директории
mkdir -p dist installers build

# 1. Компилируем бинарники для всех платформ
echo "📦 Компиляция бинарников..."
./build.sh

# 2. Windows инсталлятор (требует Inno Setup на Windows или Wine)
echo ""
echo "🪟 Windows инсталлятор..."
if command -v iscc &> /dev/null; then
    iscc installer-windows.iss
    echo "✅ prostoy-setup-${VERSION}-windows.exe"
else
    echo "⚠️  Inno Setup не найден, пропускаем Windows инсталлятор"
    echo "   Установите: https://jrsoftware.org/isinfo.php"
fi

# 3. macOS DMG (только на macOS)
echo ""
echo "🍎 macOS DMG..."
if [[ "$OSTYPE" == "darwin"* ]]; then
    chmod +x build-macos-dmg.sh
    ./build-macos-dmg.sh
    echo "✅ prostoy-${VERSION}-macos.dmg"
else
    echo "⚠️  Сборка DMG доступна только на macOS"
fi

# 4. Linux DEB пакет
echo ""
echo "🐧 Linux DEB пакет..."
if command -v dpkg-deb &> /dev/null; then
    chmod +x build-deb.sh
    ./build-deb.sh
    echo "✅ prostoy_${VERSION}_amd64.deb"
else
    echo "⚠️  dpkg-deb не найден, пропускаем DEB пакет"
fi

# 5. Создаем архивы для ручной установки
echo ""
echo "📦 Создание архивов..."

# Linux
tar -czf "installers/prostoy-${VERSION}-linux-amd64.tar.gz" \
    -C dist prostoy-linux-amd64 \
    -C .. РУКОВОДСТВО.md README.md
echo "✅ prostoy-${VERSION}-linux-amd64.tar.gz"

# macOS
tar -czf "installers/prostoy-${VERSION}-macos-universal.tar.gz" \
    -C dist prostoy-macos-amd64 prostoy-macos-arm64 \
    -C .. РУКОВОДСТВО.md README.md
echo "✅ prostoy-${VERSION}-macos-universal.tar.gz"

# Windows
zip -j "installers/prostoy-${VERSION}-windows-amd64.zip" \
    dist/prostoy-windows-amd64.exe \
    РУКОВОДСТВО.md README.md
echo "✅ prostoy-${VERSION}-windows-amd64.zip"

# 6. Создаем checksums
echo ""
echo "🔐 Создание контрольных сумм..."
cd installers
sha256sum * > SHA256SUMS
cd ..
echo "✅ SHA256SUMS"

echo ""
echo "╔════════════════════════════════════════╗"
echo "║       Сборка завершена! ✅             ║"
echo "╚════════════════════════════════════════╝"
echo ""
echo "Созданные файлы в installers/:"
ls -lh installers/
echo ""
echo "Загрузите на GitHub Releases:"
echo "  gh release create v${VERSION} installers/* --title 'Простой v${VERSION}'"
