//go:build darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd

package plan

import (
	"os"
	"syscall"
)

// lockFile takes the exclusive advisory lock on an open lock file, waiting for whoever holds it.
//
// The platforms that carry `flock` are listed one by one rather than covered by a broad `unix` tag, because the
// rest of that family (aix, solaris) has no Flock and would not compile; those targets get the fail-closed
// implementation instead of a build that pretends to lock.
//
// The kernel releases a flock when the holding process exits, so a writer that crashes mid-transaction leaves
// nothing for the next writer to clean up, and the lock file itself is never unlinked: removing a file another
// process may already be waiting on is how two writers end up holding two different locks for one plan.
func lockFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX)
}

// unlockFile releases the lock lockFile took. Closing the descriptor releases it anyway; this is the explicit
// half, so the lock is given back before the file that carries it is closed.
func unlockFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}
