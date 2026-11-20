package speed

import (
	"context"
	"sync/atomic"

	"afa/internal/diagnostic"
)

var running atomic.Bool

func beginMeasurement(ctx context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, diagnostic.Error{Code: diagnostic.CodeOf(err, diagnostic.Canceled)}
	}
	if !running.CompareAndSwap(false, true) {
		return nil, diagnostic.Error{Code: diagnostic.Busy}
	}
	unreserve, err := reserveMeasurement()
	if err != nil {
		running.Store(false)
		return nil, err
	}
	return func() {
		unreserve()
		running.Store(false)
	}, nil
}
