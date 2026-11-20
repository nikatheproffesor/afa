//go:build !windows

package network

import (
	"context"
	"errors"
	"time"
)

func Current() Observation {
	return Observation{ConnectionType: "unknown", ObservedAt: time.Now().UTC()}
}

func Watch(context.Context) (<-chan struct{}, error) {
	return nil, errors.New("network notifications unavailable on this platform")
}
