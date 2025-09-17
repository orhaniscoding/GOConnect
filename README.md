# GOConnect (Prototype)

GOConnect, created by [orhaniscoding](https://github.com/orhaniscoding), is a Windows-first virtual overlay network agent, inspired by ZeroTier/Tailscale. The repository now focuses on a single long-running service (agent + local HTTP API + transport) and a bundled Web UI served from http://127.0.0.1:2537 for configuration and diagnostics.

Notes: The v1.x line now ships with a functional Wintun integration (optional build tag), QUIC peer manager with STUN-based public endpoint discovery, DPAPI-backed secret helpers, a persistent local network registry (join/leave), and an internationalised Web UI. Controller federation and advanced routing remain roadmap items.

## Directory Structure

GOConnect/
- `cmd/goconnectservice` – GOConnect Service (agent supervisor + API bootstrap)
- `internal/api` – HTTP handlers, CSRF, SSE log stream, asset resolution
- `internal/core` – service state machine, settings, tunnel orchestration
- `internal/tun` – TUN abstraction (stub + Wintun implementation via build tag)
- `internal/transport` – QUIC peer manager and STUN probe loop
- `internal/ipam` – deterministic local IP allocator
- `internal/config` – config.yaml load/save, ProgramData directory helpers
- `internal/logging` – file logger with rotation
- `internal/security` – DPAPI wrapper (Windows) with cross-platform fallbacks
- `internal/updater` – self-update stub
- `internal/diag` – diagnostics stub
- `internal/i18n` – English/Turkish strings for service & Web UI
- `webui/` – Static web UI + i18n bundles (also embedded for single-binary builds)
- `build/scripts/` – PowerShell install/uninstall helper scripts
- `stubs/` – Offline stubs for `kardianos/service` (legacy systray removed)

## Security & Defaults
- HTTP binds to `127.0.0.1:2537`
- Basic anti-CSRF: SameSite=Strict cookie `goc_csrf` + `X-CSRF-Token` header for non-GET requests
- Files under `%ProgramData%\\GOConnect\\{config,logs,secrets}` (config.yaml now persists joined networks + transport defaults)
- Default config on first run: `port: 2537`, `mtu: 1280`, `log_level: info`, `language: system locale`, `stun_servers: ["stun.l.google.com:19302"]`
- Secrets helper uses Windows DPAPI (no-op on non-Windows builds)
- Log rotation: simple size-based rollover (`agent.log` -> `agent.log.1`)

## Internationalization
* Service i18n JSON: `internal/i18n/en.json`, `internal/i18n/tr.json`
* Web UI i18n JSON: `webui/i18n/en.json`, `webui/i18n/tr.json`
* API returns status codes; UI resolves readable labels via translation bundles

## HTTP API
- GET `/api/status` – service/tunnel/controller states, tunnel self-test errors, detected public endpoint, active language
- POST `/api/service/start` | `/api/service/stop` | `/api/service/restart`
- GET `/api/networks` – local network registry (config-backed)
- POST `/api/networks/join` | `/api/networks/leave` – persist network membership (assign deterministic address, update config)
- GET `/api/peers` – live QUIC peer snapshot (RTT + relay flags), falls back to configured peers
- GET `/api/logs/stream` – SSE stream of live log events
- GET/PUT `/api/settings` – port, MTU, log level, language, autostart, controller URL, relay URLs, UDP port, peers, STUN servers
- POST `/api/diag/run` – diagnostics hook (stub)
- POST `/api/update/check` | `/api/update/apply` – updater stub
- POST `/api/exit` – graceful shutdown

### Versioned Network Scope (Experimental `/api/v1`)
These endpoints introduce per-network versioned resources with optimistic concurrency and simple effective policy derivation.

- GET/PUT `/api/v1/networks/{id}/settings` – Versioned network settings object
- GET/PUT `/api/v1/networks/{id}/me/preferences` – Member ("me") preferences
- GET `/api/v1/networks/{id}/effective?node=me` – Derived effective policy snapshot

Optimistic concurrency: clients MUST send the `Version` they last observed when performing a PUT. If the stored version diverged, server returns `409 {"error":"version_conflict"}`. On success, server increments `Version`.

Error format (standardized):
```json
{ "error": "code", "message": "human readable message" }
```

### Persistence
Ephemeral per-network state (settings, preferences) is atomically written to `%ProgramData%/GOConnect/state/state.json` after successful PUT operations. On startup the agent attempts a best-effort load. Missing file is non-fatal.

### OpenAPI Specification
An initial machine-readable spec is available at `openapi.yaml` (root). It currently covers core and experimental v1 paths and will evolve with schema detail.

### Sample Update Flow
1. GET `/api/v1/networks/n1/settings` → `{ "version":1, ... }`
2. Client modifies fields, sends PUT with body including `"Version":1`.
3. Response → updated object `{ "version":2, ... }`.
4. A second PUT reusing `Version":1` now fails with 409.

### Effective Policy Logic (Expanded)
The derived policy combines member preferences and network settings:
* Base policy: `allow_all` unless `AllowInternet=false` (then `restricted_no_internet`).
* Flags surfaced: encryption requirement, relay fallback allowance, broadcast/IPv6 allowed, idle disconnect timer, default DNS list.
* Reason string aggregates active constraints (e.g., `member disabled internet access; encryption required`).

### Metrics
`GET /api/metrics` returns basic JSON counters: uptime, service/tun/controller states, network counts, peer count, SSE subscribers, and MTU.

## Build & Run

Prerequisites: Go 1.22+ on Windows. (Offline mode uses the included `kardianos/service` stub.)

- Build all: `go build ./...`
- Build service: `go build -o bin/GOConnectService.exe ./cmd/goconnectservice`
- Run service (dev, foreground): `go run ./cmd/goconnectservice`
- Optional (Wintun): install the Wintun driver + DLL, then `go run -tags=wintun ./cmd/goconnectservice`
- Open Web UI: http://127.0.0.1:2537

Service install (example, admin PowerShell):
- Install: `powershell -ExecutionPolicy Bypass -File build\scripts\install-service.ps1 -ExePath "C:\\path\\to\\GOConnectService.exe"`
- Uninstall: `powershell -ExecutionPolicy Bypass -File build\scripts\uninstall-service.ps1`

ProgramData paths:
- Config: `%ProgramData%\GOConnect\config\config.yaml`
- Logs: `%ProgramData%\GOConnect\logs\agent.log`
- Secrets: `%ProgramData%\GOConnect\secrets\`

## How to Run (Quick)
1. `go run ./cmd/goconnectservice`
2. Open http://127.0.0.1:2537
3. Navigate tabs, use Start/Stop/Restart, tweak Settings (including STUN servers), and watch the live log stream.

## Roadmap / TODOs
- v1.x: Real controller sync, network membership CRUD, relay promotion
- v2: ACL/DNS and advanced policies
- v3: SSO (AzureAD/OIDC) and central policy management
 - (Planned) Rich policy graph + multi-member preference negotiation

## Notes & Suggested Improvements
- Switch to official `kardianos/service` when git/network access is available (stub remains for offline builds).
- Harden CSRF/auth before exposing beyond localhost; add auth tokens and origin checks.
- Replace naive YAML parsing improvements (validation, schema) and enhance locale detection via Windows APIs.
- Extend diagnostics (MTU/STUN latency), persistent peer cache, and controller heartbeat for production readiness.






