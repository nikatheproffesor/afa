# Üçüncü taraf bildirimleri

AFA'nın kendi kaynak kodu MIT lisansı altındadır. Windows arşivinin `licenses/` dizini, üretimde kullanılan Go araç zincirinin lisansını, varsa patent beyanını ve ikilinin kullandığı Go standart kütüphanesine gömülü `golang.org/x` bileşenlerinin bildirimlerini içerir. Paketleyici bu envanteri hedefe göre `go list -deps` ile çıkarır. [Go lisansı](https://go.dev/LICENSE)

Microsoft'un bağlantı yoklama adresi ve Cloudflare'ın hız ölçüm adresi, ilgili tanı komutunun gönderdiği isteklerde kullanılan harici hizmetlerdir. Microsoft NCSI yazılımı ve `cloudflare/speedtest` SDK'sı AFA ile dağıtılmaz. [Microsoft NCSI](https://learn.microsoft.com/en-us/windows-server/networking/ncsi/ncsi-overview) · [Cloudflare Speedtest](https://github.com/cloudflare/speedtest)

Hello DPI (MIT) ve `gsbwifi-bypass` (açık lisans beyanı bulunamadı) ilk tasarım incelemesinde başvurulan projelerdir. AFA paketleri bu projelerin kaynaklarını veya ikililerini içermez. [Hello DPI lisansı](https://github.com/nikatheproffesor/hello-dpi/blob/v5.5.0/LICENSE) · [GSB deposu](https://github.com/denizZz009/gsbwifi-bypass)
