package ui

import (
	"context"
	"strings"
	"time"

	"afa/internal/diagnostic"
	"afa/internal/portal"
)

// taskGate is owned by the UI thread. Workers receive only a derived context.
type taskGate struct {
	kind   string
	cancel context.CancelFunc
}

func (g *taskGate) begin(parent context.Context, kind string, timeout time.Duration) (context.Context, bool) {
	if g.kind != "" || parent.Err() != nil {
		return nil, false
	}
	var ctx context.Context
	if timeout > 0 {
		ctx, g.cancel = context.WithTimeout(parent, timeout)
	} else {
		ctx, g.cancel = context.WithCancel(parent)
	}
	g.kind = kind
	return ctx, true
}

func (g *taskGate) stop() {
	if g.cancel != nil {
		g.cancel()
	}
}

func (g *taskGate) finish() {
	g.stop()
	*g = taskGate{}
}

func localized(language, turkish, english string) string {
	if language == "en" {
		return english
	}
	return turkish
}

func portalLabel(language, state string) string {
	tr := func(turkish, english string) string { return localized(language, turkish, english) }
	switch state {
	case portal.ProbeSuccessful:
		return tr("Yoklamadan beklenen yanıt alındı", "Probe returned the expected response")
	case portal.PossiblePortal:
		return tr("Olası ağ giriş sayfası", "Possible network login page")
	case portal.UnexpectedResponse:
		return tr("Beklenmeyen yoklama yanıtı", "Unexpected probe response")
	case portal.ProbeUnreachable:
		return tr("Yoklama adresine ulaşılamadı", "Probe endpoint unreachable")
	case portal.RequestCanceled:
		return tr("Yoklama iptal edildi", "Probe canceled")
	default:
		return tr("Bağlantı durumu bilinmiyor", "Connection state unknown")
	}
}

func errorLabel(language string, code diagnostic.Code) string {
	tr := func(turkish, english string) string { return localized(language, turkish, english) }
	switch code {
	case diagnostic.DNSFailure:
		return tr("Alan adı çözümlenemedi.", "DNS resolution failed.")
	case diagnostic.TCPFailure:
		return tr("TCP bağlantısı kurulamadı.", "TCP connection failed.")
	case diagnostic.TLSFailure:
		return tr("TLS bağlantısı kurulamadı.", "TLS negotiation failed.")
	case diagnostic.TLSCertificate:
		return tr("TLS sertifikası doğrulanamadı.", "TLS certificate validation failed.")
	case diagnostic.Timeout:
		return tr("İşlem zaman aşımına uğradı.", "The operation timed out.")
	case diagnostic.Canceled:
		return tr("İşlem iptal edildi.", "The operation was canceled.")
	case diagnostic.HTTPFailure:
		return tr("Sunucu beklenmeyen bir HTTP yanıtı verdi.", "The server returned an unexpected HTTP response.")
	case diagnostic.ProxyFailure:
		return tr("Proxy bağlantısı başarısız oldu.", "The proxy connection failed.")
	case diagnostic.PortalUncertain:
		return tr("Tek yoklamayla ağın durumu doğrulanamadı.", "This single probe could not establish the network state.")
	case diagnostic.InvalidInput:
		return tr("Girilen değer geçersiz.", "The value is invalid.")
	case diagnostic.Busy:
		return tr("Başka bir işlem çalışıyor.", "Another operation is running.")
	default:
		return tr("İşlem tamamlanamadı.", "The operation could not be completed.")
	}
}

func routeLabel(language, route string) string {
	switch route {
	case diagnostic.RouteDirect:
		return localized(language, "Doğrudan HTTP bağlantısı", "Direct HTTP connection")
	case diagnostic.RouteEnvProxy:
		return localized(language, "Ortam proxy’si", "Environment proxy")
	default:
		return localized(language, "Ağ yolu bilinmiyor", "Network path unknown")
	}
}

func displayTime(t time.Time) string {
	return t.Local().Format("02.01.2006 15:04:05")
}

func windowsLines(s string) string {
	return strings.ReplaceAll(s, "\n", "\r\n")
}

func observeNetworkChanges(ctx context.Context, changes <-chan struct{}, quietTime time.Duration, event func()) {
	var timer *time.Timer
	var settled <-chan time.Time
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-changes:
			if !ok {
				return
			}
			if timer == nil {
				timer = time.NewTimer(quietTime)
			} else {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(quietTime)
			}
			settled = timer.C
		case <-settled:
			settled = nil
			if ctx.Err() == nil {
				event()
			}
		}
	}
}
