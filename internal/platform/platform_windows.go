//go:build windows

package platform

// Windows is not implemented yet. Grove builds for GOOS=windows so the gap is
// visible in CI rather than discovered at runtime, but every operation below
// returns ErrUnsupported or an empty result.
//
// Use WSL2 on Windows today: the Linux build is fully supported, and a
// repository that relies on symlinks (a common monorepo pattern) needs
// Developer Mode plus core.symlinks on native Windows anyway.
//
// What a native port would need:
//   - Listeners:     Get-NetTCPConnection / GetExtendedTcpTable
//   - Children:      Win32_Process.ParentProcessId
//   - ProcCwd:       no public API — requires reading the PEB via
//                    NtQueryInformationProcess. This is the hard one, and it is
//                    what makes the working-directory sweep possible.
//   - SpawnDetached: DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP, ideally with
//                    a Job Object so the whole tree dies together.
//   - CloneDir:      ReFS block cloning, else a real copy.

// ProcCwd is unimplemented on Windows.
func ProcCwd(pid int) (string, error) { return "", ErrUnsupported }

// CloneDir is unimplemented on Windows.
func CloneDir(src, dst string) error { return ErrUnsupported }

// Listeners is unimplemented on Windows.
func Listeners(port int) []int { return nil }

// Children is unimplemented on Windows.
func Children(pid int) []int { return nil }

// KillTree is unimplemented on Windows.
func KillTree(pid int, sig Signal) {}

// Kill is unimplemented on Windows.
func Kill(pid int, sig Signal) {}

// ProcsMatching is unimplemented on Windows.
func ProcsMatching(patterns []string) []int { return nil }

// ProcsUnder is unimplemented on Windows.
func ProcsUnder(dir string, candidates []int) []int { return nil }

// SpawnDetached is unimplemented on Windows.
func SpawnDetached(dir, cmdline, logPath string, env []string) (int, error) {
	return 0, ErrUnsupported
}

// Alive is unimplemented on Windows.
func Alive(pid int) bool { return false }

// SameVolume compares drive letters as a coarse approximation.
func SameVolume(a, b string) (bool, error) { return false, ErrUnsupported }
