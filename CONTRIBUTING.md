# Contributing to DevVault

Thanks for helping make DevVault better!

## Getting started

```bash
git clone https://github.com/atharvaphadnis-ai/devvault
cd devvault
npm --prefix frontend install
wails dev            # or: go test ./internal/... for backend-only work
```

## Ground rules

1. **Keep the architecture clean.** Business logic lives in `internal/<pkg>`
   services with no GUI imports; the `App` facade only delegates. If your
   feature needs a new event, emit it via `events.Sink`.
2. **Never fake functionality.** If something isn't implemented on a platform,
   fail fast with a clear message and document it (see the terminal on Windows).
   No simulated terminals, no mock data in production paths.
3. **Never touch user project files silently.** DevVault records paths; it does
   not modify registered projects outside explicit user actions.
4. **Tests travel with features.** Every `internal/` package has tests; new
   services need at least CRUD/edge-case coverage. Run:
   ```bash
   gofmt -l internal cmd && go vet ./internal/... ./cmd/... && go test ./internal/... -count=1
   cd frontend && npm run build
   ```
5. **Frontend style.** Tailwind utility classes with the semantic palette
   (`bg-surface`, `text-muted`, …) so dark/light theming keeps working. Use the
   shared components in `src/components/ui.tsx`.
6. **Security review.** Anything touching secrets, paths, the relay protocol or
   the plugin host requires a second reviewer. No new crypto — standard library
   only.

## Commit & PR style

- Conventional commits preferred: `feat(terminal): …`, `fix(relay): …`.
- PRs should state what was tested (commands + platforms) and update docs when
  behaviour or setup changes.

## Project layout refresher

See `docs/ARCHITECTURE.md` for the module map and `docs/PLUGINS.md` for the
plugin protocol.
