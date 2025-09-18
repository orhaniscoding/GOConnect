

# GOConnect

![GitHub release (latest by date)](https://img.shields.io/github/v/release/orhaniscoding/GOConnect)
![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/orhaniscoding/GOConnect/ci.yml?branch=main)
![Platform](https://img.shields.io/badge/platform-Windows-blue)

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

### Kullanım Senaryoları ve Kurulum Rehberi

#### 1. Sadece Controller Kurmak İsteyenler İçin (Sunucu/VPS)
**Gereken Dosyalar:**
- `goconnectcontroller.exe`
- `build/scripts/install-service-controller.ps1`
- `build/scripts/uninstall-service-controller.ps1`
- (İlk kurulumda) `secrets/controller_token.txt`
**Kurulum Adımları:**
1. Yukarıdaki dosyaları bir klasöre kopyalayın (ör: `C:\GOConnect`).
2. PowerShell’i yönetici olarak açın.
3. Komutları çalıştırın:
   ```powershell
   cd C:\GOConnect\build\scripts
   ./install-service-controller.ps1
   ```
4. Servis otomatik başlar. Web arayüzüne bağlanmak için: http://localhost:2537
5. Token’ı `secrets/controller_token.txt` dosyasından alıp istemcilerde kullanabilirsiniz.
**Not:** Sadece controller kurmak için agent kurmak zorunda değilsiniz.

#### 2. Sadece Agent Kurmak İsteyenler İçin (İstemci Cihazlar)
**Gereken Dosyalar:**
- `goconnect-service.exe`
- `build/scripts/install-service-agent.ps1`
- `build/scripts/uninstall-service-agent.ps1`
**Kurulum Adımları:**
1. Dosyaları bir klasöre kopyalayın (ör: `C:\GOConnect`).
2. PowerShell’i yönetici olarak açın.
3. Komutları çalıştırın:
   ```powershell
   cd C:\GOConnect\build\scripts
   ./install-service-agent.ps1
   ```
4. Servis otomatik başlar. Web arayüzüne bağlanmak için: http://localhost:2537
5. Controller’dan aldığınız token ve adres ile Web UI’dan ağa katılın.

#### 3. Hem Controller Hem Agent Kurmak (Test veya Geliştirici)
Her iki binary ve ilgili scriptleri aynı makinede kurup yukarıdaki adımları uygulayabilirsiniz. Her servis kendi başına çalışır.

### Web UI ve Token Yönetimi
- Web arayüzü: http://localhost:2537
- Controller token’ı: `secrets/controller_token.txt` dosyasında bulunur
- Agent’lar ağa katılırken bu token’ı kullanır
 - Güvenilir eş sertifikaları: Ayarlardaki `trusted_peer_certs` alanı hem dosya yolu (göreli yollar `ProgramData/GOConnect/secrets` altına göre çözümlenir) hem de satır içi PEM içeriğini ("-----BEGIN CERTIFICATE-----" ile başlayan) destekler.
 - Katılım sırrı (JoinSecret): Bir ağ için sır tanımlandıysa katılım sırasında boş bırakılamaz ve eşleşmelidir; aksi halde 400/403 döner.

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

### License
Licensed under the GPL 3.0 License. See [LICENSE](LICENSE) for details.
