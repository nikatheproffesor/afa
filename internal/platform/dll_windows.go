package platform

import (
	"errors"
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"
)

var systemDirectory = sync.OnceValues(func() (string, error) {
	kernel := syscall.NewLazyDLL("kernel32.dll")
	get := kernel.NewProc("GetSystemDirectoryW")
	if err := get.Find(); err != nil {
		return "", err
	}
	var path [260]uint16
	length, _, _ := get.Call(uintptr(unsafe.Pointer(&path[0])), uintptr(len(path)))
	if length == 0 || length >= uintptr(len(path)) {
		return "", errors.New("system directory unavailable")
	}
	return syscall.UTF16ToString(path[:length]), nil
})

var systemLibraries sync.Map

// SystemProcedure resolves a built-in Windows API from the system directory.
func SystemProcedure(dll, name string) (*syscall.LazyProc, error) {
	dir, err := systemDirectory()
	if err != nil {
		return nil, err
	}
	if filepath.Base(dll) != dll {
		return nil, errors.New("invalid system library name")
	}
	path := filepath.Join(dir, dll)
	library, _ := systemLibraries.LoadOrStore(path, syscall.NewLazyDLL(path))
	procedure := library.(*syscall.LazyDLL).NewProc(name)
	return procedure, procedure.Find()
}
