package main

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/olekukonko/tablewriter"
)

// SelectableTable represents a CLI table with selectable rows.
type SelectableTable struct {
	header     string
	currentRow string
	rows       []string
	border     string

	writtenLines int
}

// Write lets SelectableTable implement io.Writer so we can compartmentalize the header, borders, and rows from the
// tablewriter.
func (t *SelectableTable) Write(p []byte) (int, error) {
	str := string(p)
	selectorSpacing := strings.Repeat(" ", len("> [ ]  "))

	if t.writtenLines == 0 {
		if str != "\n" {
			if len(t.border) == 0 {
				t.border += selectorSpacing
			}

			t.border += str
		}
	}

	if t.writtenLines == 1 {
		if str != "\n" {
			if len(t.header) == 0 {
				t.header += selectorSpacing
			}

			t.header += str
		}
	}

	if t.writtenLines > 1 {
		if str == "\n" {
			t.rows = append(t.rows, t.currentRow)
			t.currentRow = ""
		} else {
			t.currentRow += str
		}
	}

	if str == "\n" {
		t.writtenLines++
	}

	return len(p), nil
}

// NewSelectableTable creates a new Selectable table from the given header and rows.
func NewSelectableTable(header []string, data [][]string) SelectableTable {
	table := SelectableTable{rows: make([]string, 0, len(data))}

	t := tablewriter.NewWriter(&table)
	t.SetAutoWrapText(false)
	t.SetAlignment(tablewriter.ALIGN_LEFT)
	t.SetHeader(header)
	t.SetHeaderLine(false)
	t.AppendBulk(data)
	t.SetBorders(tablewriter.Border{Left: true, Right: true, Top: true, Bottom: false})

	t.Render()

	return table
}

// multiSelectQuestionTemplate overwrites the default survey package template to accommodate table rows as selectable options.
const multiSelectQuestionTemplate = `
{{- define "option"}}
    {{- if eq .SelectedIndex .CurrentIndex }}{{color .Config.Icons.SelectFocus.Format }}{{ .Config.Icons.SelectFocus.Text }}{{color "reset"}}{{else}} {{end}}
    {{- if index .Checked .CurrentOpt.Index }}{{color .Config.Icons.MarkedOption.Format }} {{ .Config.Icons.MarkedOption.Text }} {{else}}{{color .Config.Icons.UnmarkedOption.Format }} {{ .Config.Icons.UnmarkedOption.Text }} {{end}}
    {{- color "reset"}}
    {{- " "}}{{- .CurrentOpt.Value}}
    {{- $size := 0}}
    {{- range $i, $o := .Options}}
      {{- if $i}} {{- $size = $i}}{{- end}}
    {{- end}}
    {{- if or (and (eq .CurrentOpt.Index $size) (gt $size 0)) .FilterMessage}}
    {{- "\n%s"}}
    {{- end}}
{{end}}
{{- if gt (len .PageEntries) 0 }}
  {{- color "default+hb"}}{{ .Message }}{{color "reset"}}
  {{- "\n"}}
{{- end}}
{{- if .FilterMessage}}
  {{- "Filter: "}}{{- color "default+hb"}}{{ .FilterMessage }}{{color "reset"}}
  {{- "\n"}}
{{- end}}
{{- if gt (len .PageEntries) 0 }}
      {{- "%s\n"}}
      {{- "%s\n"}}
      {{- "%s\n"}}
{{- end}}
{{- range $ix, $option := .PageEntries}}
  {{- template "option" $.IterateOption $ix $option}}
{{- end}}
`

// Render outputs the SelectableTable and returns a slice of selected rows.
func (t *SelectableTable) Render(entries []string) ([]string, error) {
	survey.MultiSelectQuestionTemplate = fmt.Sprintf(multiSelectQuestionTemplate, t.border, t.border, t.header, t.border)
	prompt := &survey.MultiSelect{
		Message: `Space to select; Enter to confirm; Esc to exit; Type to filter results.
Up/Down to move; Right to select all; Left to select none.`,
		Options:  entries,
		PageSize: 15,
	}

	selected := []string{}
	err := survey.AskOne(prompt, &selected)
	if err != nil {
		return nil, fmt.Errorf("Failed to confirm selection: %w", err)
	}

	return selected, nil
}
