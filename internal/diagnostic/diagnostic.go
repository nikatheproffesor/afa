package diagnostic

import (
	"context"
	"errors"
	"net"
	"time"

	"afa/internal/buildinfo"
)

type Code string

const (
	DNSFailure      Code = "dns_failure"
	TCPFailure      Code = "tcp_failure"
	TLSFailure      Code = "tls_failure"
	TLSCertificate  Code = "tls_certificate_failure"
	Timeout         Code = "timeout"
	Canceled        Code = "canceled"
	HTTPFailure     Code = "http_response_failure"
	ProxyFailure    Code = "proxy_failure"
	PortalUncertain Code = "portal_uncertain"
	ExternalFailure Code = "external_service_failure"
	InvalidInput    Code = "invalid_input"
	Busy            Code = "operation_busy"
	LaunchFailure   Code = "browser_launch_failure"
)

const (
	RouteUnknown  = "unknown"
	RouteDirect   = "direct"
	RouteEnvProxy = "environment_proxy"
)

// Error carries a safe, stable cause without retaining remote error text.
type Error struct{ Code Code }

func (e Error) Error() string { return e.Code.Message() }

func (c Code) Message() string {
	switch c {
	case DNSFailure:
		return "Alan adı çözümlenemedi."
	case TCPFailure:
		return "TCP bağlantısı kurulamadı."
	case TLSFailure:
		return "TLS el sıkışması tamamlanamadı."
	case TLSCertificate:
		return "Sunucu sertifikası doğrulanamadı."
	case Timeout:
		return "İşlem zaman aşımına uğradı."
	case Canceled:
		return "İşlem iptal edildi."
	case HTTPFailure:
		return "Beklenmeyen bir HTTP yanıtı alındı."
	case ProxyFailure:
		return "Proxy üzerinden istek tamamlanamadı."
	case PortalUncertain:
		return "Ağ giriş isteyebilir. Portalın kimliği doğrulanmadı."
	case InvalidInput:
		return "Geçersiz girdi."
	case Busy:
		return "Bir hız ölçümü zaten çalışıyor."
	case LaunchFailure:
		return "Varsayılan tarayıcı başlatılamadı."
	default:
		return "Dış ölçüm hizmeti isteği tamamlayamadı."
	}
}

func CodeOf(err error, fallback Code) Code {
	var typed Error
	var networkError net.Error
	switch {
	case err == nil:
		return ""
	case errors.As(err, &typed):
		return typed.Code
	case errors.Is(err, context.Canceled):
		return Canceled
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &networkError) && networkError.Timeout():
		return Timeout
	default:
		return fallback
	}
}

type Meta struct {
	SchemaVersion  int       `json:"schema_version"`
	AppVersion     string    `json:"app_version"`
	Operation      string    `json:"operation"`
	Timestamp      time.Time `json:"timestamp"`
	DurationMillis int64     `json:"duration_millis"`
	Status         string    `json:"status"`
	NetworkContext string    `json:"network_context"`
}

func NewMeta(operation string, started time.Time, status, route string) Meta {
	if route == "" {
		route = RouteUnknown
	}
	return Meta{
		SchemaVersion: 1, AppVersion: buildinfo.Version, Operation: operation,
		Timestamp: started.UTC(), DurationMillis: time.Since(started).Milliseconds(),
		Status: status, NetworkContext: route,
	}
}
