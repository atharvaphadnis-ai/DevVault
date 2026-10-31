# DevVault Architecture

## Overview

DevVault is a Wails v2 desktop application: a Go backend hosting the business
logic and a React/TypeScript frontend rendered in the system webview. The two
layers communicate exclusively through:

1. **Bound methods** — the `App` facade in `app.go` exposes ~100 typed methods
   (`window.go.main.App.ProjectsList()` etc.).
2. **Events** — services emit through an `events.Sink`; in the app it bridges to
   Wails runtime events (`term:data`, `proc:log`, `remote:client-pending`, …).

Because every service is defined in `internal/` and depends only on interfaces
(`events.Sink`, `secrets.Store`), the whole backend is testable without a GUI —
which is exactly how the test suite runs.

```
┌──────────────────────────── Wails runtime ────────────────────────────┐
│  React + TS (frontend/src)        ← events ──  Go services (internal/)│
│  Layout · CommandPalette · pages  ── calls →  App facade (app.go)     │
└──────────────────────────────────────────────────────────────────────┘
          │                                      │
          │                              SQLite (WAL, migrated)
          │                              OS keyring / AES-GCM file
          │                              system git · docker CLI · PTY
          ▼
   DevVault Anywhere: agent ⇄ (TLS, pinned) ⇄ relay ⇄ remote clients
```

## Key design decisions

### 1. Services, not spaghetti
Each domain is an `internal/<name>` package with explicit constructors and no
global state. The `App` facade is a thin delegation layer (~1 line per method),
so adding a service is mechanical and reviewable.

### 2. SQLite as the single source of truth
`internal/store` opens one SQLite (pure-Go `modernc.org/sqlite` — no CGO, no
libc coupling) with WAL mode and runs ordered migrations in transactions. All
modules persist there: projects, connections, request history, tasks, snippets,
process registrations, plugins, settings, and the append-only **audit log**.

### 3. Secrets never in the database
Passwords/tokens go to the OS keyring via `zalando/go-keyring`. When no keyring
service exists (headless Linux, CI), a **AES-256-GCM encrypted file** (0600,
random 32-byte key) takes over. Both implement `secrets.Store`.

### 4. Real tools over reimplementation
- Git: execs the **system git** with `GIT_TERMINAL_PROMPT=0` (no hangs), parses
  porcelain output, and validates every path with `filepath.Rel` to refuse
  traversal outside the repository.
- Terminal: `creack/pty` on Unix behind build tags; Windows returns a typed
  "not supported" error rather than a fake terminal.
- Docker: the docker CLI, detected on demand; the module hides when absent.
- Databases: `database/sql` with `lib/pq` and `go-sql-driver/mysql` plus the
  built-in SQLite driver.

### 5. DevVault Anywhere — outbound-only security
```
desktop agent ──(outbound TLS, cert pinned)──► relay ◄──(TLS)── remote client
      │                                     │
      └── HMAC-signed hello + replay guard  ├── pairing codes (5 min TTL)
      └── tunnels attach on 2nd conn        ├── token-hashed clients
      └── pipes to loopback agent API       └── audit + rate limits
```
The agent's HTTP API binds to `127.0.0.1` on a random port and requires the
`X-DevVault-Client` header — reaching it through a tunnel means the request was
authorised end-to-end. The wire protocol (`internal/relaywire`) is plain
newline-delimited JSON over standard TLS 1.2+; **no custom cryptography** —
only `crypto/tls`, `crypto/hmac`, `crypto/aes` from the standard library.

### 6. Plugins as isolated processes
Plugins are separate OS processes speaking newline-delimited JSON-RPC 2.0 on
stdio. A single reader goroutine per plugin dispatches responses, notifications
and (permission-checked) `host/*` requests back to the plugin. Plugins declare
permissions in `plugin.json`; the host rejects undeclared calls with
`-32001 permission denied`.

## Performance notes

- Frontend code-splits xterm.js and CodeMirror; first paint stays light.
- SQLite is capped to one writer connection (modernc driver) to avoid
  `SQLITE_BUSY` churn; hot paths use indexed queries.
- Process/terminal output streams are ring-buffered (500 lines) so long-running
  jobs don't grow memory unbounded.
- Dashboard CPU sampling is 200 ms per refresh cycle, off the UI thread.

## Testing strategy

| Layer | Coverage |
| --- | --- |
| store/projects/env/snippets/tasks | CRUD + edge cases (duplicate paths, mask handling, status normalisation) |
| gitsvc | real temp repositories: init/commit/branch/stage/diff + traversal rejection |
| terminal | real PTY echo test with a unique marker |
| procs | real process lifecycle incl. confirmation gate |
| dbsvc/apisvc/localapi | SQLite end-to-end + httptest servers |
| relay (E2E) | full flow: enrol → pair → approve → list → tunnel → HTTP through tunnel → 401 without header → audit assertions |
| plugins | bundled Node plugin: discover → enable → handshake → ping → disable |

## Extension points

- New module: add `internal/<mod>`, delegate methods on `App`, add a page.
- New event: `sink.Emit(name, payload)` from any service, subscribe via
  `onEvent(name, cb)` in the frontend.
- New plugin capability: add a `host/<method>` handler in `plugins.CallHost`
  and document it in `docs/PLUGINS.md`.
