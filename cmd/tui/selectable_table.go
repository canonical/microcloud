package tui

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/muesli/reflow/wrap"
)

// RemoveMsg is a table notification that returns a row index of the table to remove.
type RemoveMsg int

// DisableMsg is a table notification that returns a row index of the table to disable.
// TODO: Use DisableMsg to inform the initiator when a joiner's websocket has closed.
type DisableMsg int

// EnableMsg is a table notification that returns a row index of the table to enable, if it was disabled.
type EnableMsg int

// InsertMsg is a table notification that returns a set of row data to add to the table.
type InsertMsg []string

// ErrorMsg is an error that should be returned by the selectable table.
// The table cannot recover from this error.
type ErrorMsg error

// defaultTableSize is the maximum number of rows that will be displayed at once.
const defaultTableSize = 10

type selectableTable struct {
	table   *table.Table
	tableMu sync.Mutex
	program *tea.Program

	// size is the number of rows that will be displayed at once.
	size int

	// table headers.
	header []string

	// raw row text, without any formatting.
	rawRows [][]string

	// displayed rows with style formatting.
	formatRows [][]string

	// filterMap is a mapping of displayed indexes to absolute indexes in the rawRows list.
	filterMap map[int]int

	// row index for the current cursor position in the whole table.
	currentRow int

	// startIndex is the start index of the current viewport.
	startIndex int

	// selected rows.
	activeRows map[int]bool

	// restricted rows that cannot be selected.
	disabledRows map[int]bool

	// filter is the string used to filter the table results.
	filter string

	// active records whether the table has completed its selection, and thus should erase itself.
	active bool

	// title is displayed at the top of the table.
	title string

	// err is set by the test console when the table fails a constraint.
	err error

	// testMode is set if the associated input handler is in test mode.
	testMode bool

	// windowWidth contains the current width of the terminal window.
	windowWidth int
}

// SummarizeResult formats the result string and args with the standard style for table result summaries.
func SummarizeResult(tmpl string, args ...any) string {
	fmtArgs := []Fmt{}
	for _, arg := range args {
		fmtArgs = append(fmtArgs, Fmt{Arg: arg, Color: Yellow, Bold: true})
	}

	return Printf(Fmt{Arg: " " + tmpl, Color: White}, fmtArgs...)
}

// NewSelectableTable takes a slice of structs and adds table rows for them.
func NewSelectableTable(header []string, rows [][]string) *selectableTable {
	s := &selectableTable{
		header:  header,
		rawRows: rows,
		size:    defaultTableSize,
	}

	return s
}

// Render is a blocking function that renders the table until the user exits out, and then returns the selection from the supplied rows.
// Optionally takes a set of rows to replace the initial set.
func (s *selectableTable) Render(ctx context.Context, handler *InputHandler, title string, newRows ...[]string) ([]map[string]string, error) {
	if s.active || handler.table.active {
		return nil, errors.New("Cannot render table while another is already active")
	}

	s.tableMu.Lock()
	s.active = true
	s.title = title
	s.testMode = handler.testMode

	if newRows != nil {
		s.rawRows = newRows
	}

	// record the table in the handler for testing.
	handler.table = s
	s.tableMu.Unlock()

	handler.setActive(true)
	defer func() {
		s.active = false
		handler.setActive(false)
	}()

	s.program = tea.NewProgram(s, tea.WithContext(ctx), tea.WithInput(handler.input), tea.WithOutput(handler.output))
	result, err := s.program.Run()
	if err != nil {
		return nil, fmt.Errorf("Failed to render table: %w", err)
	}

	// unset the program, as the table has finished running.
	s.program = nil

	if s.err != nil {
		return nil, s.err
	}

	table, ok := result.(*selectableTable)
	if !ok {
		return nil, errors.New("Unexpected result type")
	}

	resultMap := make([]map[string]string, 0, len(table.rawRows))
	for i := range table.activeRows {
		if table.disabledRows[i] {
			continue
		}

		row := table.rawRows[i]
		rowMap := make(map[string]string, len(row))
		for j, col := range row {
			rowMap[table.header[j]] = col
		}

		resultMap = append(resultMap, rowMap)
	}

	sort.SliceStable(resultMap, func(i, j int) bool {
		return resultMap[i][s.header[0]] < resultMap[j][s.header[0]]
	})

	return resultMap, nil
}

// SendUpdate sends a synchronous update to the table. It waits until the program exists.
func (s *selectableTable) SendUpdate(msg tea.Msg) {
	// If the program has already errored, then ignore updates.
	if s.err != nil {
		return
	}

	// Sleep until the program is set.
	for s.program == nil {
		time.Sleep(300 * time.Millisecond)
	}

	s.program.Send(msg)
}

