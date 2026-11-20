package portal

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"afa/internal/diagnostic"
)

func TestCheckResponses(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		body     string
		location string
		state    string
		evidence string
		code     diagnostic.Code
	}{
		{name: "expected response", status: http.StatusOK, body: probeBody, state: ProbeSuccessful, evidence: "expected_probe_body"},
		{name: "redirect", status: http.StatusFound, location: "/login?session=private-id", state: PossiblePortal, evidence: "http_redirect", code: diagnostic.PortalUncertain},
		{name: "redirect with unusable location", status: http.StatusFound, location: "/login%z", state: PossiblePortal, evidence: "http_redirect", code: diagnostic.PortalUncertain},
		{name: "redirect missing location", status: http.StatusFound, state: UnexpectedResponse, evidence: "unexpected_http_status", code: diagnostic.HTTPFailure},
		{name: "network authentication", status: http.StatusNetworkAuthenticationRequired, state: PossiblePortal, evidence: "network_authentication_required", code: diagnostic.PortalUncertain},
		{name: "login interstitial", status: http.StatusOK, body: "<!doctype html><title>Oturum aç</title>", state: PossiblePortal, evidence: "unexpected_html", code: diagnostic.PortalUncertain},
		{name: "wrong response", status: http.StatusOK, body: "Wrong response", state: UnexpectedResponse, evidence: "unexpected_probe_body", code: diagnostic.HTTPFailure},
		{name: "bad probe content with expected prefix", status: http.StatusOK, body: probeBody + strings.Repeat("x", 1024), state: UnexpectedResponse, evidence: "unexpected_probe_body", code: diagnostic.HTTPFailure},
		{name: "forbidden", status: http.StatusForbidden, state: UnexpectedResponse, evidence: "unexpected_http_status", code: diagnostic.HTTPFailure},
		{name: "proxy authentication", status: http.StatusProxyAuthRequired, state: UnexpectedResponse, evidence: "proxy_authentication_required", code: diagnostic.ProxyFailure},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.URL.Path != "/probe" {
					t.Errorf("unexpected request for %s", req.URL.Path)
					w.WriteHeader(http.StatusNotFound)
					return
				}
				if tt.location != "" {
					w.Header().Set("Location", tt.location)
				}
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.body)
			}))
			defer server.Close()

			got := checkRoute(context.Background(), server.Client().Transport, server.URL+"/probe", func() string { return diagnostic.RouteDirect })
			if got.State != tt.state || got.HTTPStatus != tt.status || got.Evidence != tt.evidence || got.ErrorCode != tt.code {
				t.Errorf("check() = %+v, want state %q and HTTP %d", got, tt.state, tt.status)
			}
			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(encoded), "private-id") {
				t.Errorf("check exposed the portal session identifier")
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type testBody struct {
	read   func([]byte) (int, error)
	closed bool
}

func (b *testBody) Read(p []byte) (int, error) { return b.read(p) }
func (b *testBody) Close() error               { b.closed = true; return nil }

func TestProbeLimitsAndClosesResponseBody(t *testing.T) {
	var consumed int
	content := strings.NewReader(strings.Repeat("x", 4*maxRead))
	body := &testBody{read: func(p []byte) (int, error) {
		n, err := content.Read(p)
		consumed += n
		return n, err
	}}
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: body}, nil
	})
	got := checkRoute(context.Background(), transport, ProbeURL, directRoute)
	if got.State != UnexpectedResponse || consumed != maxRead || !body.closed {
		t.Fatalf("got %+v, consumed %d bytes, closed=%v", got, consumed, body.closed)
	}
}

func TestProbeProductionTransportRejectsOversizedHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Test-Oversize", strings.Repeat("x", maxHeaders*2))
		_, _ = io.WriteString(w, probeBody)
	}))
	defer server.Close()
	route := new(diagnostic.HTTPRoute)
	transport := newTransport(route)
	defer transport.CloseIdleConnections()
	got := checkRoute(context.Background(), transport, server.URL, route.String)
	if got.State != ProbeUnreachable || got.HTTPStatus != 0 || got.Route != diagnostic.RouteDirect {
		t.Fatalf("oversized response headers were accepted: %+v", got)
	}
}

func TestProbeCanceledAfterHeaders(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	body := &testBody{read: func([]byte) (int, error) {
		cancel()
		return 0, ctx.Err()
	}}
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: body}, nil
	})
	got := checkRoute(ctx, transport, ProbeURL, directRoute)
	if got.State != RequestCanceled || got.ErrorCode != diagnostic.Canceled || got.HTTPStatus != http.StatusOK || !body.closed {
		t.Fatalf("got %+v, closed=%v", got, body.closed)
	}
}

func TestProbeTimeoutAndRedactedProxyFailure(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 0)
		defer cancel()
		transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return nil, request.Context().Err()
		})
		got := checkRoute(ctx, transport, ProbeURL, directRoute)
		if got.State != ProbeUnreachable || got.ErrorCode != diagnostic.Timeout {
			t.Fatalf("unexpected timeout result: %+v", got)
		}
	})
	t.Run("proxy", func(t *testing.T) {
		const marker = "synthetic-sensitive-marker"
		transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, &net.OpError{Op: "dial", Err: errors.New(marker)}
		})
		got := checkRoute(context.Background(), transport, ProbeURL, func() string { return diagnostic.RouteEnvProxy })
		encoded, err := json.Marshal(got)
		if err != nil || got.State != ProbeUnreachable || got.ErrorCode != diagnostic.ProxyFailure || strings.Contains(string(encoded), marker) {
			t.Fatalf("unexpected proxy result: %+v, %v", got, err)
		}
	})
}

func directRoute() string { return diagnostic.RouteDirect }

func TestCheckCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := Check(ctx)
	if got.State != RequestCanceled || got.ErrorCode != diagnostic.Canceled || got.HTTPStatus != 0 {
		t.Errorf("Check() = %+v, want canceled with no HTTP response", got)
	}
}
