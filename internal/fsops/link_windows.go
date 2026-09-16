//go:build windows

package fsops

import (
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	fsctlSetReparsePoint   = 0x000900A4
	ioReparseTagMountPoint = 0xA0000003
	errnoPriv              = 1314 // ERROR_PRIVILEGE_NOT_HELD
)

func dirLink(target, link string) error {
	err := os.Symlink(target, link)
	if err == nil {
		return nil
	}
	if errno, ok := err.(syscall.Errno); ok && errno == errnoPriv {
		return createJunction(link, target)
	}
	var se syscall.Errno
	if errorsAsErrno(err, &se) && se == errnoPriv {
		return createJunction(link, target)
	}
	return err
}

func errorsAsErrno(err error, e *syscall.Errno) bool {
	for err != nil {
		if n, ok := err.(syscall.Errno); ok {
			*e = n
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

type reparseDataBuffer struct {
	ReparseTag           uint32
	ReparseDataLength    uint16
	Reserved             uint16
	SubstituteNameOffset uint16
	SubstituteNameLength uint16
	PrintNameOffset      uint16
	PrintNameLength      uint16
	PathBuffer           [16384 / 2]uint16
}

func createJunction(link, target string) error {
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if err := os.Mkdir(link, 0o755); err != nil {
		return err
	}
	nt := `\??\` + targetAbs
	sub, err := windows.UTF16FromString(nt)
	if err != nil {
		return err
	}
	prt, err := windows.UTF16FromString(targetAbs)
	if err != nil {
		return err
	}
	sub = sub[:len(sub)-1]
	prt = prt[:len(prt)-1]

	var buf reparseDataBuffer
	buf.ReparseTag = ioReparseTagMountPoint
	buf.SubstituteNameOffset = 0
	buf.SubstituteNameLength = uint16(len(sub) * 2)
	buf.PrintNameOffset = uint16((len(sub) + 1) * 2)
	buf.PrintNameLength = uint16(len(prt) * 2)
	copy(buf.PathBuffer[:], sub)
	copy(buf.PathBuffer[len(sub)+1:], prt)
	buf.ReparseDataLength = 8 + buf.PrintNameOffset + buf.PrintNameLength

	pathp, err := windows.UTF16PtrFromString(link)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(pathp, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	var ret uint32
	return windows.DeviceIoControl(h, fsctlSetReparsePoint, (*byte)(unsafe.Pointer(&buf)),
		uint32(8+buf.ReparseDataLength), nil, 0, &ret, nil)
}
