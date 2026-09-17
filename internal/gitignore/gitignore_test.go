package gitignore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRewritePreservesOutsideBlock(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(p, []byte("keep-me\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Rewrite(p, []string{"foo", "bar"}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "keep-me") || !strings.Contains(s, "foo") {
		t.Fatalf("got %q", s)
	}
	if err := Rewrite(p, []string{"bar"}); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(p)
	s = string(b)
	if strings.Contains(s, "\nfoo\n") || !strings.Contains(s, "bar") || !strings.Contains(s, "keep-me") {
		t.Fatalf("got %q", s)
	}
	if got := Names(p); len(got) != 1 || got[0] != "bar" {
		t.Fatalf("names %v", got)
	}
}
