# Windows kabul planı

Bu belge test planıdır. Her çalıştırma için AFA sürümünü, EXE özetini, Windows sürüm ve yapı numarasını, mimariyi, kullanıcı yetkisini ve sonucu kaydedin. Yapılmayan senaryoyu **yürütülmedi** işaretleyin. Bir Windows Server CI çalıştırıcısını Windows 10/11 masaüstü kanıtı olarak kullanmayın.

Birincil hedef desteklenen Windows 11 x64 istemci sürümüdür. Windows 10 desteği ilan edilecekse tam yapı ve destek dalı belirtilmeli. Windows 10 22H2 genel desteği 14 Ekim 2025’te sona erdi. LTSC ve ESU farklı yaşam döngülerine tabidir. [Microsoft Windows yaşam döngüsü](https://learn.microsoft.com/en-us/windows/release-health/release-information)

| Senaryo | Beklenen sonuç ve kayıt |
|---|---|
| Temiz Windows istemcisi ve standart kullanıcı | ZIP açılır. CLI yardım ve sürüm verir. Masaüstü erişilebilir biçimde açılır. Yönetici istemi yordamın bir parçası değildir. |
| Kurumda ayrı yetkili kullanıcı hesabı | Tanı sonucu kullanıcı yetkisi nedeniyle daha geniş erişim iddia etmez. Başlatma kısıtları varsa kaydedilir. |
| Varsayılan tarayıcı, tarayıcı yok veya yönetilen politika | Giriş kaynağı hakkında yönlendirme tarayıcıdan önce görülür. Başlatma hatası ve başarılı devir ayrı kaydedilir. Portalın açılması oturumun açıldığını kanıtlamaz. |
| Bağlantısız cihaz veya erişilemeyen dış uç | Sonuç, yoklamanın erişemediğini gösterir. Tek yoklama hatası tüm internetin çevrimdışı olduğunu bildirmez. |
| Kontrollü portal, sağlayıcı girişi ve süre sonu | Eğitim ağı veya kişiye yetkili gerçek ağ kullanılır. Kullanıcı resmî giriş sayfasında oturum açar. Önceki ve sonraki tanı sonuçları ayrı kaydedilir. |
| Mevcut kurumsal proxy ve VPN | Yalnızca izinli mevcut yapılandırma kullanılır. Proxy kaynaklı hata ve doğrudan tanı yolu doğru etiketlenir. AFA ayar değiştirmez. |
| Wi‑Fi kesintisi, ağ değişimi ve uyku sonrası dönüş | Devam eden tanı iptal edilebilir. Eski sonucun zamanı görünür. İzleme etkinse okumalar ve bildirimler etkin ayara uyar. |
| Hız esnasında iptal, çift başlatma veya diğer AFA işlemi | İstek sonlanır. Aynı Windows oturumundaki masaüstü ve CLI ikinci paralel ölçümü kabul etmez. İptal veya süreç sonlandırma sonrasında yeni ölçüm başlatılabilir. |
| Pencereyi kapatma, tepsiden açma ve tepsiden Çıkış | Tepsi ve pencere davranışı belgede anlatıldığı gibi ayrılır. Tam çıkış sonrası AFA süreci kalmaz. |
| Zorla sonlandırma, uygulama hatası ve Windows yeniden başlatma | Ağ yapılandırması bozulmaz. Geçerli yerel ayarlar okunabilir. Bozuk ayar ve sıfırlama senaryosu ayrıca kaydedilir. |
| Windows güvenlik ve uç nokta koruması | Koruma açık kalır. Windows'un imzasız önizlemeye ilişkin gözlenen davranışı kaydedilir. |
| Türkçe, koyu/açık tema, klavye ve ekran okuyucu | Metin taşması, sekme sırası, odak, erişilebilir isimler ve durum bildirimi ayrı gözden geçirilir. |

Asgari kayıt alanları `Windows build`, `AFA version`, `EXE SHA-256`, `standard user`, `scenario`, `status`, `evidence`, `known limitation`. Destek çıktısında ağ veya kullanıcı kimliğini yazmayın. Bu plan, amaçlı olarak izinli ve kontrollü ortamlarda çalıştırılacak senaryoları tarif eder.
