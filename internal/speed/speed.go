package speed

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"afa/internal/diagnostic"
)

const (
	Host       = "speed.cloudflare.com"
	DefaultMiB = 8
	MaxMiB     = 8
	bytesInMiB = 1 << 20
)

type Result struct {
	Host         string  `json:"host"`
	Bytes        int64   `json:"bytes"`
	HeadersMS    int64   `json:"headers_ms"`
	DurationMS   int64   `json:"duration_ms"`
	Mbps         float64 `json:"mbps"`
	TransferMS   int64   `json:"transfer_ms"`
	TransferMbps float64 `json:"transfer_mbps"`
	HTTPStatus   int     `json:"http_status,omitempty"`
	Route        string  `json:"route"`
}

func Check(ctx context.Context, mib int) (Result, error) {
	return Measure(ctx, mib, nil)
}

// Measure reports application body bytes as they are read. The callback must return promptly.
func Measure(ctx context.Context, mib int, onProgress func(bytes, total int64)) (Result, error) {
	if mib < 1 || mib > MaxMiB {
		return Result{}, diagnostic.Error{Code: diagnostic.InvalidInput}
	}
	release, err := beginMeasurement(ctx)
	if err != nil {
		return Result{}, err
	}
	defer release()

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	route := new(diagnostic.HTTPRoute)
	transport := newTransport(route)
	defer transport.CloseIdleConnections()
	return download(ctx, transport, "https://"+Host+"/__down", int64(mib)*bytesInMiB, route.String, onProgress)
}

func newTransport(route *diagnostic.HTTPRoute) *http.Transport {
	return &http.Transport{
		Proxy:                  route.Proxy,
		ForceAttemptHTTP2:      true,
		DisableCompression:     true,
		MaxResponseHeaderBytes: 64 << 10,
		OnProxyConnectResponse: func(_ context.Context, _ *url.URL, _ *http.Request, response *http.Response) error {
			if response.StatusCode != http.StatusOK {
				return diagnostic.Error{Code: diagnostic.ProxyFailure}
			}
			return nil
		},
	}
}

func download(ctx context.Context, transport http.RoundTripper, endpoint string, size int64, route func() string, onProgress func(bytes, total int64)) (Result, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s?bytes=%d", endpoint, size), nil)
	if err != nil {
		return Result{}, diagnostic.Error{Code: diagnostic.InvalidInput}
	}
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("Cache-Control", "no-store, no-cache")

	started := time.Now()
	// RoundTrip neither follows redirects nor parses their private location values.
	response, err := transport.RoundTrip(request)
	result := Result{Host: request.URL.Hostname(), Route: route()}
	if err != nil {
		return result, diagnostic.Error{Code: diagnostic.HTTPCode(err, result.Route)}
	}
	defer response.Body.Close()
	headersAt := time.Now()
	result.HTTPStatus = response.StatusCode
	result.HeadersMS = headersAt.Sub(started).Milliseconds()
	switch response.StatusCode {
	case http.StatusOK:
	case http.StatusProxyAuthRequired:
		return result, diagnostic.Error{Code: diagnostic.ProxyFailure}
	default:
		return result, diagnostic.Error{Code: diagnostic.HTTPFailure}
	}
	if encoding := response.Header.Get("Content-Encoding"); encoding != "" && !strings.EqualFold(encoding, "identity") {
		return result, diagnostic.Error{Code: diagnostic.HTTPFailure}
	}
	if response.ContentLength >= 0 && response.ContentLength != size {
		return result, diagnostic.Error{Code: diagnostic.HTTPFailure}
	}

	var sink io.Writer = io.Discard
	if onProgress != nil {
		sink = &progressWriter{total: size, report: onProgress}
	}
	n, err := io.Copy(sink, io.LimitReader(response.Body, size+1))
	result.Bytes = n
	if err != nil {
		return result, diagnostic.Error{Code: diagnostic.CodeOf(err, diagnostic.ExternalFailure)}
	}
	if n != size {
		return result, diagnostic.Error{Code: diagnostic.HTTPFailure}
	}
	finished := time.Now()
	duration, transfer := finished.Sub(started), finished.Sub(headersAt)
	if duration <= 0 || transfer <= 0 {
		return result, diagnostic.Error{Code: diagnostic.ExternalFailure}
	}
	result.DurationMS, result.TransferMS = duration.Milliseconds(), transfer.Milliseconds()
	result.Mbps = float64(n) * 8 / duration.Seconds() / 1e6
	result.TransferMbps = float64(n) * 8 / transfer.Seconds() / 1e6
	return result, nil
}

type progressWriter struct {
	current int64
	total   int64
	report  func(bytes, total int64)
}

func (w *progressWriter) Write(buffer []byte) (int, error) {
	w.current += int64(len(buffer))
	w.report(min(w.current, w.total), w.total)
	return len(buffer), nil
}
