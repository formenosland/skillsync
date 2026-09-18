package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type style struct {
	reset, bold, dim, red, green, yellow, cyan string
	ok, warn, err                              string
	fancy                                      bool
}

func newStyle(stderrTTY bool) style {
	s := style{ok: "+", warn: "!", err: "x"}
	if stderrTTY && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb" {
		s.reset = "\033[0m"
		s.bold = "\033[1m"
		s.dim = mutedColor()
		s.red = "\033[31m"
		s.green = "\033[32m"
		s.yellow = "\033[33m"
		s.cyan = "\033[36m"
		s.ok, s.warn, s.err = "✓", "▲", "✗"
		s.fancy = true
	}
	return s
}

// mutedColor is secondary text that stays readable on glass/transparent
// terminals. SGR 2 (faint) multiplies the cell against the background and
// often disappears there. COLORFGBG (xterm/konsole) picks a light vs dark tone.
func mutedColor() string {
	bg := -1
	if v := os.Getenv("COLORFGBG"); v != "" {
		if i := strings.LastIndex(v, ";"); i >= 0 {
			if n, err := strconv.Atoi(v[i+1:]); err == nil {
				bg = n
			}
		}
	}
	if bg >= 8 {
		return "\033[38;5;60m"
	}
	return "\033[38;5;146m"
}

func (a *App) ok(msg string) {
	a.nOK++
	fmt.Fprintf(a.Stderr, "  %s%s%s %s\n", a.ui.green, a.ui.ok, a.ui.reset, msg)
}

func (a *App) info(msg string) {
	fmt.Fprintf(a.Stderr, "  %s%s%s\n", a.ui.dim, msg, a.ui.reset)
}

func (a *App) warn(msg string) {
	a.nWarn++
	fmt.Fprintf(a.Stderr, "  %s%s%s %s\n", a.ui.yellow, a.ui.warn, a.ui.reset, msg)
}

func (a *App) err(msg string) {
	fmt.Fprintf(a.Stderr, "  %s%s%s %s\n", a.ui.red, a.ui.err, a.ui.reset, msg)
}

func (a *App) end(msg string) {
	fmt.Fprintln(a.Stderr, msg)
}
