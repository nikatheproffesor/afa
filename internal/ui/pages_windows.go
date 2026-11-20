package ui

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"unsafe"

	"afa/internal/buildinfo"
	"afa/internal/config"
)

const (
	pageDashboard = iota
	pageDiagnostics
	pageSpeed
	pageSettings
	pageAbout
)

const (
	idPageBase     = 200
	idCheck        = 220
	idLogin        = 221
	idService      = 222
	idSpeed        = 223
	idCancel       = 224
	idSaveSettings = 225
	idReset        = 226
	idExit         = 227
)

func (a *nativeWindow) tr(turkish, english string) string {
	return localized(a.settings.Language, turkish, english)
}

func (a *nativeWindow) control(class, caption string, extra, style uintptr, id, x, y, width, height int, font uintptr) uintptr {
	hwnd, _, _ := createWindow.Call(extra, uintptr(unsafe.Pointer(wide(class))), uintptr(unsafe.Pointer(wide(caption))),
		wsChild|wsVisible|style, uintptr(a.scale(x)), uintptr(a.scale(y)), uintptr(a.scale(width)), uintptr(a.scale(height)),
		a.hwnd, uintptr(id), a.module, 0)
	if hwnd == 0 {
		a.controlsFailed = true
		return 0
	}
	a.children = append(a.children, hwnd)
	if font != 0 {
		sendMessage.Call(hwnd, wmSetFont, font, 1)
	}
	return hwnd
}

func (a *nativeWindow) label(caption string, x, y, width, height int, font uintptr) uintptr {
	return a.control("STATIC", caption, 0, 0, 0, x, y, width, height, font)
}

func (a *nativeWindow) button(id int, caption string, x, y, width int) uintptr {
	return a.control("BUTTON", caption, 0, wsTabStop|buttonOwnerDraw, id, x, y, width, 34, a.font)
}

func (a *nativeWindow) checkbox(caption string, value bool, x, y, width int) uintptr {
	hwnd := a.control("BUTTON", caption, 0, wsTabStop|buttonAutoCheck, 0, x, y, width, 27, a.font)
	sendMessage.Call(hwnd, buttonSetCheck, boolean(value), 0)
	return hwnd
}

func (a *nativeWindow) combo(options []string, current, x, y, width int) uintptr {
	hwnd := a.control("COMBOBOX", "", 0, wsTabStop|wsVScroll|comboDropList, 0, x, y, width, 120, a.font)
	for _, option := range options {
		sendMessage.Call(hwnd, comboAddString, 0, uintptr(unsafe.Pointer(wide(option))))
	}
	sendMessage.Call(hwnd, comboSet, uintptr(current), 0)
	return hwnd
}

func (a *nativeWindow) entry(value string, limit, x, y, width int) uintptr {
	hwnd := a.control("EDIT", value, wsExClientEdge, wsTabStop|editAutoHScroll, 0, x, y, width, 30, a.font)
	sendMessage.Call(hwnd, editSetLimit, uintptr(limit), 0)
	return hwnd
}

func (a *nativeWindow) readout(y, height int) uintptr {
	return a.control("EDIT", "", wsExClientEdge, wsTabStop|wsVScroll|editMultiline|editAutoVScroll|editReadOnly,
		0, 24, y, 672, height, a.font)
}

func (a *nativeWindow) clearPage() {
	if a.input != 0 {
		switch a.page {
		case pageDiagnostics:
			a.target = text(a.input)
		case pageSpeed:
			a.size = text(a.input)
		}
	}
	for _, child := range a.children {
		destroyWindow.Call(child)
	}
	a.children = a.children[:0]
	a.input, a.output, a.status, a.action, a.login, a.keyboardCancel, a.progress = 0, 0, 0, 0, 0, 0, 0
	a.settingsTheme, a.settingsLanguage, a.settingsTimeout = 0, 0, 0
	// The window remains active if rendering one page fails.
	a.controlsFailed = false
}