// Update handles table updates.
func (s *selectableTable) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.windowWidth = msg.Width
		return s, nil
	case tea.KeyMsg:
		return s.handleKeyEvent(msg)
	case InsertMsg:
		return s.handleInsertEvent(msg)
	case RemoveMsg:
		return s.handleRemoveEvent(msg)
	case EnableMsg:
		return s.handleEnableEvent(msg)
	case DisableMsg:
		return s.handleDisableEvent(msg)
	case ErrorMsg:
		s.err = msg

		return s, tea.Quit
	}

	return s, nil
}

// Init initializes a new selectable table.
func (s *selectableTable) Init() tea.Cmd {
	s.activeRows = make(map[int]bool)
	s.disabledRows = make(map[int]bool)
	s.formatRows = make([][]string, 0, len(s.rawRows))
	s.filterMap = make(map[int]int, len(s.rawRows))

	s.currentRow = 0
	s.startIndex = 0
	s.filter = ""

	for i, row := range s.rawRows {
		s.filterMap[i] = i
		col := make([]string, 0, len(row))
		col = append(col, row...)

		s.formatRows = append(s.formatRows, col)
	}

	header := make([]string, len(s.header))
	copy(header, s.header)

	s.table = baseTableTemplate(header, false)
	s.updateTableRows()
	return nil
}

// View draws the table and its menus and returns it as a string.
func (s *selectableTable) View() string {
	if !s.active {
		return ""
	}

	tableStr := s.table.String()
	parts := strings.Split(tableStr, "\n")

	// These are the number of rows taken up by the table header and footer.
	headerLength := 3
	footerLength := 1

	for i, part := range parts {
		row := i + s.startIndex
		var selector string
		if i == 2 && s.startIndex > 0 {
			selector = lipgloss.NewStyle().SetString("↑").Bold(true).Foreground(White).String()
		} else if i == len(parts)-footerLength && s.startIndex < len(s.formatRows)-s.size {
			selector = lipgloss.NewStyle().SetString("↓").Bold(true).Foreground(White).String()
		} else if i == len(parts)-footerLength {
			selector = " "
		} else if i < headerLength {
			selector = " "
		} else if s.activeRows[s.filterMap[row-headerLength]] {
			selector = SuccessSymbol()
		} else if s.disabledRows[s.filterMap[row-headerLength]] {
			selector = ErrorSymbol()
		} else if s.currentRow+headerLength == row {
			selector = SetColor(Yellow, ">", false)
		} else {
			selector = " "
		}

		parts[i] = fmt.Sprintf(" %s %s", selector, part)
	}

	title := s.title
	if title != "" {
		title = wrap.String(lipgloss.NewStyle().SetString(s.title).Bold(true).String(), s.windowWidth)
	}

	// If the test console is active, just print the title so that the table output isn't corrupted by debug messages.
	if s.testMode {
		return title + "\n"
	}

	var filter string
	if s.filter == "" {
		filter = Printf(Fmt{Arg: "\n Filter | %s\n", Color: White}, Fmt{Arg: "<type to filter rows>", Color: White})
	} else {
		filter = Printf(Fmt{Arg: "\n Filter | %s\n", Color: White}, Fmt{Arg: s.filter, Color: Yellow})
	}

	helpEnter := Fmt{Color: Bright, Arg: "enter", Bold: true}
	helpSpace := Fmt{Color: Bright, Arg: "space", Bold: true}
	helpRight := Fmt{Color: Bright, Arg: "→", Bold: true}
	helpLeft := Fmt{Color: Bright, Arg: "←", Bold: true}
	helpUp := Fmt{Color: Bright, Arg: "↑", Bold: true}
	helpDown := Fmt{Color: Bright, Arg: "↓", Bold: true}

	helpTmpl := Fmt{Arg: " %s to select; %s to confirm\n %s/%s to move; %s to select all; %s to select none"}
	help := Printf(helpTmpl, helpSpace, helpEnter, helpUp, helpDown, helpRight, helpLeft)

	return title + filter + strings.Join(parts, "\n") + "\n" + help
}

func (s *selectableTable) filterRows(updatePos bool) {
	s.formatRows = [][]string{}
	s.filterMap = map[int]int{}
	if updatePos {
		s.currentRow = 0
		s.startIndex = 0
	}

	index := 0
	for i, row := range s.rawRows {
		match := len(s.filter) == 0
		if !match {
			for _, col := range row {
				if strings.Contains(col, s.filter) {
					match = true
					break
				}
			}
		}

		if match {
			col := make([]string, 0, len(row))
			col = append(col, row...)
			s.formatRows = append(s.formatRows, col)

			s.filterMap[index] = i
			index++
		}
	}

	s.updateTableRows()
}

func (s *selectableTable) updateTableRows() {
	filter := table.NewFilter(table.NewStringData(s.formatRows...))
	filter = filter.Filter(func(row int) bool {
		match := row >= s.startIndex && row < s.startIndex+s.size
		s.rowStyle(row)

		return match
	})

	s.table = s.table.Data(filter)
}

