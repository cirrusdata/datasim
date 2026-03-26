package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	colorModeAuto   = "auto"
	colorModeAlways = "always"
	colorModeNever  = "never"
)

const (
	ansiReset         = "\x1b[0m"
	ansiBold          = "\x1b[1m"
	ansiDim           = "\x1b[2m"
	ansiFgBlack       = "\x1b[30m"
	ansiFgRed         = "\x1b[31m"
	ansiFgGreen       = "\x1b[32m"
	ansiFgYellow      = "\x1b[33m"
	ansiFgBlue        = "\x1b[34m"
	ansiFgMagenta     = "\x1b[35m"
	ansiFgCyan        = "\x1b[36m"
	ansiFgWhite       = "\x1b[37m"
	ansiFgBrightBlack = "\x1b[90m"
	ansiFgBrightWhite = "\x1b[97m"
	ansiFgMuted       = "\x1b[38;5;246m"
	ansiFgKey         = "\x1b[38;5;250m"
	ansiBgRed         = "\x1b[41m"
	ansiBgGreen       = "\x1b[42m"
	ansiBgBlue        = "\x1b[44m"
	ansiBgMagenta     = "\x1b[45m"
	ansiBgCyan        = "\x1b[46m"
)

// detailRow represents one labeled line in styled command output.
type detailRow struct {
	Label  string
	Value  string
	Value2 string
}

// terminalTheme describes how CLI output should be styled for a command.
type terminalTheme struct {
	color     bool
	stdoutTTY bool
	stderrTTY bool
	width     int
}

// newTerminalTheme resolves terminal capabilities and color preferences.
func newTerminalTheme(cmd *cobra.Command) terminalTheme {
	stdout := stdoutFile(cmd)
	stderr := stderrFile(cmd)

	return terminalTheme{
		color:     colorEnabled(stdout, readColorMode(cmd)),
		stdoutTTY: isTerminalFile(stdout),
		stderrTTY: isTerminalFile(stderr),
		width:     terminalWidth(stdout),
	}
}

// stdoutFile returns the command stdout as an *os.File when available.
func stdoutFile(cmd *cobra.Command) *os.File {
	if file, ok := cmd.OutOrStdout().(*os.File); ok {
		return file
	}

	return os.Stdout
}

// readColorMode returns the configured CLI color mode.
func readColorMode(cmd *cobra.Command) string {
	mode, _ := cmd.Flags().GetString("color")
	return normalizeColorMode(mode)
}

// normalizeColorMode canonicalizes a color mode string.
func normalizeColorMode(mode string) string {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	if normalized == "" {
		return colorModeAuto
	}

	return normalized
}

// validateColorMode reports whether a color mode is supported.
func validateColorMode(mode string) error {
	switch normalizeColorMode(mode) {
	case colorModeAuto, colorModeAlways, colorModeNever:
		return nil
	default:
		return fmt.Errorf("invalid color mode %q: expected auto, always, or never", mode)
	}
}

// colorEnabled determines whether ANSI styling should be emitted.
func colorEnabled(file *os.File, mode string) bool {
	switch normalizeColorMode(mode) {
	case colorModeAlways:
		return true
	case colorModeNever:
		return false
	default:
		if os.Getenv("NO_COLOR") != "" {
			return false
		}
		if strings.EqualFold(os.Getenv("TERM"), "dumb") {
			return false
		}
		return isTerminalFile(file)
	}
}

// isTerminalFile reports whether the file descriptor is attached to a terminal.
func isTerminalFile(file *os.File) bool {
	if file == nil {
		return false
	}
	stat, err := file.Stat()
	if err != nil {
		return false
	}

	return stat.Mode()&os.ModeCharDevice != 0
}

// terminalWidth returns the current terminal width when known.
func terminalWidth(file *os.File) int {
	if file != nil && isTerminalFile(file) {
		width, _, err := term.GetSize(int(file.Fd()))
		if err == nil && width > 0 {
			return width
		}
	}

	columns, err := strconv.Atoi(strings.TrimSpace(os.Getenv("COLUMNS")))
	if err == nil && columns > 0 {
		return columns
	}

	if file != nil && isTerminalFile(file) {
		return 80
	}

	return 0
}

// style wraps text in ANSI sequences when color is enabled.
func (t terminalTheme) style(value string, sequences ...string) string {
	if !t.color || value == "" {
		return value
	}

	return strings.Join(sequences, "") + value + ansiReset
}

// bold emphasizes a short value.
func (t terminalTheme) bold(value string) string {
	return t.style(value, ansiBold)
}

// muted renders supporting text in a quieter tone.
func (t terminalTheme) muted(value string) string {
	return t.style(value, ansiFgMuted)
}

// key renders a detail label.
func (t terminalTheme) key(value string) string {
	return t.style(value, ansiBold, ansiFgKey)
}

// success renders a success accent.
func (t terminalTheme) success(value string) string {
	return t.style(value, ansiBold, ansiFgGreen)
}

// info renders a neutral accent.
func (t terminalTheme) info(value string) string {
	return t.style(value, ansiBold, ansiFgBlue)
}

// warning renders a warning accent.
func (t terminalTheme) warning(value string) string {
	return t.style(value, ansiBold, ansiFgYellow)
}

// danger renders an error accent.
func (t terminalTheme) danger(value string) string {
	return t.style(value, ansiBold, ansiFgRed)
}

