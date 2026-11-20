# AFA 0.2.0-alpha.1 doğrulama kaydı

İnceleme tarihi 20 Kasım 2026. Derleme ortamı macOS 27.0.1 arm64, Apple M5, 10 çekirdek ve 24 GiB RAM. Sürüm araç zinciri Go 1.27.4. `go.mod` dil tabanı 1.22. `go list -m all` yalnızca `afa` döndürdü. Paketleme ve doğrulama Python standart kütüphanesini kullanıyor. `govulncheck` sürüm doğrulama aracıdır.

| Alan | Kanıt |
|---|---|
| Mevcut komut satırı | `go test -count=1 ./...`, `go test -race -count=1 ./...` ve `go vet ./...` yerel macOS hedefinde başarılı. Komut ayrıştırma, JSON, güvenli hata ve varsayılan tarayıcıya devri taklit eden testler mevcut. |
| Ağ tanıları | Kontrollü HTTP/TLS sunucuları, sahte çözümleyiciler, iki adres ailesi, yanıt sınırı, zaman aşımı ve iptal test edildi. Canlı yoklama, DNS/TCP/TLS denetimi, 1 MiB indirme ve HTTPS kalite ölçümü macOS’te başarıyla tamamlandı. |
| Windows | `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go vet ./...` ve çapraz derleme başarılı. Masaüstü ve CLI ayrı PE32+ yürütülebilir dosyalar. Windows istemci üzerinde çalışma henüz yürütülmedi. |
| Yapılandırma | Doğrulama, yerel dosya gidiş dönüşü, eski sürüm geçişi ve bozuk dosyadan varsayılanlarla kurtarma test edildi. Windows dosya değiştirme çağrısı için yerel Windows kabulü açık. |
| Yayın | Kaynak ZIP’i, Windows x64 ZIP’i, SHA-256 listesi ve derleme kaydı. İki yerel üretim karşılaştırılır. İkili imzası ve yayımlanmış GitHub Release kaydı bulunmaz. |
| Güvenlik | TLS zinciri ve hostname doğrulaması korunur. Sabit devretme adresi, kullanıcıya önce gösterilen uyarı, tamsayı HTTP durumu ve güvenli hata kodları kullanılır. Go zafiyet taramasının kapsamı kaynak ve paketlenen ikililerdir. |

Bu doğrulama kaydı Git deposu oluşturulmadan önceki yerel alfa paketine aittir. O paketin derleme kaydı kaynak ağacının SHA-256 özetini içerir. GitHub iş akışından üretilen paketlerde ilgili commit ayrıca kaydedilir. Kabul sırasında bakılacak gerçek Windows sonuçları için [Windows kabul planını](windows-acceptance.md) kullanın.

## Ölçülen komut satırı kaynak tüketimi

`/usr/bin/time -lp` ile `version` beş kez, tanı komutları birer kez ve hız testi 1 MiB olarak ölçüldü. CPU verisi kullanıcı ve çekirdek süreleri toplamı. `0,00 s` aracın gösterim hassasiyetini yansıtır.

| Komut | Tepe RSS | CPU | Duvar saati ve sonuç |
|---|---:|---:|---|
| `version` | 9,98–10,19 MiB | 0,00 s | İlk başlatma 0,61 s. Diğer dört gözlem araç çıktısında 0,00 s. |
| `portal --json` | 12,91 MiB | 0,00 s | 0,12 s. Microsoft yoklaması HTTP 200. |
| `check --json https://example.com` | 14,23 MiB | 0,00 s | 0,21 s. DNS, TCP ve TLS başarılı. |
| `speed --mib 1 --json` | 15,00 MiB | 0,02 s | 0,70 s. Cloudflare’a yapılan tek ölçüm 12,11 Mbps. |
| `quality --samples 3 --json` | 14,95 MiB | 0,00 s | 0,52 s. Üç HTTPS örneğinin başlık süresi ortancası 147,96 ms. Başarısız istek 0. |

Yerel CLI JSON mikro ölçümü `go test ./cmd/afa -run '^$' -bench BenchmarkPortalJSON -benchmem -benchtime=150ms -count=3` ile çalıştırıldı. Taklit portal cevabıyla üç tekrarın ortancası 756 ns/işlem, aralığı 740–1354 ns/işlem, 1273 B ve 15 ayırma/işlem. Bu ölçüm CLI ayrıştırma ve JSON üretim maliyetini kapsar. Süreç başlangıcı ve ağ dahil değildir.

Windows masaüstü boşta veya etkin RAM/CPU, açılış süresi, goroutine sayısı ve uzun süreli bellek davranışı ölçülmedi. Bunlar [performans yöntemi](performance.md) ile Windows istemcide ayrıca yürütülmeli.
