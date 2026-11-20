package ui

import (
	"path/filepath"
	"syscall"
	"unsafe"
)

var kernel = syscall.NewLazyDLL("kernel32.dll")

// Resolve Windows libraries from the system directory. A portable AFA copy
// can be launched from a folder with unrelated DLLs.
func systemDLL(name string) *syscall.LazyDLL {
	var directory [32768]uint16
	length, _, _ := kernel.NewProc("GetSystemDirectoryW").Call(uintptr(unsafe.Pointer(&directory[0])), uintptr(len(directory)))
	if length == 0 || length >= uintptr(len(directory)) {
		panic("Windows system directory is unavailable")
	}
	return syscall.NewLazyDLL(filepath.Join(syscall.UTF16ToString(directory[:length]), name))
}

var (
	user  = systemDLL("user32.dll")
	gdi   = systemDLL("gdi32.dll")
	shell = systemDLL("shell32.dll")
	dwm   = systemDLL("dwmapi.dll")

	getModuleHandle       = kernel.NewProc("GetModuleHandleW")
	createMutex           = kernel.NewProc("CreateMutexW")
	readProcessMemory     = kernel.NewProc("ReadProcessMemory")
	registerClass         = user.NewProc("RegisterClassExW")
	registerWindowMessage = user.NewProc("RegisterWindowMessageW")
	createWindow          = user.NewProc("CreateWindowExW")
	defWindowProc         = user.NewProc("DefWindowProcW")
	destroyWindow         = user.NewProc("DestroyWindow")
	findWindow            = user.NewProc("FindWindowW")
	showWindow            = user.NewProc("ShowWindow")
	isWindowVisible       = user.NewProc("IsWindowVisible")
	setForegroundWindow   = user.NewProc("SetForegroundWindow")
	getClientRect         = user.NewProc("GetClientRect")
	getCursorPos          = user.NewProc("GetCursorPos")
	adjustWindowRect      = user.NewProc("AdjustWindowRectEx")
	getMessage            = user.NewProc("GetMessageW")
	isDialogMessage       = user.NewProc("IsDialogMessageW")
	translateMessage      = user.NewProc("TranslateMessage")
	dispatchMessage       = user.NewProc("DispatchMessageW")
	postMessage           = user.NewProc("PostMessageW")
	postQuitMessage       = user.NewProc("PostQuitMessage")
	sendMessage           = user.NewProc("SendMessageW")
	getWindowTextLength   = user.NewProc("GetWindowTextLengthW")
	getWindowText         = user.NewProc("GetWindowTextW")
	setWindowText         = user.NewProc("SetWindowTextW")
	enableWindow          = user.NewProc("EnableWindow")
	setFocus              = user.NewProc("SetFocus")
	invalidateRect        = user.NewProc("InvalidateRect")
	loadIcon              = user.NewProc("LoadIconW")
	loadCursor            = user.NewProc("LoadCursorW")
	messageBox            = user.NewProc("MessageBoxW")
	setProcessDPIAware    = user.NewProc("SetProcessDPIAware")
	getDPIForSystem       = user.NewProc("GetDpiForSystem")
	systemParametersInfo  = user.NewProc("SystemParametersInfoW")
	getSystemColor        = user.NewProc("GetSysColor")
	createPopupMenu       = user.NewProc("CreatePopupMenu")
	appendMenu            = user.NewProc("AppendMenuW")
	trackPopupMenu        = user.NewProc("TrackPopupMenu")
	destroyMenu           = user.NewProc("DestroyMenu")
	fillRect              = user.NewProc("FillRect")
	drawText              = user.NewProc("DrawTextW")
	drawFocusRect         = user.NewProc("DrawFocusRect")
	getStockObject        = gdi.NewProc("GetStockObject")
	createFont            = gdi.NewProc("CreateFontW")
	createBrush           = gdi.NewProc("CreateSolidBrush")
	deleteObject          = gdi.NewProc("DeleteObject")
	selectObject          = gdi.NewProc("SelectObject")
	saveDevice            = gdi.NewProc("SaveDC")
	restoreDevice         = gdi.NewProc("RestoreDC")
	setTextColor          = gdi.NewProc("SetTextColor")
	setBackgroundColor    = gdi.NewProc("SetBkColor")
	setBackgroundMode     = gdi.NewProc("SetBkMode")
	notifyIcon            = shell.NewProc("Shell_NotifyIconW")
	setDWMWindowAttribute = dwm.NewProc("DwmSetWindowAttribute")
)

