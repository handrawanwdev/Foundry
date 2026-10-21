// Package safefile holds the two file primitives shared by every process
// that touches Foundry state: an exclusive cross-process lock and an atomic
// file replace.
//
// The lock is an OS lock (flock on Unix, LockFileEx on Windows) on a lock
// file that is never deleted. The kernel owns it: only the open file that
// took it can release it, and it is released when the owner exits or
// crashes. So a live owner is never robbed however long it holds the lock,
// a crashed owner never blocks anyone, and PID reuse does not matter.
package safefile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ErrTimeout: another live process held the lock for the whole wait.
var ErrTimeout = errors.New("lock dipegang proses lain")

// Lock is an exclusive lock held until Unlock.
type Lock struct{ f *os.File }

// Acquire takes the exclusive lock on path, waiting up to wait for the
// current owner to release it.
func Acquire(path string, wait time.Duration) (*Lock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(wait)
	for {
		ok, err := tryLock(f)
		if err != nil {
			f.Close()
			return nil, err
		}
		if ok {
			// The owner's PID, for people inspecting a stuck lock only.
			f.Truncate(0)
			f.WriteAt([]byte(fmt.Sprintf("pid %d\n", os.Getpid())), 0)
			return &Lock{f: f}, nil
		}
		if time.Now().After(deadline) {
			f.Close()
			return nil, ErrTimeout
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// Unlock releases the lock. The lock file stays: removing it would let a
// waiter lock a file that a newcomer then recreates and locks too.
func (l *Lock) Unlock() error {
	err := unlock(l.f)
	if cerr := l.f.Close(); err == nil {
		err = cerr
	}
	return err
}

// WriteFile replaces path with data atomically: a reader sees the old or
// the new content, never a mix. The temporary file is unique, so writers
// never clobber each other's half-written file.
func WriteFile(path string, data []byte, perm os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(data)
	if err == nil {
		err = f.Chmod(perm)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = retry(func() error { return os.Rename(tmp, path) })
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}

// ReadFile reads path like os.ReadFile, riding out a concurrent WriteFile.
func ReadFile(path string) ([]byte, error) {
	var b []byte
	err := retry(func() (err error) {
		b, err = os.ReadFile(path)
		return err
	})
	return b, err
}

// retryWindow bounds how long a transient sharing error is retried.
const retryWindow = 2 * time.Second

// retry runs op until it succeeds, fails for good, or retryWindow passes.
// On Windows a file another process has open cannot be replaced, and a file
// being replaced cannot be opened, for a moment; elsewhere op runs once.
func retry(op func() error) error {
	deadline := time.Now().Add(retryWindow)
	delay := time.Millisecond
	for {
		err := op()
		if err == nil || !transient(err) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(delay)
		if delay < 64*time.Millisecond {
			delay *= 2
		}
	}
}
