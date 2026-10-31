# DevVault

**The developer workbench** — a production-grade, cross-platform desktop app built with **Go + Wails v2** and **React + TypeScript**. Projects, Git, a real terminal, databases, API testing, environments, snippets, tasks, processes, containers, and secure remote access to your machine from anywhere — in one fast, native-feeling app.

![stack](https://img.shields.io/badge/Go-1.23-00ADD8) ![stack](https://img.shields.io/badge/Wails-v2-DF0000) ![stack](https://img.shields.io/badge/React-18-61DAFB) ![license](https://img.shields.io/badge/license-MIT-green)

---

## Overview
<img width="1920" height="1200" alt="download" src="https://github.com/user-attachments/assets/172e1ad2-f13e-45d8-9dd5-77c36760b8e2" />

## Highlights

- **DevVault Anywhere** — reach your DevVault (and its agent API) securely from another machine through a **managed relay**: the desktop agent makes *outbound-only* TLS connections, so there is **no port forwarding, no public IP, no firewall changes**. Pairing codes + manual approval + certificate pinning + a one-click kill switch.
- **Real integrated terminal** — genuine PTY sessions (Linux/macOS), multi-tab, live streaming output. Nothing is simulated.
- **Real tools, not toys** — the Git manager drives your **system git**; the DB workbench talks to **SQLite, PostgreSQL, MySQL/MariaDB**; the API workbench sends real HTTP requests with environments and history.
- **Privacy-first storage** — one SQLite database, secrets in the **OS keyring** (encrypted-file fallback), an append-only audit log, and a strict rule: **DevVault never silently modifies your project files**.
- **Polished UI** — dark-first design with a complete light theme (persisted), collapsible sidebar, `Ctrl+K` command palette with global search, toasts, modals, keyboard shortcuts, and animations that respect `prefers-reduced-motion`.

## Feature status (honest matrix)

| Module | Status | Notes |
| --- | --- | --- |
| A. Dashboard | ✅ Implemented | Stats, CPU/RAM, task summary, remote status, quick actions |
| B. Project manager | ✅ Implemented | Register/organise/tag/favourite; metadata only, never modifies project files |
| C. Git manager | ✅ Implemented | Status/log/branches/stage/commit/amend/push/pull/fetch/clone/diff via system git; clear error when git is missing |
| D. Integrated terminal | ✅ Linux/macOS · ⚠️ Windows | Real PTY (creack/pty). Windows ConPTY is **not implemented yet** — the UI states this instead of faking it |
| E. Database workbench | ✅ Implemented | SQLite/Postgres/MySQL connections, SQL editor, table browse, history, CSV/JSON export |
| F. API workbench | ✅ Implemented | All HTTP methods, collections, environments (`{{var}}`), history |
| G. Env manager | ✅ Implemented | Per-project/per-env, secrets masked + audited reveal, .env import/export |
| H. Snippets & notes | ✅ Implemented | CodeMirror highlighting, tags, markdown preview (sanitised) |
| I. Processes/services | ✅ Implemented | Register/start/stop with explicit confirmation gate; live log streaming |
| J. Containers | ✅ Implemented (optional) | Docker CLI integration; module auto-hides when Docker is absent |
| K. Tasks | ✅ Implemented | Kanban (drag & drop) + list view |
| DevVault Anywhere (dev mode) | ✅ Implemented & integration-tested | Local relay + agent + CLI; full E2E test in `internal/relay` |
| Hosted cloud relay | ❌ Not operated by this project | The relay is a standalone binary you can run anywhere (VPS) in minutes; see `docs/DEVVAULT_ANYWHERE.md` |
| Plugin system | ✅ Implemented | JSON-RPC/stdio host, permissions, bundled example plugin |
| Windows ConPTY terminal | ❌ Planned | Tracked; fails fast with a clear message |
| OS window dialogs (file pickers) | ❌ Planned | Paths are typed in v1 |

## Quick start

See **[BUILD.md](BUILD.md)** for the full guide. TL;DR:

```bash
# 1. Install prerequisites (once)
#    Go 1.23+, Node.js 20+, and the Wails CLI with your platform's webview deps
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# 2. Run in dev mode (hot reload)
cd devvault
npm --prefix frontend install
wails dev

# 3. Build a release binary
wails build          # -> build/bin/DevVault(.exe)
```

## DevVault Anywhere in 60 seconds

```bash
# Terminal 1 — run the relay (local dev; in production run it on a VPS)
go run ./cmd/devvault-relay --addr :7921
# note the printed TLS fingerprint

# In DevVault: Remote page -> paste relay address + fingerprint -> Enable
#              -> Generate pairing code

# Terminal 2 — from another machine
go build -o devvault-cli ./cmd/devvault-cli
./devvault-cli pair --relay 127.0.0.1:7921 --fingerprint <fp> --code <code> --name laptop
# approve the client in the DevVault UI, then:
./devvault-cli devices --identity ~/.devvault/client.json
./devvault-cli tunnel --identity ~/.devvault/client.json --device <device-id> --port 7922
curl -H "X-DevVault-Client: cli" http://127.0.0.1:7922/api/health
```

## Repository layout

```
devvault/
├── main.go, app.go        # Wails application + bound API facade
├── internal/              # Go services (no GUI imports, fully unit-tested)
│   ├── store/             # SQLite + migrations + settings + audit log
│   ├── secrets/           # OS keyring w/ AES-256-GCM file fallback
│   ├── projects/ gitsvc/ terminal/ dbsvc/ apisvc/ envsvc/
│   ├── snippets/ tasks/ procs/ containers/ searchsvc/ dashsvc/
│   ├── localapi/          # opt-in localhost REST API (off by default)
│   ├── remote/ relay/ relaywire/  # DevVault Anywhere agent, relay, protocol
│   └── plugins/           # JSON-RPC plugin host
├── cmd/devvault-relay/    # standalone relay server binary
├── cmd/devvault-cli/      # remote client CLI (pair / devices / tunnel)
├── frontend/              # React + TS + Tailwind + Vite app
├── plugins/hello-plugin/  # bundled example plugin
├── scripts/               # platform build scripts
├── docs/                  # architecture, anywhere, plugins, troubleshooting
└── .github/workflows/     # CI (test + build on 3 platforms)
```

## Security model (summary)

- Secrets (DB passwords, tokens) go to the **OS credential store**; if unavailable, an AES-256-GCM file with 0600 perms is used. No plain-text secrets, ever.
- The optional local REST API is **off by default**, binds to `127.0.0.1`, and requires a bearer token.
- Git operations refuse paths that escape the repository; plugin host calls are **permission-checked** against the manifest.
- Remote access: TLS with cert pinning, HMAC handshakes with replay protection, expiring pairing codes, rate limiting, manual device-owner approval, session + audit logging, one-click disable.
- Full details in [SECURITY.md](SECURITY.md) and `docs/DEVVAULT_ANYWHERE.md`.

## What was verified in development

Executed in the development workspace (Linux x86-64, Go 1.23.4, Node 24):

- `gofmt` clean, `go vet ./internal/... ./cmd/...` clean
- `go test ./internal/...` — **13 packages, all passing**, including the full relay→agent→client E2E flow, PTY session test, plugin host E2E (Node runtime), DB/API/localapi suites
- `npm run build` (tsc strict + vite) — clean production build
- `devvault-relay` and `devvault-cli` binaries compile and run

**Not verified here** (no GUI stack in the sandbox): the final `wails build` link step for the desktop binary — it is exercised in CI on Linux (webkit2gtk installed) and by following `BUILD.md` on Windows/macOS. Windows and macOS behaviour of PTY/keyring features is documented as untested in `docs/TROUBLESHOOTING.md`.

## License

MIT — see [LICENSE](LICENSE).
