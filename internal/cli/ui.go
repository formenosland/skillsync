package cli

import (
	"fmt"
	"os"
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
		s.dim = "\033[2m"
		s.red = "\033[31m"
		s.green = "\033[32m"
		s.yellow = "\033[33m"
		s.cyan = "\033[36m"
		s.ok, s.warn, s.err = "✓", "▲", "✗"
		s.fancy = true
	}
	return s
}

func (a *App) header(msg string) {
	fmt.Fprintln(a.Stderr, a.ui.bold+msg+a.ui.reset)
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
