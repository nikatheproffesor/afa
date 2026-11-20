//go:build !windows

package platform

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"time"
)

func openURL(ctx context.Context, address string) error {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	var executable string
	switch runtime.GOOS {
	case "darwin":
		executable = "/usr/bin/open"
	case "linux":
		executable = "xdg-open"
	default:
		return errors.New("no default browser launcher for this platform")
	}
	err := exec.CommandContext(ctx, executable, address).Run()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}
