package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
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

func (s *inputSuite) Test_consoleEnv() {
	s.NoError(os.Setenv("TEST_CONSOLE", "1"))
	c, err := NewTestConsole()
	s.NoError(err)
	s.NotNil(c)
	s.NoError(c.Close())

	s.NoError(os.Setenv("TEST_CONSOLE", "0"))
	c, err = NewTestConsole()
	s.NoError(err)
	s.Nil(c)
}

func (s *inputSuite) Test_consoleComments() {
	s.NoError(os.Setenv("TEST_CONSOLE", "1"))
	c, err := NewTestConsole()
	s.NoError(err)
	s.NotNil(c)

	b := bytes.Buffer{}
	_, _ = b.WriteString("just a line\n")
	_, _ = b.WriteString("a line with a comment # this is the comment\n")
	_, _ = b.WriteString("a line with an invalid comment1# this is the comment\n")
	_, _ = b.WriteString("a line with an invalid comment2#this is the comment\n")
	_, _ = b.WriteString("a line with an invalid comment2 #this is the comment\n")
	_, _ = b.WriteString("a line with a comment and more delimiters # this is the comment with a # symbol in it\n")
	_, _ = b.WriteString(" # this is a comment after an empty string\n")
	_, _ = b.WriteString("a line with a comment with more spaces         #     this is the comment\n")

	r := bufio.NewReader(bytes.NewReader(b.Bytes()))
	prepareTestAsker(r)

	line, _, err := reader.ReadLine()
	s.NoError(err)
	s.Equal("just a line", string(line))
	line, _, err = reader.ReadLine()
	s.NoError(err)
	s.Equal("a line with a comment", string(line))
	line, _, err = reader.ReadLine()
	s.NoError(err)
	s.Equal("a line with an invalid comment1# this is the comment", string(line))
	line, _, err = reader.ReadLine()
	s.NoError(err)
	s.Equal("a line with an invalid comment2#this is the comment", string(line))
	line, _, err = reader.ReadLine()
	s.NoError(err)
	s.Equal("a line with an invalid comment2 #this is the comment", string(line))
	line, _, err = reader.ReadLine()
	s.NoError(err)
	s.Equal("a line with a comment and more delimiters", string(line))
	line, _, err = reader.ReadLine()
	s.NoError(err)
	s.Equal("", string(line))
	line, _, err = reader.ReadLine()
	s.NoError(err)
	s.Equal("a line with a comment with more spaces", string(line))
}

func (s *inputSuite) Test_parseInput() {
	s.NoError(os.Setenv("TEST_CONSOLE", "1"))
	c, err := NewTestConsole()
	s.NoError(err)
	s.NotNil(c)

	// This is necessary so that the parser detects a table & begins reading.
	_, _ = c.Tty().WriteString("Space to select; enter to confirm; type to filter results.\n")

	cases := []struct {
		in  []string
		out []string
	}{
		{
			in:  []string{"down with a suffix", "select", "---"},
			out: []string{},
		},
		{
			in:  []string{"prefix with down", "select", "---"},
			out: []string{},
		},
		{
			in:  []string{"prefix with wait", "select", "---"},
			out: []string{},
		},
		{
			in:  []string{"wait", "select", "---"},
			out: []string{},
		},
		{
			in:  []string{"prefix with expect", "select", "---"},
			out: []string{},
		},
		{
			in:  []string{"expect", "select", "---"},
			out: []string{},
		},
		{
			in:  []string{"b2", "select", "---"},
			out: []string{"| a2 | b2 |"},
		},
		{
			in:  []string{"", "select", "---"},
			out: []string{"| a1 | b1 |"},
		},
		{
			in:  []string{"afdasdfa", "select", "---"},
			out: []string{},
		},
		{
			in:  []string{"select", "---"},
			out: []string{"| a1 | b1 |"},
		},
		{
			in:  []string{"select-all", "---"},
			out: []string{"| a1 | b1 |", "| a2 | b2 |"},
		},
		{
			in:  []string{"select-all", "select-none", "---"},
			out: []string{},
		},
		{
			in:  []string{"b2", "clear", "select", "---"},
			out: []string{"| a1 | b1 |"},
		},
		{
			in:  []string{"down", "select", "---"},
			out: []string{"| a2 | b2 |"},
		},
		{
			in:  []string{"down", "up", "select", "---"},
			out: []string{"| a1 | b1 |"},
		},
		{
			in:  []string{"up", "select", "---"},
			out: []string{"| a2 | b2 |"},
		},
		{
			in:  []string{"---"},
			out: []string{},
		},
	}

	for _, t := range cases {
		b := bytes.Buffer{}
		for _, line := range t.in {
			_, _ = b.WriteString(fmt.Sprintf("%s\n", line))
		}

		r := bufio.NewReader(bytes.NewReader(b.Bytes()))
		prepareTestAsker(r)

		table := NewSelectableTable([]string{"a", "b"}, [][]string{{"a1", "b1"}, {"a2", "b2"}})
		s.NoError(table.Render(table.rows))

		answers, err := table.GetSelections()
		s.NoError(err)
		s.Equal(t.out, answers)
	}

	s.NoError(c.Close())
}

