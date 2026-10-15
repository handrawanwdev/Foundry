package safefile

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = kernel32.NewProc("LockFileEx")
	procUnlockFileEx = kernel32.NewProc("UnlockFileEx")
)

const (
	lockfileFailImmediately = 0x1
	lockfileExclusiveLock   = 0x2
	errorLockViolation      = syscall.Errno(33)
)

// The lock covers one byte far past any content, so the owner can still
// write its PID at the start of the file.
const lockOffsetHigh = 0x7fffffff

func tryLock(f *os.File) (bool, error) {
	ol := syscall.Overlapped{OffsetHigh: lockOffsetHigh}
	r, _, err := procLockFileEx.Call(f.Fd(), lockfileExclusiveLock|lockfileFailImmediately, 0, 1, 0, uintptr(unsafe.Pointer(&ol)))
	if r != 0 {
		return true, nil
	}
	if errors.Is(err, errorLockViolation) {
		return false, nil
	}
	return false, err
}

func unlock(f *os.File) error {
	ol := syscall.Overlapped{OffsetHigh: lockOffsetHigh}
	r, _, err := procUnlockFileEx.Call(f.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(&ol)))
	if r == 0 {
		return err
	}
	return nil
}
