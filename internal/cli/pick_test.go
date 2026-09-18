package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/formenosland/skillsync/internal/skill"
)

func TestDecodeKey(t *testing.T) {
	cases := []struct {
		in   []byte
		want keyAction
	}{
		{[]byte{3}, keyAbort},
		{[]byte{'q'}, keyAbort},
		{[]byte{0x1b}, keyAbort},
		{[]byte{'\r'}, keyEnter},
		{[]byte{' '}, keyToggle},
		{[]byte{'a'}, keyAll},
		{[]byte{'n'}, keyNoneAll},
		{[]byte{'j'}, keyDown},
		{[]byte{'k'}, keyUp},
		{[]byte{0x1b, '[', 'A'}, keyUp},
		{[]byte{0x1b, '[', 'B'}, keyDown},
		{[]byte{0x1b, '[', '5', '~'}, keyPageUp},
		{[]byte{0x1b, '[', '6', '~'}, keyPageDown},
		{[]byte{0xe0, 0x48}, keyUp},
		{[]byte{'z'}, keyNone},
	}
	for _, c := range cases {
		if got := decodeKey(c.in); got != c.want {
			t.Fatalf("decodeKey(%q)=%d want %d", c.in, got, c.want)
		}
	}
}

func TestPickApply(t *testing.T) {
	p := &pickList{
		items: []pickItem{
			{value: "alpha", on: true},
			{value: "beta", on: false},
			{value: "gamma", on: true},
		},
	}
	p.apply(keyDown)
	if p.cursor != 1 {
		t.Fatalf("cursor %d", p.cursor)
	}
	p.apply(keyToggle)
	if !p.items[1].on {
		t.Fatal("toggle on")
	}
	p.apply(keyNoneAll)
	if p.items[0].on || p.items[1].on || p.items[2].on {
		t.Fatal("none")
	}
	p.apply(keyAll)
	for _, it := range p.items {
		if !it.on {
			t.Fatal("all")
		}
	}
	p.apply(keyUp)
	p.apply(keyUp)
	p.apply(keyUp)
	if p.cursor != 0 {
		t.Fatalf("cursor clamp %d", p.cursor)
	}
	done, abort := p.apply(keyEnter)
	if !done || abort {
		t.Fatal("enter")
	}
	_, abort = p.apply(keyAbort)
	if !abort {
		t.Fatal("abort")
	}
}

func TestPickGroupAndCategory(t *testing.T) {
	p := nestPick("remove?", []pickItem{
		{label: "alpha", value: "alpha", group: "src-a", cat: "tools", invoke: skill.UserFlag, hint: "Alpha skill", on: true},
		{label: "beta", value: "beta", group: "src-a", cat: "web", on: true},
		{label: "gamma", value: "gamma", group: "src-b", cat: "", on: true},
	})
	body := strings.Join(p.lines(style{}), "\n")
	if !strings.Contains(body, "src-a") || !strings.Contains(body, "src-b") {
		t.Fatalf("groups: %s", body)
	}
	if !strings.Contains(body, "tools") || !strings.Contains(body, "web") {
		t.Fatalf("categories: %s", body)
	}
	if !strings.Contains(body, skill.UserFlag) || !strings.Contains(body, "Alpha skill") {
		t.Fatalf("invocation: %s", body)
	}
}

func TestPickToggleCategory(t *testing.T) {
	p := nestPick("install?", []pickItem{
		{label: "keep", value: "keep", cat: "tools", on: true},
		{label: "clash", value: "clash", cat: "tools", on: false},
		{label: "other", value: "other", cat: "web", on: true},
	})
	if p.items[0].kind != rowCat || p.items[0].label != "tools" {
		t.Fatalf("header %+v", p.items[0])
	}
	p.apply(keyToggle)
	if got := strings.Join(p.selected(), " "); got != "keep clash other" {
		t.Fatalf("all tools on: %q", got)
	}
	p.apply(keyToggle)
	if got := strings.Join(p.selected(), " "); got != "other" {
		t.Fatalf("tools off: %q", got)
	}
}

