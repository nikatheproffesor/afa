package network

import (
	"context"
	"errors"
	"testing"
)

func TestConnectionTypeMapping(t *testing.T) {
	for code, want := range map[uint16]string{6: "ethernet", 23: "ppp", 24: "loopback", 71: "wifi", 131: "tunnel", 243: "cellular", 244: "cellular", 255: "unknown"} {
		if got := connectionType(code); got != want {
			t.Errorf("interface type %d mapped to %q instead of %q", code, got, want)
		}
	}
}

func TestWatchCanceledBeforeRegistration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	changes, err := Watch(ctx)
	if changes != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Watch returned %v, %v", changes, err)
	}
}
