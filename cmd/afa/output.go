package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"afa/internal/diagnostic"
	"afa/internal/platform"
	"afa/internal/portal"
	"afa/internal/service"
	"afa/internal/speed"
)

type outcome struct {
	value    any
	text     string
	code     int
	toStderr bool
}

type failure struct {
	diagnostic.Meta
	ErrorCode diagnostic.Code `json:"error_code"`
}

func invalid(operation string, started time.Time, text string) outcome {
	return outcome{
		value: failure{diagnostic.NewMeta(operation, started, "invalid_input", diagnostic.RouteUnknown), diagnostic.InvalidInput},
		text:  text, code: 2,
	}
}

type browserResult struct {
	Status    string          `json:"status"`
	ErrorCode diagnostic.Code `json:"error_code,omitempty"`
}

type portalOutput struct {
	diagnostic.Meta
	portal.Result
	Browser *browserResult `json:"browser,omitempty"`
}

func runPortal(ctx context.Context, started time.Time, open bool, guidance io.Writer, ops operations) outcome {
	probe := ops.portal(ctx)
	report := portalOutput{Result: probe}
	text := probe.Detail
	if probe.HTTPStatus != 0 {
		text += fmt.Sprintf(" (HTTP %d)", probe.HTTPStatus)
	}
	code := 0
	if probe.State != portal.ProbeSuccessful {
		code = 1
	}
	if open {
		launch := browserResult{Status: "requested"}
		var err error
		if ctx.Err() != nil {
			err = ctx.Err()
		} else if _, err = fmt.Fprintln(guidance, platform.PortalGuidance); err == nil {
			err = ops.open(ctx)
		}
		if err != nil {
			launch.ErrorCode = diagnostic.CodeOf(err, diagnostic.LaunchFailure)
			launch.Status = "failed"
			if launch.ErrorCode == diagnostic.Canceled {
				launch.Status = "canceled"
			}
			code = 1
			text += "\n" + launch.ErrorCode.Message()
		} else {
			text += "\nVarsayılan tarayıcıya açma isteği gönderildi."
		}
		report.Browser = &launch
	}
	report.Meta = diagnostic.NewMeta("portal", started, probe.State, probe.Route)
	return outcome{value: report, text: text, code: code}
}

func runCheck(ctx context.Context, started time.Time, target string, ops operations) outcome {
	result, err := ops.service(ctx, target)
	if err != nil {
		return invalid("check", started, "Alan adı içeren geçerli bir HTTPS adresi girin.")
	}
	report := struct {
		diagnostic.Meta
		service.Result
	}{diagnostic.NewMeta("check", started, operationStatus(result.ErrorCode), diagnostic.RouteDirect), result}
	return outcome{value: report, text: serviceText(result), code: errorExit(result.ErrorCode)}
}

func runProfile(ctx context.Context, started time.Time, id string, ops operations) outcome {
	result, err := ops.profile(ctx, id)
	if err != nil {
		return invalid("check_profile", started, "Geçerli bir profil seçin: discord, roblox, steam veya epic.")
	}
	status := operationStatus(result.ErrorCode)
	code := errorExit(result.ErrorCode)
	if result.HTTP == "redirect" && code == 0 {
		status, code = "uncertain", 1
	}
	report := struct {
		diagnostic.Meta
		service.ProfileResult
	}{diagnostic.NewMeta("check_profile", started, status, diagnostic.RouteDirect), result}
	text := serviceText(result.Result) + "\nHTTPS HEAD  " + statusLabel(result.HTTP)
	if result.HTTPStatus != 0 {
		text += fmt.Sprintf(" (HTTP %d)", result.HTTPStatus)
	}
	text += "\nProfil, yalnızca herkese açık web adresini sınar."
	return outcome{value: report, text: text, code: code}
}

func serviceText(result service.Result) string {
	var out strings.Builder
	fmt.Fprintf(&out, "%s:%s\nDNS  %s", result.Host, result.Port, statusLabel(result.DNS))
	if result.DNSMillis != nil {
		fmt.Fprintf(&out, " (%d ms)", *result.DNSMillis)
	}
	fmt.Fprintf(&out, "\nTCP  %s", statusLabel(result.TCP))
	if result.TCPMillis != nil {
		fmt.Fprintf(&out, " (%d ms)", *result.TCPMillis)
	}
	fmt.Fprintf(&out, "\nTLS  %s", statusLabel(result.TLS))
	if result.TLSMillis != nil {
		fmt.Fprintf(&out, " (%d ms)", *result.TLSMillis)
	}
	if result.ErrorCode != "" {
		fmt.Fprintln(&out)
		fmt.Fprint(&out, result.ErrorCode.Message())
	}
	return out.String()
}

