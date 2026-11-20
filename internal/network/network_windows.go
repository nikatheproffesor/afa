package network

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"afa/internal/platform"
)

var (
	probeRouteAPI = sync.OnceValues(func() (*syscall.LazyProc, error) {
		return platform.SystemProcedure("iphlpapi.dll", "GetBestInterfaceEx")
	})
	interfaceTypeAPI = sync.OnceValues(func() (*syscall.LazyProc, error) {
		return platform.SystemProcedure("iphlpapi.dll", "ConvertInterfaceIndexToLuid")
	})
	watchers struct {
		sync.Mutex
		next     uintptr
		channels map[uintptr]chan struct{}
	}
	callback = syscall.NewCallback(func(token, _, _ uintptr) uintptr {
		watchers.Lock()
		defer watchers.Unlock()
		if changes, ok := watchers.channels[token]; ok {
			select {
			case changes <- struct{}{}:
			default:
			}
		}
		return 0
	})
)

func Current() Observation {
	result := Observation{ConnectionType: "unknown", ObservedAt: time.Now().UTC()}
	best, err := probeRouteAPI()
	if err != nil {
		return result
	}
	result.Supported = true
	// The documentation addresses select an example route locally. No socket is opened.
	ipv4 := syscall.RawSockaddrInet4{Family: syscall.AF_INET, Addr: [4]byte{192, 0, 2, 1}}
	ipv6 := syscall.RawSockaddrInet6{Family: syscall.AF_INET6, Addr: [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}}
	status, _, _ := best.Call(uintptr(unsafe.Pointer(&ipv4)), uintptr(unsafe.Pointer(&result.InterfaceIndex)))
	if status != 0 || result.InterfaceIndex == 0 {
		status, _, _ = best.Call(uintptr(unsafe.Pointer(&ipv6)), uintptr(unsafe.Pointer(&result.InterfaceIndex)))
	}
	runtime.KeepAlive(&ipv4)
	runtime.KeepAlive(&ipv6)
	if status != 0 {
		result.InterfaceIndex = 0
		return result
	}
	getType, err := interfaceTypeAPI()
	if err != nil {
		return result
	}
	var luid uint64
	if status, _, _ := getType.Call(uintptr(result.InterfaceIndex), uintptr(unsafe.Pointer(&luid))); status == 0 {
		result.ConnectionType = connectionType(uint16(luid >> 48))
	}
	return result
}

func connectionType(kind uint16) string {
	switch kind {
	case 6:
		return "ethernet"
	case 23:
		return "ppp"
	case 24:
		return "loopback"
	case 71:
		return "wifi"
	case 131:
		return "tunnel"
	case 243, 244:
		return "cellular"
	default:
		return "unknown"
	}
}

// Watch reports coalesced route, interface and address events until canceled.
func Watch(ctx context.Context) (<-chan struct{}, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	deregister, err := platform.SystemProcedure("iphlpapi.dll", "CancelMibChangeNotify2")
	if err != nil {
		return nil, err
	}
	changes := make(chan struct{}, 1)
	watchers.Lock()
	if watchers.channels == nil {
		watchers.channels = make(map[uintptr]chan struct{})
	}
	watchers.next++
	token := watchers.next
	watchers.channels[token] = changes
	watchers.Unlock()

	var handles []uintptr
	stop := func() {
		watchers.Lock()
		delete(watchers.channels, token)
		close(changes)
		watchers.Unlock()
		for _, handle := range handles {
			deregister.Call(handle)
		}
	}
	for _, name := range []string{"NotifyIpInterfaceChange", "NotifyUnicastIpAddressChange", "NotifyRouteChange2"} {
		register, err := platform.SystemProcedure("iphlpapi.dll", name)
		if err != nil {
			stop()
			return nil, err
		}
		var handle uintptr
		status, _, _ := register.Call(0, callback, token, 0, uintptr(unsafe.Pointer(&handle)))
		if status != 0 {
			stop()
			return nil, errors.New("Windows network notifications unavailable")
		}
		handles = append(handles, handle)
	}
	go func() {
		<-ctx.Done()
		stop()
	}()
	return changes, nil
}
