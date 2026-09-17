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

func names(found []Found) map[string]string {
	m := map[string]string{}
	for _, f := range found {
		m[f.Name] = f.Dir
	}
	return m
}

func TestFindInSourceNestedCategories(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "skills", "engineering", "verify"), "verify")
	writeSkill(t, filepath.Join(root, "skills", "productivity", "brainstorming"), "brainstorming")
	writeSkill(t, filepath.Join(root, "skills", "shallow"), "shallow")
	writeSkill(t, filepath.Join(root, "top"), "top")
	writeSkill(t, filepath.Join(root, "ns", "mid", "leaf"), "leaf")
	writeSkill(t, filepath.Join(root, "skills", "engineering", "verify", "examples", "nested"), "nested-example")
	writeSkill(t, filepath.Join(root, ".git", "hidden-skill"), "hidden-skill")
	writeSkill(t, filepath.Join(root, "node_modules", "pkg-skill"), "pkg-skill")

	got := names(FindInSource(root))
	for _, n := range []string{"verify", "brainstorming", "shallow", "top", "leaf"} {
		if _, ok := got[n]; !ok {
			t.Fatalf("missing %s in %v", n, got)
		}
	}
	for _, n := range []string{"nested-example", "hidden-skill", "pkg-skill"} {
		if _, ok := got[n]; ok {
			t.Fatalf("unexpected %s", n)
		}
	}
}

func TestFindInSourceMaxDepth(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "a", "b", "c", "d"), "at-limit")
	writeSkill(t, filepath.Join(root, "a", "b", "c", "d", "e"), "too-deep")
	got := names(FindInSource(root))
	if _, ok := got["at-limit"]; !ok {
		t.Fatalf("depth %d should be found: %v", MaxFindDepth, got)
	}
	if _, ok := got["too-deep"]; ok {
		t.Fatal("skill past the 4-depth limit should be skipped")
	}
}

func TestFindInSourceRootSkill(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "root-skill")
	writeSkill(t, filepath.Join(root, "extra"), "extra")
	got := names(FindInSource(root))
	if len(got) != 1 || got["root-skill"] == "" {
		t.Fatalf("want only root skill, got %v", got)
	}
}
