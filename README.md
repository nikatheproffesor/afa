# AFA · Access for All

AFA, Wi‑Fi girişini açmaya yardımcı olan ve bağlantıyı tanılayan bir Windows uygulaması. Sistem tepsisinden bir ağ yoklaması başlatabilir, bir web adresi için DNS/TCP/TLS sonuçlarını görebilir ve seçtiğiniz boyutta indirme ölçümü alabilirsiniz. AFA 0.2.0-alpha.1 erken deneme sürümüdür. Komut satırı ve masaüstü uygulaması aynı Go tanı paketlerini kullanır.

## Windows’ta çalıştırma

Windows x64 taşınabilir ZIP’ini çıkarın. Grafik arayüz için `AFA-Desktop.exe`, PowerShell’den çalışmak için `afa.exe` kullanın. Paket alfanın imzasız derlemesidir. Dosyaları [yayın doğrulama adımları](docs/release.md) ile kontrol edebilirsiniz.

Paket Windows 11 x64 hedeflenerek çapraz derlendi. Windows 10 x64 uyumluluğu gerçek istemci makinesinde doğrulanmayı bekliyor. Microsoft'un Windows 10 sürümleri için yayınladığı yaşam döngüsünü dikkate alın. [Windows sürüm bilgisi](https://learn.microsoft.com/en-us/windows/release-health/release-information)

