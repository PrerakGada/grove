package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/PrerakGada/grove/internal/config"
	"github.com/PrerakGada/grove/internal/platform"
	"github.com/PrerakGada/grove/internal/registry"
	"github.com/PrerakGada/grove/internal/ui"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check the config, the registry, and this machine",
	Long: `Validates .grove.json, checks port bands against every other project
registered on this machine, and verifies the machine can actually deliver cheap
per-worktree dependencies.`,
	RunE: runDoctor,
}

func init() { rootCmd.AddCommand(doctorCmd) }

func runDoctor(cmd *cobra.Command, args []string) error {
	fmt.Println()
	problems := 0

	root, err := repoRoot()
	if err != nil {
		return err
	}
	ui.Ok("repository root: %s", root)

	cfg, err := config.Load(root)
	if err != nil {
		ui.Err("%v", err)
		return fmt.Errorf("cannot continue without a config")
	}
	if err := cfg.Validate(); err != nil {
		ui.Err("%v", err)
		problems++
	} else {
		ui.Ok("%s is valid — %d services, maxSlots %d", config.FileName, len(cfg.Services), cfg.MaxSlots)
	}

	reg, err := registry.Load()
	if err != nil {
		ui.Err("registry: %v", err)
		problems++
	} else {
		proj := reg.Projects[cfg.Project]
		switch {
		case proj == nil:
			ui.Warn("project %q is not registered in %s — run `grove init` to reserve its bands",
				cfg.Project, registry.Home())
			problems++
		case proj.Root != root:
			ui.Warn("registry points project %q at %s, but this repository is %s",
				cfg.Project, proj.Root, root)
			problems++
		default:
			ui.Ok("registered in %s — %d slot(s) allocated", registry.Home(), len(proj.Slots))
		}

		// Only a collision with another project is a failure. A band that
		// merely contains a well-known default port bites at one slot and is
		// reported, not counted — same judgement `grove init` makes.
		hard := reg.ProjectConflicts(cfg.Project, cfg.Bands(), cfg.MaxSlots)
		soft := reg.ReservedConflicts(cfg.Bands(), cfg.MaxSlots)
		for _, c := range hard {
			ui.Err("%s", c)
		}
		problems += len(hard)
		for _, c := range soft {
			ui.Warn("%s", c)
		}
		if len(hard) == 0 && len(soft) == 0 {
			ui.Ok("port bands %s are clear of other projects and well-known ports", bandList(cfg.Bands()))
		} else if len(hard) == 0 {
			ui.Ok("port bands %s do not collide with any other project", bandList(cfg.Bands()))
		}
	}

	// Cheap per-worktree dependencies rely on copy-on-write cloning, which only
	// works within one filesystem. Across volumes the same install silently
	// becomes real copies — gigabytes per worktree instead of megabytes.
	if cfg.Deps != nil {
		wtDir := filepath.Join(root, cfg.WorktreeDir)
		for _, mgr := range cfg.Deps.Managers() {
			store, err := storePath(mgr)
			if err != nil || store == "" {
				ui.Warn("could not locate the %s store — skipping its copy-on-write check", mgr)
				continue
			}
			same, err := platform.SameVolume(store, wtDir)
			switch {
			case err != nil:
				ui.Warn("copy-on-write check for %s failed: %v", mgr, err)
			case !same:
				ui.Err("the %s store (%s) is on a different volume from %s", mgr, store, wtDir)
				fmt.Println("    Installs cannot clone or hardlink across volumes, so every worktree")
				fmt.Println("    would pay full size. Move the store onto this volume.")
				problems++
			default:
				ui.Ok("%s store shares a volume with %s — installs stay copy-on-write", mgr, cfg.WorktreeDir)
			}
		}
	}

	if runtime.GOOS == "windows" {
		ui.Warn("native Windows is not implemented yet — use WSL2")
		problems++
	}

	fmt.Println()
	if problems == 0 {
		ui.Ok("no problems found")
		return nil
	}
	return fmt.Errorf("%d problem(s) found", problems)
}

func storePath(manager string) (string, error) {
	switch manager {
	case "pnpm":
		return runCapture("pnpm", "store", "path")
	case "npm", "yarn":
		return runCapture(manager, "cache", "dir")
	case "bun":
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, ".bun", "install", "cache"), nil
		}
	}
	return "", nil
}
