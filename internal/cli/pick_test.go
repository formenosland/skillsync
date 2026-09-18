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
	p := nestPick("remove?", []pickItem{
		{label: "alpha", value: "alpha", group: "src-a", cat: "tools", on: true},
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

func installPickList(cands []installCand) *pickList {
	var skills []pickItem
	for _, c := range cands {
		skills = append(skills, installItem(c))
	}
	return nestPick("install?", skills)
}
