// Package platform isolates every OS-specific operation Grove needs.
//
// Everything here was learned the hard way in the bash predecessor, and each
// function exists because a naive version of it leaked processes or ate disk:
//
//   - Listeners + ProcsUnder: killing a dev server by recorded pid is not
//     enough. Vite and Next fork children that outlive their parent, and a
//     server started before a port re-map holds a port the slot no longer owns.
//     Both a port sweep and a working-directory sweep are required.
//   - SameVolume: pnpm clones from its store with copy-on-write, but only
//     within one filesystem. Across volumes the same install silently becomes
//     real copies — gigabytes per worktree instead of megabytes.
//   - CloneDir: the copy-on-write primitive itself (APFS clonefile, btrfs/XFS
//     reflink), used to seed expensive generated artifacts for ~0 bytes.
//
// Windows implementations are stubbed. Linux is supported, which means Windows
// users are covered by WSL2 today — and WSL2 is what they want anyway, since
// repositories that rely on symlinks need Developer Mode on native Windows.
package platform

import "errors"

// ErrUnsupported is returned by stubs on platforms Grove does not implement yet.
var ErrUnsupported = errors.New("not supported on this platform yet")

// Signal is a minimal, portable subset of process signals.
type Signal int

const (
	// SigTerm asks a process to shut down.
	SigTerm Signal = iota
	// SigKill terminates it unconditionally.
	SigKill
)

func (s Signal) String() string {
	if s == SigKill {
		return "KILL"
	}
	return "TERM"
}
