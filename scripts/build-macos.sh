#!/usr/bin/env bash
# DevVault build script for macOS. Verifies prerequisites, then builds.
set -euo pipefail

echo "==> DevVault macOS build"

command -v go >/dev/null || { echo "ERROR: Go is not installed (https://go.dev/dl)"; exit 1; }
command -v node >/dev/null || { echo "ERROR: Node.js is not installed (https://nodejs.org)"; exit 1; }

if ! xcode-select -p >/dev/null 2>&1; then
  echo "==> Installing Xcode command line tools (may prompt)"
  xcode-select --install
fi

command -v wails >/dev/null || {
  echo "==> Installing Wails CLI"
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  export PATH="$HOME/go/bin:$PATH"
}

echo "==> Running backend checks"
gofmt -l internal cmd && go vet ./internal/... ./cmd/... && go test ./internal/... -count=1

echo "==> Building frontend + application (universal binary)"
wails build -platform darwin/universal

echo
echo "==> Done: build/bin/DevVault.app"
