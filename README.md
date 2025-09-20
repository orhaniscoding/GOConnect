

# GOConnect

![GitHub release (latest by date)](https://img.shields.io/github/v/release/orhaniscoding/GOConnect)
![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/orhaniscoding/GOConnect/ci.yml?branch=main)
![Platform](https://img.shields.io/badge/platform-Windows-blue)
<!-- Optional (enable after integrating Codecov or similar): -->
<!-- ![Coverage](https://img.shields.io/codecov/c/github/orhaniscoding/GOConnect?label=coverage) -->

## 🇹🇷 Türkçe Açıklama

### Genel Bakış
GOConnect, Windows için geliştirilmiş, merkezi controller destekli, modern ve güvenli bir overlay ağ (VPN) çözümüdür. ZeroTier/Tailscale esintili, kolay kurulum ve merkezi yönetim sunar. Tüm istemciler (agent) ve controller ayrı binary olarak çalışır, Web UI ile kolayca yönetilir.

### Desteklenen Platformlar ve Gereksinimler
- **Sadece Windows 10/11 veya Windows Server**
- PowerShell yüklü olarak gelir, ek bir program gerekmez
- .NET, Go veya başka bir ek yazılım gerekmez (kullanıcı için)

### Dosya ve Dizin Yapısı
- `goconnectcontroller.exe` — Controller binary'si (merkezi yönetim için)
- `goconnect-service.exe` — Agent binary'si (istemci cihazlar için)
- `build/scripts/install-service-controller.ps1` — Controller servisini kurar
- `build/scripts/uninstall-service-controller.ps1` — Controller servisini kaldırır
- `build/scripts/install-service-agent.ps1` — Agent servisini kurar
- `build/scripts/uninstall-service-agent.ps1` — Agent servisini kaldırır
- `secrets/controller_token.txt` — Controller token dosyası (otomatik oluşur)
- `webui/` — Web arayüzü dosyaları (gömülü gelir)
- `internal/` — Backend ve ağ yönetim kodları

### Derleme ve Çalıştırma (Geliştiriciler)

Gereksinimler: Go 1.23+, Windows 10/11 (PowerShell ile)

- Derleme (controller):
   ```powershell
   go build -o bin/goconnectcontroller.exe ./cmd/goconnectcontroller
   ```
- Derleme (agent/service):
   ```powershell
   go build -o bin/goconnect-service.exe ./cmd/goconnectservice
   ```
- Wintun ile derleme (sadece compile smoke):
   ```powershell
   go build -tags=wintun ./...
   ```
- Servisi geliştirme modunda çalıştır:
   ```powershell
   go run ./cmd/goconnectservice
   # veya Wintun ile
   go run -tags=wintun ./cmd/goconnectservice
   ```

HTTP UI/API varsayılan olarak http://127.0.0.1:2537 adresinde açılır.

### Yapılandırma (config.yaml) Örneği

Yol: `%ProgramData%/GOConnect/config/config.yaml`

```yaml
port: 2537
mtu: 1280
language: en
controller_url: "http://127.0.0.1:8080"  # opsiyonel, controller proxy
stun_servers: ["stun.l.google.com:19302"]
trusted_peer_certs: []
api:
   auth: bearer
   bearer_token: "changeme-local-owner"
   rate_limit:
      rps: 10
      burst: 20
   validation: true
logging:
   format: json  # json | text
   level: info   # trace|debug|info|warn|error
metrics:
   enabled: false
   addr: 127.0.0.1:9090  # /metrics Prometheus endpoint
diag:
   mtu_probe_max: 1500
updater:
   enabled: false
   repo: "orhaniscoding/GOConnect"
   require_signature: false
   public_key: ""
networks:
   - id: "home"
      name: "Home"
      joined: true
      address: "10.83.0.2/24"
      join_secret: "optional"
```

Notlar:
- API güvenliği: Bearer token gereklidir. Değiştiriniz ve gizli tutunuz.
- Oran sınırlama: `api.rate_limit` ile RPS/Burst ayarlanır.
- Günlükler: `%ProgramData%/GOConnect/logs/agent.log` (JSON varsayılan).
- Metrics: `metrics.enabled: true` ise `/metrics` (Prometheus formatı) yayınlanır.

### Diagnostik (CLI ve API)

- CLI (JSON çıktı, STUN + MTU):
   ```powershell
   bin/goconnect-service.exe diag
   ```
   Örnek çıktı:
   ```json
   {
      "stun_ok": true,
      "public_endpoint": "203.0.113.10:51820",
      "stun_server": "stun.l.google.com:19302",
      "stun_rtt_ms": 24,
      "mtu_ok": true,
      "mtu": 1500,
      "mtu_source": "interface:Ethernet",
      "errors": [],
      "duration_ms": 53
   }
   ```

- API (POST, CSRF ve Bearer gerekir):
   ```powershell
   # CSRF çerezini almak için önce GET
   Invoke-WebRequest http://127.0.0.1:2537/api/status -OutFile $null -SessionVariable S
   $csrf = $S.Cookies.GetCookies('http://127.0.0.1:2537')['goc_csrf'].Value
   Invoke-RestMethod -Method Post http://127.0.0.1:2537/api/diag/run -Headers @{ 'Authorization'='Bearer changeme-local-owner'; 'X-CSRF-Token'=$csrf }
   ```

### Updater (Güncelleme)

`config.yaml` altında `updater.enabled: true` ve `updater.repo: "owner/repo"` tanımlandığında:

- Sürüm kontrolü:
   ```powershell
   bin/goconnect-service.exe self-update --check
   ```
- Uygula (staging):
   ```powershell
   bin/goconnect-service.exe self-update
   ```

Windows'ta çalışan exe kilitli olabileceği için yeni sürüm `goconnect-service.exe.new` olarak aynı klasöre indirilir; mümkünse atomik olarak `.bak` ile değiş-tokuş yapılır. Kilit varsa `.update-staged` işaret dosyası bırakılır ve bir sonraki yeniden başlatmada etkinleşir.

### Controller Store (Varsayılan: SQLite)

`goconnectcontroller.exe` depolama motoru varsayılan olarak SQLite kullanır. Ortam değişkenleri ile değiştirilebilir:

```powershell
$env:GOCONNECT_STORE_TYPE = 'memory'   # 'sqlite' | 'memory' (varsayılan: sqlite)
$env:GOCONNECT_DATA_DIR = '.\data'     # sqlite dosyaları için dizin
bin/goconnectcontroller.exe
### Dosya ve Dizin Yapısı
- `goconnectcontroller.exe` — Controller binary'si (merkezi yönetim için)
- `goconnect-service.exe` — Agent binary'si (istemci cihazlar için)
- `build/scripts/install-service-controller.ps1` — Controller servisini kurar
- `build/scripts/uninstall-service-controller.ps1` — Controller servisini kaldırır
- `build/scripts/install-service-agent.ps1` — Agent servisini kurar
- `build/scripts/uninstall-service-agent.ps1` — Agent servisini kaldırır
- `secrets/controller_token.txt` — Controller token dosyası (repo içinde örnek). Çalışma zamanında gerçek token `%ProgramData%/GOConnect/secrets/controller_token.txt` altında tutulur (ilk çalıştırmada otomatik oluşturulur).
- `webui/` — Web arayüzü dosyaları (gömülü gelir)
- `internal/` — Backend ve ağ yönetim kodları

HTTP UI/API (Agent) varsayılan olarak http://127.0.0.1:2537 adresinde açılır.
Controller HTTP API ve Admin sayfası varsayılan olarak http://127.0.0.1:2538 üzerinde çalışır.
### Test ve CI

Tüm testler:
```powershell
go test ./... -count=1
```

### Kalıcı Durum (Controller Store)

Şu an controller, basit bir JSON dosyasıyla kalıcı durum tutar (varsayılan: `controller_state.json`, exe'nin yanına yazılır). İleride alternatif depolama seçenekleri (ör. SQLite) eklenebilir.
Kapsam (coverage) profili ile:
```powershell
CI (GitHub Actions) iş akışları:
- Windows: vet + golangci-lint + build + test + kapsam eşiği (>= %60)
- Windows (Wintun): `go build -tags=wintun ./...` (compile-only)
- Linux (opsiyonel): `go test -race`

---

### Kullanım Senaryoları ve Kurulum Rehberi
4. Servis otomatik başlar. Admin sayfası: http://127.0.0.1:2538/admin/token (Varsayılan olarak sadece loopback erişimine açıktır.)
5. Controller API adresi: `http://<sunucu-ip>:2538` (Agent’larda “Controller URL” olarak kullanın).
6. Token dosyası çalışma zamanında `%ProgramData%/GOConnect/secrets/controller_token.txt` altında tutulur. Gerekirse Admin sayfasından yenileyebilir veya temizleyebilirsiniz.
    - Uzaktan Admin erişimi için `CONTROLLER_ADMIN_PASSWORD` ortam değişkenini ayarlayın; bu durumda Basic Auth: `admin:<şifre>` gerekir. Aksi halde Admin yalnızca 127.0.0.1’den erişilebilir.
#### 1. Sadece Controller Kurmak İsteyenler İçin (Sunucu/VPS)
**Gereken Dosyalar:**
### Web UI ve Token Yönetimi
- Agent web arayüzü: http://localhost:2537
- Controller Admin: http://localhost:2538/admin/token (loopback; `CONTROLLER_ADMIN_PASSWORD` ayarlı ise Basic Auth ile uzaktan erişilebilir)
- Controller token’ı çalışma zamanında `%ProgramData%/GOConnect/secrets/controller_token.txt` dosyasında bulunur (ilk çalıştırmada otomatik oluşturulur).
- Agent tarafı: Ayarlar ekranından “Controller Token” alanı ile token’ı UI üzerinden set/clear yapabilirsiniz; ayrıca “Controller URL”’yi de girin. Elle dosya düzenlemek gerekmez.
- Web UI 401 düzeltmesi: Server, HttpOnly `goc_bearer` çerezi ayarlayarak tarayıcıdaki UI’nin bearer token’ını JS’e sızdırmadan otomatik kullanmasını sağlar.
 - Controller proxy: Agent artık `/api/controller/*` isteklerini `settings.controller_url` adresine iletir ve `%ProgramData%/GOConnect/secrets/controller_token.txt` içindeki bearer token'ı otomatik ekler.
**Kurulum Adımları:**
1. Yukarıdaki dosyaları bir klasöre kopyalayın (ör: `C:\GOConnect`).
2. PowerShell’i yönetici olarak açın.
3. Komutları çalıştırın:
   ```powershell
   cd C:\GOConnect\build\scripts
   ./install-service-controller.ps1
### Persistent Store (Controller)

Currently, the controller persists its state in a simple JSON file (default: `controller_state.json`, written next to the executable). Alternative backends (e.g., SQLite) may be added later.
   ```
### File & Directory Structure
- `goconnectcontroller.exe` — Controller binary (for central management)
- `goconnect-service.exe` — Agent binary (for client devices)
- `build/scripts/install-service-controller.ps1` — Installs controller as a service
- `build/scripts/uninstall-service-controller.ps1` — Uninstalls controller service
- `build/scripts/install-service-agent.ps1` — Installs agent as a service
- `build/scripts/uninstall-service-agent.ps1` — Uninstalls agent service
- `secrets/controller_token.txt` — Example token file in repo. At runtime the real token lives under `%ProgramData%/GOConnect/secrets/controller_token.txt` (auto-generated on first run).
- `webui/` — Web UI files (embedded)
- `internal/` — Backend and network management code
**Kurulum Adımları:**
The Agent HTTP UI/API starts at http://127.0.0.1:2537 by default.
The Controller HTTP API and Admin page run on http://127.0.0.1:2538 by default.
1. Dosyaları bir klasöre kopyalayın (ör: `C:\GOConnect`).
2. PowerShell’i yönetici olarak açın.
   ```
4. Servis otomatik başlar. Web arayüzüne bağlanmak için: http://localhost:2537
5. Controller’dan aldığınız token ve adres ile Web UI’dan ağa katılın.

#### 3. Hem Controller Hem Agent Kurmak (Test veya Geliştirici)
Her iki binary ve ilgili scriptleri aynı makinede kurup yukarıdaki adımları uygulayabilirsiniz. Her servis kendi başına çalışır.

### Web UI ve Token Yönetimi
4. The service will start automatically. Admin page: http://127.0.0.1:2538/admin/token (By default only loopback is allowed.)
5. Controller API base URL: `http://<server-ip>:2538` (set this as “Controller URL” on agents).
6. The token is stored at `%ProgramData%/GOConnect/secrets/controller_token.txt`. You can regenerate or clear it from the Admin page.
    - To allow remote Admin access, set `CONTROLLER_ADMIN_PASSWORD` env var; then use Basic Auth `admin:<password>`. Otherwise Admin is loopback-only.
- Controller token’ı: `secrets/controller_token.txt` dosyasında bulunur
- Agent’lar ağa katılırken bu token’ı kullanır
### Web UI & Token Management
- Agent Web UI: http://localhost:2537
- Controller Admin: http://localhost:2538/admin/token (loopback; if `CONTROLLER_ADMIN_PASSWORD` is set, Basic Auth allows remote access)
- Controller token lives under `%ProgramData%/GOConnect/secrets/controller_token.txt` (auto-generated on first run).
- On the agent, you can set/clear the Controller Token from Settings in the Web UI; also set the “Controller URL”. No manual file edits are required.
- 401 UX fix: the server sets a HttpOnly `goc_bearer` cookie so the browser UI can authenticate without exposing the token to JS.
 - Controller proxy: The agent now forwards `/api/controller/*` requests to the configured `settings.controller_url`, automatically attaching the bearer token from `%ProgramData%/GOConnect/secrets/controller_token.txt`.
 - Admin uç noktaları (controller tarafı, `X-Owner-Token` gerektirir):
    - `POST /api/controller/networks/{id}/admin/visibility` {visible}
    - `POST /api/controller/networks/{id}/admin/secret` {joinSecret}
    - `POST /api/controller/networks/{id}/admin/kick` {nodeId}
    - `POST /api/controller/networks/{id}/admin/ban` {nodeId}
    - `POST /api/controller/networks/{id}/admin/unban` {nodeId}
    - `POST /api/controller/networks/{id}/admin/approve` {requestId}
    - `POST /api/controller/networks/{id}/admin/reject` {requestId}
    - `DELETE /api/controller/networks/{id}`

### Sıkça Sorulanlar (FAQ)
- **Sadece controller kurarsam agent kurmak zorunda mıyım?** Hayır, sadece controller kurmak yeterlidir. Agent sadece istemci cihazlara kurulmalıdır.
- **Controller ve agent aynı binary mi?** Hayır, iki ayrı binary vardır.
- **Web UI nasıl açılır?** Her agent veya controller servisi çalıştığında http://localhost:2537 adresinde Web UI başlar.
- **Token kaybolursa ne yapmalıyım?** Controller üzerinde `secrets/controller_token.txt` dosyasını tekrar kontrol edin veya yeni token oluşturun.
- **Tray desteği var mı?** Hayır, tray/ikon desteği tamamen kaldırıldı.
- **Linux/Mac desteği var mı?** Hayır, sadece Windows desteklenmektedir.

### Geliştiriciler İçin Derleme ve Gelişmiş Kullanım
- Go 1.22+ gereklidir (sadece geliştirme için)
- Derleme: `go build -o bin/goconnectcontroller.exe ./cmd/goconnectcontroller`
- Derleme: `go build -o bin/goconnect-service.exe ./cmd/goconnectservice`
- Web UI ve backend kodları gömülü gelir, ekstra işlem gerekmez

### Lisans
GPL 3.0 ile lisanslanmıştır. Ayrıntılar için [LICENSE](LICENSE) dosyasına bakınız.

---

## 🇬🇧 English Description

### Overview
GOConnect is a modern, secure, controller-based overlay network (VPN) solution for Windows. Inspired by ZeroTier/Tailscale, it offers easy setup and centralized management. Controller and agent are separate binaries. Everything is managed via a modern Web UI.

### Supported Platforms & Requirements
- **Windows 10/11 or Windows Server only**
- PowerShell is included by default, no extra software needed
- No .NET, Go, or other dependencies required (for end users)

### File & Directory Structure
- `goconnectcontroller.exe` — Controller binary (for central management)
- `goconnect-service.exe` — Agent binary (for client devices)
- `build/scripts/install-service-controller.ps1` — Installs controller as a service
- `build/scripts/uninstall-service-controller.ps1` — Uninstalls controller service
- `build/scripts/install-service-agent.ps1` — Installs agent as a service
- `build/scripts/uninstall-service-agent.ps1` — Uninstalls agent service
- `secrets/controller_token.txt` — Controller token file (auto-generated)
- `webui/` — Web UI files (embedded)
- `internal/` — Backend and network management code

### Build & Run (Developers)

Requirements: Go 1.23+, Windows 10/11 with PowerShell

- Build controller:
   ```powershell
   go build -o bin/goconnectcontroller.exe ./cmd/goconnectcontroller
   ```
- Build agent/service:
   ```powershell
   go build -o bin/goconnect-service.exe ./cmd/goconnectservice
   ```
- Build with Wintun tag (compile smoke):
   ```powershell
   go build -tags=wintun ./...
   ```
- Run service in dev mode:
   ```powershell
   go run ./cmd/goconnectservice
   # or with Wintun
   go run -tags=wintun ./cmd/goconnectservice
   ```

The HTTP UI/API starts at http://127.0.0.1:2537 by default.

### Configuration (config.yaml) Example

Path: `%ProgramData%/GOConnect/config/config.yaml`

```yaml
port: 2537
mtu: 1280
language: en
controller_url: "http://127.0.0.1:8080"
stun_servers: ["stun.l.google.com:19302"]
trusted_peer_certs: []
api:
   auth: bearer
   bearer_token: "changeme-local-owner"
   rate_limit:
      rps: 10
      burst: 20
   validation: true
logging:
   format: json
   level: info
metrics:
   enabled: false
   addr: 127.0.0.1:9090
diag:
   mtu_probe_max: 1500
updater:
   enabled: false
   repo: "orhaniscoding/GOConnect"
   require_signature: false
   public_key: ""
networks:
   - id: "home"
      name: "Home"
      joined: true
      address: "10.83.0.2/24"
      join_secret: "optional"
```

Notes:
- API security: Bearer token must be set and kept secret for local admin.
- Rate limit: tune via `api.rate_limit`.
- Logs: `%ProgramData%/GOConnect/logs/agent.log` (JSON by default).
- Metrics: enabling `metrics.enabled` exposes a Prometheus `/metrics` endpoint.

### Diagnostics (CLI and API)

- CLI (JSON output):
   ```powershell
   bin/goconnect-service.exe diag
   ```
   Sample:
   ```json
   {
      "stun_ok": true,
      "public_endpoint": "203.0.113.10:51820",
      "stun_server": "stun.l.google.com:19302",
      "stun_rtt_ms": 24,
      "mtu_ok": true,
      "mtu": 1500,
      "mtu_source": "interface:Ethernet",
      "errors": [],
      "duration_ms": 53
   }
   ```

- API (POST, requires CSRF + Bearer):
   ```powershell
   Invoke-WebRequest http://127.0.0.1:2537/api/status -OutFile $null -SessionVariable S
   $csrf = $S.Cookies.GetCookies('http://127.0.0.1:2537')['goc_csrf'].Value
   Invoke-RestMethod -Method Post http://127.0.0.1:2537/api/diag/run -Headers @{ 'Authorization'='Bearer changeme-local-owner'; 'X-CSRF-Token'=$csrf }
   ```

### Updater

With `updater.enabled: true` and a configured `updater.repo` (GitHub `owner/repo`):

- Check for updates:
   ```powershell
   bin/goconnect-service.exe self-update --check
   ```
- Apply update (staged swap):
   ```powershell
   bin/goconnect-service.exe self-update
   ```

On Windows, the running binary may be locked; the updater downloads to `goconnect-service.exe.new` and attempts an atomic swap with a `.bak` backup. If locked, it leaves `.new` plus a `.update-staged` marker for activation on restart.

### Controller Store (Default: SQLite)

`goconnectcontroller.exe` defaults to SQLite. Override via env:

```powershell
$env:GOCONNECT_STORE_TYPE = 'memory'   # 'sqlite' | 'memory' (default: sqlite)
$env:GOCONNECT_DATA_DIR = '.\data'
bin/goconnectcontroller.exe
```

### Security Notes

- API bearer token: configure `api.bearer_token` (strong) for local admin.
- CSRF protection: mutating POSTs require `X-CSRF-Token` (cookie-provided).
- Future: mTLS support is planned.
- Secrets: Windows DPAPI is used for machine-bound encryption where applicable.
- Trusted peer certs: `trusted_peer_certs` accepts file paths or inline PEM.

### Testing & CI

Run all tests:
```powershell
go test ./... -count=1
```

With coverage profile:
```powershell
go test -count=1 -covermode=atomic -coverpkg=./... -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

CI (GitHub Actions):
- Windows: vet + golangci-lint + build + tests + coverage gate (>= 60%)
- Windows (Wintun compile-only): `go build -tags=wintun ./...`
- Linux (optional): `go test -race`

### Usage Scenarios & Installation Guide

#### 1. For Controller-Only Users (Server/VPS)
**Required Files:**
- `goconnectcontroller.exe`
- `build/scripts/install-service-controller.ps1`
- `build/scripts/uninstall-service-controller.ps1`
- (First setup) `secrets/controller_token.txt`
**Installation Steps:**
1. Copy the above files to a folder (e.g., `C:\GOConnect`).
2. Open PowerShell as Administrator.
3. Run:
   ```powershell
   cd C:\GOConnect\build\scripts
   ./install-service-controller.ps1
   ```
4. The service will start automatically. Access the web interface at: http://localhost:2537
5. Get the token from `secrets/controller_token.txt` and use it on clients.
**Note:** You do NOT need to install the agent if you only want to run the controller.

#### 2. For Agent-Only Users (Client Devices)
**Required Files:**
- `goconnect-service.exe`
- `build/scripts/install-service-agent.ps1`
- `build/scripts/uninstall-service-agent.ps1`
**Installation Steps:**
1. Copy the files to a folder (e.g., `C:\GOConnect`).
2. Open PowerShell as Administrator.
3. Run:
   ```powershell
   cd C:\GOConnect\build\scripts
   ./install-service-agent.ps1
   ```
4. The service will start automatically. Access the web interface at: http://localhost:2537
5. Join the network via Web UI using the controller address and token.

#### 3. Running Both Controller and Agent (Test or Development)
You can install both binaries and their scripts on the same machine and follow the above steps for each. Each service runs independently.

### Web UI & Token Management
- Web UI: http://localhost:2537
- Controller token: found in `secrets/controller_token.txt`
- Agents use this token to join the network
 - Trusted peer certificates: The `trusted_peer_certs` setting accepts either file paths (relative paths are resolved against `ProgramData/GOConnect/secrets`) or inline PEM strings containing "-----BEGIN CERTIFICATE-----".
 - JoinSecret enforcement: If a network has a `join_secret` configured, clients must supply a non-empty matching secret when joining; otherwise, the API returns 400 (missing) or 403 (mismatch).
 - Controller proxy: The agent now forwards `/api/controller/*` requests to the configured `settings.controller_url`, automatically attaching the bearer token from `secrets/controller_token.txt`.
 - Owner Tools UI: Enter your network owner token in Settings under "Owner Token (local only)" (stored locally in the browser). In Networks, click a joined network to reveal the Owner Tools card with visibility, secret rotation, kick/ban/unban, approve/reject requests, and delete network actions.
 - Admin endpoints (on the controller, require `X-Owner-Token`):
    - `POST /api/controller/networks/{id}/admin/visibility` {visible}
    - `POST /api/controller/networks/{id}/admin/secret` {joinSecret}
    - `POST /api/controller/networks/{id}/admin/kick` {nodeId}
    - `POST /api/controller/networks/{id}/admin/ban` {nodeId}
    - `POST /api/controller/networks/{id}/admin/unban` {nodeId}
    - `POST /api/controller/networks/{id}/admin/approve` {requestId}
    - `POST /api/controller/networks/{id}/admin/reject` {requestId}
    - `DELETE /api/controller/networks/{id}`

### Frequently Asked Questions (FAQ)
- **If I only install the controller, do I need the agent?** No, the agent is only for client devices. The controller can run alone.
- **Are controller and agent the same binary?** No, they are separate binaries.
- **How do I open the Web UI?** The Web UI is available at http://localhost:2537 whenever the agent or controller service is running.
- **What if I lose the token?** Check or regenerate `secrets/controller_token.txt` on the controller.
- **Is there tray support?** No, tray/icon support has been completely removed.
- **Is there Linux/Mac support?** No, only Windows is supported.

### For Developers: Building & Advanced Usage
- Requires Go 1.22+ (for development only)
- Build controller: `go build -o bin/goconnectcontroller.exe ./cmd/goconnectcontroller`
- Build agent: `go build -o bin/goconnect-service.exe ./cmd/goconnectservice`
- Web UI and backend code are embedded, no extra steps needed

#### Windows + Wintun builds
- Compile-only check for Wintun path:
   - `go build -tags=wintun ./...`
- Run service with Wintun (local dev):
   - PowerShell: `build\scripts\run-service-wintun.ps1`
   - Note: Bringing up the actual TUN interface may require running PowerShell as Administrator and Wintun driver/DLL installed.
- Optional smoke tool:
   - Build: `go build -o bin\tunsmoke.exe ./cmd/tunsmoke`
   - Run: `bin\tunsmoke.exe` (add `-loopback` to try a simple UDP loopback if the interface IP is configured)

### License
Licensed under the GPL 3.0 License. See [LICENSE](LICENSE) for details.
