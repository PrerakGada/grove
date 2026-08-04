package provision

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/PrerakGada/grove/internal/config"
	"github.com/PrerakGada/grove/internal/platform"
	"github.com/PrerakGada/grove/internal/ui"
)

// Deps modes.
const (
	DepsReal  = "real"  // own dependency tree — the default
	DepsLight = "light" // symlinked to the main checkout — the escape hatch
)

// Modes.
const (
	ModeFrontend  = "frontend"
	ModeFullstack = "fullstack"
)

type Options struct {
	Config   *config.Config
	MainRoot string
	Path     string
	Name     string
	Slot     int
	Mode     string
	Deps     string
	// SkipInstall leaves dependencies alone, for re-running setup only to
	// repair ports, env files and links.
	SkipInstall bool
}

// Setup provisions a worktree that already exists on disk.
func Setup(o Options) error {
	if o.Mode == "" {
		o.Mode = ModeFullstack
	}
	if o.Deps == "" {
		o.Deps = DepsReal
	}

	if err := WriteEnvFiles(o.Config, o.MainRoot, o.Path, o.Name, o.Slot,
		o.Mode == ModeFullstack); err != nil {
		return err
	}
	ui.Ok("env written (slot %d, %s mode)", o.Slot, o.Mode)

	if !o.SkipInstall {
		mode, err := installDeps(o)
		if err != nil {
			return err
		}
		o.Deps = mode
	}

	if err := applyLinks(o); err != nil {
		return err
	}
	if err := seedPaths(o); err != nil {
		return err
	}

	writeMarker(o.Path, config.ModeFile, o.Mode)
	writeMarker(o.Path, config.DepsFile, o.Deps)
	return nil
}

// installDeps materialises the worktree's dependencies, returning the mode it
// actually used — a machine that cannot clone cheaply is downgraded rather than
// silently charged gigabytes.
func installDeps(o Options) (string, error) {
	if o.Config.Deps == nil || len(o.Config.Deps.Steps) == 0 {
		return o.Deps, nil
	}

	if o.Deps == DepsLight && !o.Config.Deps.Shareable {
		ui.Warn("this project does not allow shared dependencies — installing its own")
		o.Deps = DepsReal
	}

	if o.Deps == DepsReal && !cowViable(o) {
		ui.Warn("the package store is on a different volume from %s, so a real install", o.Path)
		ui.Warn("cannot clone or hardlink and would cost full size. Falling back to shared.")
		if !o.Config.Deps.Shareable {
			return "", fmt.Errorf("shared dependencies are not allowed for this project — " +
				"move the package store onto the same volume as the worktrees")
		}
		o.Deps = DepsLight
	}

	if o.Deps == DepsLight {
		return DepsLight, linkDeps(o)
	}

	// A symlink inherited from a previous light setup must go first: installing
	// through it would write into the main checkout's dependency tree.
	removeDepSymlinks(o)

	for _, step := range o.Config.Deps.Steps {
		dir := filepath.Join(o.Path, step.Dir)
		if _, err := os.Stat(dir); err != nil {
			continue // a directory this branch does not have
		}
		ui.Info("installing %s in %s ...", step.Manager, step.Dir)
		cmd := exec.Command("/bin/sh", "-c", step.Install)
		cmd.Dir = dir
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("install failed in %s: %w", step.Dir, err)
		}
	}
	ui.Ok("dependencies installed — builds and code generation are safe here")
	return DepsReal, nil
}

// depDirs are the directories that hold a dependency tree.
func depDirs(o Options) []string {
	dirs := []string{"."}
	for _, s := range o.Config.Deps.Steps {
		if s.Dir != "." {
			dirs = append(dirs, s.Dir)
		}
	}
	dirs = append(dirs, o.Config.Deps.Workspaces...)
	return dirs
}

func linkDeps(o Options) error {
	for _, d := range depDirs(o) {
		src := filepath.Join(o.MainRoot, d, "node_modules")
		dst := filepath.Join(o.Path, d, "node_modules")
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if fi, err := os.Lstat(dst); err == nil && fi.Mode()&os.ModeSymlink != 0 {
			continue
		}
		_ = os.RemoveAll(dst)
		if err := os.Symlink(src, dst); err != nil {
			return err
		}
	}
	ui.Ok("dependencies shared with the main checkout (0 MB)")
	ui.Warn("code generation that writes into node_modules is unsafe here — it would")
	ui.Warn("rewrite the main checkout's copy. Convert with: grove setup %s --own-deps", o.Name)
	return nil
}

