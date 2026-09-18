package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

const occPrefix = "occupied by "

var errPickAbort = errors.New("selection cancelled")

type keyAction int

const (
	keyNone keyAction = iota
	keyUp
	keyDown
	keyPageUp
	keyPageDown
	keyToggle
	keyAll
	keyNoneAll
	keyEnter
	keyAbort
)

type rowKind int

const (
	rowSkill rowKind = iota
	rowGroup
	rowCat
)

type pickItem struct {
	kind                                                rowKind
	label, hint, invoke, status, occ, group, cat, value string
	on, locked                                          bool
}

type pickList struct {
	prompt string
	items  []pickItem
	cursor int
	view   int // first visible item
	page   int // visible item rows (for PgUp/PgDn)
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
		p.move(-1)
	case keyDown:
		p.move(1)
	case keyPageUp:
		n := p.page
		if n < 1 {
			n = 1
		}
		p.move(-n)
	case keyPageDown:
		n := p.page
		if n < 1 {
			n = 1
		}
		p.move(n)
	case keyToggle:
		p.toggleAt(p.cursor)
	case keyAll:
		p.setSkills(func(pickItem) bool { return true }, true)
	case keyNoneAll:
		p.setSkills(func(pickItem) bool { return true }, false)
	}
	return false, false
}

func (p *pickList) rowLocked(i int) bool {
	if i < 0 || i >= len(p.items) {
		return true
	}
	it := p.items[i]
	if p.isSkill(it) {
		return it.locked
	}
	for _, s := range p.items {
		if p.covers(it, s) && !s.locked {
			return false
		}
	}
	return true
}

func (p *pickList) move(delta int) {
	i := p.cursor + delta
	for i >= 0 && i < len(p.items) {
		if !p.rowLocked(i) {
			p.cursor = i
			return
		}
		i += delta
	}
}

func (p *pickList) snapCursor() {
	if !p.rowLocked(p.cursor) {
		return
	}
	p.move(1)
	if !p.rowLocked(p.cursor) {
		return
	}
	p.move(-1)
}

func (p *pickList) isSkill(it pickItem) bool {
	return it.kind == rowSkill
}

func (p *pickList) covers(h, s pickItem) bool {
	if !p.isSkill(s) {
		return false
	}
	switch h.kind {
	case rowGroup:
		return s.group == h.group
	case rowCat:
		return s.group == h.group && s.cat == h.cat
	default:
		return false
	}
}

func (p *pickList) setSkills(match func(pickItem) bool, on bool) {
	for i, it := range p.items {
		if p.isSkill(it) && !it.locked && match(it) {
			p.items[i].on = on
		}
	}
}

func (p *pickList) childState(h pickItem) (all, any bool) {
	n, on := 0, 0
	for _, s := range p.items {
		if !p.covers(h, s) || s.locked {
			continue
		}
		n++
		if s.on {
			on++
		}
	}
	return n > 0 && on == n, on > 0
}

func (p *pickList) toggleAt(i int) {
	if p.rowLocked(i) {
		return
	}
	it := p.items[i]
	if p.isSkill(it) {
		p.items[i].on = !it.on
		return
	}
	all, _ := p.childState(it)
	p.setSkills(func(s pickItem) bool { return p.covers(it, s) }, !all)
}

func (p *pickList) selected() []string {
	var out []string
	for _, it := range p.items {
		if p.isSkill(it) && it.on && !it.locked && it.value != "" {
			out = append(out, it.value)
		}
	}
	return out
}

func nestPick(prompt string, skills []pickItem) *pickList {
	p := &pickList{prompt: prompt}
	prevG, prevC := "\x00", "\x00"
	for _, s := range skills {
		s.kind = rowSkill
		if s.group != prevG {
			if s.group != "" {
				p.items = append(p.items, pickItem{kind: rowGroup, label: s.group, group: s.group})
			}
			prevG = s.group
			prevC = "\x00"
		}
		if s.cat != prevC {
			if s.cat != "" {
				p.items = append(p.items, pickItem{kind: rowCat, label: s.cat, group: s.group, cat: s.cat})
			}
			prevC = s.cat
		}
		p.items = append(p.items, s)
	}
	p.snapCursor()
	return p
}

