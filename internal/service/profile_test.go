package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"afa/internal/diagnostic"
)

func TestWebsiteEvidenceUsesHEADWithoutFollowingRedirect(t *testing.T) {
	tests := []struct {
		name   string
		status int
		result string
		code   diagnostic.Code
	}{
		{"success", 204, statusOK, ""},
		{"observed redirect", 302, "redirect", ""},
		{"denied", 403, statusFailed, diagnostic.HTTPFailure},
		{"proxy auth", 407, statusFailed, diagnostic.ProxyFailure},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/" || r.Method != http.MethodHead || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
					t.Errorf("unexpected request form: %s %s", r.Method, r.URL.Path)
				}
				w.Header().Set("Location", "/untested")
				w.WriteHeader(test.status)
			}))
			defer server.Close()
			report := ProfileResult{Profile: Profile{URL: server.URL}}
			report.checkWebsite(context.Background(), server.Client().Transport, &diagnostic.HTTPRoute{})
			if report.HTTPStatus != test.status || report.HTTP != test.result || report.HTTPErrorCode != test.code || report.HTTPMillis == nil {
				t.Fatalf("unexpected website evidence: %+v", report)
			}
		})
	}
}

func TestWebsiteDoesNotExposeProxyConnectError(t *testing.T) {
	const marker = "synthetic-proxy-detail"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			t.Error("only CONNECT should reach the local proxy")
		}
		w.Header().Set("Proxy-Authenticate", `Basic realm="`+marker+`"`)
		w.WriteHeader(http.StatusProxyAuthRequired)
	}))
	defer server.Close()
	proxy, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	route := new(diagnostic.HTTPRoute)
	transport := websiteTransport(route)
	transport.Proxy = http.ProxyURL(proxy)
	defer transport.CloseIdleConnections()
	report := ProfileResult{Profile: Profile{URL: "https://example.invalid"}}
	report.checkWebsite(context.Background(), transport, route)
	if report.HTTPErrorCode != diagnostic.ProxyFailure || report.ErrorStage != "http" ||
		strings.Contains(report.HTTPErrorCode.Message(), marker) {
		t.Fatalf("unexpected proxy rejection report: %+v", report)
	}
}

func TestUnknownProfileIsRejectedBeforeNetworking(t *testing.T) {
	_, err := CheckProfile(context.Background(), "unknown")
	var invalid diagnostic.Error
	if !errors.As(err, &invalid) || invalid.Code != diagnostic.InvalidInput {
		t.Fatalf("wanted invalid input, got %v", err)
	}
}

func TestCanceledProfileOmitsUnstartedStages(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	report, err := CheckProfile(ctx, "discord")
	if err != nil || report.ErrorCode != diagnostic.Canceled || report.DNSMillis != nil ||
		report.HTTP != statusSkipped || report.HTTPMillis != nil || report.HTTPErrorCode != diagnostic.Canceled {
		t.Fatalf("unexpected cancellation: %+v, %v", report, err)
	}
}
