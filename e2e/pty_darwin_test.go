package e2e_test

import (
	"bytes"
	"os"
	"syscall"
	"unsafe"
)

const (
	tiocptygrant = 0x20007454
	tiocptyunlk  = 0x20007452
	tiocptygname = 0x40807453
	tiocswinsz   = 0x80087467
)

const ptySupported = true

// openPty opens a pseudo-terminal pair with the standard library only.
func openPty() (master, slave *os.File, err error) {
	m, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	fd := m.Fd()
	if err := ioctl(fd, tiocptygrant, 0); err != nil {
		m.Close()
		return nil, nil, err
	}
	if err := ioctl(fd, tiocptyunlk, 0); err != nil {
		m.Close()
		return nil, nil, err
	}
	buf := make([]byte, 128)
	if err := ioctl(fd, tiocptygname, uintptr(unsafe.Pointer(&buf[0]))); err != nil {
		m.Close()
		return nil, nil, err
	}
	name := string(buf[:bytes.IndexByte(buf, 0)])
	s, err := os.OpenFile(name, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		m.Close()
		return nil, nil, err
	}
	return m, s, nil
}