func (a *nativeWindow) showPage(page int) {
	a.clearPage()
	a.page = page
	a.label(windowTitle, 24, 16, 672, 40, a.titleFont)
	a.label(a.tr("Bağlantı, giriş ve ağ performansı", "Connectivity, sign-in, and network performance"), 24, 61, 672, 24, a.font)
	sections := []string{
		a.tr("&Pano", "&Dashboard"), a.tr("&Tanı", "D&iagnostics"), a.tr("&Hız", "&Speed test"),
		a.tr("&Ayarlar", "&Settings"), a.tr("Hakkı&nda", "A&bout"),
	}
	for index, title := range sections {
		hwnd := a.button(idPageBase+index, title, 24+136*index, 101, 128)
		if page == index {
			a.navStarter = hwnd
		}
	}
	switch page {
	case pageDashboard:
		a.dashboardPage()
	case pageDiagnostics:
		a.diagnosticsPage()
	case pageSpeed:
		a.speedPage()
	case pageSettings:
		a.preferencesPage()
	case pageAbout:
		a.aboutPage()
	}
	a.render()
	invalidateRect.Call(a.hwnd, 0, 1)
	if visible, _, _ := isWindowVisible.Call(a.hwnd); visible != 0 {
		setFocus.Call(a.navStarter)
	}
}

func (a *nativeWindow) titleAndDescription(title, description string) {
	a.label(title, 24, 153, 672, 32, a.headingFont)
	a.label(description, 24, 190, 672, 41, a.font)
}

func (a *nativeWindow) dashboardPage() {
	a.titleAndDescription(a.tr("Bağlantı panosu", "Connection dashboard"),
		a.tr("Son gözlemleri inceleyin. Portal yoklaması yalnızca sınama adresinin yanıtını gösterir.",
			"Review the latest observations. The portal probe reports the response from a single endpoint."))
	a.output = a.readout(234, 162)
	a.action = a.button(idCheck, a.tr("Bağlantıyı &denetle", "&Check connection"), 24, 414, 184)
	a.login = a.button(idLogin, a.tr("Ağ &girişini aç", "Open network &login"), 218, 414, 154)
	a.keyboardCancel = a.button(idCancel, a.tr("İ&ptal", "Ca&ncel"), 382, 414, 96)
	a.button(idExit, a.tr("Çı&kış", "E&xit"), 588, 414, 108)
	a.status = a.label("", 24, 464, 672, 46, a.font)
}

func (a *nativeWindow) diagnosticsPage() {
	a.titleAndDescription(a.tr("Servis tanısı", "Service diagnostics"),
		a.tr("Hedefin DNS, TCP ve TLS aşamalarını ölçün. Oyun veya uygulama oturumları ayrıca sınanır.",
			"Measure DNS, TCP, and TLS for a destination. Application and game sessions require separate tests."))
	a.label(a.tr("&HTTPS adresi", "&HTTPS address"), 24, 236, 122, 27, a.font)
	a.input = a.entry(a.target, 2048, 152, 231, 544)
	a.output = a.readout(278, 126)
	a.action = a.button(idService, a.tr("&Tanıyı çalıştır", "&Run diagnostics"), 24, 422, 172)
	a.keyboardCancel = a.button(idCancel, a.tr("İ&ptal", "Ca&ncel"), 206, 422, 110)
	a.status = a.label("", 24, 467, 672, 42, a.font)
}

func (a *nativeWindow) speedPage() {
	a.titleAndDescription(a.tr("İndirme ölçümü", "Download measurement"),
		a.tr("Cloudflare’a tek, kısa bir HTTPS isteği yapılır. Sonuç, bağlantı kurulumunu da içerir.",
			"A single short HTTPS request to Cloudflare measures the download including setup time."))
	a.label(a.tr("&Boyut (1–8 MiB)", "&Size (1–8 MiB)"), 24, 239, 160, 27, a.font)
	a.input = a.entry(a.size, 2, 194, 234, 75)
	a.action = a.button(idSpeed, a.tr("&Ölçümü başlat", "&Start measurement"), 279, 232, 172)
	a.keyboardCancel = a.button(idCancel, a.tr("İ&ptal", "Ca&ncel"), 461, 232, 110)
	a.progress = a.label("", 24, 283, 672, 27, a.font)
	a.output = a.readout(319, 130)
	a.status = a.label("", 24, 465, 672, 42, a.font)
}

