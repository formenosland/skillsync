//go:build !windows

package fsops

import "os"

func dirLink(target, link string) error {
	return os.Symlink(target, link)
}
