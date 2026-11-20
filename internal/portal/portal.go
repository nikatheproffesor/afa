package portal

import (
	"context"
	"io"
	"mime"
	"net/http"
	"time"

	"afa/internal/diagnostic"
)

const (
	// ProbeURL is the documented Windows connectivity check.
	ProbeURL   = "http://www.msftconnecttest.com/connecttest.txt"
	probeBody  = "Microsoft Connect Test"
	maxRead    = 512
	maxHeaders = 16 << 10
)

const (
	ProbeSuccessful    = "probe_successful"
	PossiblePortal     = "possible_portal"
	UnexpectedResponse = "unexpected_response"
	ProbeUnreachable   = "probe_unreachable"
	RequestCanceled    = "request_canceled"
	Unknown            = "unknown"
)

type Result struct {
	State      string          `json:"state"`
	Detail     string          `json:"detail"`
	Evidence   string          `json:"evidence"`
	HTTPStatus int             `json:"http_status,omitempty"`
	Route      string          `json:"route"`
	ErrorCode  diagnostic.Code `json:"error_code,omitempty"`
}

// Check probes one endpoint. Its result describes this probe, not the whole connection.
func Check(ctx context.Context) Result {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	route := new(diagnostic.HTTPRoute)
	transport := newTransport(route)
	defer transport.CloseIdleConnections()
	return checkRoute(ctx, transport, ProbeURL, route.String)
}

func newTransport(route *diagnostic.HTTPRoute) *http.Transport {
	return &http.Transport{
		Proxy:                  route.Proxy,
		DisableKeepAlives:      true,
		DisableCompression:     true,
		MaxResponseHeaderBytes: maxHeaders,
	}
}

func checkRoute(ctx context.Context, transport http.RoundTripper, endpoint string, route func() string) Result {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Result{State: Unknown, Evidence: "request_not_created", Detail: "Yoklama isteği hazırlanamadı.", Route: route(), ErrorCode: diagnostic.InvalidInput}
	}
	// A single RoundTrip leaves redirect locations unread by the HTTP client.
	response, err := transport.RoundTrip(request)
	if err != nil {
		return requestFailure(err, route())
	}
	defer response.Body.Close()

	result := Result{
		State: UnexpectedResponse, Evidence: "unexpected_http_status", Detail: "Yoklama beklenmeyen bir HTTP yanıtı verdi.",
		HTTPStatus: response.StatusCode, Route: route(), ErrorCode: diagnostic.HTTPFailure,
	}
	switch response.StatusCode {
	case http.StatusNetworkAuthenticationRequired:
		result.State, result.Evidence = PossiblePortal, "network_authentication_required"
		result.Detail = "Yoklama ağ kimlik doğrulaması istendiğini bildirdi."
		result.ErrorCode = diagnostic.PortalUncertain
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		if response.Header.Get("Location") != "" {
			result.State, result.Evidence = PossiblePortal, "http_redirect"
			result.Detail = "Yoklama yönlendirildi. Ağ giriş isteyebilir."
			result.ErrorCode = diagnostic.PortalUncertain
		}
	case http.StatusProxyAuthRequired:
		result.Evidence, result.Detail = "proxy_authentication_required", diagnostic.ProxyFailure.Message()
		result.ErrorCode = diagnostic.ProxyFailure
	case http.StatusOK:
		body, err := io.ReadAll(io.LimitReader(response.Body, maxRead))
		if err != nil {
			result.Evidence = "body_unreadable"
			result.ErrorCode = diagnostic.CodeOf(err, diagnostic.HTTPFailure)
			result.Detail = result.ErrorCode.Message()
			if result.ErrorCode == diagnostic.Canceled {
				result.State = RequestCanceled
			}
			return result
		}
		switch {
		case string(body) == probeBody:
			result.State, result.Evidence = ProbeSuccessful, "expected_probe_body"
			result.Detail = "Yoklama adresinden beklenen yanıt alındı."
			result.ErrorCode = ""
		case isHTML(response.Header.Get("Content-Type"), body):
			result.State, result.Evidence = PossiblePortal, "unexpected_html"
			result.Detail = "Yoklamaya bir HTML sayfası döndü. Ağ giriş isteyebilir."
			result.ErrorCode = diagnostic.PortalUncertain
		default:
			result.Evidence = "unexpected_probe_body"
		}
	}
	return result
}

func isHTML(contentType string, body []byte) bool {
	mediaType, _, _ := mime.ParseMediaType(contentType)
	return mediaType == "text/html" || http.DetectContentType(body) == "text/html; charset=utf-8"
}

func requestFailure(err error, route string) Result {
	code := diagnostic.HTTPCode(err, route)
	state, evidence := ProbeUnreachable, "no_http_response"
	if code == diagnostic.Canceled {
		state, evidence = RequestCanceled, "request_canceled"
	}
	return Result{State: state, Detail: code.Message(), Evidence: evidence, Route: route, ErrorCode: code}
}
