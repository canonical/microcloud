//go:build !test

package main

import (
	"context"
	"os"

	"github.com/canonical/microcloud/microcloud/cmd/tui"
)

func setupAsker(ctx context.Context) (*tui.InputHandler, error) {
	noColor := os.Getenv("NO_COLOR")
	if noColor != "" {
		tui.DisableColors()
	}

	return tui.NewInputHandler(os.Stdin, os.Stdout), nil
}
