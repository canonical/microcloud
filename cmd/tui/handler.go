package tui

import (
	"bufio"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
)

// ContextError is the charmbracelet representation of a context cancellation error.
var ContextError error = tea.ErrProgramKilled

// InvalidInputError is used to indicate false input to an asked question.
var InvalidInputError func() = func() { PrintError("Invalid input, try again") }

// InputHandler handles input dialogs.
type InputHandler struct {
	input  *os.File
	output *os.File

	// testMode is set to true if the handler is initialized in test mode with PrepareTestAsker.
	testMode bool

	table *selectableTable

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

// countAllRows returns the number of all filtered and unflitered rows from the current table.
func (i *InputHandler) countAllRows() int {
	return i.table.countRawRows()
}

// formatQuestion enriches the plain question string with default and accepted answers.
func (i *InputHandler) formatQuestion(question string, defaultAnswer string, acceptedAnswers []string) string {
	var acceptedAnswersBlock string
	if len(acceptedAnswers) > 0 {
		acceptedAnswersBlock = Printf(Fmt{Arg: " (%s)"}, Fmt{Arg: strings.Join(acceptedAnswers, "/"), Bold: true})
	}

	var defaultAnswerBlock string
	if defaultAnswer != "" {
		defaultAnswerBlock = Printf(Fmt{Arg: " [%s]"}, Fmt{Arg: "default=" + defaultAnswer, Bold: true})
	}

	return fmt.Sprintf("%s%s%s: ", question, acceptedAnswersBlock, defaultAnswerBlock)
}

// Ask a question on the output stream and read the answer from the input stream.
func (i *InputHandler) askQuestion(question, defaultAnswer string) (string, error) {
	fmt.Print(question)

	return i.readAnswer(defaultAnswer)
}

// Read the user's answer from the input stream, trimming newline and providing a default.
func (i *InputHandler) readAnswer(defaultAnswer string) (string, error) {
	answer, err := bufio.NewReader(i.input).ReadString('\n')
	answer = strings.TrimSpace(strings.TrimSuffix(answer, "\n"))
	if answer == "" {
		answer = defaultAnswer
	}

	return answer, err
}

// AskBoolWarn is the same as AskBool but it prints the given warning before asking.
func (i *InputHandler) AskBoolWarn(warning string, question string, defaultAnswer bool) (bool, error) {
	PrintWarning(warning)
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

	for {
		answer, err := i.askQuestion(i.formatQuestion(question, defaultAnswerStr, []string{"yes", "no"}), defaultAnswerStr)
		if err != nil {
			return false, err
		}

		if slices.Contains([]string{"yes", "y"}, strings.ToLower(answer)) {
			return true, nil
		} else if slices.Contains([]string{"no", "n"}, strings.ToLower(answer)) {
			return false, nil
		}

		InvalidInputError()
	}
}

// AskStringWarn is the same as AskString but it prints the given warning before asking.
func (i *InputHandler) AskStringWarn(warning string, question string, defaultAnswer string, validator func(string) error) (string, error) {
	PrintWarning(warning)
	return i.AskString(question, defaultAnswer, validator)
}

// AskString prints the given prompt and accepts a string answer. If no answer is provided, it uses the default answer.
func (i *InputHandler) AskString(question string, defaultAnswer string, validator func(string) error) (string, error) {
	i.setActive(true)
	defer i.setActive(false)

	for {
		answer, err := i.askQuestion(i.formatQuestion(question, defaultAnswer, nil), defaultAnswer)
		if err != nil {
			return "", err
		}

		if validator != nil {
			err = validator(answer)
			if err != nil {
				InvalidInputError()
				continue
			}

			return answer, err
		}

		if len(answer) != 0 {
			return answer, err
		}

		InvalidInputError()
	}
}
