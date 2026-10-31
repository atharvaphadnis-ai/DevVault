# DevVault Anywhere

Reach your DevVault desktop from another machine — securely, with **no port
forwarding, no public IP and no firewall changes**. This document explains the
model, the dev-mode setup, and how to operate your own relay in production.

## The model

```
┌─────────────────┐        outbound TLS        ┌──────────────┐        TLS         ┌────────────────┐
│  DevVault       │ ─────────────────────────► │    relay     │ ◄───────────────── │  devvault-cli  │
│  desktop agent  │   (cert fingerprint        │ (any VPS or  │  (pinned cert)     │  remote client │
│  (your PC)      │    pinned locally)         │  localhost)  │                    │  (laptop/phone)│
└─────────────────┘                            └──────────────┘                    └────────────────┘
```

- The **agent** (inside the desktop app) dials *out* to the relay and keeps that
  connection alive. Your machine stays unreachable from the internet.
- The **relay** is a rendezvous: it authenticates both sides and pipes encrypted
  tunnels between them. It can pair devices and clients but **cannot decrypt
  application traffic beyond what it relays** — the agent API itself checks an
  application-level header on every request.
- A **remote client** (`devvault-cli`) pairs once (pairing code + your manual
  approval in the DevVault UI), then opens tunnels on demand.

## Security controls

| Control | Implementation |
| --- | --- |
| Transport | TLS 1.2+ with **certificate pinning** (relay prints its SHA-256 fingerprint; clients refuse mismatches — verified by test) |
| Device auth | Per-device secret; HMAC-SHA256 over device id + timestamp with a 120 s replay window |
| Client pairing | 8-char code, 5-minute TTL, rate-limited per IP, requires **manual approval** in the DevVault UI (approve/deny) |
| Client tokens | 256-bit random, stored **hashed** (SHA-256) on the relay |
| Tunnels | Per-tunnel TLS connections, attach handshake, audited open/close |
| Kill switch | One click disables the agent, tears down all connections, audited |
| Agent API | Binds to `127.0.0.1` only; requires `X-DevVault-Client` header on every request |
| Crypto | Standard library only (`crypto/tls`, `crypto/hmac`, `crypto/aes`) — no custom cryptography |

## Dev mode (2 minutes)

```bash
# 1) start the relay locally
go run ./cmd/devvault-relay --addr :7921
# → prints: TLS fingerprint (SHA-256): ab12…

# 2) in DevVault: Remote page
#    relay address: 127.0.0.1:7921
#    fingerprint:   ab12…        → Enable remote access
#    → Generate pairing code (e.g. 9fA3bC22)

# 3) on another machine (or another terminal)
go build -o devvault-cli ./cmd/devvault-cli
./devvault-cli pair --relay 127.0.0.1:7921 --fingerprint <fp> --code 9fA3bC22 --name work-laptop
# → approve the request in the DevVault UI within 2 minutes

# 4) list devices and open a tunnel
./devvault-cli devices --identity ~/.devvault/client.json
./devvault-cli tunnel --identity ~/.devvault/client.json --device <device-id> --port 7922

# 5) use the agent API through the tunnel
curl -H "X-DevVault-Client: cli" http://127.0.0.1:7922/api/health
curl -H "X-DevVault-Client: cli" http://127.0.0.1:7922/api/projects
```

This whole flow — enrolment, pairing, approval, listing, tunnelling, the
401 for header-less requests and the audit trail — is covered by the
automated E2E test in `internal/relay/relay_test.go`.

## Production relay (self-hosted)

> **Honest note:** this project does **not** operate a hosted relay service yet.
> The relay is a single ~10 MB static binary you can run anywhere in minutes.

```bash
# on a VPS with a domain (e.g. relay.example.com)
./devvault-relay --addr :7921 --data-dir /var/lib/devvault-relay
# open/allow TCP 7921 in the firewall — that is the only exposed port
```

Recommendations for production:

- Terminate an outer TLS layer (Caddy/nginx or a cloud LB) in front of the relay,
  or distribute the printed fingerprint to your devices and keep the direct TLS.
- The first agent to claim a device id enrolls it and receives its secret once.
  For multi-user deployments, put the relay behind your own invite token /
  VPN boundary; the relay code is small and forkable by design.
- Monitor `relay_audit` in the relay's SQLite database (`--data-dir/relay.db`).

## Limitations (by design, today)

- Device enrollment trusts "first come, first served" per relay — suitable for
  personal/small-team relays; multi-tenant hosting needs an invite layer.
- The bundled agent API is intentionally small (health/stats/projects/echo) and
  read-only. More capabilities should ship together with scoped permissions.
- CLI tunnels are TCP pipes; there is no multi-plexing yet (one TLS conn per
  connection) — fine for interactive/API use.
