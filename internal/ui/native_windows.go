package ui

import (
	"context"
	"errors"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"afa/internal/config"
	"afa/internal/network"
	"afa/internal/portal"
	"afa/internal/service"
	"afa/internal/speed"
)

const (
	windowClassName = "AFA.Desktop.Window.v1"
	windowTitle     = "AFA — Access For All"
	windowWidth     = 720
	windowHeight    = 512
)

var activeWindow *nativeWindow

var windowCallback = syscall.NewCallback(windowProcedure)

type palette struct {
	background, surface, foreground, muted, button, pressed  uintptr
	backgroundBrush, surfaceBrush, buttonBrush, pressedBrush uintptr
}

type nativeWindow struct {
	hwnd, module, icon, font, headingFont, titleFont uintptr
	dpi                                              int
	colors                                           palette
	taskbarMessage                                   uint32
	tray                                             bool
	trayV4                                           bool
	page                                             int
	children                                         []uintptr
	navStarter                                       uintptr
	input, output, status, keyboardCancel            uintptr
	action, progress, login                          uintptr
	transferred, transferTotal                       int64
	settingsTheme, settingsLanguage                  uintptr
	settingsStartup, settingsMonitor, settingsNotice uintptr
	settingsTimeout                                  uintptr
	settings                                         config.Settings
	settingsPath                                     string
	configWarning                                    bool
	controlsFailed                                   bool
	target, size                                     string
	note, speedNote, serviceNote                     string
	lastPortal                                       *portal.Result
	lastService                                      *service.Result
	lastSpeed                                        *speed.Result
	lastProbeTime, lastSuccessful                    time.Time
	observation                                      network.Observation
	monitorFailed, monitorPending                    bool
	monitorCancel                                    context.CancelFunc
	monitorGeneration                                uint64
	gate                                             taskGate
	life                                             context.Context
	close                                            context.CancelFunc
	events                                           chan func()
}

func Run() error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	mutex, _, mutexErr := createMutex.Call(0, 0, uintptr(unsafe.Pointer(wide(`Local\AFA.Desktop.v1`))))
	if mutex == 0 {
		return errors.New("desktop instance guard could not be created")
	}
	defer syscall.CloseHandle(syscall.Handle(mutex))
	if mutexErr == syscall.ERROR_ALREADY_EXISTS {
		existing, _, _ := findWindow.Call(uintptr(unsafe.Pointer(wide(windowClassName))), 0)
		if existing != 0 {
			postMessage.Call(existing, wmOpenDashboard, 0, 0)
		}
		return nil
	}

	setProcessDPIAware.Call()
	a := &nativeWindow{events: make(chan func(), 32), target: "https://example.com", size: "8", dpi: 96}
	a.settingsPath, _ = config.UserPath()
	a.settings = config.Default()
	if a.settingsPath != "" {
		var err error
		a.settings, err = config.Load(a.settingsPath)
		a.configWarning = err != nil
	}
	a.life, a.close = context.WithCancel(context.Background())
	defer a.close()
	if getDPIForSystem.Find() == nil {
		dpi, _, _ := getDPIForSystem.Call()
		if dpi > 0 {
			a.dpi = int(dpi)
		}
	}
	a.module, _, _ = getModuleHandle.Call(0)
	a.icon, _, _ = loadIcon.Call(0, 32512)
	a.font = a.makeFont(16, 400)
	a.headingFont = a.makeFont(23, 600)
	a.titleFont = a.makeFont(31, 650)
	defer a.releaseFonts()
	a.colors = a.newPalette()
	defer func() { a.colors.release() }()
	taskbarMessage, _, _ := registerWindowMessage.Call(uintptr(unsafe.Pointer(wide("TaskbarCreated"))))
	a.taskbarMessage = uint32(taskbarMessage)
	activeWindow = a
	defer func() { activeWindow = nil }()
	if err := a.createMainWindow(); err != nil {
		return err
	}
	a.showPage(pageDashboard)
	if a.controlsFailed {
		destroyWindow.Call(a.hwnd)
		return errors.New("desktop controls could not be created")
	}
	a.installTray()
	a.applyTheme()
	if !a.settings.StartInTray || !a.tray {
		showWindow.Call(a.hwnd, swShow)
		setFocus.Call(a.navStarter)
	}
	a.configureMonitoring()
	a.refreshNetworkContext()
	return a.messageLoop()
}

func ShowStartupError() {
	alert(0, windowTitle, "AFA masaüstü başlatılamadı. Ağ tanısı için afa.exe komut satırını kullanabilirsiniz.", mbOK|mbIconWarning)
}