type speedOutput struct {
	diagnostic.Meta
	*speed.Result
	ErrorCode        diagnostic.Code `json:"error_code,omitempty"`
	FailedHTTPStatus int             `json:"failed_http_status,omitempty"`
	Samples          []speed.Result  `json:"samples,omitempty"`
	MedianMbps       *float64        `json:"median_mbps,omitempty"`
}

func runSpeed(ctx context.Context, started time.Time, mib, count int, ops operations) outcome {
	result := speedOutput{}
	var route = diagnostic.RouteUnknown
	status := "ok"
	for range count {
		sample, err := ops.speed(ctx, mib)
		if sample.Route != "" {
			route = sample.Route
		}
		if err != nil {
			result.ErrorCode = diagnostic.CodeOf(err, diagnostic.ExternalFailure)
			result.FailedHTTPStatus = sample.HTTPStatus
			status = operationStatus(result.ErrorCode)
			if len(result.Samples) > 0 && status != "canceled" {
				status = "partial"
			}
			break
		}
		result.Samples = append(result.Samples, sample)
	}
	result.Meta = diagnostic.NewMeta("speed", started, status, route)
	if len(result.Samples) > 0 {
		result.Result = &result.Samples[0]
	}
	if count == 1 {
		result.Samples = nil
	} else {
		var throughput []float64
		for _, sample := range result.Samples {
			throughput = append(throughput, sample.Mbps)
		}
		result.MedianMbps = speed.Median(throughput)
	}
	if result.ErrorCode != "" {
		text := result.ErrorCode.Message()
		if result.FailedHTTPStatus != 0 {
			text += fmt.Sprintf(" (HTTP %d)", result.FailedHTTPStatus)
		}
		return outcome{value: result, text: text, code: 1, toStderr: true}
	}
	text := fmt.Sprintf("%s\nİndirme  %.2f Mbps (%d MiB)\nHTTP başlıkları  %d ms · Toplam  %d ms",
		result.Host, result.Mbps, mib, result.HeadersMS, result.DurationMS)
	if result.TransferMbps != nil {
		text += fmt.Sprintf(" · Aktarım  %.2f Mbps", *result.TransferMbps)
	}
	if result.MedianMbps != nil {
		text += fmt.Sprintf("\n%d örnek · Ortanca %.2f Mbps", len(result.Samples), *result.MedianMbps)
	}
	return outcome{value: result, text: text}
}

func runQuality(ctx context.Context, started time.Time, samples int, ops operations) outcome {
	result, err := ops.quality(ctx, samples)
	if err == nil && result.MedianMillis == nil {
		err = diagnostic.Error{Code: diagnostic.ExternalFailure}
	}
	code := diagnostic.CodeOf(err, diagnostic.ExternalFailure)
	status := operationStatus(code)
	if code == "" && result.FailedRequests > 0 {
		status = "partial"
	}
	report := struct {
		diagnostic.Meta
		speed.QualityResult
		ErrorCode diagnostic.Code `json:"error_code,omitempty"`
	}{diagnostic.NewMeta("quality", started, status, result.Route), result, code}
	if code != "" {
		return outcome{value: report, text: code.Message(), code: 1, toStderr: true}
	}
	text := fmt.Sprintf("%s\nHTTPS yanıt süresi ortancası  %.2f ms\nBaşarılı istek  %d/%d",
		result.Host, *result.MedianMillis, len(result.LatencySamplesMillis), samples)
	if result.JitterMillis != nil {
		text += fmt.Sprintf("\nArdışık süre farkı ortancası  %.2f ms", *result.JitterMillis)
	}
	return outcome{value: report, text: text}
}

func operationStatus(code diagnostic.Code) string {
	switch code {
	case "":
		return "ok"
	case diagnostic.Canceled:
		return "canceled"
	default:
		return "failed"
	}
}

func errorExit(code diagnostic.Code) int {
	if code != "" {
		return 1
	}
	return 0
}

func statusLabel(value string) string {
	switch value {
	case "ok":
		return "başarılı"
	case "failed":
		return "başarısız"
	case "redirect":
		return "yönlendirildi"
	default:
		return "atlanmış"
	}
}
