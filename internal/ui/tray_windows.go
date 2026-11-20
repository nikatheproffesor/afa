package ui

import (
	"unsafe"
)

const (
	trayDashboard = 110
	trayCheck     = 111
	traySettings  = 112
	trayExit      = 113
)

func (a *nativeWindow) trayData(flags uint32) notifyData {
	data := notifyData{
		Window: a.hwnd, ID: 1, Flags: flags, Callback: wmTray, Icon: a.icon,
	}
	data.Size = uint32(unsafe.Sizeof(data))
	if flags&nifTip != 0 {
		putWide(data.Tip[:], "AFA · "+a.currentPortalLabel())
	}
	return data
}

func (a *nativeWindow) installTray() {
	data := a.trayData(nifMessage | nifIcon | nifTip | nifShowTip)
	added, _, _ := notifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&data)))
	a.tray = added != 0
	a.trayV4 = false
	if a.tray {
		data.Version = 4
		result, _, _ := notifyIcon.Call(nimSetVersion, uintptr(unsafe.Pointer(&data)))
		a.trayV4 = result != 0
	}
}

func (a *nativeWindow) removeTray() {
	if a.tray {
		data := a.trayData(0)
		notifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&data)))
		a.tray = false
	}
}

func (a *nativeWindow) refreshTray() {
	if a.tray {
		data := a.trayData(nifTip | nifShowTip)
		notifyIcon.Call(nimModify, uintptr(unsafe.Pointer(&data)))
	}
}

func (a *nativeWindow) showTrayNotice() {
	if !a.tray || !a.settings.Notifications {
		return
	}
	data := a.trayData(nifInfo)
	putWide(data.Title[:], windowTitle)
	putWide(data.Info[:], a.tr("Olası giriş sayfası algılandı. Bağlantı durumunu AFA’da inceleyebilirsiniz.",
		"A possible login page was detected. You can inspect the connection in AFA."))
	data.InfoFlags = 1 | 0x80 // NIIF_INFO | NIIF_RESPECT_QUIET_TIME
	notifyIcon.Call(nimModify, uintptr(unsafe.Pointer(&data)))
}

func (a *nativeWindow) trayInteraction(wparam, lparam uintptr) {
	const rightButtonUp = 0x0205
	switch uint16(lparam) {
	case ninSelect, ninKeySelect, wmLeftButtonUp:
		a.showPage(pageDashboard)
		a.showMainWindow()
	case wmContextMenu, rightButtonUp:
		p := point{X: int32(int16(wparam)), Y: int32(int16(wparam >> 16))}
		if !a.trayV4 {
			getCursorPos.Call(uintptr(unsafe.Pointer(&p)))
		}
		a.showTrayMenu(p)
	}
}

func (a *nativeWindow) showTrayMenu(p point) {
	menu, _, _ := createPopupMenu.Call()
	if menu == 0 {
		return
	}
	defer destroyMenu.Call(menu)
	add := func(flags, id uintptr, caption string) {
		appendMenu.Call(menu, flags, id, uintptr(unsafe.Pointer(wide(caption))))
	}
	add(1, 0, a.currentPortalLabel())
	add(0x800, 0, "")
	add(0, trayDashboard, a.tr("&Panoyu aç", "&Open dashboard"))
	add(boolean(a.gate.kind != ""), trayCheck, a.tr("&Bağlantıyı denetle", "&Check connection"))
	add(0, traySettings, a.tr("&Ayarlar", "&Settings"))
	add(0, trayExit, a.tr("Çı&kış", "E&xit"))
	setForegroundWindow.Call(a.hwnd)
	const returnCommand = 0x100
	const trackRightButton = 2
	result, _, _ := trackPopupMenu.Call(menu, returnCommand|trackRightButton, uintptr(p.X), uintptr(p.Y), 0, a.hwnd, 0)
	postMessage.Call(a.hwnd, 0, 0, 0)
	data := a.trayData(0)
	notifyIcon.Call(nimSetFocus, uintptr(unsafe.Pointer(&data)))
	switch result {
	case trayDashboard:
		a.showPage(pageDashboard)
		a.showMainWindow()
	case trayCheck:
		a.checkPortal(false)
	case traySettings:
		a.showPage(pageSettings)
		a.showMainWindow()
	case trayExit:
		destroyWindow.Call(a.hwnd)
	}
}