GitHub’dan indirmek için hesabınızla oturum açın. **Actions → Build and verify** bölümünde başarılı bir manuel çalıştırmayı açın. **Artifacts** altındaki `afa-windows-x64-review` paketini indirin. İçindeki `afa-0.2.0-alpha.1-windows-x64.zip` dosyasını da çıkarın. [GitHub Actions çalıştırmaları](https://github.com/nikatheproffesor/afa/actions/workflows/ci.yml)

### Masaüstü

- **Bağlantı** son yoklamayı ve gözlenen kanıtı gösterir. **Ağ girişini aç**, doğrulama uyarısının ardından varsayılan tarayıcıya bir açma isteği gönderir. Oturumu sağlayıcının sayfasında siz tamamlarsınız.
- **Tanı** belirttiğiniz HTTPS adresinde DNS, TCP ve TLS aşamalarını çalıştırır. **Hız** indirme boyutunu, ilerlemeyi, HTTP başlık süresini, toplam süreyi ve aktarım hızını gösterir.
- **Ayarlar** açık/koyu tema, Türkçe/İngilizce, pencere veya tepsiyle başlatma, bildirim, zaman aşımı ve isteğe bağlı ağ değişikliği gözlemi sunar. Ayarlar yerel kullanıcı yapılandırmasında saklanır. Başlatma tercihi AFA’yı açtığınızda uygulanır. Windows oturum açılışına bir kayıt eklenmez.
- Pencereyi kapatmak, simge başarıyla kaydedildiyse uygulamayı tepside bırakır. İşlemi sonlandırmak için tepsideki **Çıkış** seçeneğini kullanın. Ağ değişikliği gözlemi varsayılan olarak kapalıdır.
- Masaüstü, tepsi, klavye, tema ve Windows ağ değişikliği akışlarının gerçek Windows kabul durumu [test matrisinde](docs/windows-acceptance.md) kayıtlıdır.

### PowerShell

```powershell
.\afa.exe version
.\afa.exe portal --json
.\afa.exe portal --open
.\afa.exe check --json https://www.roblox.com
.\afa.exe check --profile discord
.\afa.exe speed --mib 1 --samples 3
.\afa.exe quality --samples 5 --json
```

**Portal.** Yoklama Windows’un belgelenmiş Microsoft HTTP adresine tek istek gönderir. Beklenen yanıt test ucunun erişilebilirliğini gösterir. Yönlendirme, HTML veya HTTP 511 olası portal işaretidir. `--open` ağın sizi yönlendirebileceği sabit adresi açmayı dener. Windows, macOS ve Linux CLI başlatıcıları platforma özgüdür. GSB girişi ve Karabük eduroam için sağlayıcının resmî akışını kullanın. [Microsoft NCSI](https://learn.microsoft.com/en-us/windows-server/networking/ncsi/ncsi-overview) · [GSB şifre işlemleri](https://www.turkiye.gov.tr/genclik-ve-spor-gsb-wifi-sifre-islemleri) · [Karabük eduroam](https://eduroam.karabuk.edu.tr/index.aspx)

**Servis tanısı.** `check` hedef adı ve portu ölçer. Varsayılan port 443. Web profilleri `discord`, `roblox`, `steam` ve `epic` için ayrıca sabit web adresine yönlendirme izlemeyen bir HTTPS HEAD isteği yapar. Sonuç doğrudan DNS/TCP/TLS bağlantısı ve test edilen web adresine özgüdür. Oyun oturumu, ses, istemci girişi ve CDN trafiği ayrı kanıt gerektirir.

**Hız ve kalite.** `speed` örnek başına 1–8 MiB indirir. `--samples` 1–5, varsayılan 1. Sonuç tek sunucuya yapılan kısa aktarım koşullarını yansıtır. `quality` en fazla sekiz gövdesiz HTTPS isteği ile başlıklara kadar süre ve ardışık süre farkı ortancası ölçer. `failed_requests` uygulama düzeyinde HTTP deneme sayısıdır. ICMP ping veya paket kaybı ölçümü bulunmaz. [Cloudflare indirme ucu](https://github.com/cloudflare/speedtest/blob/main/README.md#download--upload)

**JSON.** Tanı sonuçları `schema_version: 1`, `app_version`, `operation`, UTC `timestamp`, `duration_millis`, `status` ve `network_context` alanlarını içerir. Hata veya belirsizlikte `error_code` döner. Portalın `state` değerleri 0.1.0’dan değişti. `probe_successful`, `possible_portal`, `unexpected_response`, `probe_unreachable`, `request_canceled` veya `unknown` kullanılır. `browser` açma isteğinin sonucudur. Atlanan servis süreleri JSON’dan çıkarılır. Genelde çıkış kodu `0` başarı, `1` hata veya belirsizlik, `2` geçersiz kullanım. Bayraklar hedef URL’den önce yazılır.

## Gizlilik ve bilinen sınırlar

AFA normal çalışmada yönetici yetkisi istemez, sistem DNS’ini, proxy’yi veya VPN’i değiştirmez. Parola toplama, portal formu gönderme, sağlayıcıya giriş denemesi yapma, otomatik güncelleme ve telemetri akışı bulunmaz. Windows ağ gözlemi etkinleştirildiğinde işletim sistemi olayları dinlenir. SSID ve donanım adresleri saklanmaz. Uygulama, sağlayıcı kota ve oturum verilerini henüz okumuyor. Ayrıntılar [gizlilik bildirimi](PRIVACY.md) ve [tehdit modelinde](docs/threat-model.md).

Tanı paketleri işletim sisteminin mevcut rotasını kullanır. Portal, web profili ve hız testleri ortam proxy değişkenlerinden etkilenebilir. Doğrudan TLS denetimi ile tarayıcı farklı yol kullanabilir. Beklenmeyen sonuçta ağ ayarlarını değiştirmeden önce portal çıktı kanıtına ve zaman damgasına bakın. Hız sonuçlarını karşılaştırırken aynı erişim noktası ve örnek boyutunu kullanın.

## Kaynaktan derleme

Go dil tabanı 1.22. Sürüm üretimi için `go.mod` içindeki Go 1.27.4 araç zinciri kullanılır. Windows masaüstü Win32 denetimleri ve işletim sisteminin tepsi API’sini kullanır. Fyne değerlendirildi. Windows için gereken hedef C derleyicisinin bulunmadığı geliştirme ortamında, yerel Win32 arayüzü `CGO_ENABLED=0` ile aynı paketin üretilmesini sağladı. [Fyne çapraz derleme gereksinimleri](https://docs.fyne.io/started/cross-compiling/) · [Go yayın geçmişi](https://go.dev/doc/devel/release)

```sh
go version
go test ./...
go vet ./...
go test -race ./...
go build -o ../afa-cli ./cmd/afa
python3 scripts/package.py --out ../afa-release
python3 scripts/package.py --verify ../afa-release
```

Paketleme hedefi mevcut olmayan bir dizin olmalı. Aynı kaynak ve araç zinciriyle Windows x64 ZIP, kaynak ZIP, SHA‑256 listesi ve derleme kaydı üretilir. Windows kabul testi, performans ölçümü ve yayın adımları `docs/` altında. macOS ve Linux, CLI geliştirme hedefleridir. Masaüstü uygulaması bu sürümde Windows’a özgüdür.

## Lisans ve destek

AFA [MIT lisansı](LICENSE) ile dağıtılır. Go çalışma zamanı dahil ikili dağıtım bildirimleri [üçüncü taraf envanterinde](THIRD_PARTY_NOTICES.md). [Güvenlik bildirim süreci](SECURITY.md) ve [katkı rehberi](CONTRIBUTING.md) yayıma hazır. [AFA kaynak deposu](https://github.com/nikatheproffesor/afa)
