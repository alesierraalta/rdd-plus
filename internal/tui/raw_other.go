//go:build !linux && !darwin

package tui

import (
	"errors"
	"os"
)

// makeRaw refuses a character-device pair here: without a portable ioctl there is no honest
// way to enter raw mode, and silently running cooked would break the key-per-press model.
// Non-terminal files are not an error — they skip raw mode like plain readers do.
func makeRaw(in, out *os.File) (func() error, error) {
	if isCharDevice(in) && isCharDevice(out) {
		return nil, errors.New("interactive mode supports Linux and macOS terminals")
	}
	return nil, errNotTerminal
}

func isCharDevice(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// IsTerminal reports whether f is a terminal this package can drive interactively.
func IsTerminal(f *os.File) bool { return false }
