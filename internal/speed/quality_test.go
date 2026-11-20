package speed

import (
	"context"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"afa/internal/diagnostic"
)

func TestQualityCountsRequestsAndDoesNotBridgeFailedSamples(t *testing.T) {
	statuses := []int{http.StatusOK, http.StatusBadGateway, http.StatusOK, http.StatusOK}
	count := 0
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.Query().Get("bytes") != "0" {
			t.Fatalf("unexpected quality request: %s %s", request.Method, request.URL)
		}
		status := statuses[count]
		count++
		return &http.Response{StatusCode: status, Body: http.NoBody}, nil
	})
	report, err := measureQuality(context.Background(), transport, "https://example.invalid/?bytes=0", 4, directRoute)
	if err != nil || count != 4 || report.RequestedSamples != 4 || report.FailedRequests != 1 ||
		len(report.LatencySamplesMillis) != 3 || report.MedianMillis == nil || report.JitterMillis == nil {
		t.Fatalf("unexpected quality result: %+v, %v, requests=%d", report, err, count)
	}
	if want := math.Abs(report.LatencySamplesMillis[2] - report.LatencySamplesMillis[1]); *report.JitterMillis != want {
		t.Fatalf("jitter %f crossed an unsuccessful request; expected %f", *report.JitterMillis, want)
	}
}

func TestQualityPreservesProxyCauseAndPreCanceledDoesNotRequest(t *testing.T) {
	report, err := measureQuality(context.Background(), roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, &net.OpError{Op: "dial", Err: errors.New("synthetic proxy detail")}
	}), "https://example.invalid", 3, func() string { return diagnostic.RouteEnvProxy })
	if diagnostic.CodeOf(err, "") != diagnostic.ProxyFailure || report.FailedRequests != 3 || report.MedianMillis != nil {
		t.Fatalf("unexpected proxy result: %+v, %v", report, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	report, err = measureQuality(ctx, roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("pre-canceled operation attempted a request")
		return nil, context.Canceled
	}), "https://example.invalid", 3, directRoute)
	if diagnostic.CodeOf(err, "") != diagnostic.Canceled || report.FailedRequests != 0 || report.MedianMillis != nil {
		t.Fatalf("unexpected cancellation result: %+v, %v", report, err)
	}
}

func TestMedianDoesNotMutateSamples(t *testing.T) {
	if Median(nil) != nil {
		t.Fatal("empty samples must not have a median")
	}
	samples := []float64{5, 2, 1, 4}
	if got := Median(samples); got == nil || *got != 3 || !reflect.DeepEqual(samples, []float64{5, 2, 1, 4}) {
		t.Fatalf("unexpected median or mutated samples: %v %v", got, samples)
	}
}

func TestQualityRejectsNonemptyBodyWithoutReadingItAll(t *testing.T) {
	body := strings.NewReader("oversized")
	_, code := qualitySample(context.Background(), roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(body)}, nil
	}), "https://example.invalid", directRoute)
	if code != diagnostic.HTTPFailure || body.Len() != len("oversized")-1 {
		t.Fatalf("condition=%s remaining=%d", code, body.Len())
	}
}
