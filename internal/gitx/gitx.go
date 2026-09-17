package gitx

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/formenosland/skillsync/internal/paths"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

func IsGitURL(s string) bool {
	switch {
	case strings.HasPrefix(s, "http://"), strings.HasPrefix(s, "https://"),
		strings.HasPrefix(s, "git@"), strings.HasPrefix(s, "ssh://"):
		return true
	case paths.LooksLikeLocalPath(s):
		return false
	case strings.Contains(s, "/"):
		return true
	default:
		return false
	}
}

func NormalizeGitURL(s string) string {
	switch {
	case strings.HasPrefix(s, "http://"), strings.HasPrefix(s, "https://"),
		strings.HasPrefix(s, "git@"), strings.HasPrefix(s, "ssh://"):
		return s
	case strings.Contains(s, "/"):
		return "https://github.com/" + s
	default:
		return s
	}
}

// RelClonePath is host/owner/repo (slash-separated) under sources/.
func RelClonePath(raw string) (string, error) {
	u := NormalizeGitURL(raw)
	p := u
	p = strings.TrimPrefix(p, "https://")
	p = strings.TrimPrefix(p, "http://")
	p = strings.TrimPrefix(p, "ssh://git@")
	p = strings.TrimPrefix(p, "ssh://")
	p = strings.TrimPrefix(p, "git@")
	p = strings.Replace(p, ":", "/", 1)
	p = strings.TrimSuffix(p, ".git")
	if i := strings.Index(p, "?"); i >= 0 {
		p = p[:i]
	}
	if paths.HasDotDotSegment(p) {
		return "", fmt.Errorf("unsafe path in source URL: %s", raw)
	}
	if _, err := url.Parse("https://" + strings.TrimPrefix(p, "/")); err != nil && strings.Contains(p, "://") {
		return "", fmt.Errorf("unsafe path in source URL: %s", raw)
	}
	return p, nil
}

func CloneDir(sourcesDir, raw string) (string, error) {
	rel, err := RelClonePath(raw)
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{sourcesDir}, strings.Split(rel, "/")...)...), nil
}

func CloneOrPull(sourcesDir, raw string) error {
	dest, err := CloneDir(sourcesDir, raw)
	if err != nil {
		return err
	}
	if paths.HasDotDotSegment(dest) {
		return fmt.Errorf("unsafe path in source URL: %s", raw)
	}
	base, err := filepath.Abs(sourcesDir)
	if err != nil {
		base = sourcesDir
	}
	absDest, err := filepath.Abs(dest)
	if err != nil {
		absDest = dest
	}
	rel, err := filepath.Rel(base, absDest)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("clone path escapes sources directory: %s", dest)
	}
	remote := NormalizeGitURL(raw)
	if _, err := os.Stat(filepath.Join(dest, ".git")); err == nil {
		r, err := git.PlainOpen(dest)
		if err != nil {
			return err
		}
		w, err := r.Worktree()
		if err != nil {
			return err
		}
		err = w.Pull(&git.PullOptions{RemoteName: "origin"})
		if err == nil || err == git.NoErrAlreadyUpToDate {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	_, err = git.PlainClone(dest, false, &git.CloneOptions{
		URL: remote,
	})
	return err
}

// CheckoutRef checks out a branch, tag, or commit in an existing clone.
func CheckoutRef(cloneDir, ref string) error {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil
	}
	r, err := git.PlainOpen(cloneDir)
	if err != nil {
		return err
	}
	w, err := r.Worktree()
	if err != nil {
		return err
	}
	if plumbing.IsHash(ref) {
		err = w.Checkout(&git.CheckoutOptions{Hash: plumbing.NewHash(ref)})
		if err == nil {
			return nil
		}
	}
	for _, n := range []plumbing.ReferenceName{
		plumbing.NewBranchReferenceName(ref),
		plumbing.NewTagReferenceName(ref),
		plumbing.NewRemoteReferenceName("origin", ref),
	} {
		if err := w.Checkout(&git.CheckoutOptions{Branch: n, Force: true}); err == nil {
			return nil
		}
	}
	return fmt.Errorf("checkout %s: ref %q not found", cloneDir, ref)
}
