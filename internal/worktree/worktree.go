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
		case line == "detached":
			cur.Detached = true
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

func countChanges(dir string) int {
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if err != nil {
		return -1
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return 0
	}
	return len(strings.Split(s, "\n"))
}

func realPath(p string) string {
	if rp, err := filepath.EvalSymlinks(p); err == nil {
		return rp
	}
	return p
}
