// Package skilltree compares an embedded skill tree with the copy a host has installed.
package skilltree

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
)

// Identical reports whether every embedded file of the skill exists at target with the same bytes; files the
// user added next to them (run artifacts, notes) do not count as a difference.
//
// It is the one reading of that comparison. `doctor` and `sync` ask the same question about the same tree, and
// asking it twice is how two answers drift; the failure they already drifted on is the error below: one caller
// dropped it and the other returned it.
//
// The error is returned rather than decided here. A tree that cannot be walked is not an answer, and what to do
// with that belongs to the caller: the doctor refuses to call a skill verified it never read, and sync refuses to
// touch a copy it could not compare. `true` is never returned beside an error — identical means the whole tree
// was read and matched, and a caller cannot be asked to remember that.
func Identical(skills fs.FS, name, target string) (bool, error) {
	if _, err := os.Stat(target); err != nil {
		return false, nil
	}
	same := true
	err := fs.WalkDir(skills, name, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !same {
			return err
		}
		want, err := fs.ReadFile(skills, p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(name, filepath.FromSlash(p))
		got, err := os.ReadFile(filepath.Join(target, rel))
		if err != nil || !bytes.Equal(want, got) {
			same = false
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	return same, nil
}
