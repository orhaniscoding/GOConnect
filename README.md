

# GOConnect

![GitHub release (latest by date)](https://img.shields.io/github/v/release/orhaniscoding/GOConnect)
![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/orhaniscoding/GOConnect/ci.yml?branch=main)
![Platform](https://img.shields.io/badge/platform-Windows-blue)

---

## 🇹🇷 Türkçe Açıklama

**GOConnect**; Windows için geliştirilmiş, merkezi controller destekli, modern ve güvenli bir overlay ağ (VPN) çözümüdür. ZeroTier/Tailscale esintili, kolay kurulum ve merkezi yönetim sunar. Tüm istemciler ve controller tek binary ile çalışır, Web UI ile kolayca yönetilir.

### Özellikler
- Merkezi controller ile birden fazla cihazı (VPS, masaüstü, laptop) aynı ağa kolayca dahil etme
- Modern Web UI (http://127.0.0.1:2537) ile ağ, sohbet ve ayar yönetimi
- Wintun tabanlı TUN arayüzü (isteğe bağlı build tag)
- QUIC tabanlı hızlı ve güvenli peer-to-peer iletişim
- Token tabanlı kimlik doğrulama (controller/agent arası)
- Sohbet, ağ ayarları, üyelik ve daha fazlası
- Türkçe ve İngilizce arayüz desteği

### Hızlı Kurulum
1. **Controller (VPS veya ana makine):**
	- `go build -o bin/goconnectcontroller.exe ./cmd/goconnectcontroller`
	- Controller'ı başlatın, `secrets/controller_token.txt` içeriğini not edin.
2. **İstemciler (Agent):**
	- `go build -o bin/goconnect-service.exe ./cmd/goconnectservice`
	- Web UI'dan Controller URL ve Token girerek ağa katılın.
3. **Web UI:**
	- http://127.0.0.1:2537 adresinden tüm yönetim ve izleme işlemlerini yapabilirsiniz.

### Mimarî ve Temel Bileşenler
- **cmd/goconnectservice**: Ana agent/servis binary'si (her istemciye kurulur)
- **cmd/goconnectcontroller**: Controller binary'si (merkezi yönetim için VPS veya ana makineye kurulur)
- **internal/**: Tüm backend ve ağ yönetim kodları
- **webui/**: Modern Web UI ve i18n dosyaları
- **build/scripts/**: Windows servis kurulum scriptleri
- **stubs/**: Kardianos/service için offline stub (Windows servis entegrasyonu)

### Güvenlik
- Tüm controller-agent iletişimi için bearer token zorunlu
- CSRF koruması ve local-only HTTP API
- Windows DPAPI ile gizli anahtarlar şifrelenir

### Desteklenen Platformlar
- **Sadece Windows** (Linux/Mac desteği yoktur)

### Katkı ve Destek
- Sorun bildirimi ve katkı için GitHub Issues ve Pull Request'leri kullanabilirsiniz.
- [orhaniscoding](https://github.com/orhaniscoding) ile iletişime geçebilirsiniz.

### Lisans
MIT Lisansı ile lisanslanmıştır. Ayrıntılar için [LICENSE](LICENSE) dosyasına bakınız.

### Sıkça Sorulanlar
- **Controller ve agent aynı binary mi?** Hayır, `goconnectcontroller.exe` (controller) ve `goconnect-service.exe` (agent) olarak iki ayrı binary vardır.
- **Web UI nasıl açılır?** Her agent çalıştığında otomatik olarak http://127.0.0.1:2537 adresinde Web UI başlar.
- **Token kaybolursa ne yapmalıyım?** Controller üzerinde `secrets/controller_token.txt` dosyasını tekrar kontrol edin veya yeni token oluşturun.
- **Tray desteği var mı?** Hayır, tray/ikon desteği tamamen kaldırıldı.
- **Linux/Mac desteği var mı?** Hayır, sadece Windows desteklenmektedir.

---

## 🇬🇧 English Description

**GOConnect** is a modern, secure, controller-based overlay network (VPN) solution for Windows. Inspired by ZeroTier/Tailscale, it offers easy setup and centralized management. All clients and the controller run as a single binary, managed via a modern Web UI.

### Features
- Central controller: easily connect multiple devices (VPS, desktop, laptop) to the same network
- Modern Web UI (http://127.0.0.1:2537) for network, chat, and settings management
- Wintun-based TUN interface (optional build tag)
- Fast and secure peer-to-peer communication over QUIC
- Token-based authentication (controller/agent)
- Chat, network settings, membership, and more
- UI available in Turkish and English

### Quick Start
1. **Controller (VPS or main machine):**
	- `go build -o bin/goconnectcontroller.exe ./cmd/goconnectcontroller`
	- Start the controller, note the contents of `secrets/controller_token.txt`.
2. **Clients (Agent):**
	- `go build -o bin/goconnect-service.exe ./cmd/goconnectservice`
	- Join the network via Web UI by entering Controller URL and Token.
3. **Web UI:**
	- Manage and monitor everything at http://127.0.0.1:2537

### Architecture & Main Components
- **cmd/goconnectservice**: Main agent/service binary (install on every client)
- **cmd/goconnectcontroller**: Controller binary (for central management, install on VPS or main machine)
- **internal/**: All backend and network management code
- **webui/**: Modern Web UI and i18n files
- **build/scripts/**: Windows service install scripts
- **stubs/**: Offline stub for kardianos/service (Windows service integration)

### Security
- All controller-agent communication requires a bearer token
- CSRF protection and local-only HTTP API
- Secrets are encrypted with Windows DPAPI

### Supported Platforms
- **Windows only** (No Linux/Mac support)

### Contribution & Support
- Use GitHub Issues and Pull Requests for bug reports and contributions.
- Contact [orhaniscoding](https://github.com/orhaniscoding) for further support.

### License
Licensed under the MIT License. See [LICENSE](LICENSE) for details.

### FAQ
- **Are controller and agent the same binary?** No, there are two binaries: `goconnectcontroller.exe` (controller) and `goconnect-service.exe` (agent).
- **How do I open the Web UI?** The Web UI is automatically available at http://127.0.0.1:2537 when the agent runs.
- **What if I lose the token?** Check or regenerate `secrets/controller_token.txt` on the controller.
- **Is there tray support?** No, tray/icon support has been completely removed.
- **Is there Linux/Mac support?** No, only Windows is supported.

---

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
- `stubs/` – Offline stubs for `kardianos/service`

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






