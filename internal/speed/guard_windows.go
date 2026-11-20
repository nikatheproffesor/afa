package speed

import (
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"afa/internal/diagnostic"
	"afa/internal/platform"
)

var createMeasurementMarker = sync.OnceValues(func() (*syscall.LazyProc, error) {
	return platform.SystemProcedure("kernel32.dll", "CreateMutexW")
})

func reserveMeasurement() (func(), error) {
	create, err := createMeasurementMarker()
	if err != nil {
		return nil, diagnostic.Error{Code: diagnostic.ExternalFailure}
	}
	name, _ := syscall.UTF16PtrFromString(`Local\AFA.NetworkMeasurement.v1`)
	// The kernel object is a session-local marker, without thread ownership.
	// Its handle is also closed by Windows if the process terminates.
	value, _, callErr := create.Call(0, 0, uintptr(unsafe.Pointer(name)))
	runtime.KeepAlive(name)
	if value == 0 {
		return nil, diagnostic.Error{Code: diagnostic.ExternalFailure}
	}
	handle := syscall.Handle(value)
	closeMarker := func() { _ = syscall.CloseHandle(handle) }
	if callErr == syscall.ERROR_ALREADY_EXISTS {
		closeMarker()
		return nil, diagnostic.Error{Code: diagnostic.Busy}
	}
	return closeMarker, nil
}