func (a *nativeWindow) preferencesPage() {
	a.titleAndDescription(a.tr("Ayarlar", "Settings"),
		a.tr("Tercihler bu cihazda, kullanıcı hesabınız içinde saklanır.", "Preferences are stored on this device under your user account."))
	a.label(a.tr("&Tema", "&Theme"), 24, 235, 100, 25, a.font)
	a.settingsTheme = a.combo([]string{a.tr("Açık", "Light"), a.tr("Koyu", "Dark")}, int(boolean(a.settings.Theme == "dark")), 128, 231, 176)
	a.label(a.tr("&Dil", "&Language"), 328, 235, 115, 25, a.font)
	a.settingsLanguage = a.combo([]string{"Türkçe", "English"}, int(boolean(a.settings.Language == "en")), 448, 231, 178)
	a.label(a.tr("Tanı &süresi (3–60 sn)", "Diagnostic &timeout (3–60 s)"), 24, 278, 230, 25, a.font)
	a.settingsTimeout = a.entry(strconv.Itoa(a.settings.TimeoutSeconds), 2, 254, 273, 61)
	a.settingsStartup = a.checkbox(a.tr("&Tepside başlat", "Start in t&ray"), a.settings.StartInTray, 338, 273, 330)
	a.settingsMonitor = a.checkbox(a.tr("Ağ değişikliklerinde &yokla", "&Probe after network changes"), a.settings.Monitoring, 24, 313, 320)
	a.settingsNotice = a.checkbox(a.tr("&Portal bildirimi göster", "Show portal &notifications"), a.settings.Notifications, 352, 313, 320)
	a.label(a.tr("AFA telemetri toplamaz. İzleme açıksa ağ değişiklikleri portal yoklaması başlatabilir. Wi-Fi adları ve kimlik bilgileri kaydedilmez.",
		"AFA collects no telemetry. Monitoring may start a portal probe following network changes. Wi-Fi names and credentials are not stored."),
		24, 356, 672, 50, a.font)
	a.button(idSaveSettings, a.tr("&Kaydet", "&Save"), 24, 425, 118)
	a.button(idReset, a.tr("Ayarları sıfı&rla", "&Reset settings"), 152, 425, 184)
	a.status = a.label("", 24, 470, 672, 37, a.font)
}

func (a *nativeWindow) aboutPage() {
	a.titleAndDescription(a.tr("AFA hakkında", "About AFA"),
		a.tr("Kullanıcı tarafından başlatılan bağlantı, servis ve performans tanıları.",
			"User-initiated connectivity, service, and performance diagnostics."))
	a.output = a.readout(232, 194)
	setText(a.output, windowsLines(strings.Join([]string{
		fmt.Sprintf("%s %s", a.tr("Sürüm", "Version"), buildinfo.Version),
		fmt.Sprintf("Commit %s", buildinfo.Commit),
		fmt.Sprintf("Go %s", strings.TrimPrefix(runtime.Version(), "go")),
		a.tr("Lisans  MIT", "License  MIT"),
		a.tr("Üçüncü taraf bildirimleri dağıtım paketindedir.", "Third-party notices are included in the distribution package."),
		a.tr("Bu derlemede GitHub adresi yapılandırılmadı. Güvenlik bildirimi için SECURITY.md dosyasına bakın.", "This build has no configured GitHub URL. See SECURITY.md for reporting."),
	}, "\n")))
}

func (a *nativeWindow) render() {
	if a.action != 0 {
		enableWindow.Call(a.action, boolean(a.gate.kind == ""))
	}
	if a.login != 0 {
		enableWindow.Call(a.login, boolean(a.gate.kind == ""))
	}
	if a.keyboardCancel != 0 {
		enableWindow.Call(a.keyboardCancel, boolean(a.gate.kind != ""))
	}
	switch a.page {
	case pageDashboard:
		a.renderDashboard()
	case pageDiagnostics:
		a.renderService()
	case pageSpeed:
		a.renderSpeed()
	case pageSettings:
		if a.configWarning && a.note == "" {
			setText(a.status, a.tr("Ayarlar okunamadı. Varsayılanlar kullanılıyor.", "Settings could not be read. Defaults are in use."))
		} else {
			setText(a.status, a.note)
		}
	}
}

