//go:build darwin || linux

package e2e_test

import (
	"syscall"
	"unsafe"
)

func ioctl(fd, req, arg uintptr) error {
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, arg); e != 0 {
		return e
	}
	return nil
}

// setSize sets the terminal's size; the kernel sends SIGWINCH to the
// foreground process group.
func setSize(fd uintptr, w, h int) error {
	ws := struct{ rows, cols, x, y uint16 }{uint16(h), uint16(w), 0, 0}
	return ioctl(fd, tiocswinsz, uintptr(unsafe.Pointer(&ws)))
}

func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true, Setctty: true}
}
