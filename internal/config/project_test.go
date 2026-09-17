package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadProjectRejectsUnknownKey(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "skillsync.toml")
	if err := os.WriteFile(p, []byte("[skills]\nnope = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadProject(p)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadProjectRejectsAbsPath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "skillsync.toml")
	if err := os.WriteFile(p, []byte("[skills]\nsources = [{ url = \"/tmp/skills\" }]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadProject(p)
	if err == nil || !strings.Contains(err.Error(), "absolute") {
		t.Fatalf("got %v", err)
	}
}

func TestFindManifest(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	man := filepath.Join(root, ManifestName)
	if err := os.WriteFile(man, []byte("[skills]\nsources = []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gotRoot, gotMan, err := FindManifest(sub)
	if err != nil {
		t.Fatal(err)
	}
	if gotMan != man {
		t.Fatalf("manifest %s", gotMan)
	}
	if gotRoot != root {
		t.Fatalf("root %s want %s", gotRoot, root)
	}
}
