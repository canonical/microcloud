package main

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/core"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/olekukonko/tablewriter"
)

// SelectableTable represents a CLI table with selectable rows.
type SelectableTable struct {
	askChan  chan error
	testChan bool // When TEST_CONSOLE=1, sets whether askChan should expect an additional error for parsing the test input.
	prompt   *survey.MultiSelect
	writer   *tablewriter.Table

	rows         []string
	currentRow   string
	writtenLines int

	rawRows   [][]string
	rawHeader []string

	answers []string
	data    map[string]map[string]string

	tmpl templateData
}

// templateData is configuration passed to the table template.
// Since we abuse the `Default` field of survey.MultiSelect to pass this info to the template,
// the original functionality of the `Default` field is passed along to the DefaultRows field in this struct.
type templateData struct {
	Border string // The line representing a border/separator in the table.
	Header string // The line representing the table header.

	DefaultRows any // A []int of indexes, or []string of rows that will be selected by default.
}

// set up the template as early as possible.
func init() {
	prepareTemplate()
}

// Write lets SelectableTable implement io.Writer so we can compartmentalize the header, borders, and rows from the
// tablewriter.
func (t *SelectableTable) Write(p []byte) (int, error) {
	str := string(p)
	selectorSpacing := strings.Repeat(" ", len("> [ ]  "))

	if t.writtenLines == 0 {
		if str != "\n" {
			if len(t.tmpl.Border) == 0 {
				t.tmpl.Border += selectorSpacing
			}

			t.tmpl.Border += str
		}
	}

	if t.writtenLines == 1 {
		if str != "\n" {
			if len(t.tmpl.Header) == 0 {
				t.tmpl.Header += selectorSpacing
			}

			t.tmpl.Header += str
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
	  {{- $checked := (apply_defaults .Checked .Options .Default.DefaultRows ) }}
	  {{- if (eq .CurrentOpt.Value (index .PageEntries 0).Value) }}
	         {{- print (scroll_hint_top .Default.Border .FilterMessage .PageEntries .Options) "\n" }}
	  {{- end}}
    {{- if eq .SelectedIndex .CurrentIndex }}{{color .Config.Icons.SelectFocus.Format }}{{ .Config.Icons.SelectFocus.Text }}{{color "reset"}}{{else}} {{end}}
    {{- if index .Checked .CurrentOpt.Index }}{{color .Config.Icons.MarkedOption.Format }} {{ .Config.Icons.MarkedOption.Text }} {{else}}{{color .Config.Icons.UnmarkedOption.Format }} {{ .Config.Icons.UnmarkedOption.Text }} {{end}}
    {{- color "reset"}}
    {{- " "}}{{- .CurrentOpt.Value}}
	  {{- if eq (add 1 .CurrentIndex) (len .PageEntries) }}
	        {{- print "\n" (scroll_hint_bot .Default.Border .FilterMessage .PageEntries .Options) }}
	  {{- end }}
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
      {{- .Default.Border }}{{ "\n" }}
      {{- .Default.Header }}{{ "\n" }}
{{- end}}
{{- range $ix, $option := .PageEntries}}
  {{- template "option" $.IterateOption $ix $option}}
{{- end}}
`

// DefaultAskOptions is the default options on ask, using the OS stdio.
func defaultPromptConfig() *survey.PromptConfig {
	return &survey.PromptConfig{
		PageSize:     15,
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
			filter = strings.TrimPrefix(filter, " ")
			filter = strings.ToLower(filter)

			// include this option if it matches
			return strings.Contains(strings.ToLower(value), filter)
		},
		KeepFilter:       true,
		ShowCursor:       false,
		RemoveSelectAll:  false,
		RemoveSelectNone: false,
	}
}

// prepareTemplate sets the proper functions and strings to allow formatting a pretty table.
func prepareTemplate() {
	core.TemplateFuncsWithColor["add"] = func(a int, b int) int {
		return a + b
	}

	filterFunc := defaultPromptConfig().Filter
	core.TemplateFuncsWithColor["scroll_hint_top"] = func(line string, filter string, onScreen []core.OptionAnswer, allOpts []string) string {
		if filter == "" {
			if onScreen[0].Index > 0 {
				return strings.Replace(line, "     ", "   ^ ", 1)
			}

			return line
		}

		topIndex := -1
		for i, opt := range allOpts {
			if filterFunc(filter, opt, i) {
				topIndex = i
				break
			}
		}

		if topIndex < onScreen[0].Index {
			return strings.Replace(line, "     ", "   ^ ", 1)
		}

		return line
	}

	core.TemplateFuncsWithColor["scroll_hint_bot"] = func(line string, filter string, onScreen []core.OptionAnswer, allOpts []string) string {
		if filter == "" {
			if onScreen[len(onScreen)-1].Index+1 < len(allOpts) {
				return strings.Replace(line, "     ", "   v ", 1)
			}

			return line
		}

		botIndex := -1
		for i, opt := range allOpts {
			if filterFunc(filter, opt, i) {
				botIndex = i
			}
		}

		if botIndex > onScreen[len(onScreen)-1].Index {
			return strings.Replace(line, "     ", "   v ", 1)
		}

		return line
	}

	core.TemplateFuncsWithColor["apply_defaults"] = func(checked map[int]bool, allOpts []string, defaults any) map[int]bool {
		if checked == nil {
			checked = map[int]bool{}
		}

		if defaults != nil {
			// if the default is string values
			defaultValues, ok := defaults.([]string)
			if ok {
				for _, dflt := range defaultValues {
					for i, opt := range allOpts {
						// if the option corresponds to the default
						if opt == dflt {
							// we found our initial value
							checked[i] = true
							// stop looking
							break
						}
					}
				}
				// if the default value is index values
			} else {
				defaultIndices, ok := defaults.([]int)
				if ok {
					// go over every index we need to enable by default
					for _, idx := range defaultIndices {
						// and enable it
						checked[idx] = true
					}
				}
			}
		}

		return checked
	}

	core.TemplateFuncsNoColor["add"] = core.TemplateFuncsWithColor["add"]
	core.TemplateFuncsNoColor["scroll_hint_bot"] = core.TemplateFuncsWithColor["scroll_hint_bot"]
	core.TemplateFuncsNoColor["scroll_hint_top"] = core.TemplateFuncsWithColor["scroll_hint_top"]
	core.TemplateFuncsNoColor["apply_defaults"] = core.TemplateFuncsWithColor["apply_defaults"]
	survey.MultiSelectQuestionTemplate = multiSelectQuestionTemplate
}

// Render outputs the SelectableTable and returns a slice of selected rows.
func (t *SelectableTable) Render(entries []string) error {
	t.prompt = &survey.MultiSelect{
		Message: `Space to select; enter to confirm; type to filter results.
Up/down to move; right to select all; left to select none.`,
		Options:  entries,
		Default:  t.tmpl,
		PageSize: 15,
	}

	t.answers = []string{}

	surveyOpts := []survey.AskOpt{survey.WithKeepFilter(true)}

	// Setup the test console if TEST_CONSOLE=1 is set.
	c, err := NewTestConsole()
	if err != nil {
		return err
	}

	if c != nil {
		surveyOpts = append(surveyOpts, survey.WithStdio(c.Tty(), c.Tty(), c.Tty()))
		t.testChan = true
		go func() {
			err := c.parseInput(&t.rawRows)
			if err != nil {
				t.askChan <- fmt.Errorf("Failed to parse test input: %w", err)
				return
			}

			t.askChan <- nil
		}()
	}

	go func() {
		err := survey.AskOne(t.prompt, &t.answers, surveyOpts...)
		if err != nil && err.Error() != "please provide options to select from" {
			t.askChan <- fmt.Errorf("Failed to confirm selection: %w", err)
			return
		}

		if c != nil {
			err = c.Close()
			if err != nil {
				t.askChan <- err
				return
			}
		}

		t.askChan <- nil
	}()

	return nil
}

// GetSelections blocks until the user selections are made, and returns them once available.
func (t *SelectableTable) GetSelections() ([]string, error) {
	err := <-t.askChan
	if err != nil {
		return nil, err
	}

	// If TEST_CONSOLE=1, we need to wait for the second blocking read of stdin.
	if t.testChan {
		err = <-t.askChan
		if err != nil {
			return nil, err
		}
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
	t.tmpl.Header = ""
	t.tmpl.Border = ""
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
	t.prompt.Default = t.tmpl
	t.prompt.Options = newEntries
	t.prompt.OnChange(terminal.IgnoreKey, defaultPromptConfig())
}
