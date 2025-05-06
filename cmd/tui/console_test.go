package tui

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type inputSuite struct {
	suite.Suite
}

func TestInputSuite(t *testing.T) {
	suite.Run(t, new(inputSuite))
}

func (s *inputSuite) Test_consoleComments() {
	b := bytes.Buffer{}
	_, _ = b.WriteString("just a line\n")
	_, _ = b.WriteString("a line with a comment # this is the comment\n")
	_, _ = b.WriteString("a line with an invalid comment1# this is the comment\n")
	_, _ = b.WriteString("a line with an invalid comment2#this is the comment\n")
	_, _ = b.WriteString("a line with an invalid comment2 #this is the comment\n")
	_, _ = b.WriteString("a line with a comment and more delimiters # this is the comment with a # symbol in it\n")
	_, _ = b.WriteString(" # this is a comment after an empty string\n")
	_, _ = b.WriteString("a line with a comment with more spaces         #     this is the comment\n")

	out, err := os.CreateTemp("", "test-output")
	s.NoError(err)
	defer os.Remove(out.Name())

	r := bufio.NewReader(bytes.NewReader(b.Bytes()))
	c, err := newTestConsole(r, out)
	s.NoError(err)
	s.NotNil(c)

	line, _, err := c.queue.ReadLine()
	s.NoError(err)
	s.Equal("just a line", string(line))
	line, _, err = c.queue.ReadLine()
	s.NoError(err)
	s.Equal("a line with a comment", string(line))
	line, _, err = c.queue.ReadLine()
	s.NoError(err)
	s.Equal("a line with an invalid comment1# this is the comment", string(line))
	line, _, err = c.queue.ReadLine()
	s.NoError(err)
	s.Equal("a line with an invalid comment2#this is the comment", string(line))
	line, _, err = c.queue.ReadLine()
	s.NoError(err)
	s.Equal("a line with an invalid comment2 #this is the comment", string(line))
	line, _, err = c.queue.ReadLine()
	s.NoError(err)
	s.Equal("a line with a comment and more delimiters", string(line))
	line, _, err = c.queue.ReadLine()
	s.NoError(err)
	s.Equal("", string(line))
	line, _, err = c.queue.ReadLine()
	s.NoError(err)
	s.Equal("a line with a comment with more spaces", string(line))
}

func (s *inputSuite) Test_parseInput() {
	cases := []struct {
		in  []string
		out []map[string]string
	}{
		{
			in:  []string{"test:down with a suffix", "table:select", "table:done"},
			out: []map[string]string{},
		},
		{
			in:  []string{"prefix with table:down", "table:select", "table:done"},
			out: []map[string]string{},
		},
		{
			in:  []string{"prefix with table:wait", "table:select", "table:done"},
			out: []map[string]string{},
		},
		{
			in:  []string{"table:wait", "table:select", "table:done"},
			out: []map[string]string{},
		},
		{
			in:  []string{"prefix with table:expect", "table:select", "table:done"},
			out: []map[string]string{},
		},
		{
			in:  []string{"table:expect", "table:select", "table:done"},
			out: []map[string]string{},
		},
		{
			in:  []string{"table:filter b2", "table:select", "table:done"},
			out: []map[string]string{{"a": "a2", "b": "b2"}},
		},
		{
			in:  []string{"table:filter afdasdfa", "table:select", "table:done"},
			out: []map[string]string{},
		},
		{
			in:  []string{"table:select", "table:done"},
			out: []map[string]string{{"a": "a1", "b": "b1"}},
		},
		{
			in:  []string{"table:select-all", "table:done"},
			out: []map[string]string{{"a": "a1", "b": "b1"}, {"a": "a2", "b": "b2"}},
		},
		{
			in:  []string{"table:select-all", "table:select-none", "table:done"},
			out: []map[string]string{},
		},
		{
			in:  []string{"table:down", "table:select", "table:done"},
			out: []map[string]string{{"a": "a2", "b": "b2"}},
		},
		{
			in:  []string{"table:down", "table:up", "table:select", "table:done"},
			out: []map[string]string{{"a": "a1", "b": "b1"}},
		},
		{
			in:  []string{"table:up", "table:select", "table:done"},
			out: []map[string]string{{"a": "a1", "b": "b1"}},
		},
		{
			in:  []string{"table:done"},
			out: []map[string]string{},
		},
	}

	for _, t := range cases {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		b := bytes.Buffer{}
		for _, line := range t.in {
			_, _ = b.WriteString(line + "\n")
		}

		out, err := os.CreateTemp("", "test-output")
		s.NoError(err)
		defer os.Remove(out.Name())
		r := bufio.NewReader(bytes.NewReader(b.Bytes()))
		asker, err := PrepareTestAsker(ctx, r, out)
		s.NoError(err)
		table := NewSelectableTable([]string{"a", "b"}, [][]string{{"a1", "b1"}, {"a2", "b2"}})
		answers, err := table.Render(ctx, asker, "title")
		s.NoError(err)
		s.Equal(t.out, answers)

		os.Remove(out.Name())
		cancel()
	}
}

