package service

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/url"
	"strings"
	"time"

	"afa/internal/diagnostic"
)

type Profile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Profiles describe public website measurements. Client login, voice, game
// servers and content download endpoints are outside these measurements.
func Profiles() []Profile {
	return []Profile{
		{ID: "discord", Name: "Discord", URL: "https://discord.com/"},
		{ID: "roblox", Name: "Roblox", URL: "https://www.roblox.com/"},
		{ID: "steam", Name: "Steam", URL: "https://store.steampowered.com/"},
		{ID: "epic", Name: "Epic Games", URL: "https://store.epicgames.com/"},
	}
}

type ProfileResult struct {
	Result
	Profile       Profile         `json:"profile"`
	HTTP          string          `json:"http"`
	HTTPStatus    int             `json:"http_status,omitempty"`
	HTTPMillis    *int64          `json:"http_millis,omitempty"`
	HTTPRoute     string          `json:"http_route"`
	HTTPErrorCode diagnostic.Code `json:"http_error_code,omitempty"`
}

// CheckProfile compares a direct DNS/TCP/TLS diagnostic with a single HTTPS
// HEAD request that can use the environment proxy. No redirect is followed.
func CheckProfile(ctx context.Context, id string) (ProfileResult, error) {
	for _, profile := range Profiles() {
		if strings.EqualFold(id, profile.ID) {
			return checkProfile(ctx, profile)
		}
	}
	return ProfileResult{}, diagnostic.Error{Code: diagnostic.InvalidInput}
}

func checkProfile(ctx context.Context, profile Profile) (ProfileResult, error) {
	base, err := Check(ctx, profile.URL)
	if err != nil {
		return ProfileResult{}, err
	}
	report := ProfileResult{Result: base, Profile: profile, HTTP: statusSkipped, HTTPRoute: diagnostic.RouteUnknown}
	if err := ctx.Err(); err != nil {
		report.recordHTTPError(statusSkipped, diagnostic.CodeOf(err, diagnostic.Canceled))
		return report, nil
	}
	route := &diagnostic.HTTPRoute{}
	transport := websiteTransport(route)
	defer transport.CloseIdleConnections()
	report.checkWebsite(ctx, transport, route)
	return report, nil
}

func websiteTransport(route *diagnostic.HTTPRoute) *http.Transport {
	return &http.Transport{
		Proxy:                  route.Proxy,
		DisableKeepAlives:      true,
		DisableCompression:     true,
		MaxResponseHeaderBytes: 16 << 10,
		ForceAttemptHTTP2:      true,
		TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS12},
		OnProxyConnectResponse: func(_ context.Context, _ *url.URL, _ *http.Request, response *http.Response) error {
			if response.StatusCode != http.StatusOK {
				return diagnostic.Error{Code: diagnostic.ProxyFailure}
			}
			return nil
		},
	}
}

func (report *ProfileResult) checkWebsite(ctx context.Context, transport http.RoundTripper, route *diagnostic.HTTPRoute) {
	ctx, cancel := context.WithTimeout(ctx, connectBudget)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, report.Profile.URL, nil)
	if err != nil {
		report.recordHTTPError(statusFailed, diagnostic.InvalidInput)
		return
	}
	started := time.Now()
	response, err := transport.RoundTrip(request)
	duration := time.Since(started).Milliseconds()
	report.HTTPMillis = &duration
	report.HTTPRoute = route.String()
	if err != nil {
		report.recordHTTPError(statusFailed, diagnostic.HTTPCode(err, report.HTTPRoute))
		return
	}
	defer response.Body.Close()
	report.HTTPStatus = response.StatusCode
	switch {
	case report.HTTPStatus >= 200 && report.HTTPStatus < 300:
		report.HTTP = statusOK
	case report.HTTPStatus >= 300 && report.HTTPStatus < 400:
		report.HTTP = "redirect"
	case report.HTTPStatus == http.StatusProxyAuthRequired:
		report.recordHTTPError(statusFailed, diagnostic.ProxyFailure)
	default:
		report.recordHTTPError(statusFailed, diagnostic.HTTPFailure)
	}
}

func (report *ProfileResult) recordHTTPError(status string, code diagnostic.Code) {
	report.HTTP = status
	report.HTTPErrorCode = code
	if report.ErrorCode == "" {
		report.ErrorCode = code
		report.ErrorStage = "http"
	}
}
