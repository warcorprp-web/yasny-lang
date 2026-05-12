#!/bin/bash
# Сборка и публикация VS Code расширения

set -e

cd vscode-extension

echo "╔════════════════════════════════════════╗"
echo "║   Сборка VS Code расширения Простой   ║"
echo "╚════════════════════════════════════════╝"
echo ""

# 1. Установка зависимостей
echo "📦 Установка зависимостей..."
npm install

# 2. Компиляция TypeScript
echo "🔨 Компиляция TypeScript..."
npm run compile

# 3. Создание VSIX пакета
echo "📦 Создание VSIX пакета..."
npm run package

echo ""
echo "✅ Расширение собрано: prostoy-lang-0.40.0.vsix"
echo ""
echo "Установка локально:"
echo "  code --install-extension prostoy-lang-0.40.0.vsix"
echo ""
echo "Публикация в Marketplace:"
echo "  vsce publish"
echo ""
echo "Требуется Personal Access Token от:"
echo "  https://marketplace.visualstudio.com/manage"