func (a *nativeWindow) currentPortalLabel() string {
	state := "unknown"
	if a.lastPortal != nil {
		state = a.lastPortal.State
	}
	return portalLabel(a.settings.Language, state)
}

func (a *nativeWindow) renderDashboard() {
	lines := []string{a.currentPortalLabel()}
	if a.lastPortal != nil && a.lastPortal.HTTPStatus != 0 {
		lines = append(lines, fmt.Sprintf("HTTP %d", a.lastPortal.HTTPStatus))
	}
	if !a.lastProbeTime.IsZero() {
		lines = append(lines, a.tr("Son yoklama", "Last probe")+"  "+displayTime(a.lastProbeTime))
	}
	if !a.lastSuccessful.IsZero() {
		lines = append(lines, a.tr("Son başarılı tanı", "Last successful diagnostic")+"  "+displayTime(a.lastSuccessful))
	}
	if a.observation.InterfaceIndex != 0 {
		lines = append(lines, fmt.Sprintf("%s  %s · #%d", a.tr("Örnek rota arayüzü", "Example route interface"),
			a.connectionType(), a.observation.InterfaceIndex))
	}
	if a.lastService != nil {
		lines = append(lines, fmt.Sprintf("%s · DNS %s · TLS %s", a.lastService.Host,
			a.stageLabel(a.lastService.DNS), a.stageLabel(a.lastService.TLS)))
	}
	if a.lastSpeed != nil {
		lines = append(lines, fmt.Sprintf("%s  %.2f Mbps", a.tr("Son ölçüm", "Recent measurement"), a.lastSpeed.Mbps))
	}
	lines = append(lines, "AFA "+buildinfo.Version)
	setText(a.output, windowsLines(strings.Join(lines, "\n")))
	note := a.note
	if note == "" && a.monitorFailed {
		note = a.tr("Ağ değişikliği izlemesi başlatılamadı.", "Network change monitoring could not be started.")
	}
	if note == "" && a.configWarning {
		note = a.tr("Ayarlar okunamadı. Varsayılanlar kullanılıyor.", "Settings could not be read. Defaults are in use.")
	}
	setText(a.status, note)
}

func (a *nativeWindow) connectionType() string {
	switch a.observation.ConnectionType {
	case "wifi":
		return "Wi-Fi"
	case "ethernet":
		return "Ethernet"
	case "tunnel":
		return a.tr("Tünel", "Tunnel")
	case "cellular":
		return a.tr("Mobil", "Cellular")
	default:
		return a.tr("Türü bilinmiyor", "Unknown type")
	}
}

func (a *nativeWindow) stageLabel(value string) string {
	switch value {
	case "ok":
		return a.tr("başarılı", "passed")
	case "failed":
		return a.tr("başarısız", "failed")
	default:
		return a.tr("ölçülmedi", "not measured")
	}
}

func (a *nativeWindow) renderService() {
	setText(a.status, a.serviceNote)
	result := a.lastService
	if result == nil {
		setText(a.output, a.tr("Henüz bir servis tanısı yok.", "No service diagnostic has been run."))
		return
	}
	stage := func(name, value string, duration *int64) string {
		line := name + "  " + a.stageLabel(value)
		if value != "skipped" && duration != nil {
			line += fmt.Sprintf(" (%d ms)", *duration)
		}
		return line
	}
	lines := []string{result.Host + ":" + result.Port,
		stage("DNS", result.DNS, result.DNSMillis), stage("TCP", result.TCP, result.TCPMillis), stage("TLS", result.TLS, result.TLSMillis)}
	if len(result.AddressFamilies) != 0 {
		lines = append(lines, a.tr("Adres aileleri", "Address families")+"  "+strings.Join(result.AddressFamilies, ", "))
	}
	if result.ErrorCode != "" {
		lines = append(lines, errorLabel(a.settings.Language, result.ErrorCode))
	}
	setText(a.output, windowsLines(strings.Join(lines, "\n")))
}

