#!/bin/bash
# Создание DMG инсталлятора для macOS

set -e

VERSION="0.40"
APP_NAME="Простой"
DMG_NAME="prostoy-${VERSION}-macos"

echo "Создание macOS DMG инсталлятора..."

# Создаем структуру .app
mkdir -p "build/${APP_NAME}.app/Contents/MacOS"
mkdir -p "build/${APP_NAME}.app/Contents/Resources"

# Info.plist
cat > "build/${APP_NAME}.app/Contents/Info.plist" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>prostoy</string>
    <key>CFBundleIdentifier</key>
    <string>com.prostoy.lang</string>
    <key>CFBundleName</key>
    <string>${APP_NAME}</string>
    <key>CFBundleVersion</key>
    <string>${VERSION}</string>
    <key>CFBundleShortVersionString</key>
    <string>${VERSION}</string>
    <key>CFBundleIconFile</key>
    <string>icon.icns</string>
    <key>LSMinimumSystemVersion</key>
    <string>10.13</string>
    <key>NSHighResolutionCapable</key>
    <true/>
</dict>
</plist>
EOF

# Определяем архитектуру
ARCH=$(uname -m)
if [ "$ARCH" = "arm64" ]; then
    BINARY="dist/prostoy-macos-arm64"
else
    BINARY="dist/prostoy-macos-amd64"
fi

# Копируем бинарник
cp "$BINARY" "build/${APP_NAME}.app/Contents/MacOS/prostoy"
chmod +x "build/${APP_NAME}.app/Contents/MacOS/prostoy"

# Создаем wrapper скрипт для Terminal
cat > "build/${APP_NAME}.app/Contents/MacOS/prostoy-terminal" << 'EOF'
#!/bin/bash
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
osascript -e 'tell application "Terminal" to do script "'"$DIR/prostoy"'"'
EOF
chmod +x "build/${APP_NAME}.app/Contents/MacOS/prostoy-terminal"

# Копируем документацию
mkdir -p "build/Документация"
cp РУКОВОДСТВО.md "build/Документация/"
cp README.md "build/Документация/"

# Создаем симлинк для установки в /usr/local/bin
mkdir -p "build/Установить в PATH"
cat > "build/Установить в PATH/install.command" << 'EOF'
#!/bin/bash
cd "$(dirname "$0")"
sudo ln -sf "/Applications/Простой.app/Contents/MacOS/prostoy" /usr/local/bin/prostoy
echo "✅ Установлено в /usr/local/bin/prostoy"
echo "Теперь можно использовать команду 'prostoy' в терминале"
read -p "Нажмите Enter для закрытия..."
EOF
chmod +x "build/Установить в PATH/install.command"

# Создаем DMG
echo "Создание DMG образа..."
hdiutil create -volname "${APP_NAME} ${VERSION}" \
    -srcfolder build \
    -ov -format UDZO \
    "installers/${DMG_NAME}.dmg"

echo "✅ DMG создан: installers/${DMG_NAME}.dmg"
echo ""
echo "Пользователь сможет:"
echo "  1. Перетащить ${APP_NAME}.app в /Applications"
echo "  2. Запустить 'Установить в PATH/install.command' для доступа из терминала"
