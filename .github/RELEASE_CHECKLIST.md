# GOConnect Release Checklist (Windows Only)

## 1. Sürüm Numarası ve Değişiklikler
- [ ] `go.mod` ve gerekirse diğer dosyalarda sürüm numarasını güncelleyin.
- [ ] `CHANGELOG.md` dosyası oluşturun veya güncelleyin. (Önemli değişiklikleri, yeni özellikleri, hata düzeltmelerini yazın.)

## 2. Derleme (Build)
- [ ] Windows için: `go build -o bin/goconnect-service.exe ./cmd/goconnectservice`
- [ ] Windows için: `go build -o bin/goconnect-tray.exe ./cmd/goconnecttray`
- [ ] (Varsa) Windows için: `go build -o bin/goconnectcontroller.exe ./cmd/goconnectcontroller`
- [ ] Web UI dosyalarını (webui/) güncellediğinizden emin olun.

> Not: Tray bileşeni projeden tamamen kaldırıldığı için `goconnect-tray.exe` satırı silinebilir (gelecek sürümde checklist'ten çıkarılacak).

> **Not:** GOConnect sadece Windows için derlenir ve çalışır. Linux desteği yoktur.

## 3. Release Hazırlığı
- [ ] `bin/` klasöründeki derlenmiş dosyaları ve gerekli scriptleri (ör: `build/scripts/`) bir araya getirin.
- [ ] `README.md` ve varsa `INSTALL.md` dosyalarını güncelleyin.
- [ ] Gereken tüm config ve örnek dosyaları ekleyin (ör: `secrets/controller_token.txt.example`).

## 4. GitHub Release
- [ ] GitHub'da yeni bir tag oluşturun: 
  ```
  git tag vX.Y.Z
  git push origin vX.Y.Z
  ```
- [ ] GitHub arayüzünde "Releases" kısmına girin, yeni bir release başlatın.
- [ ] Release başlığına sürüm numarasını ve kısa özet yazın.
- [ ] `CHANGELOG.md` içeriğini release notlarına ekleyin.
- [ ] Derlenmiş dosyaları (örn. `goconnect-service.exe`, `goconnectcontroller.exe`, scriptler) ekleyin.
- [ ] Yayınla (Publish) butonuna basın.

## 5. Son Kontrol
- [ ] Release dosyalarını indirip test edin.
- [ ] Gerekirse hızlı bir kurulum/test rehberi ekleyin.

---

Daha fazla otomasyon için GitHub Actions ile CI/CD pipeline ekleyebilirsiniz. İsterseniz örnek bir workflow dosyası da hazırlayabilirim.