func (a *nativeWindow) renderSpeed() {
	setText(a.status, a.speedNote)
	if a.transferTotal > 0 {
		setText(a.progress, fmt.Sprintf("%s  %.0f%% · %.2f / %.0f MiB", a.tr("İndirilen", "Downloaded"),
			float64(a.transferred)*100/float64(a.transferTotal), float64(a.transferred)/(1<<20), float64(a.transferTotal)/(1<<20)))
	} else {
		setText(a.progress, "")
	}
	result := a.lastSpeed
	if result == nil {
		setText(a.output, a.tr("Henüz bir indirme ölçümü yok. Test, en çok seçilen boyutta gövde ister.",
			"No download measurement has been run. The test requests a body up to the selected size."))
		return
	}
	transfer := a.tr("Gövde aktarımı  Ölçüm çözünürlüğünün altında", "Body transfer  Below timer resolution")
	if result.TransferMbps != nil && result.TransferMS != nil {
		transfer = fmt.Sprintf("%s  %.2f Mbps · %d ms", a.tr("Gövde aktarımı", "Body transfer"), *result.TransferMbps, *result.TransferMS)
	}
	lines := []string{fmt.Sprintf("%s  %.2f Mbps", a.tr("Toplam indirme", "End-to-end download"), result.Mbps),
		transfer,
		fmt.Sprintf("%s  %s · %.0f MiB", a.tr("Uç nokta", "Endpoint"), result.Host, float64(result.Bytes)/(1<<20)),
		fmt.Sprintf("%s  %d ms · %s  %d ms", a.tr("Toplam", "Total"), result.DurationMS,
			a.tr("HTTP başlıkları", "HTTP headers"), result.HeadersMS),
		routeLabel(a.settings.Language, result.Route),
		a.tr("Tek kısa aktarım. Bağlantı ve sunucu koşulları sonucu etkiler.", "One short transfer. Connection and server conditions affect the result.")}
	setText(a.output, windowsLines(strings.Join(lines, "\n")))
}

func (a *nativeWindow) handleCommand(id uint16) bool {
	if id >= idPageBase && id <= idPageBase+pageAbout {
		a.showPage(int(id) - idPageBase)
		return true
	}
	switch id {
	case idCheck:
		a.checkPortal(false)
	case idLogin:
		a.openPortal()
	case idService:
		a.runService()
	case idSpeed:
		a.runSpeed()
	case idCancel, 2:
		a.cancelOperation()
	case idSaveSettings:
		a.saveSettings(false)
	case idReset:
		a.saveSettings(true)
	case idExit:
		destroyWindow.Call(a.hwnd)
	default:
		return false
	}
	return true
}

func (a *nativeWindow) saveSettings(reset bool) {
	settings := config.Default()
	if reset {
		confirmation := alert(a.hwnd, windowTitle, a.tr("AFA ayarları varsayılan değerlere döndürülsün mü?", "Reset AFA settings to their defaults?"),
			mbOKCancel|mbIconWarning|mbDefaultButton2)
		if confirmation != idOK {
			return
		}
	} else {
		if selected(a.settingsTheme) == 1 {
			settings.Theme = "dark"
		}
		if selected(a.settingsLanguage) == 1 {
			settings.Language = "en"
		}
		settings.StartInTray = checked(a.settingsStartup)
		settings.Monitoring = checked(a.settingsMonitor)
		settings.Notifications = checked(a.settingsNotice)
		settings.TimeoutSeconds, _ = strconv.Atoi(strings.TrimSpace(text(a.settingsTimeout)))
	}
	if settings.Validate() != nil {
		a.note = a.tr("Tanı zaman aşımı 3 ile 60 saniye arasında olmalı.", "Diagnostic timeout must be between 3 and 60 seconds.")
		a.render()
		return
	}
	if a.settingsPath == "" || config.Save(a.settingsPath, settings) != nil {
		a.note = a.tr("Ayarlar kaydedilemedi.", "Settings could not be saved.")
		a.render()
		return
	}
	changeMonitoring := a.settings.Monitoring != settings.Monitoring
	a.settings = settings
	a.configWarning = false
	a.note = a.tr("Ayarlar kaydedildi.", "Settings saved.")
	a.applyTheme()
	a.showPage(pageSettings)
	a.refreshTray()
	if changeMonitoring {
		a.configureMonitoring()
	}
}