func (a *nativeWindow) createMainWindow() error {
	cursor, _, _ := loadCursor.Call(0, 32512)
	class := windowClass{
		Style:     0x0003,
		Procedure: windowCallback,
		Module:    a.module,
		Icon:      a.icon,
		Cursor:    cursor,
		ClassName: wide(windowClassName),
		SmallIcon: a.icon,
	}
	class.Size = uint32(unsafe.Sizeof(class))
	if result, _, _ := registerClass.Call(uintptr(unsafe.Pointer(&class))); result == 0 {
		return errors.New("desktop window class could not be registered")
	}
	style := uintptr(wsCaption | wsSystemMenu | wsMinimizeBox | wsClipChildren)
	exStyle := uintptr(wsExAppWindow | wsExCtrlParent)
	size := rect{Right: int32(a.scale(windowWidth)), Bottom: int32(a.scale(windowHeight))}
	adjustWindowRect.Call(uintptr(unsafe.Pointer(&size)), style, 0, exStyle)
	const defaultPosition = 0x80000000
	a.hwnd, _, _ = createWindow.Call(exStyle, uintptr(unsafe.Pointer(class.ClassName)), uintptr(unsafe.Pointer(wide(windowTitle))), style,
		defaultPosition, defaultPosition, uintptr(size.Right-size.Left), uintptr(size.Bottom-size.Top), 0, 0, a.module, 0)
	if a.hwnd == 0 {
		return errors.New("desktop window could not be created")
	}
	return nil
}

func (a *nativeWindow) messageLoop() error {
	var m winMessage
	for {
		result, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		switch int32(result) {
		case -1:
			destroyWindow.Call(a.hwnd)
			return errors.New("desktop message queue failed")
		case 0:
			return nil
		}
		if handled, _, _ := isDialogMessage.Call(a.hwnd, uintptr(unsafe.Pointer(&m))); handled == 0 {
			translateMessage.Call(uintptr(unsafe.Pointer(&m)))
			dispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
		}
	}
}

func windowProcedure(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	a := activeWindow
	if a == nil || a.hwnd == 0 || hwnd != a.hwnd {
		return defaultProcedure(hwnd, msg, wparam, lparam)
	}
	if a.taskbarMessage != 0 && msg == a.taskbarMessage {
		a.installTray()
		if visible, _, _ := isWindowVisible.Call(hwnd); visible == 0 && !a.tray {
			a.showMainWindow()
		}
		return 0
	}
	switch msg {
	case wmClose:
		if a.tray {
			showWindow.Call(hwnd, swHide)
		} else {
			destroyWindow.Call(hwnd)
		}
		return 0
	case wmDestroy:
		a.close()
		a.gate.stop()
		a.removeTray()
		postQuitMessage.Call(0)
		return 0
	case wmWorker:
		a.applyEvents()
		return 0
	case wmOpenDashboard:
		a.showPage(pageDashboard)
		a.showMainWindow()
		return 0
	case wmTray:
		a.trayInteraction(wparam, lparam)
		return 0
	case wmCommand:
		if uint16(wparam>>16) == 0 && a.handleCommand(uint16(wparam)) {
			return 0
		}
	case wmSettingChange:
		a.applyTheme()
	case wmEraseBkgnd:
		var area rect
		getClientRect.Call(hwnd, uintptr(unsafe.Pointer(&area)))
		fillRect.Call(wparam, uintptr(unsafe.Pointer(&area)), a.colors.backgroundBrush)
		return 1
	case wmCtlColorEdit, wmCtlColorList, wmCtlColorStatic, wmCtlColorBtn:
		return a.colorControl(msg, wparam, lparam)
	case wmDrawItem:
		if item, ok := readDrawItem(lparam); ok && a.drawButton(&item) {
			return 1
		}
	}
	return defaultProcedure(hwnd, msg, wparam, lparam)
}

func defaultProcedure(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	result, _, _ := defWindowProc.Call(hwnd, uintptr(msg), wparam, lparam)
	return result
}

func (a *nativeWindow) showMainWindow() {
	showWindow.Call(a.hwnd, swRestore)
	setForegroundWindow.Call(a.hwnd)
	setFocus.Call(a.navStarter)
}

func (a *nativeWindow) scale(value int) int {
	return value * a.dpi / 96
}

func (a *nativeWindow) makeFont(size, weight int) uintptr {
	handle, _, _ := createFont.Call(uintptr(-a.scale(size)), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0,
		uintptr(unsafe.Pointer(wide("Segoe UI"))))
	return handle
}

func (a *nativeWindow) releaseFonts() {
	for _, font := range []uintptr{a.font, a.headingFont, a.titleFont} {
		if font != 0 {
			deleteObject.Call(font)
		}
	}
}

