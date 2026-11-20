# Sürüm oluşturma

Sürüm paketleri için `go.mod` içindeki Go araç zinciri ve Python 3.9 veya üzeri kullanılır. Go modülünü çekirdek uygulamanın bulunduğu kökten derleyin. Git çalışma ağacı mevcutsa derlemeden önce commit ve çalışma ağacı durumunu doğrulayın.

```sh
go env GOVERSION
go test ./...
go vet ./...
python3 -B -m unittest discover -s scripts -p 'test_*.py'
python3 scripts/package.py --out ../afa-review
python3 scripts/package.py --verify ../afa-review
```

`--out` yeni bir dizin alır. Paketleyici kaynak ağacına yazmaz ve mevcut dosyaların üzerine çıkmaz. Windows amd64 derlemesi `CGO_ENABLED=0` ile yapılır. `afa.exe` konsol uygulaması, `AFA-Desktop.exe` grafik uygulamasıdır. Yalnız CLI geliştirmesinde `--cli-only` kullanılabilir. Sürüm sürücüsü Go çalışma zamanı lisansını ve kullanılan yerleşik Go bileşenlerinin bildirimlerini taşınabilir arşive koyar.

| Dosya | İçerik |
|---|---|
| `afa-<version>-windows-x64.zip` | Taşınabilir Windows ikilileri, kullanım bilgileri, MIT lisansı, üçüncü taraf bildirimleri ve derleme kaydı |
| `afa-<version>-source.zip` | Kaynak, test, CI ve proje belgelerinden oluşan sürüm anlık görüntüsü |
| `build-provenance.json` | Go sürümü, hedef, derleme seçenekleri, kaynak özeti, mevcut Git durumu ve ikili özetleri |
| `SHA256SUMS` | İki arşiv ve derleme kaydı için SHA‑256 manifesti |

Aynı kaynak baytları, sürüm, Git durumu ve Go araç zinciriyle iki derlemenin sonuçlarını bayt bazında karşılaştırın. Arşiv sırası, dosya izinleri ve tarihleri sabit. ZIP girdileri sıkıştırmasız saklandığından sıkıştırma kütüphanesi sonucu değiştirmez. Kaynak doğrudan bir Git kaydına bağlı değilse derleme kaydı commit alanını boş bırakır. ZIP ve checksum doğrulaması bütünlüğü kontrol eder. Yayıncının kimliğinin doğrulanabilmesi için ilgili kimlik ve imza kanıtı ayrıca gerekir.

GitHub Actions, Linux ve Windows derleme çalıştırıcılarında kontroller yapacak biçimde tanımlandı. Manuel **Build and verify** çalıştırması inceleme için workflow artefaktı üretir. GitHub Release oluşturma ve yayımlama, bakımcının açık onayına bağlı ayrı adımlar. Yayın öncesi kabul kaydını tamamlayın, arşivleri doğrulayın ve sürüm notlarına desteklenen hedefleri ekleyin. Bu paketleme akışı Authenticode imzası üretmez. GitHub kaydı olmayan yerel paketlerde JSON dosyası imzalı bir derleme tasdiki yerine yerel denetim verisi sunar. [GitHub derleme tasdikleri](https://docs.github.com/en/actions/how-tos/secure-your-work/use-artifact-attestations)
