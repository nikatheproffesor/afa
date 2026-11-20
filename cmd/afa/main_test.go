package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"afa/internal/buildinfo"
	"afa/internal/diagnostic"
	"afa/internal/platform"
	"afa/internal/portal"
	"afa/internal/service"
	"afa/internal/speed"
)

func TestHelpVersionAndBadArguments(t *testing.T) {
	for _, test := range []struct {
		args   []string
		want   int
		output string
	}{
		{nil, 0, "AFA"},
		{[]string{"check", "--help"}, 0, "HTTPS_URL"},
		{[]string{"version"}, 0, buildinfo.Version},
		{[]string{"unknown"}, 2, "Komut tanınmadı"},
		{[]string{"speed", "--mib", "PRIVATE_VALUE"}, 2, "Geçersiz komut"},
		{[]string{"speed", "--mib", "0"}, 2, "1–8 MiB"},
		{[]string{"check"}, 2, "Geçersiz komut"},
	} {
		var out, stderr bytes.Buffer
		code := runWith(context.Background(), test.args, &out, &stderr, operations{})
		combined := out.String() + stderr.String()
		if code != test.want || !strings.Contains(combined, test.output) || strings.Contains(combined, "PRIVATE_VALUE") {
			t.Fatalf("run %v = %d, %q", test.args, code, combined)
		}
	}
}

func TestPortalJSONAndSafeBrowserHandoff(t *testing.T) {
	var out, stderr bytes.Buffer
	ops := operations{
		portal: func(context.Context) portal.Result {
			return portal.Result{State: portal.PossiblePortal, Detail: "Giriş gerekebilir.", Evidence: "http_redirect",
				HTTPStatus: 302, Route: diagnostic.RouteDirect, ErrorCode: diagnostic.PortalUncertain}
		},
		open: func(context.Context) error {
			if !strings.Contains(stderr.String(), platform.PortalGuidance) {
				t.Fatal("browser guidance missing before native dispatch")
			}
			return errors.New("PRIVATE_SHELL_ERROR")
		},
	}
	code := runWith(context.Background(), []string{"portal", "--json", "--open"}, &out, &stderr, ops)
	var result portalOutput
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if code != 1 || result.SchemaVersion != 1 || result.AppVersion != buildinfo.Version ||
		result.Evidence != "http_redirect" || result.ErrorCode != diagnostic.PortalUncertain ||
		result.Browser == nil || result.Browser.ErrorCode != diagnostic.LaunchFailure {
		t.Fatalf("unexpected portal report: %+v, exit %d", result, code)
	}
	if strings.Contains(out.String()+stderr.String(), "PRIVATE_SHELL_ERROR") {
		t.Fatal("raw launch error exposed")
	}

	out.Reset()
	stderr.Reset()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ops.open = func(context.Context) error {
		t.Fatal("a canceled operation launched the browser")
		return nil
	}
	code = runWith(ctx, []string{"portal", "--json", "--open"}, &out, &stderr, ops)
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if code != 1 || result.Browser.Status != "canceled" || stderr.Len() != 0 {
		t.Fatalf("expected canceled browser with no dispatch, got %+v, %s", result.Browser, &stderr)
	}
}

func TestCheckAndProfileJSON(t *testing.T) {
	ops := operations{
		service: func(_ context.Context, target string) (service.Result, error) {
			if target != "https://example.test" {
				t.Fatalf("unexpected service target %q", target)
			}
			return service.Result{Host: "example.test", DNS: "ok", TCP: "ok", TLS: "failed", ErrorCode: diagnostic.TLSCertificate}, nil
		},
		profile: func(_ context.Context, id string) (service.ProfileResult, error) {
			return service.ProfileResult{Profile: service.Profile{ID: id}, HTTP: "redirect", HTTPStatus: 302}, nil
		},
	}
	for _, test := range []struct {
		args []string
		want map[string]any
	}{
		{[]string{"check", "--json", "https://example.test"}, map[string]any{"operation": "check", "status": "failed", "dns": "ok", "error_code": string(diagnostic.TLSCertificate)}},
		{[]string{"check", "--json", "--profile", "epic"}, map[string]any{"operation": "check_profile", "status": "uncertain", "http_status": float64(302)}},
	} {
		var out, stderr bytes.Buffer
		code := runWith(context.Background(), test.args, &out, &stderr, ops)
		var result map[string]any
		if err := json.Unmarshal(out.Bytes(), &result); err != nil || code != 1 || stderr.Len() != 0 {
			t.Fatalf("invalid response: %v %d %s %s", err, code, &out, &stderr)
		}
		for key, value := range test.want {
			if !reflect.DeepEqual(result[key], value) {
				t.Errorf("%s = %v; want %v", key, result[key], value)
			}
		}
	}
}

func TestSpeedJSONSuccessAndFailure(t *testing.T) {
	var out, stderr bytes.Buffer
	samples := []float64{15, 5, 10}
	ops := operations{
		speed: func(_ context.Context, mib int) (speed.Result, error) {
			if mib != 1 {
				t.Fatalf("unexpected sample size %d", mib)
			}
			result := speed.Result{Host: speed.Host, Route: diagnostic.RouteDirect, Mbps: samples[0]}
			samples = samples[1:]
			return result, nil
		},
	}
	code := runWith(context.Background(), []string{"speed", "--json", "--mib", "1", "--samples", "3"}, &out, &stderr, ops)
	var report speedOutput
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if code != 0 || len(report.Samples) != 3 || report.MedianMbps == nil || *report.MedianMbps != 10 || report.Mbps != 15 {
		t.Fatalf("unexpected speed output: %+v, %d", report, code)
	}
	out.Reset()
	ops.speed = func(context.Context, int) (speed.Result, error) {
		return speed.Result{}, errors.New("PRIVATE_RESPONSE")
	}
	code = runWith(context.Background(), []string{"speed", "--json"}, &out, &stderr, ops)
	if code != 1 || strings.Contains(out.String()+stderr.String(), "PRIVATE_RESPONSE") ||
		!strings.Contains(out.String(), string(diagnostic.ExternalFailure)) || stderr.Len() != 0 {
		t.Fatalf("unexpected error output: %s %s %d", &out, &stderr, code)
	}
}

func BenchmarkPortalJSON(b *testing.B) {
	args := []string{"portal", "--json"}
	ops := operations{portal: func(context.Context) portal.Result {
		return portal.Result{State: portal.ProbeSuccessful, Detail: "Probe completed.",
			Evidence: "expected_probe_body", HTTPStatus: 200, Route: diagnostic.RouteDirect}
	}}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if code := runWith(ctx, args, io.Discard, io.Discard, ops); code != 0 {
			b.Fatalf("exit %d", code)
		}
	}
}