func (s *inputSuite) Test_waitInput() {
	s.NoError(os.Setenv("TEST_CONSOLE", "1"))
	c, err := NewTestConsole()
	s.NoError(err)
	s.NotNil(c)

	// This is necessary so that the parser detects a table & begins reading.
	_, _ = c.Tty().WriteString("Space to select; enter to confirm; type to filter results.\n")

	cases := []struct {
		txt        string
		timeoutSec int
	}{
		{
			txt:        "wait 0s",
			timeoutSec: 0,
		},
		{
			txt:        "wait 2s",
			timeoutSec: 2,
		},
	}

	for _, t := range cases {
		b := bytes.Buffer{}
		_, _ = b.WriteString(fmt.Sprintf("%s\n", t.txt))
		_, _ = b.WriteString("select\n")
		_, _ = b.WriteString("---\n")

		r := bufio.NewReader(bytes.NewReader(b.Bytes()))
		prepareTestAsker(r)

		table := NewSelectableTable([]string{"a", "b"}, [][]string{{"a1", "b1"}, {"a2", "b2"}})
		s.NoError(table.Render(table.rows))

		now := time.Now()
		answers, err := table.GetSelections()
		s.NoError(err)
		s.Equal([]string{"| a1 | b1 |"}, answers)
		expectedDuration := time.Duration(t.timeoutSec) * time.Second
		s.Greater(time.Second, time.Since(now)-expectedDuration)
	}

	b := bytes.Buffer{}
	_, _ = b.WriteString("wait with a suffix\n")
	_, _ = b.WriteString("select\n")
	_, _ = b.WriteString("---\n")

	r := bufio.NewReader(bytes.NewReader(b.Bytes()))
	prepareTestAsker(r)

	table := NewSelectableTable([]string{"a", "b"}, [][]string{{"a1", "b1"}, {"a2", "b2"}})
	s.NoError(table.Render(table.rows))

	_, err = table.GetSelections()

	s.Error(err)

	s.NoError(c.Close())
}

func (s *inputSuite) Test_waitExpectStatic() {
	s.NoError(os.Setenv("TEST_CONSOLE", "1"))
	c, err := NewTestConsole()
	s.NoError(err)
	s.NotNil(c)

	// This is necessary so that the parser detects a table & begins reading.
	_, _ = c.Tty().WriteString("Space to select; enter to confirm; type to filter results.\n")

	cases := []struct {
		txt         string
		numRows     int
		expectError bool
	}{
		{
			txt:         "expect 0",
			numRows:     2,
			expectError: true,
		},
		{
			txt:         "expect 2",
			numRows:     2,
			expectError: false,
		},
	}

	for _, t := range cases {
		b := bytes.Buffer{}
		_, _ = b.WriteString(fmt.Sprintf("%s\n", t.txt))
		_, _ = b.WriteString("---\n")

		r := bufio.NewReader(bytes.NewReader(b.Bytes()))
		prepareTestAsker(r)

		rows := [][]string{}
		for i := 1; i <= t.numRows; i++ {
			rows = append(rows, []string{fmt.Sprintf("a%d", i), fmt.Sprintf("b%d", i)})
		}

		table := NewSelectableTable([]string{"a", "b"}, rows)
		s.NoError(table.Render(table.rows))

		_, err = table.GetSelections()
		if t.expectError {
			s.Error(err)
		} else {
			s.NoError(err)
		}
	}

	s.NoError(c.Close())
}

func (s *inputSuite) Test_waitExpectDynamic() {
	s.NoError(os.Setenv("TEST_CONSOLE", "1"))
	c, err := NewTestConsole()
	s.NoError(err)
	s.NotNil(c)

	// This is necessary so that the parser detects a table & begins reading.
	_, _ = c.Tty().WriteString("Space to select; enter to confirm; type to filter results.\n")

	cases := []struct {
		txt         string
		initialRows int
		finalRows   int
		expectError bool
	}{
		{
			txt:         "expect 4",
			initialRows: 2,
			finalRows:   4,
			expectError: false,
		},
		{
			txt:         "expect 4",
			initialRows: 4,
			finalRows:   4,
			expectError: false,
		},
		{
			txt:         "expect 4",
			initialRows: 3,
			finalRows:   5,
			expectError: false,
		},
		{
			txt:         "expect 4",
			initialRows: 4,
			finalRows:   5,
			expectError: false,
		},
	}

	for _, t := range cases {
		b := bytes.Buffer{}
		_, _ = b.WriteString(fmt.Sprintf("%s\n", t.txt))
		_, _ = b.WriteString("---\n")

		r := bufio.NewReader(bytes.NewReader(b.Bytes()))
		prepareTestAsker(r)

		rows := [][]string{}
		for i := 1; i <= t.initialRows; i++ {
			rows = append(rows, []string{fmt.Sprintf("a%d", i), fmt.Sprintf("b%d", i)})
		}

		table := NewSelectableTable([]string{"a", "b"}, rows)
		s.NoError(table.Render(table.rows))

		// Add new rows over time.
		for i := t.initialRows + 1; i <= t.finalRows; i++ {
			time.Sleep(time.Second)
			table.Update([]string{fmt.Sprintf("a%d", i), fmt.Sprintf("b%d", i)})
		}

		_, err = table.GetSelections()
		if t.expectError {
			s.Error(err)
		} else {
			s.NoError(err)
		}
	}

	s.NoError(c.Close())
}
