package diagnostic

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestHTTPCodeReportsSafeCauses(t *testing.T) {
	const marker = "synthetic-internal-detail"
	tests := []struct {
		name  string
		err   error
		route string
		want  Code
	}{
		{"wrapped safe code", fmt.Errorf("%s: %w", marker, Error{Busy}), RouteUnknown, Busy},
		{"cancel", context.Canceled, RouteDirect, Canceled},
		{"timeout", context.DeadlineExceeded, RouteDirect, Timeout},
		{"DNS", &net.DNSError{Err: marker, Name: "example.invalid"}, RouteDirect, DNSFailure},
		{"proxy DNS", &net.DNSError{Err: marker, Name: "proxy.invalid"}, RouteEnvProxy, ProxyFailure},
		{"dial", &net.OpError{Op: "dial", Err: errors.New(marker)}, RouteDirect, TCPFailure},
		{"proxy CONNECT", &net.OpError{Op: "proxyconnect", Err: errors.New(marker)}, RouteUnknown, ProxyFailure},
		{"certificate", &tls.CertificateVerificationError{Err: errors.New(marker)}, RouteDirect, TLSCertificate},
		{"unknown", errors.New(marker), RouteDirect, ExternalFailure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HTTPCode(tt.err, tt.route)
			if got != tt.want || strings.Contains(got.Message(), marker) {
				t.Fatalf("want %s and safe message, got %s %q", tt.want, got, got.Message())
			}
		})
	}
}

func TestMetaPreservesMachineReadableEvidence(t *testing.T) {
	started := time.Now().Add(-time.Second)
	got := NewMeta("check", started, "failed", RouteDirect)
	if got.SchemaVersion != 1 || got.AppVersion == "" || got.Operation != "check" || got.Status != "failed" ||
		got.NetworkContext != RouteDirect || got.DurationMillis < 1000 ||
		!got.Timestamp.Equal(started) || got.Timestamp.Location() != time.UTC {
		t.Fatalf("unexpected operation metadata: %+v", got)
	}
}
