package platform

import (
	"context"
	"errors"
	"strings"
	"testing"

	"afa/internal/diagnostic"
	"afa/internal/portal"
)

func TestOpenPortalRecognizesCancellationBeforeBrowserHandoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := openPortal(ctx, unexpectedLaunch(t)); diagnostic.CodeOf(err, "") != diagnostic.Canceled {
		t.Fatalf("expected safe cancellation before handoff, got %v", err)
	}
}

func TestOpenPortalRecognizesExpiredDeadlineBeforeBrowserHandoff(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	if err := openPortal(ctx, unexpectedLaunch(t)); diagnostic.CodeOf(err, "") != diagnostic.Timeout {
		t.Fatalf("expected safe timeout before handoff, got %v", err)
	}
}

func TestOpenPortalPassesOnlyFixedURLAndRedactsLauncherFailure(t *testing.T) {
	const marker = "synthetic-launcher-detail"
	ctx := context.Background()
	calls := 0
	err := openPortal(ctx, func(gotContext context.Context, url string) error {
		calls++
		if gotContext != ctx || url != portal.ProbeURL {
			t.Errorf("launcher received unexpected context or URL: %q", url)
		}
		return errors.New(marker)
	})
	if calls != 1 || diagnostic.CodeOf(err, "") != diagnostic.LaunchFailure || strings.Contains(err.Error(), marker) {
		t.Fatalf("launch calls=%d, error=%v", calls, err)
	}
}

func unexpectedLaunch(t *testing.T) func(context.Context, string) error {
	t.Helper()
	return func(context.Context, string) error {
		t.Error("browser launcher was called for inactive context")
		return errors.New("unexpected launch")
	}
}