const (
	wmDestroy        = 0x0002
	wmClose          = 0x0010
	wmEraseBkgnd     = 0x0014
	wmSettingChange  = 0x001A
	wmDrawItem       = 0x002B
	wmSetFont        = 0x0030
	wmContextMenu    = 0x007B
	wmCommand        = 0x0111
	wmCtlColorEdit   = 0x0133
	wmCtlColorList   = 0x0134
	wmCtlColorBtn    = 0x0135
	wmCtlColorStatic = 0x0138
	wmLeftButtonUp   = 0x0202
	wmApp            = 0x8000
	wmWorker         = wmApp + 1
	wmTray           = wmApp + 2
	wmOpenDashboard  = wmApp + 3
	ninSelect        = 0x0400
	ninKeySelect     = 0x0401

	wsChild         = 0x40000000
	wsVisible       = 0x10000000
	wsClipChildren  = 0x02000000
	wsCaption       = 0x00C00000
	wsSystemMenu    = 0x00080000
	wsMinimizeBox   = 0x00020000
	wsTabStop       = 0x00010000
	wsBorder        = 0x00800000
	wsVScroll       = 0x00200000
	wsExCtrlParent  = 0x00010000
	wsExClientEdge  = 0x00000200
	wsExAppWindow   = 0x00040000
	buttonOwnerDraw = 0x0000000B
	buttonAutoCheck = 0x00000003
	buttonGetCheck  = 0x00F0
	buttonSetCheck  = 0x00F1
	editAutoHScroll = 0x0080
	editMultiline   = 0x0004
	editAutoVScroll = 0x0040
	editReadOnly    = 0x0800
	editSetLimit    = 0x00C5
	comboDropList   = 0x0003
	comboAddString  = 0x0143
	comboGet        = 0x0147
	comboSet        = 0x014E

	swHide            = 0
	swShow            = 5
	swRestore         = 9
	mbOK              = 0
	mbOKCancel        = 1
	mbIconWarning     = 0x30
	mbIconInformation = 0x40
	mbDefaultButton2  = 0x100
	idOK              = 1

	nimAdd        = 0
	nimModify     = 1
	nimDelete     = 2
	nimSetFocus   = 3
	nimSetVersion = 4
	nifMessage    = 1
	nifIcon       = 2
	nifTip        = 4
	nifInfo       = 0x10
	nifShowTip    = 0x80

	odtButton   = 4
	odsSelected = 1
	odsDisabled = 4
	odsFocus    = 0x10
)

type point struct{ X, Y int32 }
type rect struct{ Left, Top, Right, Bottom int32 }

type winMessage struct {
	Window  uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pos     point
	Private uint32
}

type windowClass struct {
	Size        uint32
	Style       uint32
	Procedure   uintptr
	ClassExtra  int32
	WindowExtra int32
	Module      uintptr
	Icon        uintptr
	Cursor      uintptr
	Background  uintptr
	MenuName    *uint16
	ClassName   *uint16
	SmallIcon   uintptr
}

type notifyData struct {
	Size      uint32
	Window    uintptr
	ID        uint32
	Flags     uint32
	Callback  uint32
	Icon      uintptr
	Tip       [128]uint16
	State     uint32
	StateMask uint32
	Info      [256]uint16
	Version   uint32
	Title     [64]uint16
	InfoFlags uint32
	GUID      [16]byte
	Balloon   uintptr
}

type drawItem struct {
	ControlType uint32
	ID          uint32
	Item        uint32
	Action      uint32
	State       uint32
	Window      uintptr
	DC          uintptr
	Rect        rect
	Data        uintptr
}

type highContrast struct {
	Size   uint32
	Flags  uint32
	Scheme *uint16
}

func wide(value string) *uint16 {
	ptr, _ := syscall.UTF16PtrFromString(value)
	return ptr
}

func putWide(dst []uint16, value string) {
	clear(dst)
	units, err := syscall.UTF16FromString(value)
	if err == nil {
		copy(dst[:len(dst)-1], units)
	}
}

func alert(hwnd uintptr, title, body string, style uintptr) uintptr {
	result, _, _ := messageBox.Call(hwnd, uintptr(unsafe.Pointer(wide(body))), uintptr(unsafe.Pointer(wide(title))), style)
	return result
}

func text(hwnd uintptr) string {
	length, _, _ := getWindowTextLength.Call(hwnd)
	if length == 0 || length > 4096 {
		return ""
	}
	units := make([]uint16, length+1)
	getWindowText.Call(hwnd, uintptr(unsafe.Pointer(&units[0])), uintptr(len(units)))
	return syscall.UTF16ToString(units)
}

func setText(hwnd uintptr, value string) {
	setWindowText.Call(hwnd, uintptr(unsafe.Pointer(wide(value))))
}

func checked(hwnd uintptr) bool {
	value, _, _ := sendMessage.Call(hwnd, buttonGetCheck, 0, 0)
	return value == 1
}

func selected(hwnd uintptr) uintptr {
	value, _, _ := sendMessage.Call(hwnd, comboGet, 0, 0)
	return value
}

func boolean(value bool) uintptr {
	if value {
		return 1
	}
	return 0
}

func readDrawItem(address uintptr) (drawItem, bool) {
	var item drawItem
	if address == 0 {
		return item, false
	}
	// WM_DRAWITEM provides an address in this process. ReadProcessMemory with
	// the current-process pseudo-handle also rejects invalid callback addresses.
	ok, _, _ := readProcessMemory.Call(^uintptr(0), address, uintptr(unsafe.Pointer(&item)), unsafe.Sizeof(item), 0)
	return item, ok != 0
}
