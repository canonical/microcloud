package main

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/olekukonko/tablewriter"
)

// SelectableTable represents a CLI table with selectable rows.
type SelectableTable struct {
	askChan chan error
	prompt  *survey.MultiSelect
	writer  *tablewriter.Table

	rows         []string
	header       string
	currentRow   string
	border       string
	writtenLines int

	rawRows   [][]string
	rawHeader []string

	answers []string
	data    map[string]map[string]string
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
// Additionall returns a map representing each row by its content, and each column therein.
func NewSelectableTable(header []string, data [][]string) *SelectableTable {
	table := SelectableTable{rows: make([]string, 0, len(data))}
	table.rawRows = data
	table.rawHeader = header
	table.askChan = make(chan error)

	t := tablewriter.NewWriter(&table)
	table.writer = t
	t.SetAutoWrapText(false)
	t.SetAlignment(tablewriter.ALIGN_LEFT)
	t.SetHeader(header)
	t.SetHeaderLine(false)
	t.AppendBulk(data)
	t.SetBorders(tablewriter.Border{Left: true, Right: true, Top: true, Bottom: false})

	t.Render()

	// map the rows (as strings) to the associated row.
	table.data = make(map[string]map[string]string, len(data))
	for i, row := range table.rows {
		table.data[row] = make(map[string]string, len(data[i]))
		for j, h := range header {
			table.data[row][h] = data[i][j]
		}
	}

	return &table
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

// DefaultAskOptions is the default options on ask, using the OS stdio.
func defaultPromptConfig() *survey.PromptConfig {
	return &survey.PromptConfig{
		PageSize:     7,
		HelpInput:    "?",
		SuggestInput: "tab",
		Icons: survey.IconSet{
			Error: survey.Icon{
				Text:   "X",
				Format: "red",
			},
			Help: survey.Icon{
				Text:   "?",
				Format: "cyan",
			},
			Question: survey.Icon{
				Text:   "?",
				Format: "green+hb",
			},
			MarkedOption: survey.Icon{
				Text:   "[x]",
				Format: "green",
			},
			UnmarkedOption: survey.Icon{
				Text:   "[ ]",
				Format: "default+hb",
			},
			SelectFocus: survey.Icon{
				Text:   ">",
				Format: "cyan+b",
			},
		},
		Filter: func(filter string, value string, index int) (include bool) {
			filter = strings.ToLower(filter)

			// include this option if it matches
			return strings.Contains(strings.ToLower(value), filter)
		},
		KeepFilter:       false,
		ShowCursor:       false,
		RemoveSelectAll:  false,
		RemoveSelectNone: false,
	}
}

// Render outputs the SelectableTable and returns a slice of selected rows.
func (t *SelectableTable) Render(entries []string) {
	survey.MultiSelectQuestionTemplate = fmt.Sprintf(multiSelectQuestionTemplate, t.border, t.border, t.header, t.border)
	t.prompt = &survey.MultiSelect{
		Message: `Space to select; Enter to confirm; Esc to exit; Type to filter results.
Up/Down to move; Right to select all; Left to select none.`,
		Options:  entries,
		PageSize: 15,
	}

	t.answers = []string{}

	go func() {
		err := survey.AskOne(t.prompt, &t.answers)
		if err != nil && err.Error() != "please provide options to select from" {

			t.askChan <- fmt.Errorf("Failed to confirm selection: %w", err)
			return
		}

		t.askChan <- nil
	}()
}

// GetSelections blocks until the user selections are made, and returns them once available.
func (t *SelectableTable) GetSelections() ([]string, error) {
	err := <-t.askChan
	if err != nil {
		return nil, err
	}

	if t.answers == nil {
		return nil, fmt.Errorf("Failed to find any answers")
	}

	return t.answers, nil
}

// SelectionValue returns the value of a selection by its row and column.
// - The row is the key returned by the list of selections
// - The column is the name of the corresponding column header.
func (t *SelectableTable) SelectionValue(selectionRow string, columnName string) string {
	return t.data[selectionRow][columnName]
}

// Update redraws the table with the new row added at the end.
func (t *SelectableTable) Update(row []string) {
	// Save the old rows so we can update the entries in the actual selection table.
	oldRows := t.rows

	// Clear the entire table so there's no artifacts.
	t.writer.ClearRows()
	t.rows = []string{}
	t.currentRow = ""
	t.header = ""
	t.border = ""
	t.writtenLines = 0
	t.rawRows = append(t.rawRows, row)
	t.writer.AppendBulk(t.rawRows)
	t.writer.Render()

	// Build the subset of rows to show from the old list.
	newEntries := make([]string, 0, len(t.prompt.Options)+1)
	for _, entry := range t.prompt.Options {
		for i := range oldRows {
			if oldRows[i] != entry {
				continue
			}

			newEntries = append(newEntries, t.rows[i])
		}
	}

	// Add the new entry to the displayed rows.
	newEntries = append(newEntries, t.rows[len(t.rows)-1])

	// Update the map of answers with new keys.
	for i, row := range t.rows {
		t.data[row] = map[string]string{}
		for j, h := range t.rawHeader {
			t.data[row][h] = t.rawRows[i][j]
		}
	}

	// Update the template as the size of the header and borders may have changed.
	survey.MultiSelectQuestionTemplate = fmt.Sprintf(multiSelectQuestionTemplate, t.border, t.border, t.header, t.border)
	t.prompt.Options = newEntries
	t.prompt.OnChange(terminal.IgnoreKey, defaultPromptConfig())
}
