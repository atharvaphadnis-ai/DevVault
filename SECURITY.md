# Security Policy

## Reporting a vulnerability

Please open a GitHub issue with the `security` label **or** contact the
maintainers directly. Include reproduction steps and affected versions. We aim
to acknowledge reports within 72 hours.

## Design principles

1. **No custom cryptography.** All crypto is the Go standard library:
   TLS 1.2+ (`crypto/tls`) with certificate pinning, HMAC-SHA256 handshakes,
   AES-256-GCM at-rest encryption, SHA-256 token hashing, `crypto/rand` everywhere.
2. **Secrets at rest.** Database passwords and tokens live in the OS credential
   store; the fallback is an AES-256-GCM file with `0600` permissions in the
   app data directory. The SQLite database never contains secret values.
3. **Least privilege by default.** The local REST API is disabled by default,
   binds to `127.0.0.1` only, and requires a regenerated-on-enable bearer token.
   Remote access is disabled by default with a one-click kill switch.
4. **Project isolation.** DevVault stores project *metadata* only. Git paths are
   validated against the repository root; traversal outside it is rejected.
5. **Explicit destruction.** Stopping a process requires an explicit
   confirmation flag from the UI, and only processes DevVault started can be
   stopped by it.
6. **Auditable.** Logins of note — settings changes, project add/remove, git
   push/commit, DB/API activity markers, env reveals, remote pairing, approvals,
   tunnel open/close, plugin toggles, local API enable/disable — are appended to
   the audit log and viewable in Settings.

## Remote access surface

See `docs/DEVVAULT_ANYWHERE.md` for the full control list (pinning, replay
window, pairing TTL, rate limits, hashed client tokens, per-tunnel connections).

## Known limitations (documented, not hidden)

- Device enrollment on a relay is first-come-first-served; a multi-tenant relay
  needs an invite/auth layer before hosting strangers.
- The Windows terminal backend is unimplemented (fails fast, no silent fallback).
- The local API is read-only but unauthenticated clients on the same machine
  with the token could read project metadata — keep the token private.

## Dependency policy

Dependencies are pinned in `go.mod` / `package-lock.json`; CI runs `go vet` and
`npm audit`-style checks on every push. Keep dependencies minimal; new ones
require justification in the PR description.
