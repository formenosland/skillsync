package cli

import (
	"strings"
	"testing"
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
	p := &pickList{
		prompt: "remove?",
		items: []pickItem{
			{label: "alpha", value: "alpha", group: "src-a", cat: "tools", on: true},
			{label: "beta", value: "beta", group: "src-a", cat: "web", on: true},
			{label: "gamma", value: "gamma", group: "src-b", cat: "", on: true},
		},
	}
	body := strings.Join(p.lines(style{}), "\n")
	if !strings.Contains(body, "src-a") || !strings.Contains(body, "src-b") {
		t.Fatalf("groups: %s", body)
	}
	if !strings.Contains(body, "tools") || !strings.Contains(body, "web") {
		t.Fatalf("categories: %s", body)
	}
}

func TestPickInstallDefaults(t *testing.T) {
	p := installPickList([]installCand{
		{name: "keep", kind: "new", cat: "tools"},
		{name: "clash", kind: "override", occ: "local", cat: "tools"},
		{name: "other", kind: "new", cat: "web"},
	})
	if got := strings.Join(p.selected(), " "); got != "keep other" {
		t.Fatalf("defaults %q", got)
	}
	body := strings.Join(p.lines(style{}), "\n")
	if !strings.Contains(body, "tools") || !strings.Contains(body, "web") {
		t.Fatalf("categories: %s", body)
	}
	if !strings.Contains(body, "override  local") || !strings.Contains(body, "new") {
		t.Fatalf("hints: %s", body)
	}
}

func installPickList(cands []installCand) *pickList {
	p := &pickList{prompt: "install?"}
	for _, c := range cands {
		it := pickItem{label: c.name, value: c.name, cat: c.cat, hint: "override  " + c.occ}
		if c.kind == "new" {
			it.on = true
			it.hint = "new"
		}
		p.items = append(p.items, it)
	}
	return p
}
