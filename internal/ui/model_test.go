package ui

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTaskGateBlocksDuplicateUntilWorkerFinishes(t *testing.T) {
	var gate taskGate
	ctx, ok := gate.begin(context.Background(), "speed", time.Minute)
	if !ok {
		t.Fatal("first operation was refused")
	}
	if _, ok := gate.begin(context.Background(), "speed", time.Minute); ok {
		t.Fatal("duplicate speed operation was accepted")
	}
	gate.stop()
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("context error = %v", ctx.Err())
	}
	if _, ok := gate.begin(context.Background(), "service", time.Minute); ok {
		t.Fatal("gate did not wait for the worker to finish")
	}
	gate.finish()
	if _, ok := gate.begin(context.Background(), "service", time.Minute); !ok {
		t.Fatal("the gate remained busy")
	}
	gate.finish()
}

func TestTaskGateCancellationFromParent(t *testing.T) {
	var gate taskGate
	parent, cancel := context.WithCancel(context.Background())
	ctx, ok := gate.begin(parent, "portal", 0)
	if !ok {
		t.Fatal("operation was refused")
	}
	cancel()
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("context error = %v", ctx.Err())
	}
	gate.finish()
	if _, ok := gate.begin(parent, "portal", 0); ok {
		t.Fatal("started with an already canceled parent")
	}
}

func TestPortalLabelDoesNotOverstateEvidence(t *testing.T) {
	for _, language := range []string{"tr", "en"} {
		if portalLabel(language, "unexpected_response") == portalLabel(language, "probe_unreachable") {
			t.Fatal("portal uncertainty collapsed into reachability failure")
		}
		if portalLabel(language, "probe_successful") == portalLabel(language, "possible_portal") {
			t.Fatal("portal inference collapsed into probe success")
		}
	}
}

func TestNetworkChangesCoalesceAndStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	changes := make(chan struct{}, 4)
	for range 4 {
		changes <- struct{}{}
	}
	observed := make(chan struct{}, 8)
	done := make(chan struct{})
	go func() {
		defer close(done)
		observeNetworkChanges(ctx, changes, 20*time.Millisecond, func() { observed <- struct{}{} })
	}()
	select {
	case <-observed:
	case <-time.After(time.Second):
		t.Fatal("network change was never reported")
	}
	select {
	case <-observed:
		t.Fatal("one event burst produced multiple probes")
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("network observer did not stop on cancel")
	}
}
