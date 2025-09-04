package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Based off https://github.com/charmbracelet/bubbletea/blob/main/examples/autocomplete/main.go.

// model is the Bubble Tea model for passphrase autocomplete.
type model struct {
	textInput   textinput.Model
	suggestions []string
	maxTokens   uint8
	testMode    bool
}

// autocompleteModel initializes a new autocomplete model with the given prompt and suggestions.
func autocompleteModel(prompt string, suggestions []string, maxTokens uint8, testMode bool) model {
	ti := textinput.New()
	ti.Prompt = prompt + ": "
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(Bright)
	ti.ShowSuggestions = true

	ti.Focus()

	return model{
		textInput:   ti,
		suggestions: suggestions,
		maxTokens:   maxTokens,
		testMode:    testMode,
	}
}

// Init initializes the model and returns a command to start blinking the cursor.
func (m model) Init() tea.Cmd {
	return textinput.Blink
}

// Update processes messages and updates the model accordingly.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter, tea.KeyCtrlC, tea.KeyEsc:
			m.textInput.Blur()
			return m, tea.Quit

		case tea.KeyTab:
			m = m.refreshTokenSuggestions(m.maxTokens)
		}

	case tea.WindowSizeMsg:
		m.textInput.Width = max(msg.Width-lipgloss.Width(m.textInput.Prompt)-2, 1)
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)

	// Refresh token suggestions on every change so ctrl+n/p works.
	m = m.refreshTokenSuggestions(m.maxTokens)

	return m, cmd
}

// View renders the model as a string.
func (m model) View() string {
	if m.testMode {
		return m.textInput.Prompt + m.textInput.Value()
	}

	return m.textInput.View() + "\n"
}

// refreshTokenSuggestions builds full-line suggestions by replacing only the last token.
// Example: input "foo a" -> suggestions like "foo apple", "foo apricot", ...
func (m model) refreshTokenSuggestions(maxTokens uint8) model {
	v := m.textInput.Value()

	if len(strings.Fields(v)) >= int(maxTokens) && strings.HasSuffix(v, " ") {
		m.textInput.SetSuggestions(nil)
		return m
	}

	// prefix: everything up to and including the last space
	// token: the last word being typed (may be empty)
	prefix, token := v, ""
	if !strings.HasSuffix(v, " ") {
		i := strings.LastIndexByte(v, ' ')
		if i >= 0 {
			prefix = v[:i+1]
			token = v[i+1:]
		} else {
			prefix = ""
			token = v
		}
	}

	var out []string
	for _, w := range m.suggestions {
		if token == "" || strings.HasPrefix(w, token) {
			out = append(out, prefix+w+" ")
		}
	}

	m.textInput.SetSuggestions(out)
	return m
}
