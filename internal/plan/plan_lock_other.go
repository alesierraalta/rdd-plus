//go:build !(darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd || windows)

package plan

import (
	"errors"
	"os"
	"runtime"
)

// errNoPlanLock is what every other target gets, and it fails closed on purpose: this package has no
// cross-process lock on this platform, and a plan written without serialization is the silently lost row the
// lock exists to prevent. Refusing to write is recoverable; a plan that quietly dropped a finding is not.
var errNoPlanLock = errors.New("rdd-plus has no cross-process file lock on " + runtime.GOOS + ", so it refuses to write a plan without serialization")

// tryLockFile refuses: there is no lock to take here.
func tryLockFile(*os.File) error { return errNoPlanLock }

// isLockHeld says whether an error from tryLockFile means the lock is in someone else's hands: here it never does,
// so the bounded wait returns the refusal immediately instead of retrying it.
func isLockHeld(err error) bool { return false }

// unlockFile refuses for symmetry. It is only reached when lockFile would have refused first.
func unlockFile(*os.File) error { return errNoPlanLock }
