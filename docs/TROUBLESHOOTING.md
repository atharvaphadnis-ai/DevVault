# Troubleshooting

## Startup / build

**`wails build` fails on Linux with `webkit2gtk` / `pkg-config` errors**
Install the GUI development packages:
```bash
sudo apt-get install libgtk-3-dev libwebkit2gtk-4.1-dev build-essential pkg-config
```
(older distributions: `libwebkit2gtk-4.0-dev`). Run `wails doctor` to verify.

**`window.go.main.App is undefined` in the UI**
You opened the frontend directly in a browser instead of running the app.
Start it with `wails dev` or launch the built binary — the bridge only exists
inside the Wails webview.

**App starts but the window is blank**
Run `npm --prefix frontend install && npm --prefix frontend run build` once, or
use `wails dev` (it builds automatically).

## Git module

**"git executable not found on PATH"**
DevVault uses your system git. Install it (`https://git-scm.com`) and restart
the app. Linux: `sudo apt install git`; macOS: `xcode-select --install`;
Windows: https://git-scm.com/download/win.

**Push/pull asks for credentials or hangs**
DevVault disables interactive prompts (`GIT_TERMINAL_PROMPT=0`). Configure a
credential helper (`git config --global credential.helper`) or use SSH remotes
with your agent loaded.

## Terminal module

**"Integrated terminal is not supported on this platform yet" (Windows)**
Windows ConPTY support is not implemented in this version — the app says so
explicitly instead of faking output. Track progress in the README feature
matrix. Linux/macOS terminals are real PTY sessions.
Also check that `$SHELL` points to an existing binary, or pass a shell explicitly.

## Databases

**Postgres connection fails with `sslmode is invalid` / timeout**
DevVault connects with `sslmode=disable` in v1 — for TLS endpoints use an SSH
tunnel or run the workbench against a reachable host. Verify host/port with
`psql` first.

**MySQL "unknown driver"**
Not possible — the driver is compiled in. Check the error text; usually the
server refuses the connection (grant/ host-based ACLs).

## DevVault Anywhere

**Agent says "relay unreachable"**
Check the address (`host:port`), that the relay is running, and that TCP is
allowed. The relay prints its listen address on startup.

**"relay certificate fingerprint mismatch"**
Good — that's pinning protecting you. Compare the fingerprint the relay printed
on startup with the value on the Remote page (typos happen). Never disable the
check to "make it work".

**Pairing code rejected / expired**
Codes live 5 minutes and are single-use. Generate a fresh one. If the relay
rate-limited you, wait ~10 seconds.

**Client stuck on "waiting for approval"**
Approve it in the DevVault UI (Remote → Paired clients). Requests expire after
2 minutes.

**Tunnel opens but HTTP calls return 401**
Send the required header: `curl -H "X-DevVault-Client: cli" http://127.0.0.1:PORT/api/health`.

## Secrets / keyring

**Secrets prompt or keyring errors on headless Linux**
No D-Bus/keyring service → DevVault automatically uses the encrypted-file
fallback in the data directory (`secrets.json`, AES-256-GCM, 0600). Nothing to
configure; the audit log notes the choice.

## Processes

**Stop says "process did not exit in time"**
The process ignored SIGTERM. On Linux/macOS DevVault signals the whole process
group; a stuck child may still need a manual `kill -9`. Registered commands are
the only ones DevVault stops — it will never touch unrelated processes.

## Platform support honesty

| Feature | Linux | macOS | Windows |
| --- | --- | --- | --- |
| App build | ✅ tested in CI | ⚠️ standard Wails build (untested in this workspace) | ⚠️ standard Wails build (untested) |
| Integrated terminal (PTY) | ✅ tested | ⚠️ expected to work (same PTY path) | ❌ not implemented yet |
| OS keyring | ⚠️ when D-Bus present, else encrypted file | ⚠️ untested | ⚠️ untested (Windows Credential Manager via keyring lib) |
| Relay/CLI | ✅ tested | ✅ pure Go | ✅ pure Go |

"⚠️ untested" means the code path is standard and expected to work, but was not
executed on that platform during this project's verification run — the CI
workflow exercises the Linux desktop build.
