package speed

import (
	"context"
	"io"
	"net/http"
	"slices"
	"time"

	"afa/internal/diagnostic"
)

const (
	DefaultQualitySamples = 3
	MaxQualitySamples     = 8
)

type QualityResult struct {
	Host                 string    `json:"host"`
	RequestedSamples     int       `json:"requested_samples"`
	FailedRequests       int       `json:"failed_requests"`
	LatencySamplesMillis []float64 `json:"latency_samples_millis"`
	MedianMillis         *float64  `json:"median_millis,omitempty"`
	JitterMillis         *float64  `json:"jitter_millis,omitempty"`
	Route                string    `json:"route"`
}

func Quality(ctx context.Context, samples int) (QualityResult, error) {
	if samples < 2 || samples > MaxQualitySamples {
		return QualityResult{}, diagnostic.Error{Code: diagnostic.InvalidInput}
	}
	release, err := beginMeasurement(ctx)
	if err != nil {
		return QualityResult{}, err
	}
	defer release()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	route := new(diagnostic.HTTPRoute)
	transport := newTransport(route)
	defer transport.CloseIdleConnections()
	return measureQuality(ctx, transport, "https://"+Host+"/__down?bytes=0", samples, route.String)
}

func measureQuality(ctx context.Context, transport http.RoundTripper, endpoint string, samples int, route func() string) (QualityResult, error) {
	report := QualityResult{Host: Host, RequestedSamples: samples, LatencySamplesMillis: []float64{}}
	var variations []float64
	var previous *float64
	lastError := diagnostic.ExternalFailure
	for index := 0; index < samples; index++ {
		if ctx.Err() != nil {
			break
		}
		elapsed, code := qualitySample(ctx, transport, endpoint, route)
		if code != "" {
			report.FailedRequests++
			lastError = code
			previous = nil
			if ctx.Err() != nil {
				break
			}
			continue
		}
		report.LatencySamplesMillis = append(report.LatencySamplesMillis, elapsed)
		if previous != nil {
			difference := elapsed - *previous
			if difference < 0 {
				difference = -difference
			}
			variations = append(variations, difference)
		}
		previous = &elapsed
	}
	report.Route = route()
	report.MedianMillis = Median(report.LatencySamplesMillis)
	report.JitterMillis = Median(variations)
	if ctx.Err() != nil {
		return report, diagnostic.Error{Code: diagnostic.CodeOf(ctx.Err(), diagnostic.Timeout)}
	}
	if report.MedianMillis == nil {
		return report, diagnostic.Error{Code: lastError}
	}
	return report, nil
}

func qualitySample(ctx context.Context, transport http.RoundTripper, endpoint string, route func() string) (float64, diagnostic.Code) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, diagnostic.InvalidInput
	}
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("Cache-Control", "no-store, no-cache")
	started := time.Now()
	response, err := transport.RoundTrip(request)
	if err != nil {
		return 0, diagnostic.HTTPCode(err, route())
	}
	elapsed := float64(time.Since(started)) / float64(time.Millisecond)
	defer response.Body.Close()
	if response.StatusCode == http.StatusProxyAuthRequired {
		return 0, diagnostic.ProxyFailure
	}
	if response.StatusCode != http.StatusOK {
		return 0, diagnostic.HTTPFailure
	}
	n, err := io.Copy(io.Discard, io.LimitReader(response.Body, 1))
	if err != nil {
		return 0, diagnostic.CodeOf(err, diagnostic.ExternalFailure)
	}
	if n != 0 {
		return 0, diagnostic.HTTPFailure
	}
	return elapsed, ""
}

func Median(values []float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	values = slices.Clone(values)
	slices.Sort(values)
	mid := len(values) / 2
	result := values[mid]
	if len(values)%2 == 0 {
		result = (values[mid-1] + result) / 2
	}
	return &result
}
