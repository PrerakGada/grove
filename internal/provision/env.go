// Package provision materialises a worktree: its environment files, symlinks,
// dependencies and marker files.
package provision

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PrerakGada/grove/internal/config"
)

const header = "# Managed by grove — worktree %q (slot %d, %s mode).\n" +
	"# Port-derived values below are rewritten on every `grove setup`.\n"

// WriteEnvFiles produces each worktree env file from the main checkout's copy,
// then applies the configured assignments.
//
// Assignments are computed key=value writes, never textual substitution of the
// main checkout's current values. The bash predecessor sed-ed main's literal
// port numbers, so the day those ports moved, every worktree silently kept
// pointing at the main backend and database.
func WriteEnvFiles(cfg *config.Config, mainRoot, wtPath, name string, slot int, fullstack bool) error {
	if cfg.Env == nil {
		return nil
	}
	vars := config.NewVars(cfg, name, mainRoot, wtPath, slot)
	mode := "frontend"
	if fullstack {
		mode = "fullstack"
	}

	for _, f := range cfg.Env.Files {
		src := filepath.Join(mainRoot, f.Path)
		dst := filepath.Join(wtPath, f.Path)

		if f.Inherit == "link" {
			if err := forceSymlink(src, dst); err != nil {
				return err
			}
			continue
		}
		if f.Inherit == "none" {
			continue
		}

		base, err := os.ReadFile(src)
		if err != nil {
			// A file that exists only to carry grove's own computed values has
			// nothing to inherit, so requiring a committed seed copy would force
			// projects to track a file grove rewrites in every worktree — which
			// then reads as uncommitted changes forever and blocks `grove delete`.
			// Start from empty instead; the assignments below fill it.
			if os.IsNotExist(err) && len(f.Set)+len(f.Fullstack) > 0 {
				base = nil
			} else if os.IsNotExist(err) {
				return fmt.Errorf("%s not found in the main checkout — "+
					"grove seeds each worktree's env from it", f.Path)
			} else {
				return err
			}
		}

		set := map[string]string{}
		for k, v := range f.Set {
			r, err := config.Render(v, vars)
			if err != nil {
				return fmt.Errorf("%s: %w", f.Path, err)
			}
			set[k] = r
		}
		if fullstack {
			for k, v := range f.Fullstack {
				r, err := config.Render(v, vars)
				if err != nil {
					return fmt.Errorf("%s: %w", f.Path, err)
				}
				set[k] = r
			}
		}

		out := applyAssignments(string(base), set)
		out = fmt.Sprintf(header, name, slot, mode) + out

		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		// The destination is often a symlink into the main checkout, inherited
		// from a fresh git checkout's postinstall. Remove it first, or the
		// write lands on the main checkout's own file.
		_ = os.Remove(dst)
		if err := os.WriteFile(dst, []byte(out), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// applyAssignments rewrites KEY=... in place, preserving order, comments and
// every key it was not asked to touch. Keys absent from the file are appended.
func applyAssignments(body string, set map[string]string) string {
	seen := map[string]bool{}
	lines := strings.Split(body, "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		k, _, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v, want := set[k]
		if !want || seen[k] {
			continue
		}
		lines[i] = k + "=" + v
		seen[k] = true
	}

	var missing []string
	for k := range set {
		if !seen[k] {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)

	out := strings.Join(lines, "\n")
	if len(missing) > 0 {
		if !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		out += "\n# Added by grove\n"
		for _, k := range missing {
			out += k + "=" + set[k] + "\n"
		}
	}
	return out
}

func forceSymlink(target, link string) error {
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return err
	}
	_ = os.Remove(link)
	return os.Symlink(target, link)
}
