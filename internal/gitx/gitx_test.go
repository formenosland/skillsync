package gitx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestRelClonePathKeepsHost(t *testing.T) {
	p, err := RelClonePath("https://github.com/acme/skills")
	if err != nil {
		t.Fatal(err)
	}
	if p != "github.com/acme/skills" {
		t.Fatalf("got %q", p)
	}
	p, err = RelClonePath("alice/my-skills")
	if err != nil {
		t.Fatal(err)
	}
	if p != "github.com/alice/my-skills" {
		t.Fatalf("shorthand got %q", p)
	}
}

func TestRelClonePathRejectsDotDot(t *testing.T) {
	_, err := RelClonePath("https://github.com/foo/../../../tmp-evil")
	if err == nil || !strings.Contains(err.Error(), "unsafe path") {
		t.Fatalf("want unsafe path, got %v", err)
	}
}

func TestIsGitURL(t *testing.T) {
	if !IsGitURL("acme/skills") {
		t.Fatal("shorthand")
	}
	if IsGitURL("/tmp/foo") {
		t.Fatal("abs path")
	}
	if IsGitURL("~/dev/skills") {
		t.Fatal("tilde")
	}
}

func TestSyncStateAheadBehind(t *testing.T) {
	dir := t.TempDir()
	r, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	h1 := mustCommit(t, r, dir, "one")
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	h2 := mustCommit(t, r, dir, "two")
	head, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	branch := "master"
	if head.Name().IsBranch() {
		branch = head.Name().Short()
	}
	ref := plumbing.NewHashReference(plumbing.NewRemoteReferenceName("origin", branch), h1)
	if err := r.Storer.SetReference(ref); err != nil {
		t.Fatal(err)
	}
	if got := SyncState(dir); got != "ahead" {
		t.Fatalf("ahead: got %s (h1=%s h2=%s)", got, h1, h2)
	}
	ref = plumbing.NewHashReference(plumbing.NewRemoteReferenceName("origin", branch), h2)
	if err := r.Storer.SetReference(ref); err != nil {
		t.Fatal(err)
	}
	if got := SyncState(dir); got != "up to date" {
		t.Fatalf("up to date: got %s", got)
	}
}

func TestPullCloneFastForwardsBranch(t *testing.T) {
	src := t.TempDir()
	sr, err := git.PlainInit(src, false)
	if err != nil {
		t.Fatal(err)
	}
	mustCommit(t, sr, src, "one")
	clone := t.TempDir()
	if _, err := git.PlainClone(clone, false, &git.CloneOptions{URL: src}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "f"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustCommit(t, sr, src, "two")
	updated, err := pullClone(clone)
	if err != nil {
		t.Fatal(err)
	}
	if !updated {
		t.Fatal("expected worktree update")
	}
	b, err := os.ReadFile(filepath.Join(clone, "f"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "two" {
		t.Fatalf("worktree not updated: %q", b)
	}
}

func TestPullCloneUpdatesDetachedHEAD(t *testing.T) {
	src := t.TempDir()
	sr, err := git.PlainInit(src, false)
	if err != nil {
		t.Fatal(err)
	}
	h1 := mustCommit(t, sr, src, "one")
	clone := t.TempDir()
	cr, err := git.PlainClone(clone, false, &git.CloneOptions{URL: src})
	if err != nil {
		t.Fatal(err)
	}
	cw, err := cr.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := cw.Checkout(&git.CheckoutOptions{Hash: h1}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "f"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustCommit(t, sr, src, "two")
	updated, err := pullClone(clone)
	if err != nil {
		t.Fatal(err)
	}
	if !updated {
		t.Fatal("expected detached clone to follow origin")
	}
	b, err := os.ReadFile(filepath.Join(clone, "f"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "two" {
		t.Fatalf("worktree not updated: %q", b)
	}
}

func mustCommit(t *testing.T, r *git.Repository, dir, msg string) plumbing.Hash {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte(msg), 0o644); err != nil {
		t.Fatal(err)
	}
	w, err := r.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Add("f"); err != nil {
		t.Fatal(err)
	}
	h, err := w.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{Name: "t", Email: "t@t", When: time.Now()},
	})
	if err != nil {
		t.Fatal(err)
	}
	return h
}