func TestPickInstallDefaults(t *testing.T) {
	p := installPickList([]installCand{
		{name: "keep", kind: "new", cat: "tools", blurb: "Keep skill description"},
		{name: "clash", kind: "override", occ: "local", cat: "tools", invoke: skill.UserFlag},
		{name: "other", kind: "new", cat: "web", blurb: "Other skill description"},
	})
	if got := strings.Join(p.selected(), " "); got != "keep other" {
		t.Fatalf("defaults %q", got)
	}
	body := strings.Join(p.lines(style{}), "\n")
	if !strings.Contains(body, "tools") || !strings.Contains(body, "web") {
		t.Fatalf("categories: %s", body)
	}
	if !strings.Contains(body, "override") || !strings.Contains(body, "local") || !strings.Contains(body, "new") {
		t.Fatalf("hints: %s", body)
	}
	if !strings.Contains(body, "Keep skill description") || !strings.Contains(body, "Other skill description") {
		t.Fatalf("blurbs: %s", body)
	}
	if !strings.Contains(body, skill.UserFlag) {
		t.Fatalf("invocation: %s", body)
	}
	var keepLine, clashLine string
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, "keep") && strings.Contains(line, "new") {
			keepLine = line
		}
		if strings.Contains(line, "clash") && strings.Contains(line, "override") {
			clashLine = line
		}
	}
	if keepLine == "" || clashLine == "" {
		t.Fatalf("rows: %s", body)
	}
	if strings.Index(keepLine, "new") != strings.Index(clashLine, "override") {
		t.Fatalf("status unaligned:\n%s\n%s", keepLine, clashLine)
	}
	var occLine string
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, "local") && !strings.Contains(line, "clash") {
			occLine = line
		}
	}
	if occLine == "" || !strings.Contains(occLine, occPrefix) || strings.Contains(occLine, "Keep skill") {
		t.Fatalf("occupant line: %s", body)
	}
}

func TestPickLockedInstalled(t *testing.T) {
	p := installPickList([]installCand{
		{name: "keep", kind: "have", cat: "tools"},
		{name: "more", kind: "new", cat: "tools"},
	})
	if p.items[0].kind != rowCat {
		t.Fatalf("header: %+v", p.items[0])
	}
	p.apply(keyDown)
	if p.items[p.cursor].value != "more" {
		t.Fatalf("skip locked, cursor %+v", p.items[p.cursor])
	}
	p.cursor = 1
	p.apply(keyToggle)
	if !p.items[1].on || !p.items[1].locked {
		t.Fatal("locked flipped")
	}
	if got := strings.Join(p.selected(), " "); got != "more" {
		t.Fatalf("selected %q", got)
	}
	if !installPickable([]installCand{{kind: "have"}, {kind: "new"}}) {
		t.Fatal("pickable")
	}
	if installPickable([]installCand{{kind: "have"}}) {
		t.Fatal("all have")
	}
}

func TestPickViewport(t *testing.T) {
	var skills []pickItem
	for i := 0; i < 20; i++ {
		n := fmt.Sprintf("s%02d", i)
		skills = append(skills, pickItem{label: n, value: n, on: true})
	}
	p := nestPick("pick?", skills)
	ls := p.viewLines(style{}, 5)
	if len(ls) != 5 {
		t.Fatalf("height cap %d: %v", len(ls), ls)
	}
	body := strings.Join(ls, "\n")
	if !strings.Contains(body, "pick?") || !strings.Contains(body, "s00") {
		t.Fatalf("top of window: %s", body)
	}
	if strings.Contains(body, "s19") {
		t.Fatalf("overflowed: %s", body)
	}
	p.page = 3
	p.apply(keyPageDown)
	ls = p.viewLines(style{}, 5)
	body = strings.Join(ls, "\n")
	if p.cursor < 3 {
		t.Fatalf("page down cursor %d", p.cursor)
	}
	if !strings.Contains(body, p.items[p.cursor].label) {
		t.Fatalf("cursor row missing: cursor=%d %s", p.cursor, body)
	}
}

func installPickList(cands []installCand) *pickList {
	var skills []pickItem
	for _, c := range cands {
		skills = append(skills, installItem(c))
	}
	return nestPick("install?", skills)
}
