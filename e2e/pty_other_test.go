//go:build !darwin && !linux

package e2e_test

import (
	"errors"
	"os"
	"syscall"
)

const ptySupported = false

func openPty() (master, slave *os.File, err error) { return nil, nil, errors.ErrUnsupported }

func setSize(fd uintptr, w, h int) error { return errors.ErrUnsupported }

func sysProcAttr() *syscall.SysProcAttr { return nil }
