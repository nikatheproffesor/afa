package main

import (
	"os"

	"afa/internal/ui"
)

func main() {
	if err := ui.Run(); err != nil {
		ui.ShowStartupError()
		os.Exit(1)
	}
}
