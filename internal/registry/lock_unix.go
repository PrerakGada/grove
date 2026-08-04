//go:build !windows

package registry

import (
	"os"
	"path/filepath"
	"syscall"
)

// lock takes an exclusive advisory lock on ~/.grove/registry.lock. It blocks until
// the lock is available; flock is released automatically if the holder dies, so
// there is no stale-lock recovery to get wrong.
func lock() (func(), error) {
	if err := os.MkdirAll(Home(), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(Home(), "registry.lock"),
		os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}
