package diagnostic

import (
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/url"
	"sync/atomic"
)

// HTTPRoute records only the kind of path selected by net/http.
type HTTPRoute struct{ selected atomic.Uint32 }

func (r *HTTPRoute) Proxy(request *http.Request) (*url.URL, error) {
	proxy, err := http.ProxyFromEnvironment(request)
	if err != nil {
		return nil, Error{ProxyFailure}
	}
	if proxy != nil {
		r.selected.Store(2)
	} else {
		r.selected.Store(1)
	}
	return proxy, nil
}

func (r *HTTPRoute) String() string {
	switch r.selected.Load() {
	case 1:
		return RouteDirect
	case 2:
		return RouteEnvProxy
	default:
		return RouteUnknown
	}
}

func HTTPCode(err error, route string) Code {
	if code := CodeOf(err, ExternalFailure); code != ExternalFailure {
		return code
	}
	var operation *net.OpError
	if errors.As(err, &operation) && operation.Op == "proxyconnect" {
		return ProxyFailure
	}
	var certificate *tls.CertificateVerificationError
	if errors.As(err, &certificate) {
		return TLSCertificate
	}
	var dns *net.DNSError
	if errors.As(err, &dns) {
		if route == RouteEnvProxy {
			return ProxyFailure
		}
		return DNSFailure
	}
	if errors.As(err, &operation) && operation.Op == "dial" {
		if route == RouteEnvProxy {
			return ProxyFailure
		}
		return TCPFailure
	}
	return ExternalFailure
}
