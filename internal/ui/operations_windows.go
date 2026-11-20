package ui

import (
	"context"
	"strconv"
	"strings"
	"time"

	"afa/internal/diagnostic"
	"afa/internal/network"
	"afa/internal/platform"
	"afa/internal/portal"
	"afa/internal/service"
	"afa/internal/speed"
)

func (a *nativeWindow) operationFinished(update func()) {
	a.post(func() {
		a.gate.finish()
		update()
		a.render()
		if a.monitorPending && a.settings.Monitoring {
			a.monitorPending = false
			a.checkPortal(true)
		}
	})
}

func (a *nativeWindow) checkPortal(automatic bool) {
	ctx, started := a.gate.begin(a.life, "portal", time.Duration(a.settings.TimeoutSeconds)*time.Second)
	if !started {
		if automatic && a.settings.Monitoring {
			a.monitorPending = true
		}
		return
	}
	a.note = a.tr("Bağlantı yoklanıyor…", "Checking the connection…")
	a.render()
	go func() {
		result := portal.Check(ctx)
		observation := network.Current()
		when := time.Now()
		a.operationFinished(func() {
			priorPossible := a.lastPortal != nil && a.lastPortal.State == portal.PossiblePortal
			a.lastPortal = &result
			a.lastProbeTime = when
			a.observation = observation
			a.note = ""
			if result.State == portal.ProbeSuccessful {
				a.lastSuccessful = when
			}
			if result.State == portal.PossiblePortal && !priorPossible {
				a.showTrayNotice()
			}
			a.refreshTray()
		})
	}()
}

func (a *nativeWindow) openPortal() {
	if a.gate.kind != "" {
		return
	}
	guidance := a.tr("Tarayıcı ağ girişini açmayı deneyecek. Parola girmeden önce adresin ağ işletmecisine ait olduğunu doğrulayın.",
		"The browser will try to open network sign-in. Verify the network operator's address before entering a password.")
	if alert(a.hwnd, windowTitle, guidance, mbOKCancel|mbIconWarning|mbDefaultButton2) != idOK {
		return
	}
	ctx, started := a.gate.begin(a.life, "browser", 8*time.Second)
	if !started {
		return
	}
	a.note = a.tr("Tarayıcı açılıyor…", "Opening the browser…")
	a.render()
	go func() {
		err := platform.OpenPortal(ctx)
		a.operationFinished(func() {
			if err != nil {
				code := diagnostic.CodeOf(err, diagnostic.LaunchFailure)
				a.note = errorLabel(a.settings.Language, code)
				if code == diagnostic.LaunchFailure {
					a.note = a.tr("Tarayıcıya açma isteği gönderilemedi.", "The browser could not be asked to open the page.")
				}
				return
			}
			a.note = a.tr("Tarayıcıya açma isteği gönderildi. Oturumu tamamladıktan sonra bağlantıyı denetleyin.",
				"The browser was asked to open the page. Check the connection after completing sign-in.")
		})
	}()
}

func (a *nativeWindow) runService() {
	target := strings.TrimSpace(text(a.input))
	if target == "" {
		a.serviceNote = a.tr("Bir HTTPS adresi girin.", "Enter an HTTPS address.")
		a.render()
		return
	}
	ctx, started := a.gate.begin(a.life, "service", time.Duration(a.settings.TimeoutSeconds)*time.Second)
	if !started {
		return
	}
	a.serviceNote = a.tr("Servis tanısı çalışıyor…", "Running service diagnostics…")
	a.render()
	go func() {
		result, err := service.Check(ctx, target)
		a.operationFinished(func() {
			if err != nil {
				a.serviceNote = errorLabel(a.settings.Language, diagnostic.CodeOf(err, diagnostic.InvalidInput))
				return
			}
			a.lastService = &result
			a.serviceNote = ""
			if result.TLS == "ok" {
				a.lastSuccessful = time.Now()
			}
		})
	}()
}

func (a *nativeWindow) runSpeed() {
	mib, err := strconv.Atoi(strings.TrimSpace(text(a.input)))
	if err != nil || mib < 1 || mib > speed.MaxMiB {
		a.speedNote = a.tr("Bir ile sekiz arasında bir boyut girin.", "Enter a size between 1 and 8.")
		a.render()
		return
	}
	ctx, started := a.gate.begin(a.life, "speed", 0)
	if !started {
		return
	}
	a.transferred, a.transferTotal = 0, int64(mib)<<20
	a.speedNote = a.tr("İndirme yoklaması sürüyor…", "Download measurement in progress…")
	a.render()
	go func() {
		var lastUpdate time.Time
		result, err := speed.Measure(ctx, mib, func(received, requested int64) {
			if time.Since(lastUpdate) >= 100*time.Millisecond || received >= requested {
				lastUpdate = time.Now()
				a.post(func() {
					a.transferred, a.transferTotal = min(received, requested), requested
					if a.page == pageSpeed {
						a.renderSpeed()
					}
				})
			}
		})
		a.operationFinished(func() {
			if err != nil {
				a.transferTotal = 0
				a.speedNote = errorLabel(a.settings.Language, diagnostic.CodeOf(err, diagnostic.ExternalFailure))
				return
			}
			a.transferred, a.transferTotal = result.Bytes, result.Bytes
			a.lastSpeed = &result
			a.lastSuccessful = time.Now()
			a.speedNote = a.tr("İndirme ölçümü tamamlandı.", "Download measurement completed.")
		})
	}()
}

func (a *nativeWindow) cancelOperation() {
	a.gate.stop()
	switch a.gate.kind {
	case "portal", "browser":
		a.note = a.tr("İptal ediliyor…", "Canceling…")
	case "service":
		a.serviceNote = a.tr("İptal ediliyor…", "Canceling…")
	case "speed":
		a.speedNote = a.tr("İptal ediliyor…", "Canceling…")
	}
	a.render()
}

func (a *nativeWindow) refreshNetworkContext() {
	go func() {
		observation := network.Current()
		a.post(func() {
			a.observation = observation
			a.render()
		})
	}()
}

func (a *nativeWindow) configureMonitoring() {
	if a.monitorCancel != nil {
		a.monitorCancel()
		a.monitorCancel = nil
	}
	a.monitorPending = false
	a.monitorFailed = false
	a.monitorGeneration++
	if !a.settings.Monitoring {
		return
	}
	ctx, cancel := context.WithCancel(a.life)
	a.monitorCancel = cancel
	generation := a.monitorGeneration
	go func() {
		changes, err := network.Watch(ctx)
		if err != nil {
			a.post(func() {
				if a.monitorGeneration == generation {
					a.monitorFailed = true
					a.render()
				}
			})
			return
		}
		observeNetworkChanges(ctx, changes, 2*time.Second, func() {
			a.post(func() {
				if a.monitorGeneration == generation {
					a.checkPortal(true)
				}
			})
		})
	}()
}
