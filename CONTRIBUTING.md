# Katkıda bulunma

Değişikliklerinizi tek bir soruna odaklayın. Davranış değiştiğinde yerel test ekleyin ve ilgili kullanım belgesini güncelleyin. Ağ tanılarını kontrollü yerel sunucularla veya test çiftleriyle sınayın. Hedef ağda gözlemlenen davranışı, hangi işletim sisteminde ve hangi sürümde gördüğünüzü belirtin.

```sh
go test ./...
go vet ./...
go test -race ./...
python3 -B -m unittest discover -s scripts -p 'test_*.py'
```

Go araç zinciri `go.mod` içinde sabitlenir. Katkıya sürüm değiştirme gereği doğduğunda Go sürüm notlarını, derleme meta verisini ve hedef platformdaki sonuçları ekleyin. Race testi için Go'nun desteklediği bir ortam gerekir.

Hata raporuna komutu, AFA sürümünü, işletim sistemi ve mimariyi, beklenen davranışı ve tekrar adımlarını ekleyin. Kimlik bilgilerini, oturum bağlantılarını, `Authorization` başlıklarını ve özel ağ tanımlayıcılarını paylaşmadan önce kaldırın. Güvenlikle ilgili sorunlar için [SECURITY.md](SECURITY.md) yolunu kullanın.
