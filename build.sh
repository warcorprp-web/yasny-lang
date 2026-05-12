#!/bin/bash

echo "Компиляция для разных платформ..."

# Linux
GOOS=linux GOARCH=amd64 go build -o dist/yasny-linux-amd64 main.go
GOOS=linux GOARCH=arm64 go build -o dist/yasny-linux-arm64 main.go

# Windows
GOOS=windows GOARCH=amd64 go build -o dist/yasny-windows-amd64.exe main.go

# macOS
GOOS=darwin GOARCH=amd64 go build -o dist/yasny-macos-amd64 main.go
GOOS=darwin GOARCH=arm64 go build -o dist/yasny-macos-arm64 main.go

echo "Готово!"
ls -lh dist/
