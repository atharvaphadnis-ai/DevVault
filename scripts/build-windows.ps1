# DevVault build script for Windows (PowerShell).
$ErrorActionPreference = "Stop"

Write-Host "==> DevVault Windows build"

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
  Write-Host "ERROR: Go is not installed (https://go.dev/dl)" -ForegroundColor Red; exit 1
}
if (-not (Get-Command node -ErrorAction SilentlyContinue)) {
  Write-Host "ERROR: Node.js is not installed (https://nodejs.org)" -ForegroundColor Red; exit 1
}
if (-not (Get-Command wails -ErrorAction SilentlyContinue)) {
  Write-Host "==> Installing Wails CLI"
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  $env:PATH = "$env:USERPROFILE\go\bin;$env:PATH"
}

Write-Host "==> Running backend checks"
gofmt -l internal cmd
go vet ./internal/... ./cmd/...
go test ./internal/... -count=1

Write-Host "==> Building frontend + application"
wails build -platform windows/amd64

Write-Host ""
Write-Host "==> Done: build\bin\DevVault.exe"
