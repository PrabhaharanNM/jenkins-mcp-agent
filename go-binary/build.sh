#!/bin/bash
set -e

echo "Building MCP Agent binaries for all platforms..."

cd "$(dirname "$0")"

# Linux AMD64
echo "  Building linux/amd64..."
GOOS=linux GOARCH=amd64 go build -o mcp-agent-linux-amd64 \
    -ldflags="-s -w" \
    ./cmd/mcp-agent

# macOS AMD64
echo "  Building darwin/amd64..."
GOOS=darwin GOARCH=amd64 go build -o mcp-agent-darwin-amd64 \
    -ldflags="-s -w" \
    ./cmd/mcp-agent

# macOS ARM64 (M1/M2)
echo "  Building darwin/arm64..."
GOOS=darwin GOARCH=arm64 go build -o mcp-agent-darwin-arm64 \
    -ldflags="-s -w" \
    ./cmd/mcp-agent

# Windows AMD64
echo "  Building windows/amd64..."
GOOS=windows GOARCH=amd64 go build -o mcp-agent-windows-amd64.exe \
    -ldflags="-s -w" \
    ./cmd/mcp-agent

# Compress with UPX if available
if command -v upx &> /dev/null; then
    echo "Compressing binaries with UPX..."
    upx --best --lzma mcp-agent-linux-amd64 mcp-agent-darwin-amd64 mcp-agent-darwin-arm64 mcp-agent-windows-amd64.exe || true
fi

echo ""
echo "Binaries built:"
ls -lh mcp-agent-*

# Copy to plugin resources
mkdir -p ../src/main/resources/binaries/
cp mcp-agent-* ../src/main/resources/binaries/
echo ""
echo "Copied to plugin resources directory."
