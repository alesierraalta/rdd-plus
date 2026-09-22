//go:build darwin

package tui

import (
	"errors"
	"os"
	"syscall"
)

// Darwin spells the termios ioctls TIOCGETA/TIOCSETA where Linux uses TCGETS/TCSETS.
func makeRaw(in, out *os.File) (func() error, error) {
	var old syscall.Termios
	if err := ioctlTermios(in.Fd(), uintptr(syscall.TIOCGETA), &old); err != nil {
		if errors.Is(err, syscall.ENOTTY) {
			return nil, errNotTerminal
		}
		return nil, err
	}
	var peer syscall.Termios
	if err := ioctlTermios(out.Fd(), uintptr(syscall.TIOCGETA), &peer); err != nil {
		if errors.Is(err, syscall.ENOTTY) {
			return nil, errNotTerminal
		}
		return nil, err
	}
	raw := old
	maskRaw(&raw)
	if err := ioctlTermios(in.Fd(), uintptr(syscall.TIOCSETA), &raw); err != nil {
		return nil, err
	}
	return func() error {
		return ioctlTermios(in.Fd(), uintptr(syscall.TIOCSETA), &old)
	}, nil
}

// IsTerminal reports whether f is a terminal this package can drive interactively.
func IsTerminal(f *os.File) bool {
	var t syscall.Termios
	return ioctlTermios(f.Fd(), uintptr(syscall.TIOCGETA), &t) == nil
}
