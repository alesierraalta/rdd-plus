//go:build windows

package plan

import (
	"errors"
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

// The standard library's syscall package does not export LockFileEx, so the two calls are reached through the
// same kernel32 entry points it wraps internally. That keeps the dependency list empty, which is the point: a
// locking implementation is not worth a new module.
var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = kernel32.NewProc("LockFileEx")
	procUnlockFileEx = kernel32.NewProc("UnlockFileEx")
)

// lockfileExclusiveLock asks for the exclusive lock. Locking the whole file this way is what flock's LOCK_EX is
// on a Unix host, so the same write path serialises the same way on both.
const lockfileExclusiveLock = 0x00000002

// lockfileFailImmediately makes a contended LockFileEx fail instead of waiting: the bounded wait lives in one
// place, in lockFile, so it behaves the same on every platform.
const lockfileFailImmediately = 0x00000001

// lockViolation is the error a contended LockFileEx answers with when it is told to fail rather than wait.
const lockViolation = syscall.Errno(33)

// tryLockFile asks for the exclusive lock over the whole file without waiting, so the bounded wait stays in one
// place on every platform. A Windows file lock is released when the handle closes or the process exits, so a
// crashed writer never leaves a lock behind, and the lock file itself is never unlinked: removing a file another
// process may already be waiting on is how two writers end up holding two different locks for one plan.
func tryLockFile(file *os.File) error {
	var overlapped syscall.Overlapped
	r1, _, err := procLockFileEx.Call(uintptr(file.Fd()), lockfileExclusiveLock|lockfileFailImmediately, 0, ^uintptr(0), ^uintptr(0), uintptr(unsafe.Pointer(&overlapped)))
	runtime.KeepAlive(&overlapped)
	if r1 == 0 {
		return err
	}
	return nil
}

// isLockHeld says whether an error from tryLockFile means the lock is in someone else's hands.
func isLockHeld(err error) bool {
	return errors.Is(err, lockViolation)
}

// unlockFile releases the lock tryLockFile took, on the same whole-file range and offset the lock was taken with,
// which is what Windows requires for the ranges to match.
func unlockFile(file *os.File) error {
	var overlapped syscall.Overlapped
	r1, _, err := procUnlockFileEx.Call(uintptr(file.Fd()), 0, ^uintptr(0), ^uintptr(0), uintptr(unsafe.Pointer(&overlapped)))
	runtime.KeepAlive(&overlapped)
	if r1 == 0 {
		return err
	}
	return nil
}
