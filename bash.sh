#!/bin/bash
echo "开始编译多平台版本..."

# Windows
GOOS=windows GOARCH=amd64 go build -o llmapi-windows.exe cmd/server/main.go

# macOS Intel
GOOS=darwin GOARCH=amd64 go build -o llmapi-macos-amd64 cmd/server/main.go

# macOS Apple Silicon
GOOS=darwin GOARCH=arm64 go build -o llmapi-macos-arm64 cmd/server/main.go

# Linux
GOOS=linux GOARCH=amd64 go build -o llmapi-linux-amd64 cmd/server/main.go

echo "✅ 编译完成！文件在当前目录中"