func (s *inputSuite) Test_waitInput() {
	cases := []struct {
		txt        string
		timeoutSec int
	}{
		{
			txt:        "table:wait 0s",
			timeoutSec: 0,
		},
		{
			txt:        "table:wait 2s",
			timeoutSec: 2,
		},
	}

	for _, t := range cases {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		b := bytes.Buffer{}
		_, _ = b.WriteString(t.txt + "\n")
		_, _ = b.WriteString("table:select\n")
		_, _ = b.WriteString("table:done\n")

		r := bufio.NewReader(bytes.NewReader(b.Bytes()))
		out, err := os.CreateTemp("", "test-output")
		s.NoError(err)
		defer os.Remove(out.Name())
		asker, err := PrepareTestAsker(ctx, r, out)
		s.NoError(err)
		table := NewSelectableTable([]string{"a", "b"}, [][]string{{"a1", "b1"}, {"a2", "b2"}})
		answers, err := table.Render(ctx, asker, "title")
		s.NoError(err)

		now := time.Now()
		s.NoError(err)
		s.Equal([]map[string]string{{"a": "a1", "b": "b1"}}, answers)
		expectedDuration := time.Duration(t.timeoutSec) * time.Second
		s.Greater(time.Second, time.Since(now)-expectedDuration)
		os.Remove(out.Name())
		cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	b := bytes.Buffer{}
	_, _ = b.WriteString("wait with a suffix\n")
	_, _ = b.WriteString("table:select\n")
	_, _ = b.WriteString("table:done\n")

	r := bufio.NewReader(bytes.NewReader(b.Bytes()))
	out, err := os.CreateTemp("", "test-output")
	s.NoError(err)
	defer os.Remove(out.Name())
	asker, err := PrepareTestAsker(ctx, r, out)
	s.NoError(err)
	table := NewSelectableTable([]string{"a", "b"}, [][]string{{"a1", "b1"}, {"a2", "b2"}})
	_, err = table.Render(ctx, asker, "title")
	s.NoError(err)
}

func (s *inputSuite) Test_waitExpectStatic() {
	cases := []struct {
		txt         string
		numRows     int
		expectError bool
	}{
		{
			txt:         "table:expect 0",
			numRows:     2,
			expectError: true,
		},
		{
			txt:         "table:expect 2",
			numRows:     2,
			expectError: false,
		},
	}

	for _, t := range cases {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		b := bytes.Buffer{}
		_, _ = b.WriteString(t.txt + "\n")
		_, _ = b.WriteString("table:done\n")

		r := bufio.NewReader(bytes.NewReader(b.Bytes()))
		out, err := os.CreateTemp("", "test-output")
		s.NoError(err)
		defer os.Remove(out.Name())
		asker, err := PrepareTestAsker(ctx, r, out)
		s.NoError(err)
		rows := [][]string{}
		for i := 1; i <= t.numRows; i++ {
			rows = append(rows, []string{fmt.Sprintf("a%d", i), fmt.Sprintf("b%d", i)})
		}

		table := NewSelectableTable([]string{"a", "b"}, rows)
		_, err = table.Render(ctx, asker, "title")
		if t.expectError {
			s.Error(err)
		} else {
			s.NoError(err)
		}

		os.Remove(out.Name())
		cancel()
	}
}

func (s *inputSuite) Test_waitExpectDynamic() {
	cases := []struct {
		txt         string
		initialRows int
		finalRows   int
		expectError bool
	}{
		{
			txt:         "table:expect 4",
			initialRows: 2,
			finalRows:   4,
			expectError: false,
		},
		{
			txt:         "table:expect 4",
			initialRows: 4,
			finalRows:   4,
			expectError: false,
		},
		{
			txt:         "table:expect 4",
			initialRows: 3,
			finalRows:   5,
			expectError: false,
		},
		{
			txt:         "table:expect 4",
			initialRows: 4,
			finalRows:   5,
			expectError: false,
		},
		{
			txt:         "table:expect 4",
			initialRows: 2,
			finalRows:   3,
			expectError: true,
		},
	}

	for _, t := range cases {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		b := bytes.Buffer{}
		_, _ = b.WriteString(t.txt + "\n")
		_, _ = b.WriteString("table:done\n")

		r := bufio.NewReader(bytes.NewReader(b.Bytes()))
		out, err := os.CreateTemp("", "test-output")
		s.NoError(err)
		defer os.Remove(out.Name())
		asker, err := PrepareTestAsker(ctx, r, out)
		s.NoError(err)
		rows := [][]string{}
		for i := 1; i <= t.initialRows; i++ {
			rows = append(rows, []string{fmt.Sprintf("a%d", i), fmt.Sprintf("b%d", i)})
		}

		table := NewSelectableTable([]string{"a", "b"}, rows)
		wg := sync.WaitGroup{}
		go func() {
			// Add new rows over time.
			for i := t.initialRows + 1; i <= t.finalRows; i++ {
				wg.Add(1)
				row := InsertMsg{fmt.Sprintf("a%d", i), fmt.Sprintf("b%d", i)}
				time.Sleep(300 * time.Millisecond)
				table.SendUpdate(row)
			}

			for i := t.initialRows + 1; i <= t.finalRows; i++ {
				wg.Done()
			}
		}()

		_, err = table.Render(ctx, asker, "title")
		if t.expectError {
			s.Error(err)
		} else {
			s.NoError(err)
		}

		wg.Wait()
		os.Remove(out.Name())
		cancel()
	}
}
