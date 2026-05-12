#!/bin/bash

VERSION="0.40"
BINARY="dist/prostoy-macos-amd64"
APP_NAME="Prostoy"
DMG_NAME="prostoy-${VERSION}-macos-intel.dmg"

echo "Создаю DMG для Intel Mac..."

# Создаю временную папку
mkdir -p dmg-temp
cp $BINARY dmg-temp/prostoy
chmod +x dmg-temp/prostoy

# Создаю install скрипт
cat > dmg-temp/install.command << 'EOF'
#!/bin/bash
cd "$(dirname "$0")"
echo "Установка Простой..."
sudo cp prostoy /usr/local/bin/prostoy
sudo chmod +x /usr/local/bin/prostoy
echo "✅ Готово! Запусти: prostoy"
read -p "Нажми Enter..."
EOF
chmod +x dmg-temp/install.command

# Создаю README
cat > dmg-temp/README.txt << 'EOF'
Установка:
1. Запусти install.command
2. Введи пароль
3. Готово!

Проверка:
prostoy --version

Документация:
https://github.com/warcorprp-web/prostoy-lang
EOF

# Создаю tar.gz (вместо DMG, т.к. нет macOS)
cd dmg-temp
tar -czf ../$DMG_NAME.tar.gz *
cd ..
rm -rf dmg-temp

echo "✅ Готово: $DMG_NAME.tar.gz"
ls -lh $DMG_NAME.tar.gz
