//go:build windows

package registry

// lock is a no-op on Windows for now. Windows support is not implemented yet
// (see internal/platform), so there is nothing to serialise against; this stub
// exists only so the package compiles for GOOS=windows in CI.
//
// When Windows lands, replace this with LockFileEx on ~/.grove/registry.lock.
func lock() (func(), error) {
	return func() {}, nil
}
