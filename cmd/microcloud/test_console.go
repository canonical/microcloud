package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/Netflix/go-expect"
	cli "github.com/canonical/lxd/shared/cmd"
	"github.com/creack/pty"
	"github.com/hinshun/vt10x"
)

var reader *bufio.Reader

// testConsole is used to simulate a terminal for the purposes of interacting with the MicroCloud CLI for testing.
type testConsole struct {
	*expect.Console
}

// prepareTestAsker removes comments from the lines read from the given reader, and assigns them to the test console reader.
func prepareTestAsker(r io.Reader) cli.Asker {
	sc := bufio.NewScanner(r)
	b := bytes.Buffer{}
	for sc.Scan() {
		txt := sc.Text()
		txt, _, _ = strings.Cut(txt, " # ")
		txt = strings.TrimSpace(txt)

		b.WriteString(txt)
		b.WriteString("\n")
	}

	reader = bufio.NewReader(bytes.NewReader(b.Bytes()))

	return cli.NewAsker(reader)
}

// NewTestConsole creates a new testConsole, with an underlying expect.Console and virtual terminal.
func NewTestConsole() (*testConsole, error) {
	useTestConsole := os.Getenv("TEST_CONSOLE")
	if useTestConsole != "1" {
		return nil, nil
	}

	pty, tty, err := pty.Open()
	if err != nil {
		return nil, fmt.Errorf("Failed to open pseudo terminal: %v", err)
	}

	term := vt10x.New(vt10x.WithWriter(tty))
	c, err := expect.NewConsole(expect.WithStdin(pty), expect.WithStdout(term), expect.WithCloser(pty, tty))
	if err != nil {
		return nil, fmt.Errorf("Failed to create console: %v", err)
	}

	return &testConsole{Console: c}, nil
}

// Close closes the testConsole.
func (t *testConsole) Close() error {
	err := t.Tty().Close()
	if err != nil {
		return err
	}

	return t.Console.Close()
}

// parseInput reads a line from the test input and sends the appropriate interaction to the MicroCloud CLI.
func (c *testConsole) parseInput(tableRows *[][]string) error {
	// Check for the first line of a selectable table.
	_, err := c.ExpectString("Space to select; enter to confirm; type to filter results.")
	if err != nil {
		return fmt.Errorf("Failed to find selectable table: %w", err)
	}

	isDone := false
	for !isDone {
		line, _, err := reader.ReadLine()
		if err != nil {
			return fmt.Errorf("Failed to read input: %w", err)
		}

		input := string(line)
		fmt.Println(input)
		var action string
		switch input {
		case "down":
			action = string(terminal.KeyArrowDown)
		case "up":
			action = string(terminal.KeyArrowUp)
		case "select-none":
			action = string(terminal.KeyArrowLeft)
		case "select-all":
			action = string(terminal.KeyArrowRight)
		case "clear":
			action = string(terminal.KeyDeleteLine)
		case "select":
			action = " "
		case "---":
			action = string(terminal.KeyEnter)
			isDone = true
		default:
			_, wait, _ := strings.Cut(input, "wait ")
			_, expect, _ := strings.Cut(input, "expect ")
			if wait != "" {
				sleepTime, err := time.ParseDuration(wait)
				if err != nil {
					return err
				}

				time.Sleep(sleepTime)

				continue
			} else if expect != "" {
				count, err := strconv.Atoi(expect)
				if err != nil {
					return err
				}

				if len(*tableRows) > count {
					return fmt.Errorf("Table has more rows (%d) than expected (%d)", len(*tableRows), count)
				}

				// If expecting 0, error out as the table will be invalid with 0 rows.
				if count == 0 {
					return fmt.Errorf("Cannot expect 0 rows")
				}

				// Wait until we receive the expected row count.
				for len(*tableRows) < count {
					time.Sleep(300 * time.Millisecond)
				}

				continue
			} else {
				action = input
			}
		}

		_, err = c.Send(action)
		if err != nil {
			return fmt.Errorf("Failed to send input to the test console: %w", err)
		}
	}

	return nil
}
