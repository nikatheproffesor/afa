package platform

import (
	"context"
	"errors"
	"runtime"
	"syscall"
	"unsafe"
)

func openURL(ctx context.Context, address string) error {
	open, err := SystemProcedure("shell32.dll", "ShellExecuteW")
	if err != nil {
		return err
	}
	initialize, err := SystemProcedure("ole32.dll", "CoInitializeEx")
	if err != nil {
		return err
	}
	uninitialize, err := SystemProcedure("ole32.dll", "CoUninitialize")
	if err != nil {
		return err
	}
	url, err := syscall.UTF16PtrFromString(address)
	if err != nil {
		return err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	const apartment = 0x2 | 0x4 // APARTMENTTHREADED | DISABLE_OLE1DDE
	result, _, _ := initialize.Call(0, apartment)
	if int32(result) < 0 {
		return errors.New("could not initialize shell apartment")
	}
	defer uninitialize.Call()
	if err := ctx.Err(); err != nil {
		return err
	}
	result, _, _ = open.Call(0, 0, uintptr(unsafe.Pointer(url)), 0, 0, 1)
	runtime.KeepAlive(url)
	if result <= 32 {
		return errors.New("shell rejected browser handoff")
	}
	return nil
}