func removeDepSymlinks(o Options) {
	for _, d := range depDirs(o) {
		p := filepath.Join(o.Path, d, "node_modules")
		if fi, err := os.Lstat(p); err == nil && fi.Mode()&os.ModeSymlink != 0 {
			_ = os.Remove(p)
		}
	}
}

// cowViable reports whether a real install can share blocks with the store.
func cowViable(o Options) bool {
	for _, mgr := range o.Config.Deps.Managers() {
		store := storePath(mgr)
		if store == "" {
			continue // unknown store — assume fine rather than block
		}
		same, err := platform.SameVolume(store, o.Path)
		if err == nil && !same {
			return false
		}
	}
	return true
}

func storePath(manager string) string {
	var out []byte
	var err error
	switch manager {
	case "pnpm":
		out, err = exec.Command("pnpm", "store", "path").Output()
	case "npm", "yarn":
		out, err = exec.Command(manager, "cache", "dir").Output()
	default:
		return ""
	}
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// applyLinks recreates the symlinks a worktree needs but a fresh checkout has
// no way to produce, because their targets are gitignored in the main checkout.
func applyLinks(o Options) error {
	vars := config.NewVars(o.Config, o.Name, o.MainRoot, o.Path, o.Slot)
	n := 0
	for _, l := range o.Config.Links {
		targets := []map[string]string{{}}
		if l.ForEach == "workspaces" && o.Config.Deps != nil {
			targets = nil
			for _, ws := range o.Config.Deps.Workspaces {
				targets = append(targets, map[string]string{"ws": ws})
			}
		}
		for _, extra := range targets {
			v := config.Vars{}
			for k, val := range vars {
				v[k] = val
			}
			for k, val := range extra {
				v[k] = val
			}

			from, err := config.Render(l.From, v)
			if err != nil {
				return err
			}
			to, err := config.Render(l.To, v)
			if err != nil {
				return err
			}
			if !filepath.IsAbs(from) && !strings.HasPrefix(from, ".") {
				from = filepath.Join(o.MainRoot, from)
			}
			dst := filepath.Join(o.Path, to)

			if !filepath.IsAbs(from) {
				// A relative target is relative to the link's own directory.
				if _, err := os.Stat(filepath.Join(filepath.Dir(dst), from)); err != nil && l.Optional {
					continue
				}
			} else if _, err := os.Stat(from); err != nil {
				if l.Optional {
					continue
				}
				return fmt.Errorf("link source missing: %s", from)
			}
			if _, err := os.Lstat(dst); err == nil {
				continue // the branch provides its own
			}
			if err := forceSymlink(from, dst); err != nil {
				return err
			}
			n++
		}
	}
	if n > 0 {
		ui.Ok("%d symlink(s) linked from the main checkout", n)
	}
	return nil
}

// seedPaths copy-on-write clones expensive generated artifacts from the main
// checkout, so a fresh worktree can run immediately without regenerating them.
// On a copy-on-write filesystem this costs essentially nothing until one side
// is written.
func seedPaths(o Options) error {
	if o.Config.Deps == nil {
		return nil
	}
	for _, rel := range o.Config.Deps.Seed {
		src := filepath.Join(o.MainRoot, rel)
		dst := filepath.Join(o.Path, rel)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if _, err := os.Stat(dst); err == nil {
			continue // the install already produced one
		}
		// Nothing to seed into a shared tree — it is the main checkout's.
		if fi, err := os.Lstat(filepath.Dir(dst)); err == nil && fi.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			continue
		}
		if err := platform.CloneDir(src, dst); err == nil {
			ui.Ok("seeded %s from the main checkout (copy-on-write)", rel)
		}
	}
	return nil
}

func writeMarker(dir, name, value string) {
	_ = os.WriteFile(filepath.Join(dir, name), []byte(value+"\n"), 0o644)
}

// ReadMarker returns a worktree marker's value, or "".
func ReadMarker(dir, name string) string {
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
