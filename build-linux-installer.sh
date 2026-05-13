#!/bin/bash

VERSION="0.43"

echo "Создаю установщик для Linux..."

# Создаю структуру
mkdir -p linux-installer
cp dist/yasny-linux-amd64 linux-installer/yasny
chmod +x linux-installer/yasny

# Создаю install.sh
cat > linux-installer/install.sh << 'EOF'
#!/bin/bash

echo "Установка Ясный v0.43..."

# Проверка прав
if [ "$EUID" -ne 0 ]; then 
    echo "Запустите с sudo: sudo ./install.sh"
    exit 1
fi

# Копируем бинарник
cp yasny /usr/local/bin/yasny
chmod +x /usr/local/bin/yasny

echo "✅ Установка завершена!"
echo "Запустите: yasny"
EOF
chmod +x linux-installer/install.sh

# Создаю README
cat > linux-installer/README.txt << 'EOF'
Установка Ясный на Linux

Быстрая установка:
  sudo ./install.sh

Ручная установка:
  sudo cp yasny /usr/local/bin/
  sudo chmod +x /usr/local/bin/yasny

Проверка:
  yasny

Документация:
  https://github.com/warcorprp-web/yasny-lang
EOF

# Создаю архив
cd linux-installer
tar -czf ../yasny-${VERSION}-linux-amd64.tar.gz *
cd ..
rm -rf linux-installer

echo "✅ Готово: yasny-${VERSION}-linux-amd64.tar.gz"
ls -lh yasny-${VERSION}-linux-amd64.tar.gz
