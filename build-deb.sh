#!/bin/bash
# Создание DEB пакета для Debian/Ubuntu

set -e

VERSION="0.40"
ARCH="amd64"  # или arm64

echo "Создание DEB пакета..."

# Создаем структуру пакета
mkdir -p "build-deb/prostoy_${VERSION}_${ARCH}/DEBIAN"
mkdir -p "build-deb/prostoy_${VERSION}_${ARCH}/usr/bin"
mkdir -p "build-deb/prostoy_${VERSION}_${ARCH}/usr/share/doc/prostoy"
mkdir -p "build-deb/prostoy_${VERSION}_${ARCH}/usr/share/man/man1"

# Control файл
cat > "build-deb/prostoy_${VERSION}_${ARCH}/DEBIAN/control" << EOF
Package: prostoy
Version: ${VERSION}
Section: devel
Priority: optional
Architecture: ${ARCH}
Maintainer: Prostoy Lang Team <team@prostoy-lang.org>
Description: Язык программирования "Простой" на русском языке
 Современный язык программирования с русским синтаксисом.
 Включает REPL, поддержку модулей, классов, функционального
 программирования и многое другое.
Homepage: https://github.com/user/prostoy-lang
EOF

# Postinst скрипт
cat > "build-deb/prostoy_${VERSION}_${ARCH}/DEBIAN/postinst" << 'EOF'
#!/bin/bash
echo "✅ Простой установлен!"
echo "Использование:"
echo "  prostoy программа.pr    # Запуск файла"
echo "  prostoy                 # REPL режим"
EOF
chmod +x "build-deb/prostoy_${VERSION}_${ARCH}/DEBIAN/postinst"

# Копируем файлы
cp "dist/prostoy-linux-${ARCH}" "build-deb/prostoy_${VERSION}_${ARCH}/usr/bin/prostoy"
chmod +x "build-deb/prostoy_${VERSION}_${ARCH}/usr/bin/prostoy"

cp РУКОВОДСТВО.md "build-deb/prostoy_${VERSION}_${ARCH}/usr/share/doc/prostoy/"
cp README.md "build-deb/prostoy_${VERSION}_${ARCH}/usr/share/doc/prostoy/"

# Man страница
cat > "build-deb/prostoy_${VERSION}_${ARCH}/usr/share/man/man1/prostoy.1" << 'EOF'
.TH PROSTOY 1 "2026-05-12" "0.40" "Prostoy Language Manual"
.SH NAME
prostoy \- язык программирования на русском языке
.SH SYNOPSIS
.B prostoy
[\fIфайл.pr\fR]
.SH DESCRIPTION
Простой - современный язык программирования с русским синтаксисом.
.SH OPTIONS
.TP
.I файл.pr
Запустить программу из файла
.TP
Без аргументов
Запустить интерактивный режим (REPL)
.SH EXAMPLES
.TP
prostoy программа.pr
Запустить программу
.TP
prostoy
Запустить REPL
.SH SEE ALSO
Документация: /usr/share/doc/prostoy/РУКОВОДСТВО.md
EOF
gzip "build-deb/prostoy_${VERSION}_${ARCH}/usr/share/man/man1/prostoy.1"

# Собираем пакет
dpkg-deb --build "build-deb/prostoy_${VERSION}_${ARCH}"
mv "build-deb/prostoy_${VERSION}_${ARCH}.deb" "installers/"

echo "✅ DEB пакет создан: installers/prostoy_${VERSION}_${ARCH}.deb"
echo ""
echo "Установка:"
echo "  sudo dpkg -i prostoy_${VERSION}_${ARCH}.deb"
echo "  или"
echo "  sudo apt install ./prostoy_${VERSION}_${ARCH}.deb"
