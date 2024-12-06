//go:build test

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/canonical/microcloud/microcloud/cmd/tui"
)

func setupAsker(ctx context.Context) (*tui.InputHandler, error) {
	noColor := os.Getenv("NO_COLOR")
	if noColor != "" {
		tui.DisableColors()
	}

	useTestConsole := os.Getenv("TEST_CONSOLE")
	if useTestConsole != "1" {
		return tui.NewInputHandler(os.Stdin, os.Stdout), nil
	}

	fmt.Fprintf(os.Stderr, "%s\n\n", `
  Detected 'TEST_CONSOLE=1', MicroCloud CLI is in testing mode. Terminal interactivity is disabled.

  Interactive microcloud commands will read text instructions by line:

cat << EOF | microcloud init
table:select                # selects an element in the table
table:select-all            # selects all elements in the table
table:select-none           # de-selects all elements in the table
table:up                    # move up in the table
table:down                  # move down in the table
table:wait <time.Duration>  # waits before the next instruction
table:expect <count>        # waits until exactly <count> peers are available, and errors out if more are found
table:filter <text>         # applies filtering text to the table output
table:done                  # confirms the table selection and exits the table

# anything not prefixed with 'table:' will be treated as a raw string. This is used for text entry for individual questions
EOF`)

	// This fd is used for debugging what the CLI sees for each question.
	file := os.NewFile(uintptr(3), "fd3")
	if file == nil {
		return nil, fmt.Errorf("Failed to open file descriptor 3")
	}

	asker, err := tui.PrepareTestAsker(ctx, os.Stdin, file)
	if err != nil {
		return nil, err
	}

	return asker, nil
}
