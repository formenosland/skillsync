package gitx

import (
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// SyncState is local clone vs last-fetched origin (no network).
// Values: up to date, behind, ahead, diverged, local, missing.
func SyncState(dir string) string {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return "missing"
	}
	r, err := git.PlainOpen(dir)
	if err != nil {
		return "missing"
	}
	head, err := r.Head()
	if err != nil {
		return "local"
	}
	remoteHash, ok := remoteTracking(r, head)
	if !ok {
		return "local"
	}
	local := head.Hash()
	if local == remoteHash {
		return "up to date"
	}
	locSet := reachable(r, local)
	remSet := reachable(r, remoteHash)
	_, localHasRemote := locSet[remoteHash]
	_, remoteHasLocal := remSet[local]
	switch {
	case localHasRemote && !remoteHasLocal:
		return "ahead"
	case remoteHasLocal && !localHasRemote:
		return "behind"
	default:
		return "diverged"
	}
}

func remoteTracking(r *git.Repository, head *plumbing.Reference) (plumbing.Hash, bool) {
	if head.Name().IsBranch() {
		ref, err := r.Reference(plumbing.NewRemoteReferenceName("origin", head.Name().Short()), true)
		if err == nil {
			return ref.Hash(), true
		}
	}
	ref, err := r.Reference(plumbing.NewRemoteReferenceName("origin", "HEAD"), true)
	if err == nil {
		return ref.Hash(), true
	}
	ref, err = r.Reference("refs/remotes/origin/master", true)
	if err == nil {
		return ref.Hash(), true
	}
	ref, err = r.Reference("refs/remotes/origin/main", true)
	if err == nil {
		return ref.Hash(), true
	}
	return plumbing.ZeroHash, false
}

func reachable(r *git.Repository, from plumbing.Hash) map[plumbing.Hash]struct{} {
	out := map[plumbing.Hash]struct{}{from: {}}
	c, err := r.CommitObject(from)
	if err != nil {
		return out
	}
	iter := object.NewCommitPreorderIter(c, nil, nil)
	_ = iter.ForEach(func(c *object.Commit) error {
		out[c.Hash] = struct{}{}
		return nil
	})
	return out
}
