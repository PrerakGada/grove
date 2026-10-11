// Package worktree reads the state of a repository's git worktrees.
package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/PrerakGada/grove/internal/config"
)

type Worktree struct {
	// Name is the directory name, which is how every command addresses it.
	Name string
	Path string
	// Branch is empty for a detached HEAD.
	Branch   string
	Detached bool
	// Head is the commit the worktree is checked out at.
	Head string

	// Locked marks a worktree the user pinned with `git worktree lock`.
	// Removal must respect it: it is an explicit "not this one", and
	// `git worktree remove --force` fails on it regardless.
	Locked     bool
	LockReason string
	// Prunable marks a worktree git has already decided is stale, usually
	// because its directory is gone. That is `grove prune`'s job, not a
	// deletion candidate.
	Prunable    bool
	PruneReason string
	// IsMain marks the primary checkout, which Grove treats as slot 0.
	IsMain bool

	Slot int
	// HasSlot is false when nothing has allocated one yet.
	HasSlot bool
	// LegacySlot marks a slot read from the bash predecessor's .slots file
	// rather than from Grove's registry.
	LegacySlot bool

	Mode      string
	Deps      string
	DBProfile string

	// Changes counts modified/untracked paths; -1 if it could not be read.
	Changes int
}

// List returns the main checkout followed by every linked worktree.
func List(root string, cfg *config.Config) ([]Worktree, error) {
	out, err := exec.Command("git", "-C", root, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return nil, err
	}

	var wts []Worktree
	var cur *Worktree
	flush := func() {
		if cur != nil {
			wts = append(wts, *cur)
			cur = nil
		}
	}
	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			flush()
			p := strings.TrimPrefix(line, "worktree ")
			cur = &Worktree{Path: p, Name: filepath.Base(p), Changes: -1}
		case cur == nil:
			continue
		case strings.HasPrefix(line, "HEAD "):
			cur.Head = strings.TrimPrefix(line, "HEAD ")
		case line == "detached":
			cur.Detached = true
		case line == "locked" || strings.HasPrefix(line, "locked "):
			cur.Locked = true
			cur.LockReason = strings.TrimSpace(strings.TrimPrefix(line, "locked"))
		case line == "prunable" || strings.HasPrefix(line, "prunable "):
			cur.Prunable = true
			cur.PruneReason = strings.TrimSpace(strings.TrimPrefix(line, "prunable"))
		case strings.HasPrefix(line, "branch "):
			cur.Branch = strings.TrimPrefix(strings.TrimPrefix(line, "branch "), "refs/heads/")
		}
	}
	flush()

	mainReal := realPath(root)
	legacy := legacySlots(root, cfg)

	for i := range wts {
		w := &wts[i]
		if realPath(w.Path) == mainReal {
			// Keep the real directory name. Renaming it to "main" collides
			// with a worktree that tracks the main branch, which is a normal
			// thing to have.
			w.IsMain = true
			w.Slot, w.HasSlot = 0, true
		} else if s, ok := legacy[w.Name]; ok {
			w.Slot, w.HasSlot, w.LegacySlot = s, true, true
		}
		w.Mode = readMarker(w.Path, config.ModeFile, ".wt-mode")
		w.Deps = readMarker(w.Path, config.DepsFile, ".wt-deps")
		w.DBProfile = readMarker(w.Path, config.DBProfileFile, ".wt-db-profile")
		w.Changes = countChanges(w.Path)
	}
	return wts, nil
}

// legacySlots reads the bash predecessor's worktrees/.slots file so worktrees
// created before Grove still show their real ports instead of a blank.
func legacySlots(root string, cfg *config.Config) map[string]int {
	out := map[string]int{}
	dir := "worktrees"
	if cfg != nil && cfg.WorktreeDir != "" {
		dir = cfg.WorktreeDir
	}
	b, err := os.ReadFile(filepath.Join(root, dir, ".slots"))
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(b), "\n") {
		slot, name, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok || name == "" {
			continue
		}
		if n, err := strconv.Atoi(slot); err == nil {
			out[name] = n
		}
	}
	return out
}

// readMarker returns the first marker file that exists, trying Grove's name
// then the predecessor's.
func readMarker(dir string, names ...string) string {
	for _, n := range names {
		if b, err := os.ReadFile(filepath.Join(dir, n)); err == nil {
			return strings.TrimSpace(string(b))
		}
	}
	return ""
}

// countChanges counts modified and untracked paths, excluding Grove's own
// per-worktree artifacts.
//
// Those markers are written into every worktree root and nothing gitignores
// them, so counting them reports a freshly provisioned, untouched worktree as
// carrying two changes. That is wrong in `list`, and it is load-bearing in
// `sweep`, which refuses to delete anything dirty: counted, no worktree Grove
// created would ever be sweepable. Excluding them by path is safe either way —
// a project that does gitignore them never had them in this output.
func countChanges(dir string) int {
	args := []string{"-C", dir, "status", "--porcelain", "--"}
	for _, p := range groveArtifacts {
		args = append(args, ":(exclude)"+p)
	}
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return -1
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return 0
	}
	return len(strings.Split(s, "\n"))
}

// groveArtifacts are the paths Grove itself writes into a worktree root.
var groveArtifacts = []string{
	config.ModeFile, config.DepsFile, config.DBProfileFile, config.RunDir,
}

func realPath(p string) string {
	if rp, err := filepath.EvalSymlinks(p); err == nil {
		return rp
	}
	return p
}
