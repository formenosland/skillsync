package fsops

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func IsSymlink(p string) bool {
	st, err := os.Lstat(p)
	return err == nil && st.Mode()&os.ModeSymlink != 0
}

func IsRealDir(p string) bool {
	st, err := os.Lstat(p)
	return err == nil && st.IsDir() && st.Mode()&os.ModeSymlink == 0
}

func ResolveDir(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

func PathsEqual(a, b string) bool {
	ra, err := ResolveDir(a)
	if err != nil {
		ra = a
	}
	rb, err := ResolveDir(b)
	if err != nil {
		rb = b
	}
	return ra == rb
}

func Exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func Readlink(p string) string {
	t, err := os.Readlink(p)
	if err != nil {
		return ""
	}
	return t
}

func CopyDir(src, dest string) error {
	src = filepath.Clean(src)
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		out := dest
		if rel != "." {
			out = filepath.Join(dest, rel)
		}
		if d.Type()&os.ModeSymlink != 0 {
			t, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(t, out)
		}
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		return copyFile(path, out)
	})
}

func CopyDirFollow(src, dest string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return CopyDir(src, dest)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return copyFile(src, dest)
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	st, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, st.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func UnderDir(child, parent string) bool {
	c, err := ResolveDir(child)
	if err != nil {
		c = filepath.Clean(child)
	}
	p, err := ResolveDir(parent)
	if err != nil {
		p = filepath.Clean(parent)
	}
	rel, err := filepath.Rel(p, c)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
