package speed

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"afa/internal/diagnostic"
)

func TestDownload(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantResult bool
	}{
		{"complete", http.StatusOK, "1234", true},
		{"short", http.StatusOK, "123", false},
		{"too long", http.StatusOK, "123456", false},
		{"rate limited", http.StatusTooManyRequests, "1234", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("bytes") != "4" || r.Header.Get("Accept-Encoding") != "identity" {
					t.Error("unexpected download request")
				}
				w.WriteHeader(test.status)
				w.(http.Flusher).Flush()
				_, _ = io.WriteString(w, test.body)
			}))
			defer server.Close()

			result, err := downloadWithClock(context.Background(), server.Client().Transport, server.URL, 4, directRoute, nil,
				testClock(0, time.Millisecond, 2*time.Millisecond))
			if test.wantResult {
				if err != nil || result.Bytes != 4 || result.Mbps <= 0 {
					t.Fatalf("download returned %+v, %v", result, err)
				}
			} else if err == nil {
				t.Fatalf("download unexpectedly returned %+v", result)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

type readerFunc func([]byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) { return f(p) }

func directRoute() string { return diagnostic.RouteDirect }

func TestDownloadProgressAndReadCap(t *testing.T) {
	for _, tooLong := range []bool{false, true} {
		content := "1234"
		if tooLong {
			content += "5678"
		}
		reader := strings.NewReader(content)
		body := io.NopCloser(readerFunc(func(p []byte) (int, error) {
			return reader.Read(p[:min(2, len(p))])
		}))
		transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, ContentLength: -1, Body: body}, nil
		})
		var seen []int64
		report, err := downloadWithClock(context.Background(), transport, "https://example.invalid", 4, directRoute, func(bytes, total int64) {
			seen = append(seen, bytes)
			if total != 4 {
				t.Errorf("unexpected progress total: %d", total)
			}
		}, testClock(0, time.Millisecond, 2*time.Millisecond))
		if tooLong {
			if diagnostic.CodeOf(err, "") != diagnostic.HTTPFailure || report.Bytes != 5 || reader.Len() != 3 ||
				!reflect.DeepEqual(seen, []int64{2, 4, 4}) {
				t.Fatalf("oversized body result was %+v, %v, progress=%v", report, err, seen)
			}
		} else if err != nil || report.Bytes != 4 || report.Mbps <= 0 || report.TransferMbps == nil || *report.TransferMbps <= 0 ||
			!reflect.DeepEqual(seen, []int64{2, 4}) {
			t.Fatalf("exact body result was %+v, %v, progress=%v", report, err, seen)
		}
	}
}

func TestDownloadWithTransferBelowClockResolution(t *testing.T) {
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, ContentLength: 4, Body: io.NopCloser(strings.NewReader("1234"))}, nil
	})
	report, err := downloadWithClock(context.Background(), transport, "https://example.invalid", 4, directRoute, nil,
		testClock(0, 2*time.Millisecond, 2*time.Millisecond))
	if err != nil || report.DurationMS != 2 || report.Mbps <= 0 || report.TransferMS != nil || report.TransferMbps != nil {
		t.Fatalf("expected a total measurement with no transfer-only measurement: %+v, %v", report, err)
	}
}

func testClock(offsets ...time.Duration) func() time.Time {
	return func() time.Time {
		next := time.Unix(0, 0).Add(offsets[0])
		offsets = offsets[1:]
		return next
	}
}

func TestDownloadProductionTransportStopsAtRedirect(t *testing.T) {
	const marker = "synthetic-private-redirect"
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/" {
			t.Error("redirect was followed")
		}
		w.Header().Set("Location", "/other/%zz?token="+marker)
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()
	route := new(diagnostic.HTTPRoute)
	transport := newTransport(route)
	transport.TLSClientConfig = server.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	defer transport.CloseIdleConnections()
	report, err := download(context.Background(), transport, server.URL, 4, route.String, nil)
	encoded, encodeErr := json.Marshal(report)
	if encodeErr != nil || diagnostic.CodeOf(err, "") != diagnostic.HTTPFailure ||
		report.HTTPStatus != http.StatusFound || requests.Load() != 1 || strings.Contains(string(encoded)+err.Error(), marker) {
		t.Fatalf("unexpected redirect behavior: %+v, %v, requests=%d", report, err, requests.Load())
	}
}

func TestDownloadMapsProxyConnectRefusal(t *testing.T) {
	const marker = "synthetic-auth-marker"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			t.Error("expected CONNECT to the local proxy")
		}
		w.Header().Set("Proxy-Authenticate", `Basic realm="`+marker+`"`)
		w.WriteHeader(http.StatusProxyAuthRequired)
	}))
	defer server.Close()
	proxyURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := newTransport(new(diagnostic.HTTPRoute))
	transport.Proxy = http.ProxyURL(proxyURL)
	defer transport.CloseIdleConnections()
	report, err := download(context.Background(), transport, "https://example.invalid", 4, func() string { return diagnostic.RouteEnvProxy }, nil)
	if diagnostic.CodeOf(err, "") != diagnostic.ProxyFailure || report.Route != diagnostic.RouteEnvProxy || strings.Contains(err.Error(), marker) {
		t.Fatalf("unexpected proxy refusal report: %+v, %v", report, err)
	}
}

func TestDownloadCancellationAfterHeaders(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	body := io.NopCloser(readerFunc(func([]byte) (int, error) {
		cancel()
		return 0, ctx.Err()
	}))
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, ContentLength: 4, Body: body}, nil
	})
	report, err := download(ctx, transport, "https://example.invalid", 4, directRoute, nil)
	if diagnostic.CodeOf(err, "") != diagnostic.Canceled || report.HTTPStatus != http.StatusOK || report.Bytes != 0 {
		t.Fatalf("unexpected cancellation: %+v, %v", report, err)
	}
}

func TestSpeedAndQualityShareMeasurementGuard(t *testing.T) {
	ctx := context.Background()
	release, err := beginMeasurement(ctx)
	if err != nil {
		t.Fatalf("could not hold measurement guard: %v", err)
	}
	defer func() {
		if release != nil {
			release()
		}
	}()
	_, speedErr := Check(ctx, DefaultMiB)
	_, qualityErr := Quality(ctx, DefaultQualitySamples)
	if diagnostic.CodeOf(speedErr, "") != diagnostic.Busy || diagnostic.CodeOf(qualityErr, "") != diagnostic.Busy {
		t.Fatalf("want busy; speed=%v quality=%v", speedErr, qualityErr)
	}
	release()
	release = nil
	secondRelease, err := beginMeasurement(ctx)
	if err != nil {
		t.Fatalf("guard did not open after release: %v", err)
	}
	secondRelease()
}

func TestMeasurementGuardDoesNotReserveForCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	release, err := beginMeasurement(ctx)
	if release != nil {
		release()
		t.Fatal("canceled request unexpectedly reserved a guard")
	}
	if diagnostic.CodeOf(err, "") != diagnostic.Canceled {
		t.Fatalf("unexpected error: %v", err)
	}
}
