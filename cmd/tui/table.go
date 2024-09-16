package tui

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"gopkg.in/yaml.v2"
)

const (
	// TableFormatCSV represents data as a CSV string.
	TableFormatCSV = "csv"

	// TableFormatJSON represents data as a JSON string.
	TableFormatJSON = "json"

	// TableFormatTable represents data as a formatted table.
	TableFormatTable = "table"

	// TableFormatYAML represents data as a YAML string.
	TableFormatYAML = "yaml"

	// TableFormatCompact represents data as a table without any border styling.
	TableFormatCompact = "compact"
)

func baseTableTemplate(header []string, compact bool) *table.Table {
	t := table.New()
	t = t.Headers(header...)
	if !compact {
		t = t.Border(lipgloss.NormalBorder())
		t = t.BorderStyle(lipgloss.NewStyle().Foreground(Border))
	} else {
		t = t.Border(lipgloss.Border{})
	}

	t = t.StyleFunc(func(row, col int) lipgloss.Style {
		tmpl := lipgloss.NewStyle()
		tmpl = tmpl.Padding(0, 1)
		tmpl = tmpl.Align(lipgloss.Center)

		if row == 0 {
			header[col] = lipgloss.NewStyle().Foreground(Bright).Bold(true).SetString(header[col]).String()
		}

		return tmpl
	})

	return t
}

// FormatData returns a string representation of the given data according to the format string.
func FormatData(format string, header []string, rows [][]string, raw any) (string, error) {
	switch format {
	case TableFormatCSV:
		var buf bytes.Buffer
		w := csv.NewWriter(&buf)
		err := w.WriteAll(rows)
		if err != nil {
			return "", err
		}

		return buf.String(), nil
	case TableFormatJSON:
		bytes, err := json.Marshal(raw)
		if err != nil {
			return "", err
		}

		return string(bytes), nil
	case TableFormatYAML:
		bytes, err := yaml.Marshal(raw)
		if err != nil {
			return "", err
		}

		return string(bytes), nil
	case TableFormatTable:
		return NewTable(header, rows), nil
	case TableFormatCompact:
		t := baseTableTemplate(header, true)
		t = t.Rows(rows...)

		return t.String(), nil
	}

	return "", fmt.Errorf("Invalid format (%s)", format)
}

// NewTable returns the string representation of a table with the given data.
func NewTable(header []string, rows [][]string) string {
	t := baseTableTemplate(header, false)
	t = t.Rows(rows...)

	return t.String()
}
