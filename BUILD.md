# Building DevVault

## Prerequisites

| Tool | Version | Notes |
| --- | --- | --- |
| Go | 1.23+ | https://go.dev/dl |
| Node.js | 20+ (24 tested) | https://nodejs.org |
| npm | 10+ | ships with Node |
| Wails CLI | v2 latest | `go install github.com/wailsapp/wails/v2/cmd/wails@latest` |
| git | any recent | required by the Git module at runtime |

### Platform webview dependencies

**Linux (Debian/Ubuntu):**
```bash
sudo apt-get install libgtk-3-dev libwebkit2gtk-4.1-dev build-essential pkg-config
# (on older distros use libwebkit2gtk-4.0-dev)
```

**Windows:** WebView2 is preinstalled on Windows 10/11; no extra deps.

**macOS:** Xcode command line tools: `xcode-select --install`.

Run `wails doctor` to verify your setup.

## Development mode (hot reload)

```bash
cd devvault
npm --prefix frontend install
wails dev
```

The app window opens with live reload; the backend runs with the Wails bridge injected
(`window.go.main.App.*` and `window.runtime` are available to the frontend).

## Production build

```bash
wails build                 # current platform -> build/bin/DevVault
wails build -platform linux/amd64,windows/amd64,darwin/universal
```

Or use the helper scripts:

```bash
scripts/build-linux.sh      # verifies deps, builds frontend + binary
scripts/build-macos.sh
scripts/build-windows.ps1   # PowerShell
```

## Running the tests

Backend (pure Go, no GUI deps required):

```bash
gofmt -l internal cmd && go vet ./internal/... ./cmd/...
go test ./internal/... -count=1
```

Frontend (type-check + production bundle):

```bash
cd frontend && npm install && npm run build
```

Integration binaries:

```bash
go build ./cmd/devvault-relay ./cmd/devvault-cli
go run ./cmd/devvault-relay --addr :7921   # prints its TLS fingerprint
```

## Continuous integration

`.github/workflows/ci.yml` runs on every push/PR:

1. **backend job** — gofmt check, `go vet`, `go test ./internal/...` (Ubuntu, Go 1.23)
2. **frontend job** — `npm ci`, `tsc --noEmit`, `vite build`
3. **desktop build job** — installs GTK/webkit dev packages and runs `wails build` on Linux; a release workflow template builds Windows/macOS/Linux binaries on tags.

## Where data lives

| What | Location |
| --- | --- |
| App database, secrets, exports | OS config dir + `/DevVault` (Linux: `~/.config/DevVault`) |
| Relay state (when self-hosting) | `--data-dir` you pass to `devvault-relay` |
| Remote CLI identity | `~/.devvault/client.json` (0600) |

## Troubleshooting

See [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md).
