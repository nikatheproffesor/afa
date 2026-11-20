# AFA tehdit modeli

## Varlıklar ve güven sınırları

AFA'nın koruduğu varlıklar giriş bilgileri, portal bağlantıları, yerel ayarlar, kullanıcıya gösterilen tanı sonucu ve dağıtılan ikilinin bütünlüğü. İşletim sistemi, tarayıcı, DNS, mevcut proxy ve uzak hizmetler AFA dışında güven sınırlarıdır. Uzak bağlantıdan gelen veri ve hata metni kullanıcıya gösterilmeden önce güvensiz kabul edilir.

| Risk | Denetim veya sınır |
|---|---|
| HTTP yoklamasıyla yanıltıcı portal veya ağ durumu | HTTP sonucu gözlem olarak işlenir. Kullanıcı giriş bilgisi girmeden önce resmî adresi doğrular. Tarayıcıya devir, giriş başarısı olarak sunulmaz. |
| Geçersiz veya farklı ada ait TLS sertifikası | Go'nun standart sertifika zinciri ve sunucu adı doğrulaması kullanılır. İstemci TLS doğrulamasını kapatan ayar sunmaz. |
| Uzak yanıtın log ve destek çıktısına veri sokması | Kullanıcıya kararlı hata kategorileri ve temizlenmiş özetler gösterilir. Kimlik bilgisi, özel yönlendirme adresi ve ham başlıklar destek çıktısına alınmaz. |
| Komut başlatma veya URL güveni | Tarayıcıya işletim sistemi üzerinden sabit bir başlangıç adresi verilir. Uzak yanıt tarayıcı komutu olarak yorumlanmaz. |
| Kalıcı ağ ayarı ve yetki | AFA normal tanıda yönetici gerektirmez ve sistem DNS, proxy, yönlendirme ile güvenlik duvarı ayarlarını yazmaz. |
| İndirilen ikilinin değiştirilmesi | Paket, ikili özeti ve derleme kaydıyla doğrulanır. Yerel özet doğrulaması yayıncının kimliğini tek başına doğrulamaz. Bu önizleme Authenticode imzası taşımaz. |
| Üçüncü taraf kodu ve güncelleme | Uygulama kendi kendini güncellemez. Go araç zinciri ve CI araçları sürüme sabitlenir. Üretime giren bileşenler ve bildirimler paketleme sırasında envantere alınır. |
| Kimlik bilgisi depolama | Uygulama portal parolası toplamaz. Tarayıcı ve sağlayıcı normal kimlik doğrulamasını yürütür. Gelecekte yetkili sağlayıcı entegrasyonu eklenirse kimlik bilgisi depolaması ayrı güvenlik değerlendirmesi gerektirir. |

İstemci ağ yapılandırmasının güvenliği işletim sistemi ve kullanıcıya bağlıdır. İşletim sistemi güven deposuna eklenmiş bir kurum sertifikası, standart doğrulamanın güven kapsamına girebilir. Windows'a özgü tarayıcı, standart kullanıcı, ağ geçişi ve güvenlik ilkesi koşulları için [kabul planını](windows-acceptance.md) kullanın.