func (s *selectableTable) handleRemoveEvent(remove RemoveMsg) (tea.Model, tea.Cmd) {
	s.tableMu.Lock()
	defer s.tableMu.Unlock()

	newRows := make([][]string, 0, len(s.rawRows)-1)
	for i, row := range s.rawRows {
		if i == int(remove) {
			delete(s.activeRows, i)
			delete(s.disabledRows, i)

			continue
		}

		col := make([]string, 0, len(row))
		col = append(col, row...)
		newRows = append(newRows, col)
	}

	s.rawRows = newRows
	s.filterRows(true)

	return s, nil
}

func (s *selectableTable) handleInsertEvent(insert InsertMsg) (tea.Model, tea.Cmd) {
	s.tableMu.Lock()
	defer s.tableMu.Unlock()

	s.rawRows = append(s.rawRows, insert)
	s.filterRows(false)

	return s, nil
}

func (s *selectableTable) handleDisableEvent(disable DisableMsg) (tea.Model, tea.Cmd) {
	s.tableMu.Lock()
	defer s.tableMu.Unlock()

	// indexes to disable should always be absolute indexes and do not need to go through s.filterMap.
	delete(s.activeRows, int(disable))
	s.disabledRows[int(disable)] = true

	return s, nil
}

func (s *selectableTable) handleEnableEvent(enable EnableMsg) (tea.Model, tea.Cmd) {
	s.tableMu.Lock()
	defer s.tableMu.Unlock()

	// indexes to enable should always be absolute indexes and do not need to go through s.filterMap.
	delete(s.disabledRows, int(enable))

	return s, nil
}

func (s *selectableTable) handleKeyEvent(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	s.tableMu.Lock()
	defer s.tableMu.Unlock()

	switch key.Type {
	case tea.KeyEsc:
		fallthrough
	case tea.KeyCtrlC:
		s.activeRows = map[int]bool{}
		fallthrough
	case tea.KeyEnter:
		s.active = false
		return s, tea.Quit

	case tea.KeyUp:
		if s.currentRow > 0 {
			s.currentRow--

			if s.currentRow < s.startIndex {
				s.startIndex--
			}
		}
	case tea.KeyDown:
		if s.currentRow < len(s.formatRows)-1 {
			s.currentRow++

			if s.currentRow > s.startIndex+(s.size-1) {
				s.startIndex++
			}
		}
	case tea.KeyLeft:
		s.activeRows = map[int]bool{}
	case tea.KeyRight:
		for i := range s.formatRows {
			if !s.disabledRows[s.filterMap[i]] {
				s.activeRows[s.filterMap[i]] = true
			}
		}

	case tea.KeySpace:
		if !s.disabledRows[s.filterMap[s.currentRow]] && len(s.formatRows) > 0 {
			if s.activeRows[s.filterMap[s.currentRow]] {
				delete(s.activeRows, s.filterMap[s.currentRow])
			} else {
				s.activeRows[s.filterMap[s.currentRow]] = true
			}
		}

	case tea.KeyBackspace:
		if len(s.filter) > 0 {
			s.filter = s.filter[:len(s.filter)-1]
		}

		s.filterRows(true)

	case tea.KeyDelete:
		if len(s.filter) > 0 {
			s.filter = s.filter[1:len(s.filter)]
		}

		s.filterRows(true)
	case tea.KeyRunes:
		for _, rune := range key.Runes {
			s.filter += string(rune)
		}

		s.filterRows(true)
	default:
		s.activeRows = map[int]bool{}
		s.active = false
		if s.err != nil {
			s.err = errors.New("Input cancelled")
		}

		return s, tea.Quit
	}

	return s, nil
}

func (s *selectableTable) rowStyle(row int) {
	for col := range s.formatRows[row] {
		rawRowIndex := s.filterMap[row]
		textStyle := lipgloss.NewStyle().SetString(s.rawRows[rawRowIndex][col])
		if row == s.currentRow {
			if s.activeRows[s.filterMap[row]] {
				textStyle = textStyle.Bold(true).Foreground(Green)
			} else if s.disabledRows[s.filterMap[row]] {
				textStyle = textStyle.Bold(true).Foreground(Red)
			} else {
				textStyle = textStyle.Bold(true).Foreground(White)
			}
		} else {
			if s.activeRows[s.filterMap[row]] {
				textStyle = textStyle.Bold(false).Foreground(Green)
			} else if s.disabledRows[s.filterMap[row]] {
				textStyle = textStyle.Bold(false).Foreground(Red)
			} else {
				textStyle = textStyle.Bold(false).Foreground(White)
			}
		}

		s.formatRows[row][col] = textStyle.String()
	}
}

func (s *selectableTable) countRawRows() int {
	s.tableMu.Lock()
	defer s.tableMu.Unlock()

	return len(s.rawRows)
}
