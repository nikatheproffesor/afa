package service

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"afa/internal/diagnostic"
)

func TestParseTarget(t *testing.T) {
	tests := []struct {
		rawURL string
		host   string
		port   string
	}{
		{"https://Example.COM/docs?q=a#section", "example.com", "443"},
		{"HTTPS://xn--rnek-4qa.example:8443", "xn--rnek-4qa.example", "8443"},
		{"https://example.com.:0443", "example.com.", "443"},
	}
	for _, test := range tests {
		host, port, err := parseTarget(test.rawURL)
		if err != nil || host != test.host || port != test.port {
			t.Errorf("parseTarget(%q) = %q, %q, %v", test.rawURL, host, port, err)
		}
	}
}

func TestParseTargetRejectsMalformedURLs(t *testing.T) {
	for _, rawURL := range []string{
		"example.com", "https:example.com", "http://example.com", "https://",
		"https://user:password@example.com", "https://example.com\n",
		"https://127.0.0.1", "https://[::1]", "https://[example.com]", "https://127.01",
		"https://example.com:", "https://example.com:0", "https://example.com:65536", "https://example.com:abc",
		"https://.example.com", "https://example..com", "https://-example.com",
	} {
		if _, _, err := parseTarget(rawURL); err == nil {
			t.Errorf("parseTarget(%q) accepted invalid input", rawURL)
		}
	}
}

func TestCheckSeparatesTCPAndTLS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("TLS diagnostic sent an HTTP request")
	}))
	defer server.Close()
	ip, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	host := server.Certificate().DNSNames[0]
	lookup := func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP(ip)}}, nil
	}
	trusted := x509.NewCertPool()
	trusted.AddCert(server.Certificate())
	tests := []struct {
		name       string
		host       string
		roots      *x509.CertPool
		wantStatus string
		wantError  diagnostic.Code
	}{
		{"verified", host, trusted, statusOK, ""},
		{"untrusted authority", host, x509.NewCertPool(), statusFailed, diagnostic.TLSCertificate},
		{"hostname mismatch", "mismatch.invalid", trusted, statusFailed, diagnostic.TLSCertificate},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := check(context.Background(), "https://"+test.host+":"+port, lookup, (&net.Dialer{}).DialContext, test.roots)
			if err != nil || result.DNS != statusOK || result.TCP != statusOK || result.TCPMillis == nil ||
				result.DNSMillis == nil || result.TLSMillis == nil || result.TLS != test.wantStatus || result.ErrorCode != test.wantError {
				t.Fatalf("check returned %+v, %v", result, err)
			}
			if test.wantError != "" && result.ErrorStage != "tls" {
				t.Fatalf("unexpected error stage %q", result.ErrorStage)
			}
		})
	}
}

func TestSkippedDurationOmittedAndZeroDurationIncludedInProfileJSON(t *testing.T) {
	var zero int64
	report := ProfileResult{
		Result:  Result{DNSMillis: &zero, TLS: statusSkipped},
		Profile: Profile{ID: "discord"},
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	jsonBody := string(encoded)
	if !strings.Contains(jsonBody, `"dns_millis":0`) || strings.Contains(jsonBody, `"tls_millis"`) ||
		!strings.Contains(jsonBody, `"profile"`) {
		t.Fatalf("unexpected profile JSON: %s", encoded)
	}
}

func TestCheckDNSFailuresSkipTCPAndTLS(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code diagnostic.Code
	}{
		{"no addresses", nil, diagnostic.DNSFailure},
		{"resolver failure", errors.New("synthetic resolver failure"), diagnostic.DNSFailure},
		{"resolver timeout", &net.DNSError{IsTimeout: true}, diagnostic.Timeout},
		{"resolver canceled", context.Canceled, diagnostic.Canceled},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lookup := func(context.Context, string) ([]net.IPAddr, error) { return nil, test.err }
			dial := func(context.Context, string, string) (net.Conn, error) {
				t.Fatal("TCP attempted after unsuccessful DNS lookup")
				return nil, errors.New("unexpected TCP call")
			}
			result, err := check(context.Background(), "https://example.com", lookup, dial, nil)
			if err != nil || result.DNS != statusFailed || result.TCP != statusSkipped || result.TLS != statusSkipped ||
				result.ErrorStage != "dns" || result.ErrorCode != test.code || result.TCPMillis != nil {
				t.Fatalf("check returned %+v, %v", result, err)
			}
		})
	}
}

func TestCheckTCPFailureSkipsTLS(t *testing.T) {
	lookup := func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.IPv4(127, 0, 0, 1)}}, nil
	}
	dial := func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("synthetic connection failure")
	}
	result, err := check(context.Background(), "https://example.com", lookup, dial, nil)
	if err != nil || result.DNS != statusOK || result.TCP != statusFailed || result.TLS != statusSkipped ||
		result.ErrorCode != diagnostic.TCPFailure || result.ErrorStage != "tcp" || result.TCPMillis == nil {
		t.Fatalf("check returned %+v, %v", result, err)
	}
}

func TestCheckCancellationReleasesTLSConnection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	lookup := func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.IPv4(127, 0, 0, 1)}}, nil
	}
	local, peer := net.Pipe()
	defer peer.Close()
	if err := peer.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	dial := func(context.Context, string, string) (net.Conn, error) {
		cancel()
		return local, nil
	}
	result, err := check(ctx, "https://example.com", lookup, dial, nil)
	if err != nil || result.TCP != statusOK || result.TLS != statusFailed ||
		result.ErrorCode != diagnostic.Canceled || result.ErrorStage != "tls" {
		t.Fatalf("check returned %+v, %v", result, err)
	}
	if _, err := peer.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("connection remained open, read error %v", err)
	}
}

func TestCheckDualStackFallsBackAndJoinsSlowDial(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("TLS diagnostic sent an HTTP request")
	}))
	defer server.Close()
	ip, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	lookup := func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.IPv6loopback}, {IP: net.ParseIP(ip)}}, nil
	}
	primaryStarted, primaryDone := make(chan struct{}), make(chan struct{})
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		if address == net.JoinHostPort(net.IPv6loopback.String(), port) {
			close(primaryStarted)
			defer close(primaryDone)
			<-ctx.Done()
			return nil, ctx.Err()
		}
		select {
		case <-primaryStarted:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}
	trusted := x509.NewCertPool()
	trusted.AddCert(server.Certificate())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	address := "https://" + server.Certificate().DNSNames[0] + ":" + port
	result, err := check(ctx, address, lookup, dial, trusted)
	if err != nil || result.TLS != statusOK || result.ConnectedFamily != "ipv4" ||
		!reflect.DeepEqual(result.AddressFamilies, []string{"ipv6", "ipv4"}) {
		t.Fatalf("check returned %+v, %v", result, err)
	}
	select {
	case <-primaryDone:
	default:
		t.Fatal("slow dial was not canceled before Check returned")
	}
}
