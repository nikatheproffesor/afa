# Performans ölçüm yöntemi

AFA'nın kaynak tüketimi, seçilmiş tek bir ağ örneğinden tüm cihazlara genellenmemeli. Her ölçüm kaydına Windows sürümünü ve yapısını, CPU ve RAM'i, güç modunu, AFA ikilisinin sürüm ve özetini, uç noktayı, ayarları ve örnek sayısını yazın.

1. Beş temiz başlatmada işlem başlangıcından etkileşim kurulabilen pencereye kadar geçen süreyi ölçün. Sıcak ve soğuk örnekleri ayrı kaydedin.
2. Pencere açık ve tepside bekleme durumlarında ayrı idle örnekleri toplayın. İşlem çalışma kümesini, özel belleği, kullanıcı ve çekirdek CPU süre farkını örnekleyin. İsteğe bağlı gözlem kapalı ve açık koşulları ayrı yürütün.
3. Portal, DNS/TCP/TLS, hız ve iptal için aynı koşullarda aktif tepe bellek ve toplam CPU süresini kaydedin. Hızı ölçerken her tekrarda aynı örnek boyutunu ve hedefi kullanın.
4. Kararlı beklemede en az 30 dakikalık trendi ve başlatılan tanı sayısını kaydedin. Arka plan süreçleri veya ağ istekleri için süreç düzeyinde gözlem yapın. Go goroutine sayısını ölçmek gerektiğinde yalnızca yerel test derlemesinde `runtime.NumGoroutine` örnekleri kullanın.
5. Ortancayı ve aralığı veya çeyrekler arası genişliği raporlayın. Birden çok donanımda sonuçları ayrı verin.

Windows yerel süreci için PowerShell `Get-Process` üzerinden `WorkingSet64`, `PrivateMemorySize64`, `TotalProcessorTime` ve `Threads.Count` okunabilir. İş parçacığı sayısı goroutine sayısını temsil etmez. Arayüz yanıt süresi için bir insan etkileşim testini süreç metrikleriyle birlikte kaydedin. Bir zaman eşiği veya bellek hedefi ilan etmeden önce gerçek istemci ölçümlerine bakın. [PowerShell Get-Process](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.management/get-process)

Kısa bir HTTPS indirmesi seçilen uzak sunucuya ait uygulama katmanı örneğidir. Kurulum süresi, ağ yolu, anlık yük ve sunucunun hizmet noktası sonucu etkileyebilir. Alternatif bağlantı koşullarını karşılaştırırken aynı ölçüm boyutunu ve benzer koşulları kullanın. Bu plan, Windows'ta tamamlanmış bir ölçüm sonucu içermez.