func (a *nativeWindow) newPalette() palette {
	colors := palette{
		background: rgb(242, 246, 250), surface: rgb(255, 255, 255), foreground: rgb(21, 38, 55), muted: rgb(72, 92, 111),
		button: rgb(216, 232, 246), pressed: rgb(184, 215, 240),
	}
	if a.settings.Theme == "dark" {
		colors = palette{
			background: rgb(20, 27, 36), surface: rgb(29, 42, 55), foreground: rgb(239, 247, 255), muted: rgb(181, 203, 224),
			button: rgb(43, 64, 84), pressed: rgb(62, 84, 106),
		}
	}
	var contrast highContrast
	contrast.Size = uint32(unsafe.Sizeof(contrast))
	systemParametersInfo.Call(0x0042, uintptr(contrast.Size), uintptr(unsafe.Pointer(&contrast)), 0)
	if contrast.Flags&1 != 0 {
		colors.background, _, _ = getSystemColor.Call(5)
		colors.surface = colors.background
		colors.foreground, _, _ = getSystemColor.Call(8)
		colors.muted = colors.foreground
		colors.button = colors.background
		colors.pressed = colors.background
	}
	colors.backgroundBrush, _, _ = createBrush.Call(colors.background)
	colors.surfaceBrush, _, _ = createBrush.Call(colors.surface)
	colors.buttonBrush, _, _ = createBrush.Call(colors.button)
	colors.pressedBrush, _, _ = createBrush.Call(colors.pressed)
	return colors
}

func (p palette) release() {
	for _, brush := range []uintptr{p.backgroundBrush, p.surfaceBrush, p.buttonBrush, p.pressedBrush} {
		if brush != 0 {
			deleteObject.Call(brush)
		}
	}
}

func (a *nativeWindow) applyTheme() {
	previous := a.colors
	a.colors = a.newPalette()
	previous.release()
	if setDWMWindowAttribute.Find() == nil {
		dark := uint32(boolean(a.settings.Theme == "dark"))
		setDWMWindowAttribute.Call(a.hwnd, 20, uintptr(unsafe.Pointer(&dark)), unsafe.Sizeof(dark))
	}
	invalidateRect.Call(a.hwnd, 0, 1)
	for _, child := range a.children {
		invalidateRect.Call(child, 0, 1)
	}
}

func (a *nativeWindow) colorControl(msg uint32, device, control uintptr) uintptr {
	setTextColor.Call(device, a.colors.foreground)
	if msg == wmCtlColorEdit || msg == wmCtlColorList || control == a.input || control == a.output || control == a.settingsTimeout {
		setBackgroundMode.Call(device, 2)
		setBackgroundColor.Call(device, a.colors.surface)
		return a.colors.surfaceBrush
	}
	setBackgroundMode.Call(device, 1)
	return a.colors.backgroundBrush
}

func (a *nativeWindow) drawButton(item *drawItem) bool {
	if item.ControlType != odtButton {
		return false
	}
	state, _, _ := saveDevice.Call(item.DC)
	if state == 0 {
		return false
	}
	defer restoreDevice.Call(item.DC, state)
	brush := a.colors.buttonBrush
	color := a.colors.foreground
	if item.State&odsSelected != 0 {
		brush = a.colors.pressedBrush
	}
	if item.State&odsDisabled != 0 {
		brush, color = a.colors.backgroundBrush, a.colors.muted
	}
	fillRect.Call(item.DC, uintptr(unsafe.Pointer(&item.Rect)), brush)
	font := a.font
	if font == 0 {
		font, _, _ = getStockObject.Call(17)
	}
	selectObject.Call(item.DC, font)
	setTextColor.Call(item.DC, color)
	setBackgroundMode.Call(item.DC, 1)
	caption := text(item.Window)
	const centeredLine = 1 | 4 | 0x20
	drawText.Call(item.DC, uintptr(unsafe.Pointer(wide(caption))), ^uintptr(0), uintptr(unsafe.Pointer(&item.Rect)), centeredLine)
	if item.State&odsFocus != 0 {
		inside := item.Rect
		inside.Left += 3
		inside.Top += 3
		inside.Right -= 3
		inside.Bottom -= 3
		drawFocusRect.Call(item.DC, uintptr(unsafe.Pointer(&inside)))
	}
	return true
}

func rgb(red, green, blue uintptr) uintptr {
	return red | green<<8 | blue<<16
}

func (a *nativeWindow) post(action func()) {
	if a.life.Err() != nil {
		return
	}
	select {
	case <-a.life.Done():
		return
	case a.events <- action:
		postMessage.Call(a.hwnd, wmWorker, 0, 0)
	}
}

func (a *nativeWindow) applyEvents() {
	for {
		select {
		case action := <-a.events:
			action()
		default:
			return
		}
	}
}
