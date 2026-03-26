package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cirrusdata/datasim/internal/fileset"
	"github.com/cirrusdata/datasim/pkg/bytefmt"
	"github.com/spf13/cobra"
)

// progressRenderer renders fileset operation progress to stderr.
type progressRenderer struct {
	out        *os.File
	disabled   bool
	finished   bool
	rich       bool
	singleLine bool
	lastRender time.Time
	lastWidth  int
	lastPhase  string
	theme      terminalTheme
}

// newProgressRenderer constructs a fileset progress renderer.
func newProgressRenderer(cmd *cobra.Command) *progressRenderer {
	out := stderrFile(cmd)
	disabled := progressBarDisabled(cmd)
	theme := newTerminalTheme(cmd)
	theme.color = colorEnabled(out, readColorMode(cmd))
	theme.width = terminalWidth(out)

	return &progressRenderer{
		out:        out,
		disabled:   disabled,
		rich:       theme.stderrTTY,
		singleLine: theme.stderrTTY && !disabled,
		theme:      theme,
	}
}

// stderrFile returns the command stderr as an *os.File when available.
func stderrFile(cmd *cobra.Command) *os.File {
	if file, ok := cmd.ErrOrStderr().(*os.File); ok {
		return file
	}

	return os.Stderr
}

// Update renders a progress update.
func (r *progressRenderer) Update(progress fileset.Progress) {
	if r.out == nil || r.disabled {
		return
	}

	now := time.Now()
	if r.singleLine && progress.CompletedItems < progress.TotalItems && progress.CompletedBytes < progress.TotalBytes && now.Sub(r.lastRender) < 120*time.Millisecond {
		return
	}
	if !r.singleLine && progress.Phase == r.lastPhase && now.Sub(r.lastRender) < time.Second {
		return
	}
	r.lastRender = now
	r.lastPhase = progress.Phase

	line := renderProgressLine(progress, r.theme, r.rich)
	if r.singleLine {
		padding := ""
		visible := visibleWidth(line)
		if visible < r.lastWidth {
			padding = strings.Repeat(" ", r.lastWidth-visible)
		}
		fmt.Fprintf(r.out, "\r%s%s", line, padding)
		r.lastWidth = visible
		return
	}

	fmt.Fprintln(r.out, line)
}

// Finish completes progress output with a trailing newline when needed.
func (r *progressRenderer) Finish() {
	if r.finished {
		return
	}
	r.finished = true

	if r.out == nil || r.disabled || !r.singleLine || r.lastWidth == 0 {
		return
	}
	fmt.Fprintln(r.out)
}

// renderProgressLine formats one progress line.
func renderProgressLine(progress fileset.Progress, theme terminalTheme, rich bool) string {
	pct := progressPercent(progress)
	bar := progressBar(pct, 24, rich, theme, progress.Operation)

	parts := []string{
		theme.badge(progress.Operation, progress.Operation),
		theme.phaseStyle(progress.Phase),
		bar,
		theme.bold(fmt.Sprintf("%5.1f%%", pct)),
	}

	if progress.TotalItems > 0 {
		parts = append(parts, theme.muted(fmt.Sprintf("%d/%d items", progress.CompletedItems, progress.TotalItems)))
	}
	if progress.TotalBytes > 0 {
		parts = append(parts, theme.muted(fmt.Sprintf("%s/%s", bytefmt.Format(progress.CompletedBytes), bytefmt.Format(progress.TotalBytes))))
	}
	if progress.CurrentAction != "" {
		parts = append(parts, theme.actionStyle(progress.CurrentAction))
	}
	line := strings.Join(parts, " ")
	if progress.CurrentPath != "" {
		current := theme.truncateForTerminal(progress.CurrentPath, visibleWidth(line)+1)
		line += " " + theme.muted(current)
	}

	return line
}

// progressPercent derives a percentage from bytes when available, otherwise items.
func progressPercent(progress fileset.Progress) float64 {
	if progress.TotalBytes > 0 {
		return boundedPercent(progress.CompletedBytes, progress.TotalBytes)
	}
	if progress.TotalItems > 0 {
		return boundedPercent(int64(progress.CompletedItems), int64(progress.TotalItems))
	}
	return 100
}

// boundedPercent converts a ratio to a progress percentage.
func boundedPercent(completed int64, total int64) float64 {
	if total <= 0 {
		return 100
	}

	pct := 100 * float64(completed) / float64(total)
	if pct < 0 {
		return 0
	}
	if pct > 100 {
		return 100
	}
	return pct
}

// progressBar renders either a Unicode or ASCII progress bar.
func progressBar(percent float64, width int, rich bool, theme terminalTheme, operation string) string {
	if rich {
		return unicodeProgressBar(percent, width, theme, operation)
	}

	return asciiProgressBar(percent, width, theme, operation)
}

// asciiProgressBar renders an ASCII progress bar.
func asciiProgressBar(percent float64, width int, theme terminalTheme, operation string) string {
	if width <= 0 {
		width = 20
	}

	filled := int((percent / 100) * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	if filled == width {
		return theme.progressBarStyle("["+strings.Repeat("=", width)+"]", true, operation)
	}

	left := strings.Repeat("=", filled)
	right := strings.Repeat(" ", max(0, width-filled-1))
	return theme.progressBarStyle("["+left+">"+right+"]", false, operation)
}

// unicodeProgressBar renders a Unicode progress bar for modern terminals.
func unicodeProgressBar(percent float64, width int, theme terminalTheme, operation string) string {
	if width <= 0 {
		width = 16
	}

	filled := int((percent / 100) * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	done := strings.Repeat("█", filled)
	remaining := strings.Repeat("░", max(0, width-filled))
	return theme.progressBarStyle(done, true, operation) + theme.muted(remaining)
}

// truncateMiddle shortens a long path for progress output.
func truncateMiddle(value string, limit int) string {
	if len(value) <= limit || limit < 8 {
		return value
	}

	left := (limit - 3) / 2
	right := limit - 3 - left
	return value[:left] + "..." + value[len(value)-right:]
}
