package cli

import (
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/term"
)

var errPickAbort = errors.New("selection cancelled")

type keyAction int

const (
	keyNone keyAction = iota
	keyUp
	keyDown
	keyToggle
	keyAll
	keyNoneAll
	keyEnter
	keyAbort
)

type pickItem struct {
	label, hint, cat, value string
	on                      bool
}

type pickList struct {
	prompt string
	items  []pickItem
	cursor int
}

func (p *pickList) apply(k keyAction) (done, abort bool) {
	if len(p.items) == 0 {
		return k == keyEnter, k == keyAbort
	}
	switch k {
	case keyAbort:
		return false, true
	case keyEnter:
		return true, false
	case keyUp:
		if p.cursor > 0 {
			p.cursor--
		}
	case keyDown:
		if p.cursor < len(p.items)-1 {
			p.cursor++
		}
	case keyToggle:
		p.items[p.cursor].on = !p.items[p.cursor].on
	case keyAll:
		for i := range p.items {
			p.items[i].on = true
		}
	case keyNoneAll:
		for i := range p.items {
			p.items[i].on = false
		}
	}
	return false, false
}

func (p *pickList) selected() []string {
	var out []string
	for _, it := range p.items {
		if it.on {
			out = append(out, it.value)
		}
	}
	return out
}

func (p *pickList) lines(ui style) []string {
	out := []string{ui.bold + p.prompt + ui.reset}
	prevCat := "\x00"
	for i, it := range p.items {
		if it.cat != prevCat {
			if it.cat != "" {
				out = append(out, "  "+ui.bold+it.cat+ui.reset)
			}
			prevCat = it.cat
		}
		cur := "  "
		if i == p.cursor {
			cur = ui.cyan + "> " + ui.reset
		}
		mark := ui.dim + "[ ]" + ui.reset
		if it.on {
			mark = ui.green + "[x]" + ui.reset
		}
		hint := ""
		if it.hint != "" {
			hint = "  " + ui.dim + it.hint + ui.reset
		}
		out = append(out, cur+mark+" "+it.label+hint)
	}
	out = append(out, "  "+ui.dim+"space toggle  a all  n none  enter accept  q abort"+ui.reset)
	return out
}

func decodeKey(b []byte) keyAction {
	if len(b) == 0 {
		return keyNone
	}
	switch b[0] {
	case 3, 'q', 'Q':
		return keyAbort
	case '\r', '\n':
		return keyEnter
	case ' ':
		return keyToggle
	case 'a', 'A':
		return keyAll
	case 'n', 'N':
		return keyNoneAll
	case 'j':
		return keyDown
	case 'k':
		return keyUp
	case 0x1b:
		if len(b) >= 3 && b[1] == '[' {
			switch b[2] {
			case 'A':
				return keyUp
			case 'B':
				return keyDown
			}
		}
		if len(b) == 1 {
			return keyAbort
		}
	case 0xe0, 0x00:
		if len(b) >= 2 {
			switch b[1] {
			case 0x48:
				return keyUp
			case 0x50:
				return keyDown
			}
		}
	}
	return keyNone
}

func readKey(in *os.File) (keyAction, error) {
	var first [1]byte
	if _, err := in.Read(first[:]); err != nil {
		return keyAbort, err
	}
	if first[0] != 0x1b && first[0] != 0xe0 && first[0] != 0x00 {
		return decodeKey(first[:]), nil
	}
	_ = in.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	rest := make([]byte, 2)
	n, _ := in.Read(rest)
	_ = in.SetReadDeadline(time.Time{})
	return decodeKey(append(first[:], rest[:n]...)), nil
}

func (a *App) runPick(p *pickList) error {
	in, ok := a.Stdin.(*os.File)
	if !ok {
		return errPickAbort
	}
	fd := int(in.Fd())
	st, err := term.MakeRaw(fd)
	if err != nil {
		return err
	}
	defer func() {
		_ = term.Restore(fd, st)
		fmt.Fprint(a.Stderr, "\033[?25h")
	}()
	fmt.Fprint(a.Stderr, "\033[?25l")
	painted := 0
	paint := func() {
		if painted > 0 {
			fmt.Fprintf(a.Stderr, "\033[%dA\033[J", painted)
		}
		ls := p.lines(a.ui)
		for _, l := range ls {
			fmt.Fprintln(a.Stderr, l)
		}
		painted = len(ls)
	}
	paint()
	for {
		k, err := readKey(in)
		if err != nil {
			return errPickAbort
		}
		done, abort := p.apply(k)
		if abort {
			return errPickAbort
		}
		if done {
			return nil
		}
		if k != keyNone {
			paint()
		}
	}
}

func (a *App) pickMulti(prompt string, items []string) ([]string, error) {
	if len(items) == 0 {
		return nil, nil
	}
	if a.Yes {
		return items, nil
	}
	if !a.interactive() {
		return nil, nil
	}
	p := &pickList{prompt: prompt}
	for _, it := range items {
		p.items = append(p.items, pickItem{label: it, value: it, on: true})
	}
	if err := a.runPick(p); err != nil {
		return nil, err
	}
	return p.selected(), nil
}

func (a *App) pickInstall(prompt string, cands []installCand) ([]string, error) {
	if len(cands) == 0 {
		return nil, nil
	}
	if a.Yes || !a.interactive() {
		var out []string
		for _, c := range cands {
			if c.kind == "new" {
				out = append(out, c.name)
			}
		}
		return out, nil
	}
	p := &pickList{prompt: prompt}
	for _, c := range cands {
		it := pickItem{label: c.name, value: c.name, cat: c.cat, hint: "override  " + c.occ}
		if c.kind == "new" {
			it.on = true
			it.hint = "new"
		}
		p.items = append(p.items, it)
	}
	if err := a.runPick(p); err != nil {
		return nil, err
	}
	return p.selected(), nil
}
