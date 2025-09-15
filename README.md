# GOConnect (Prototype)

GOConnect is a Windows-only virtual overlay network agent, inspired by ZeroTier/Tailscale. This repository contains a service (agent + local API + stubs), a tray application, and a local Web UI available at http://127.0.0.1:2537.

Notes: Networking/TUN and transport are stubs in v1.0. The API, service lifecycle, tray wiring, and Web UI function with dummy data. No external network dependencies are required to build (vendor stubs are provided for service/systray).

## Directory Structure

goconnect/
- `cmd/service` – GOConnect Service (agent + API + TUN orchestrator stub)
- `cmd/tray` – GOConnect Tray app (systray stub + open panel)
- `internal/api` – HTTP handlers, CSRF, SSE log stream
- `internal/core` – service state machine and settings
- `internal/tun` – TUN abstraction (stub)
- `internal/transport` – QUIC/ICE/STUN interfaces (stub)
- `internal/ipam` – local IP assignment (stub)
- `internal/config` – load/save config.yaml and ensure ProgramData folders
- `internal/logging` – simple file logger with rotation
- `internal/security` – DPAPI wrapper (stub)
- `internal/updater` – self-update stub
- `internal/diag` – diagnostics stub
- `internal/i18n` – English/Turkish for service/tray
- `webui/` – Static web UI (tabs + i18n + SSE)
- `build/scripts/` – PowerShell install/uninstall helper scripts
- `vendor/` – Minimal stubs for `kardianos/service` and `getlantern/systray`

## Security & Defaults
- HTTP only on `127.0.0.1:2537`
- Basic anti-CSRF: SameSite=Strict cookie `goc_csrf` and `X-CSRF-Token` header required for non-GET requests
- Files under `%ProgramData%\GOConnect\{config,logs,secrets}`
- Default config written at first run: `port: 2537`, `mtu: 1280`, `log_level: info`, `language: system locale`
- Log rotation: simple size-based rollover (`agent.log` -> `agent.log.1`)

## Internationalization
- Service/Tray i18n JSON: `internal/i18n/en.json`, `internal/i18n/tr.json`
- WebUI i18n JSON: `webui/i18n/en.json`, `webui/i18n/tr.json`
- API returns codes/keys; UI resolves into localized labels
- Tray includes a Language/Dil option to switch dynamically (applies via `/api/settings`)

## HTTP API (stub)
- GET `/api/status` – service/agent, TUN, controller states
- POST `/api/service/start` | `/api/service/stop` | `/api/service/restart`
- GET `/api/networks` – stub networks
- POST `/api/networks/join` | `/api/networks/leave`
- GET `/api/peers` – stub peer list
- GET `/api/logs/stream` – SSE stream of fake log events
- GET/PUT `/api/settings` – port, MTU, log level, language, autostart, controller/relay URLs
- POST `/api/diag/run` – stub results
- POST `/api/update/check` | `/api/update/apply` – updater stub

## Build & Run

Prerequisites: Go 1.22+ on Windows.

- Build all: `go build ./...`
- Run service (dev, foreground): `go run ./cmd/service`
- Open Web UI: http://127.0.0.1:2537
- Run tray (stub): `go run ./cmd/tray`

Service install (example, admin PowerShell):
- Install: `powershell -ExecutionPolicy Bypass -File build\scripts\install-service.ps1 -ExePath "C:\\path\\to\\goconnect-service.exe"`
- Uninstall: `powershell -ExecutionPolicy Bypass -File build\scripts\uninstall-service.ps1`

ProgramData paths:
- Config: `%ProgramData%\GOConnect\config\config.yaml`
- Logs: `%ProgramData%\GOConnect\logs\agent.log`
- Secrets: `%ProgramData%\GOConnect\secrets\`

## How to Run (Quick)
1. `go run ./cmd/service`
2. Open http://127.0.0.1:2537
3. Navigate tabs, use Start/Stop/Restart, tweak Settings, and see log stream.

## Roadmap / TODOs
- v1.1: Real TUN (Wintun), packet loopback test
- v1.2: NAT traversal skeleton with QUIC + STUN/ICE
- v1.3: DPAPI secrets
- v2: ACL/DNS and advanced policies
- v3: SSO (AzureAD/OIDC) and central policy management

## Notes & Suggested Improvements
- Vendor stubs allow offline builds. Swap to real `kardianos/service` and `systray` by removing `replace` directives in `go.mod` when network access is available.
- For production, embed `webui` assets using `embed` to ship a single binary.
- Replace naive YAML parsing with `gopkg.in/yaml.v3` when dependencies are allowed.
- Improve locale detection using Windows APIs (registry) instead of `LANG` heuristic.
- Harden CSRF/auth with session tokens and origin checks before exposing beyond localhost.

