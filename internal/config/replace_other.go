//go:build !windows

package config

import "os"

func replace(source, destination string) error {
	return os.Rename(source, destination)
}
