//go:build darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd

package plan

import (
	"errors"
	"os"
	"syscall"
)

// tryLockFile asks for the exclusive advisory lock without waiting, and reports the contention as an error the
// bounded wait in lockFile knows how to retry.
//
// The platforms that carry `flock` are listed one by one rather than covered by a broad `unix` tag, because the
// rest of that family (aix, solaris) has no Flock and would not compile; those targets get the fail-closed
// implementation instead of a build that pretends to lock.
//
// The kernel releases a flock when the holding process exits, so a writer that crashes mid-transaction leaves
// nothing for the next writer to clean up, and the lock file itself is never unlinked: removing a file another
// process may already be waiting on is how two writers end up holding two different locks for one plan.
func tryLockFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

// isLockHeld says whether an error from tryLockFile means the lock is in someone else's hands, which is the one
// failure a bounded wait should retry. A signal interrupts the request instead of refusing it, so it is a retry too.
func isLockHeld(err error) bool {
	return errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EINTR)
}

// unlockFile releases the lock lockFile took. Closing the descriptor releases it anyway; this is the explicit
// half, so the lock is given back before the file that carries it is closed.
func unlockFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}
