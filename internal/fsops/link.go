package fsops

import (
	"os"
	"path/filepath"
)

// LinkDir makes dest a directory link to src (symlink, junction, or copy).
func LinkDir(src, dest string, copy bool) error {
	if copy {
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		return CopyDir(src, dest)
	}
	abs, err := ResolveDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return dirLink(abs, dest)
}

func ReplaceWithDirLink(src, dest string, copy bool) error {
	_ = os.Remove(dest)
	return LinkDir(src, dest, copy)
}
