package tui

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// testConsole is used to simulate a terminal for the purposes of interacting with the MicroCloud CLI for testing.
type testConsole struct {
	queue   *bufio.Reader // queue of commands to send to "stdin".
	inRead  *os.File      // read by the input handler like stdin.
	inWrite *os.File      // writes to "stdin" from the test console.

	out *os.File // output of the program.
}

// newTestConsole creates a new testConsole with the given input and output files.
func newTestConsole(r io.Reader, output *os.File) (*testConsole, error) {
	inRead, inWrite, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	sc := bufio.NewScanner(r)
	b := bytes.Buffer{}
	for sc.Scan() {
		txt := sc.Text()
		txt, _, _ = strings.Cut(txt, " # ")
		txt = strings.TrimSpace(txt)

		b.WriteString(txt)
		b.WriteString("\n")
	}

	return &testConsole{
		inRead:  inRead,
		inWrite: inWrite,
		out:     output,
		queue:   bufio.NewReader(bytes.NewReader(b.Bytes())),
	}, nil
}

// PrepareTestAsker removes comments from the lines read from the given reader, and assigns them to the test console reader.
// The test console will read lines from the input file and write them one by one to one side of a pipe.
// The other side of the pipe is read by the asker to interpret as input to the current question.
// The output can be tracked with the given output file.
func PrepareTestAsker(ctx context.Context, r io.Reader, output *os.File) (*InputHandler, error) {
	console, err := newTestConsole(r, output)
	if err != nil {
		return nil, err
	}

	handler := NewInputHandler(console.inRead, console.out)
	handler.testMode = true
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		for {
			select {
			case <-ctx.Done():
				console.inRead.Close()
				console.inWrite.Close()
				console.out.Close()
				return
			default:
				err := console.parseInput(handler)
				if err != nil {
					handler.table.SendUpdate(ErrorMsg(err))
					cancel()
					continue
				}

				time.Sleep(300 * time.Millisecond)
			}
		}
	}()

	return handler, nil
}

// parseInput reads a line from the test input and sends the appropriate interaction to the MicroCloud CLI.
func (c *testConsole) parseInput(handler *InputHandler) error {
	// If there is no active asker, we don't need to read any lines yet.
	if !handler.isActive() {
		return nil
	}

	line, _, err := c.queue.ReadLine()
	if err != nil {
		return fmt.Errorf("Failed to read input: %w", err)
	}

	input := string(line)

	// Log the input string.
	_, err = c.out.WriteString(fmt.Sprintf(" ANSWER: {%s}\n", input))
	if err != nil {
		return err
	}

	var action string
	switch input {
	case "table:down":
		action = ansi.CUD1
	case "table:up":
		action = ansi.CUU1
	case "table:select-none":
		action = ansi.CUB1
	case "table:select-all":
		action = ansi.CUF1
	case "table:select":
		action = " "
	case "table:done":
		action = "\r"
	default:
		_, wait, _ := strings.Cut(input, "table:wait ")
		_, expect, _ := strings.Cut(input, "table:expect ")
		_, filter, _ := strings.Cut(input, "table:filter ")
		if wait != "" {
			sleepTime, err := time.ParseDuration(wait)
			if err != nil {
				return err
			}

			time.Sleep(sleepTime)
		} else if expect != "" {
			count, err := strconv.Atoi(expect)
			if err != nil {
				return err
			}

			// If expecting 0, error out as the table will be invalid with 0 rows.
			if count == 0 {
				return fmt.Errorf("Cannot expect 0 rows")
			}

			// Sanity check that the table didn't start with more rows than we are going to expect.
			if len(handler.getAllRows()) > count {
				return fmt.Errorf("Table has more rows (%d) than expected (%d)", len(handler.getAllRows()), count)
			}

			// Wait until we receive the expected row count.
			for len(handler.getAllRows()) < count {
				time.Sleep(300 * time.Millisecond)
			}
		} else if filter != "" {
			action = filter
		} else {
			action = input + "\r"
		}
	}

	if action != "" {
		_, err := c.inWrite.WriteString(action)
		if err != nil {
			return err
		}

		// The input handler erases the previous line when the rendering changes so add a newline.
		_, err = c.out.WriteString("\n")
		if err != nil {
			return err
		}
	}

	return nil
}
