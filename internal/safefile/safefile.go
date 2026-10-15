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
	"context"
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

// AcquireContext is Acquire waiting for as long as ctx lives, for a lock
// whose owner may rightly hold it longer than any fixed wait.
func AcquireContext(ctx context.Context, path string) (*Lock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	for {
		ok, err := tryLock(f)
		if err != nil {
			f.Close()
			return nil, err
		}
		if ok {
			f.Truncate(0)
			f.WriteAt([]byte(fmt.Sprintf("pid %d\n", os.Getpid())), 0)
			return &Lock{f: f}, nil
		}
		select {
		case <-ctx.Done():
			f.Close()
			return nil, ctx.Err()
		case <-tick.C:
		}
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
		err = os.Rename(tmp, path)
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}
