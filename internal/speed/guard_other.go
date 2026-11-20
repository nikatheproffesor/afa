//go:build !windows

package speed

func reserveMeasurement() (func(), error) { return func() {}, nil }
