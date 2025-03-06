package tui

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/canonical/lxd/shared"
	tea "github.com/charmbracelet/bubbletea"
)

// ContextError is the charmbracelet representation of a context cancellation error.
var ContextError error = tea.ErrProgramKilled

// InputHandler handles input dialogs.
type InputHandler struct {
	input  *os.File
	output *os.File

	// testMode is set to true if the handler is initialized in test mode with PrepareTestAsker.
	testMode bool

	table   *selectableTable
	tableMu sync.Mutex

	activeMu sync.RWMutex
	active   bool
	activeCh chan struct{}
}

// NewInputHandler creates a new input handler for managing dialogs.
func NewInputHandler(input *os.File, output *os.File) *InputHandler {
	return &InputHandler{
		input:    input,
		output:   output,
		table:    &selectableTable{},
		activeCh: make(chan struct{}),
	}
}

func (i *InputHandler) setActive(active bool) {
	i.activeMu.Lock()
	defer i.activeMu.Unlock()

	i.active = active
}

func (i *InputHandler) isActive() bool {
	i.activeMu.RLock()
	defer i.activeMu.RUnlock()

	return i.active
}

// getAllRows lists all filtered and unflitered rows from the current table.
func (i *InputHandler) getAllRows() [][]string {
	i.tableMu.Lock()
	defer i.tableMu.Unlock()

	allRows := make([][]string, len(i.table.rawRows))
	for i, row := range i.table.rawRows {
		copy(allRows[i], row)
	}

	return allRows
}

// printWarning prints the given warning with "!" appended to the front of the message.
func (i *InputHandler) printWarning(warning string) {
	warningParts := strings.Split(warning, "\n")
	for i := range warningParts {
		if i > 0 {
			// Add two spaces at the start of each new line, to account for the warning symbol at the start of the first line.
			warningParts[i] = "  " + warningParts[i]
		}
	}

	fmt.Printf("%s %s\n", WarningSymbol(), strings.Join(warningParts, "\n"))
}

// AskBoolWarn is the same as AskBool but it prints the given warning before asking.
func (i *InputHandler) AskBoolWarn(warning string, question string, defaultAnswer bool) (bool, error) {
	i.printWarning(warning)
	return i.AskBool(question, defaultAnswer)
}

// AskBool prints the given prompt and accepts either "yes" or "no" as answers. It returns the boolean equivalent.
func (i *InputHandler) AskBool(question string, defaultAnswer bool) (bool, error) {
	i.setActive(true)
	defer i.setActive(false)
	defaultAnswerStr := "no"
	if defaultAnswer {
		defaultAnswerStr = "yes"
	}

	result, err := i.handleQuestion(question, defaultAnswerStr, []string{"yes", "no"})
	if err != nil {
		return false, err
	}

	if shared.ValueInSlice(strings.ToLower(result.answer), []string{"yes", "y"}) {
		fmt.Println(result.View())
		return true, nil
	} else if shared.ValueInSlice(strings.ToLower(result.answer), []string{"no", "n"}) {
		fmt.Println(result.View())
		return false, nil
	}

	return false, fmt.Errorf("Response %q must be one of %v", result.answer, result.acceptedAnswers)
}

// AskStringWarn is the same as AskString but it prints the given warning before asking.
func (i *InputHandler) AskStringWarn(warning string, question string, defaultAnswer string, validator func(string) error) (string, error) {
	i.printWarning(warning)
	return i.AskString(question, defaultAnswer, validator)
}

// AskString prints the given prompt and accepts a string answer. If no answer is provided, it uses the default answer.
func (i *InputHandler) AskString(question string, defaultAnswer string, validator func(string) error) (string, error) {
	i.setActive(true)
	defer i.setActive(false)
	result, err := i.handleQuestion(question, defaultAnswer, nil)
	if err != nil {
		return "", err
	}

	err = validator(result.answer)
	if err != nil {
		return "", err
	}

	fmt.Println(result.View())

	return result.answer, nil
}

func (i *InputHandler) handleQuestion(question string, defaultAnswer string, acceptedAnswers []string) (*asker, error) {
	ask := &asker{
		question:        question,
		defaultAnswer:   defaultAnswer,
		acceptedAnswers: acceptedAnswers,
		File:            i.output,
	}

	// The standard renderer does not yet support custom cursor positions so we need to
	// manually remove the sequence from the end of the string to get proper cursor tracking.
	// see: https://github.com/charmbracelet/bubbletea/issues/918
	out, err := tea.NewProgram(ask, tea.WithOutput(ask), tea.WithInput(i.input)).Run()
	if err != nil {
		return nil, err
	}

	result, ok := out.(*asker)
	if !ok {
		return nil, fmt.Errorf("Unexpected question result")
	}

	if result.cancelled {
		return nil, fmt.Errorf("Input cancelled")
	}

	if strings.TrimSpace(result.answer) == "" {
		result.answer = result.defaultAnswer
	} else {
		result.answer = strings.TrimSpace(result.answer)
	}

	return result, nil
}
