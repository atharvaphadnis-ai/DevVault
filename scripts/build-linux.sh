#!/usr/bin/env bash
# DevVault build script for Linux. Verifies prerequisites, then builds.
set -euo pipefail

echo "==> DevVault Linux build"

command -v go >/dev/null || { echo "ERROR: Go is not installed (https://go.dev/dl)"; exit 1; }
command -v node >/dev/null || { echo "ERROR: Node.js is not installed (https://nodejs.org)"; exit 1; }
command -v npm >/dev/null || { echo "ERROR: npm is not installed"; exit 1; }

echo "==> Checking webview development packages"
if ! pkg-config --exists gtk+-3.0 2>/dev/null; then
  echo "WARNING: gtk+-3.0 dev package not found."
  echo "  Debian/Ubuntu: sudo apt-get install libgtk-3-dev libwebkit2gtk-4.1-dev build-essential pkg-config"
fi
if ! pkg-config --exists webkit2gtk-4.1 2>/dev/null && ! pkg-config --exists webkit2gtk-4.0 2>/dev/null; then
  echo "WARNING: webkit2gtk dev package not found (see command above)."
fi

command -v wails >/dev/null || {
  echo "==> Installing Wails CLI"
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  export PATH="$HOME/go/bin:$PATH"
}

echo "==> Running backend checks"
gofmt -l internal cmd && go vet ./internal/... ./cmd/... && go test ./internal/... -count=1

echo "==> Building frontend + application"
wails build -platform linux/amd64

echo
echo "==> Done: build/bin/DevVault"
