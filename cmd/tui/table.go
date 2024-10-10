package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

func baseTableTemplate(header []string) *table.Table {
	t := table.New()
	t = t.Headers(header...)
	t = t.Border(lipgloss.NormalBorder())
	t = t.BorderStyle(lipgloss.NewStyle().Foreground(Border))
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

// NewTable returns the string representation of a table with the given data.
func NewTable(header []string, rows [][]string) string {
	t := baseTableTemplate(header)
	t = t.Rows(rows...)

	return t.String()
}
