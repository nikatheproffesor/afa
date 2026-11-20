# Gizlilik

AFA, bağlantı sorunlarını kullanıcının cihazında inceleyen bir uygulamadır. Uygulamada telemetri, analitik veya AFA sunucusuna kimlik bilgisi gönderme akışı yoktur. Ayarlar desteklenen masaüstü sürümünde yerel olarak tutulur. Ağ portalına girilen kullanıcı bilgilerini işletim sisteminin tarayıcısı ve ağın kendi giriş sayfası işler.

Portal yoklaması, Microsoft'un bağlantı sınama adresine HTTP isteği yapar. Kullanıcı başlattığında hız komutu Cloudflare'a HTTPS isteği gönderir. Servis tanısı, belirtilen hedefin çözümleme ve bağlantı aşamalarını çalıştırır. Kullanıcı bir servis profili seçerse o profilin açık web adresine HTTPS bağlantısı dener. Bu isteklerin normal ağ meta verisi, kullanılan işletim sistemi, ağ, çözümleyici, proxy ve uzak hizmet tarafından görülebilir.

Varsayılan tanı çıktısı, işlemin sonuç ve hata kategorilerini göstermek için tasarlanmıştır. Destek amacıyla çıktı paylaşmadan önce hedef alan adını ve yerel sistem bilgilerini denetleyin. AFA'nın açtığı harici sayfalar ilgili kurumun veya hizmet sağlayıcının gizlilik koşullarına tabidir. [Microsoft gizlilik bildirimi](https://www.microsoft.com/privacy/privacystatement) · [Cloudflare gizlilik politikası](https://www.cloudflare.com/privacypolicy/)