func (it pickItem) indent() string {
	switch it.kind {
	case rowGroup:
		return ""
	case rowCat:
		if it.group != "" {
			return "  "
		}
		return ""
	default:
		if it.group != "" && it.cat != "" {
			return "    "
		}
		if it.group != "" || it.cat != "" {
			return "  "
		}
		return ""
	}
}

func padRight(s string, w int) string {
	if n := w - len(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

func (p *pickList) skillPad() (nameW, flagW, statusW int) {
	for _, it := range p.items {
		if it.kind != rowSkill {
			continue
		}
		if n := len(it.label); n > nameW {
			nameW = n
		}
		if n := len(it.invoke); n > flagW {
			flagW = n
		}
		if n := len(it.status); n > statusW {
			statusW = n
		}
	}
	if nameW > 32 {
		nameW = 32
	}
	return
}

func (p *pickList) skillAfterName(it pickItem) string {
	_, flagW, statusW := p.skillPad()
	var b strings.Builder
	if flagW > 0 {
		b.WriteString("  ")
		b.WriteString(padRight(it.invoke, flagW))
	}
	if statusW > 0 {
		b.WriteString("  ")
		b.WriteString(padRight(it.status, statusW))
	}
	if it.hint != "" {
		b.WriteString("  ")
		b.WriteString(it.hint)
	}
	return b.String()
}

func (p *pickList) mark(ui style, it pickItem) string {
	on := it.on
	if it.kind != rowSkill {
		all, any := p.childState(it)
		if all {
			on = true
		} else if any {
			return ui.dim + "[-]" + ui.reset
		} else {
			on = false
		}
	}
	if on {
		return ui.green + "[x]" + ui.reset
	}
	return ui.dim + "[ ]" + ui.reset
}

func (p *pickList) rowLine(ui style, i int) string {
	it := p.items[i]
	cur := "  "
	if i == p.cursor {
		cur = ui.cyan + "> " + ui.reset
	}
	label := it.label
	rest := ""
	if it.kind != rowSkill {
		label = ui.bold + it.label + ui.reset
		if it.hint != "" {
			rest = "  " + it.hint
		}
	} else {
		nameW, _, _ := p.skillPad()
		label = padRight(it.label, nameW)
		rest = p.skillAfterName(it)
	}
	hint := ""
	if rest != "" {
		hint = ui.dim + rest + ui.reset
	}
	line := cur + it.indent() + p.mark(ui, it) + " " + label + hint
	if p.rowLocked(i) {
		mk := "[x]"
		if !it.on && it.kind == rowSkill {
			mk = "[ ]"
		}
		lockedLabel := it.label
		lockedRest := it.hint
		if it.kind == rowSkill {
			nameW, _, _ := p.skillPad()
			lockedLabel = padRight(it.label, nameW)
			lockedRest = p.skillAfterName(it)
		} else if lockedRest != "" {
			lockedRest = "  " + lockedRest
		}
		line = ui.dim + "  " + it.indent() + mk + " " + lockedLabel + lockedRest + ui.reset
	}
	return line
}

func (p *pickList) rowLines(ui style, i int) []string {
	line := p.rowLine(ui, i)
	it := p.items[i]
	if it.kind != rowSkill || it.occ == "" {
		return []string{line}
	}
	sub := ui.dim + "  " + it.indent() + "    " + occPrefix + it.occ + ui.reset
	return []string{line, sub}
}

func (p *pickList) clipView(rows int) {
	n := len(p.items)
	if rows < 1 {
		rows = 1
	}
	p.page = rows
	if n <= rows {
		p.view = 0
		return
	}
	if p.cursor < p.view {
		p.view = p.cursor
	}
	if p.cursor >= p.view+rows {
		p.view = p.cursor - rows + 1
	}
	if p.view < 0 {
		p.view = 0
	}
	if max := n - rows; p.view > max {
		p.view = max
	}
}

func (p *pickList) lines(ui style) []string {
	return p.viewLines(ui, 0)
}

func (p *pickList) viewLines(ui style, height int) []string {
	help := "  " + ui.dim + "space toggle (incl. category)  a all  n none  enter accept  q abort" + ui.reset
	out := []string{ui.bold + p.prompt + ui.reset}
	n := len(p.items)
	start, end := 0, n
	if height > 0 {
		rows := height - 2
		if rows < 1 {
			rows = 1
		}
		p.clipView(rows)
		start, end = p.view, p.view+rows
		if end > n {
			end = n
		}
		if start > 0 || end < n {
			help = "  " + ui.dim + "↑↓ pgup/pgdn  space toggle  a all  n none  enter accept  q abort" + ui.reset
		}
	}
	for i := start; i < end; i++ {
		out = append(out, p.rowLines(ui, i)...)
	}
	out = append(out, help)
	if height > 0 && len(out) > height {
		out = out[:height]
	}
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
			case '5':
				if len(b) >= 4 && b[3] == '~' {
					return keyPageUp
				}
			case '6':
				if len(b) >= 4 && b[3] == '~' {
					return keyPageDown
				}
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
	rest := make([]byte, 3)
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
		fmt.Fprint(a.Stderr, "\n\033[?25h")
	}()
	fmt.Fprint(a.Stderr, "\033[?25l")
	height := 24
	if f, ok := a.Stderr.(*os.File); ok {
		if _, h, err := term.GetSize(int(f.Fd())); err == nil && h > 0 {
			height = h
		}
	}
	painted := 0
	erase := func() {
		if painted <= 0 {
			return
		}
		if painted == 1 {
			fmt.Fprint(a.Stderr, "\r\033[J")
			return
		}
		fmt.Fprintf(a.Stderr, "\r\033[%dA\033[J", painted-1)
	}
	paint := func() {
		erase()
		ls := p.viewLines(a.ui, height)
		n := len(ls)
		for i, l := range ls {
			if i == n-1 {
				fmt.Fprintf(a.Stderr, "%s\r", l)
			} else {
				fmt.Fprintf(a.Stderr, "%s\r\n", l)
			}
		}
		painted = n
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

func (a *App) pickSkills(prompt string) ([]string, error) {
	names := a.listNames()
	if len(names) == 0 {
		return nil, nil
	}
	if a.Yes {
		return names, nil
	}
	if !a.interactive() {
		return nil, nil
	}
	var skills []pickItem
	for _, r := range a.catalogRows() {
		skills = append(skills, pickItem{label: r.name, value: r.name, group: r.group, cat: r.cat, invoke: r.invoke, hint: r.blurb, on: true})
	}
	p := nestPick(prompt, skills)
	if err := a.runPick(p); err != nil {
		return nil, err
	}
	return p.selected(), nil
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

func installPickable(cands []installCand) bool {
	for _, c := range cands {
		if c.kind == "new" || c.kind == "override" {
			return true
		}
	}
	return false
}

func installItem(c installCand) pickItem {
	it := pickItem{label: c.name, value: c.name, cat: c.cat, invoke: c.invoke, hint: c.blurb}
	switch c.kind {
	case "have":
		it.on = true
		it.locked = true
		it.status = "installed"
	case "new":
		it.on = true
		it.status = "new"
	default:
		it.status = "override"
		it.occ = c.occ
	}
	return it
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
	var skills []pickItem
	for _, c := range cands {
		skills = append(skills, installItem(c))
	}
	p := nestPick(prompt, skills)
	if err := a.runPick(p); err != nil {
		return nil, err
	}
	return p.selected(), nil
}