// badge renders a filled badge for a short status label.
func (t terminalTheme) badge(value string, operation string) string {
	padded := " " + strings.ToUpper(value) + " "
	if !t.color {
		return "[" + strings.ToUpper(value) + "]"
	}

	switch operation {
	case "init":
		return t.style(padded, ansiBold, ansiFgBlack, ansiBgCyan)
	case "rotate":
		return t.style(padded, ansiBold, ansiFgBrightWhite, ansiBgBlue)
	case "destroy":
		return t.style(padded, ansiBold, ansiFgBrightWhite, ansiBgRed)
	case "update":
		return t.style(padded, ansiBold, ansiFgBlack, ansiBgGreen)
	default:
		return t.style(padded, ansiBold, ansiFgBrightWhite, ansiBgMagenta)
	}
}

// phaseStyle renders a phase label with contextual color.
func (t terminalTheme) phaseStyle(phase string) string {
	switch phase {
	case "write", "create":
		return t.style(phase, ansiBold, ansiFgCyan)
	case "mutate":
		return t.style(phase, ansiBold, ansiFgYellow)
	case "delete":
		return t.style(phase, ansiBold, ansiFgRed)
	case "save", "cleanup":
		return t.style(phase, ansiBold, ansiFgMagenta)
	default:
		return t.style(phase, ansiBold, ansiFgWhite)
	}
}

// actionStyle renders a progress action label.
func (t terminalTheme) actionStyle(action string) string {
	return t.style(action, ansiBold, ansiFgBrightWhite)
}

// progressBarStyle renders the filled portion of a progress bar.
func (t terminalTheme) progressBarStyle(value string, complete bool, operation string) string {
	switch operation {
	case "init":
		return t.style(value, ansiBold, ansiFgCyan)
	case "rotate":
		return t.style(value, ansiBold, ansiFgBlue)
	case "destroy":
		return t.style(value, ansiBold, ansiFgRed)
	default:
		if complete {
			return t.style(value, ansiBold, ansiFgGreen)
		}
		return t.style(value, ansiBold, ansiFgWhite)
	}
}

// symbolSuccess returns the preferred success symbol for interactive terminals.
func (t terminalTheme) symbolSuccess() string {
	if !t.stdoutTTY {
		return ""
	}
	return t.success("✓")
}

// symbolInfo returns the preferred info symbol for interactive terminals.
func (t terminalTheme) symbolInfo() string {
	if !t.stdoutTTY {
		return ""
	}
	return t.info("i")
}

// printSuccessBlock prints a structured success result for interactive terminals.
func printSuccessBlock(cmd *cobra.Command, title string, rows ...detailRow) {
	theme := newTerminalTheme(cmd)
	if !theme.stdoutTTY {
		if len(rows) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), title)
			return
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", title, formatRowsInline(rows))
		return
	}

	printBlock(cmd, theme.symbolSuccess(), theme.bold(title), rows...)
}

// printInfoBlock prints a structured informational block for interactive terminals.
func printInfoBlock(cmd *cobra.Command, title string, rows ...detailRow) {
	theme := newTerminalTheme(cmd)
	if !theme.stdoutTTY {
		for _, row := range rows {
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", row.Label, row.Value)
		}
		return
	}

	printBlock(cmd, theme.symbolInfo(), theme.bold(title), rows...)
}

// printBlock prints a styled title line followed by aligned detail rows.
func printBlock(cmd *cobra.Command, icon string, title string, rows ...detailRow) {
	labelWidth := 0
	valueWidth := 0
	for _, row := range rows {
		if len(row.Label) > labelWidth {
			labelWidth = len(row.Label)
		}
		if len(row.Value) > valueWidth {
			valueWidth = len(row.Value)
		}
	}

	if icon == "" {
		fmt.Fprintln(cmd.OutOrStdout(), title)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", icon, title)
	}

	theme := newTerminalTheme(cmd)
	for _, row := range rows {
		label := row.Label
		if padding := labelWidth - len(label); padding > 0 {
			label += strings.Repeat(" ", padding)
		}
		value := row.Value
		if row.Value2 != "" {
			if padding := valueWidth - len(value); padding > 0 {
				value += strings.Repeat(" ", padding)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s  %s\n", theme.key(label), value, theme.muted(row.Value2))
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s\n", theme.key(label), value)
	}
}

// formatRowsInline renders detail rows as label=value pairs.
func formatRowsInline(rows []detailRow) string {
	parts := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Value2 != "" {
			parts = append(parts, fmt.Sprintf("%s=%s %s", row.Label, row.Value, row.Value2))
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", row.Label, row.Value))
	}

	return strings.Join(parts, " ")
}

// truncateForTerminal shortens a string to fit within the active terminal width.
func (t terminalTheme) truncateForTerminal(value string, reserved int) string {
	if t.width <= 0 {
		return value
	}

	limit := t.width - reserved
	if limit <= 0 {
		return ""
	}
	if limit < 8 {
		return truncateMiddle(value, 8)
	}
	return truncateMiddle(value, limit)
}

// visibleWidth returns the printable width of a styled string.
func visibleWidth(value string) int {
	return utf8.RuneCountInString(stripANSI(value))
}

// stripANSI removes ANSI control sequences from a string.
func stripANSI(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))

	inSequence := false
	for _, r := range value {
		switch {
		case !inSequence && r == '\x1b':
			inSequence = true
		case inSequence && ((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')):
			inSequence = false
		case !inSequence:
			builder.WriteRune(r)
		}
	}

	return builder.String()
}
