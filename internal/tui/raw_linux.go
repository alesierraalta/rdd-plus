//go:build linux

package tui

import (
	"errors"
	"os"
	"syscall"
)

func makeRaw(in, out *os.File) (func() error, error) {
	var old syscall.Termios
	if err := ioctlTermios(in.Fd(), uintptr(syscall.TCGETS), &old); err != nil {
		if errors.Is(err, syscall.ENOTTY) {
			return nil, errNotTerminal
		}
		return nil, err
	}
	var peer syscall.Termios
	if err := ioctlTermios(out.Fd(), uintptr(syscall.TCGETS), &peer); err != nil {
		if errors.Is(err, syscall.ENOTTY) {
			return nil, errNotTerminal
		}
		return nil, err
	}
	raw := old
	maskRaw(&raw)
	if err := ioctlTermios(in.Fd(), uintptr(syscall.TCSETS), &raw); err != nil {
		return nil, err
	}
	return func() error {
		return ioctlTermios(in.Fd(), uintptr(syscall.TCSETS), &old)
	}, nil
}
