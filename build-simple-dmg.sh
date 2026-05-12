#!/bin/bash

VERSION="0.42"
BINARY="dist/yasny-macos-amd64"
APP_NAME="Yasny"
DMG_NAME="yasny-${VERSION}-macos-intel.dmg"

echo "Создаю DMG для Intel Mac..."

# Создаю временную папку
mkdir -p dmg-temp
cp $BINARY dmg-temp/yasny
chmod +x dmg-temp/yasny

# Создаю install скрипт
cat > dmg-temp/install.command << 'EOF'
#!/bin/bash
cd "$(dirname "$0")"
echo "Установка Ясный..."
sudo cp yasny /usr/local/bin/yasny
sudo chmod +x /usr/local/bin/yasny
echo "✅ Готово! Запусти: yasny"
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
yasny

Документация:
https://github.com/warcorprp-web/yasny-lang
EOF

# Создаю tar.gz
cd dmg-temp
tar -czf ../$DMG_NAME.tar.gz *
cd ..
rm -rf dmg-temp

echo "✅ Готово: $DMG_NAME.tar.gz"
ls -lh $DMG_NAME.tar.gz
