package tui

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	white       = "#a0a0a0"
	brightWhite = "#ffffff"

	black       = "#000000"
	brightBlack = "#505050"

	red       = "#c01c28"
	brightRed = "#f66151"

	green       = "#26a269"
	brightGreen = "#33da7a"

	yellow       = "#d97d0c"
	brightYellow = "#f99d2c"
)

var (
	// Yellow represents the common yellow color used throughout the CLI.
	Yellow lipgloss.TerminalColor = lipgloss.AdaptiveColor{Dark: brightYellow, Light: yellow}

	// Red represents the common red color used throughout the CLI.
	Red lipgloss.TerminalColor = lipgloss.AdaptiveColor{Dark: brightRed, Light: red}

	// Green represents the common green color used throughout the CLI.
	Green lipgloss.TerminalColor = lipgloss.AdaptiveColor{Dark: brightGreen, Light: green}

	// White represents white on a black background and black on a white background.
	White lipgloss.TerminalColor = lipgloss.AdaptiveColor{Dark: white, Light: black}

	// Bright represents bright white on a black background, or bright black on a white background.
	Bright lipgloss.TerminalColor = lipgloss.AdaptiveColor{Dark: brightWhite, Light: brightBlack}

	// Border represents the default border color used for tables.
	Border lipgloss.TerminalColor = lipgloss.AdaptiveColor{Dark: brightBlack, Light: black}
)

// DisableColors globally disables colors.
func DisableColors() {
	Yellow = lipgloss.Color("")
	Red = lipgloss.Color("")
	Green = lipgloss.Color("")
	White = lipgloss.Color("")
	Bright = lipgloss.Color("")
	Border = lipgloss.Color("")
}

// SetColor applies the color to the given text.
func SetColor(color lipgloss.TerminalColor, str string, bold bool) string {
	return lipgloss.NewStyle().Foreground(color).SetString(str).Bold(bold).String()
}

// Note creates a box with a note message inside. The border colour can be set with arguments.
func Note(borderColor lipgloss.TerminalColor, text string) string {
	box := lipgloss.NewStyle()
	box = box.Border(lipgloss.NormalBorder())
	box = box.BorderForeground(borderColor)
	box = box.Padding(0, 1)

	return box.Render(text)
}

// SuccessColor sets the 8-bit color of the text to 82. Green color used for happy messages.
func SuccessColor(text string, bold bool) string {
	return SetColor(Green, text, bold)
}

// WarningColor sets the 8-bit color of the text to 214. Yellow color used for warning messages.
func WarningColor(text string, bold bool) string {
	return SetColor(Yellow, text, bold)
}

// ErrorColor sets the 8-bit color of the text to 197. Red color used for error messages.
func ErrorColor(text string, bold bool) string {
	return SetColor(Red, text, bold)
}

// WarningSymbol returns a Yellow colored ! symbol to use for warnings.
func WarningSymbol() string {
	return WarningColor("!", true)
}

// ErrorSymbol returns a Red colored ⨯ symbol to use for errors.
func ErrorSymbol() string {
	return ErrorColor("⨯", true)
}

// SuccessSymbol returns a Green colored ✓ symbol to use for success messages.
func SuccessSymbol() string {
	return SuccessColor("✓", true)
}

// ColorErr is an io.Writer wrapper around os.Stderr that prints the error with colored text.
type ColorErr struct{}

// Write colors the given error red and sends it to os.Stderr.
func (*ColorErr) Write(p []byte) (n int, err error) {
	trimmedErr := strings.TrimSpace(string(p))

	// Cobra allows setting the error prefix using SetErrPrefix() func but
	// it ignores the setting when assigning an empty string.
	// Therefore we can only trim the prefix to be able to set our own
	// customized error prefix using tui styling.
	withoutPrefixErr := strings.TrimPrefix(trimmedErr, "Error: ")

	// In all cases format the error using the standard error colors.
	// But only append the error symbol in case the error prefix got set by the caller.
	// When cobra calls the error's Write, it does it twice when crafting an unknown command's help message.
	// The first line should yield our standard error using the symbol, but all of the following lines should
	// not repeat the prefix using the symbol to keep the output clean.
	if trimmedErr != withoutPrefixErr {
		withoutPrefixErr = SprintError(withoutPrefixErr)
	} else {
		withoutPrefixErr = fmt.Sprintln(ErrorColor(withoutPrefixErr, false))
	}

	return os.Stderr.WriteString(withoutPrefixErr)
}

// PrintWarning calls Println but it appends "! Warning:" to the front of the message.
func PrintWarning(s string) {
	fmt.Println(WarningSymbol(), WarningColor("Warning:", true), WarningColor(s, false))
}

// SprintError crafts the error string without writing it to any output yet.
func SprintError(s string) string {
	return fmt.Sprintln(ErrorSymbol(), ErrorColor("Error:", true), ErrorColor(s, false))
}

// PrintError calls Println but it appends "⨯ Error:" to the front of the message.
func PrintError(s string) {
	fmt.Print(SprintError(s))
}

// Fmt represents the data supplied to ColorPrintf. In particular, it takes a color to apply to the text, and the text itself.
type Fmt struct {
	Color lipgloss.TerminalColor
	Arg   any
	Bold  bool
}

// Printf works like fmt.Sprintf except it applies custom coloring to each argument, and the main template body.
func Printf(template Fmt, args ...Fmt) string {
	var builder strings.Builder

	// Match format directives.
	re := regexp.MustCompile(`%[+]?[vTtbcdoqxXUeEfFgGsqp]`)

	// Split the string on format directives
	parts := re.Split(template.Arg.(string), -1)
	directives := re.FindAllString(template.Arg.(string), -1)

	if len(directives) != len(args) {
		return fmt.Sprintf("Invalid format (expected %d args but found %d)", len(directives), len(args))
	}

	for i, part := range parts {
		styledArg := ""

		// The styling seems to cause newlines to print extra spaces so trim them.
		preNL, ok := strings.CutSuffix(part, "\n ")
		postNL := ""
		if ok {
			postNL = "\n "
		}

		styledTmpl := lipgloss.NewStyle().Bold(template.Bold).SetString(preNL)
		styledTmpl = styledTmpl.Foreground(template.Color)
		styledArg += styledTmpl.String()

		_, _ = builder.WriteString(styledArg + postNL)
		if i < len(args) {
			styledTmpl := lipgloss.NewStyle().Bold(args[i].Bold)
			styledTmpl = styledTmpl.Foreground(args[i].Color)
			_, _ = builder.WriteString(styledTmpl.SetString(fmt.Sprintf(directives[i], args[i].Arg)).String())
		}
	}

	return builder.String()
}
