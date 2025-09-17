# GOConnect Development Guide

This document complements the README with workflow notes for contributors.

## Prerequisites
* Go 1.22+
* Windows (primary target). Linux/macOS builds work for most non-TUN pieces (DPAPI becomes a no-op; service stub may differ).
* (Optional) Wintun driver + DLL for `-tags=wintun` builds.

## Repository Layout (Developer Focus)
* `cmd/goconnectservice` – Primary long‑running agent (HTTP API + transport + persistence)
* `internal/api` – Handlers, versioned `/api/v1` resources, persistence of network scoped state
* `internal/core` – High‑level orchestration points
* `internal/tun` – Wintun + stub (guards via build tags)
* `internal/transport` – QUIC peer/session manager (STUN assisted)
* `webui/` – Static UI (served from disk or embedded)
* `build/scripts` – Install/uninstall PowerShell helpers for Windows service mode
* `stubs/` – Offline fallback modules for service integration (legacy systray removed)

## VS Code Integration
Configured under `.vscode/`:

### Tasks (`tasks.json`)
* Build Service – `go build -o bin/goconnect-service.exe ./cmd/goconnectservice`
* Build All – `go build ./...` (may fail if experimental folders require extra deps; prefer service build for quick cycles)
* Test (short) – `go test ./... -run Test -count=1`
* Run Service (dev) – `go run ./cmd/goconnectservice`
* Run Service (wintun) – `go run -tags=wintun ./cmd/goconnectservice`

### Launch Configurations (`launch.json`)
* GOConnect Service (debug)
* GOConnect Service (wintun tag)

Set breakpoints in any `internal/*` package files; delve attaches automatically with the Go extension.

## Persistence Model
Network‑scoped versioned documents:
* Network Settings: `/api/v1/networks/{id}/settings`
* Member Preferences: `/api/v1/networks/{id}/me/preferences`

On successful PUT the entire in‑memory map is serialized atomically to:
`%ProgramData%/GOConnect/state/state.json`

Concurrency control: optimistic – client must echo `Version`. The server increments after successful mutation. Stale writes get `409` with JSON error payload `{"error":"version_conflict"}`.

## Testing
## Metrics Endpoint
`/api/metrics` currently returns a JSON snapshot (not Prometheus). Consider adding a separate `/metrics` in Prometheus text format later.

## Continuous Integration
GitHub Actions workflow (`.github/workflows/ci.yml`) runs build, `go vet`, and tests on pushes/PRs to `main` (Windows runner).

Currently limited unit tests exist around concurrency logic in `internal/api`. Add more coverage with focus on:
* Effective policy derivation edge cases
* Persistence load failures (corrupted JSON) – ensure graceful fallback
* TUN initialization failure paths (when Wintun missing)

Run all tests:
```
go test ./...
```

## Adding New Fields to Versioned Documents
1. Extend the struct(s) in `internal/api/api.go` (NetworkSettingsState / MemberPreferencesState)
2. Update copy logic in `v1_endpoints.go` update functions
3. Update OpenAPI schema (`openapi.yaml`)
4. Adjust Web UI panels to send/display the new fields
5. Add tests validating round‑trip + persistence

## Error Handling Pattern
All structured errors should use the helper returning:
```json
{"error":"code","message":"human readable explanation"}
```
HTTP status code aligns with error type (400/404/409/500).

## Windows Service Workflow
Install (admin PS):
```
build\scripts\install-service.ps1 -ExePath "$(Resolve-Path bin/goconnect-service.exe)"
```
Uninstall:
```
build\scripts\uninstall-service.ps1
```
Logs: `%ProgramData%/GOConnect/logs/agent.log`

## Internationalisation
Add new string keys in both `internal/i18n/*.json` and `webui/i18n/*.json`. Avoid runtime panics by always providing both languages.

## Contribution Guidelines (Lightweight)
* Keep PRs small & focused.
* Run `go vet ./...` and tests before submission.
* Update README / DEVELOPMENT.md when changing build, run, or persistence behavior.
* Maintain consistent error format; avoid returning raw internal error strings to clients – wrap with user‑friendly message.

## Future Enhancement Ideas
* Rich effective policy (merge network ACL + member overrides + relay permissions)
* Configurable auth layer (token or local OS integration) for API
* Structured logging (JSON) behind a build flag or config switch
* Metrics endpoint (Prometheus format) for observability
* Automated upgrade channel (real implementation of updater stub)

---
Happy hacking! Open issues for architectural questions before large rewrites.
