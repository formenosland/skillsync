package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSkill(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + name + "\ndescription: d\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func byName(found []Found) map[string]Found {
	m := map[string]Found{}
	for _, f := range found {
		m[f.Name] = f
	}
	return m
}

func TestFindInSourceLayouts(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "top"), "top")
	writeSkill(t, filepath.Join(root, "skills", "shallow"), "shallow")
	writeSkill(t, filepath.Join(root, "skills", "engineering", "verify"), "verify")
	writeSkill(t, filepath.Join(root, "skills", "productivity", "brainstorming"), "brainstorming")
	writeSkill(t, filepath.Join(root, "ns", "mid", "leaf"), "leaf")
	writeSkill(t, filepath.Join(root, "docs", "random"), "random")
	writeSkill(t, filepath.Join(root, "skills", "engineering", "verify", "examples", "nested"), "nested-example")
	writeSkill(t, filepath.Join(root, "skills", "engineering", "too", "deep"), "too-deep")
	writeSkill(t, filepath.Join(root, ".hidden", "dot-skill"), "dot-skill")
	writeSkill(t, filepath.Join(root, ".git", "hidden-skill"), "hidden-skill")

	got := byName(FindInSource(root))
	want := map[string]string{
		"top":           "",
		"shallow":       "",
		"verify":        "engineering",
		"brainstorming": "productivity",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d skills %v, want %d", len(got), got, len(want))
	}
	for n, cat := range want {
		f, ok := got[n]
		if !ok {
			t.Fatalf("missing %s in %v", n, got)
		}
		if f.Category != cat {
			t.Fatalf("%s category %q want %q", n, f.Category, cat)
		}
	}
}

func TestFindInSourceRootSkill(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "root-skill")
	writeSkill(t, filepath.Join(root, "extra"), "extra")
	got := byName(FindInSource(root))
	if len(got) != 2 || got["root-skill"].Name == "" || got["extra"].Name == "" {
		t.Fatalf("want root and extra, got %v", got)
	}
}

func TestCategory(t *testing.T) {
	root := "/src"
	if g := Category(root, filepath.Join(root, "skills", "engineering", "verify")); g != "engineering" {
		t.Fatalf("got %q", g)
	}
	if g := Category(root, filepath.Join(root, "skills", "verify")); g != "" {
		t.Fatalf("got %q", g)
	}
	if g := Category(root, filepath.Join(root, "verify")); g != "" {
		t.Fatalf("got %q", g)
	}
}

func TestInvocation(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "plain"), "plain")
	user := filepath.Join(root, "user")
	if err := os.MkdirAll(user, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: user\ndescription: d\ndisable-model-invocation: true\n---\n"
	if err := os.WriteFile(filepath.Join(user, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	falsy := filepath.Join(root, "falsy")
	if err := os.MkdirAll(falsy, 0o755); err != nil {
		t.Fatal(err)
	}
	body = "---\nname: falsy\ndescription: d\ndisable-model-invocation: false\n---\n"
	if err := os.WriteFile(filepath.Join(falsy, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if g := Invocation(filepath.Join(root, "plain")); g != "" {
		t.Fatalf("plain: %q", g)
	}
	if g := Invocation(user); g != UserFlag {
		t.Fatalf("user: %q", g)
	}
	if g := Invocation(falsy); g != "" {
		t.Fatalf("falsy: %q", g)
	}
}
