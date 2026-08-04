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
	Long: `Validates .wt.json, checks port bands against every other project
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
			ui.Warn("project %q is not registered in %s — run `%s init` to reserve its bands",
				cfg.Project, registry.Home(), Name())
			problems++
		case proj.Root != root:
			ui.Warn("registry points project %q at %s, but this repository is %s",
				cfg.Project, proj.Root, root)
			problems++
		default:
			ui.Ok("registered in %s — %d slot(s) allocated", registry.Home(), len(proj.Slots))
		}

		conflicts := reg.Conflicts(cfg.Project, cfg.Bands(), cfg.MaxSlots)
		if len(conflicts) > 0 {
			for _, c := range conflicts {
				ui.Err("%s", c)
			}
			problems += len(conflicts)
		} else {
			ui.Ok("port bands %s are clear of other projects and well-known ports", bandList(cfg.Bands()))
		}
	}

	// Cheap per-worktree dependencies rely on copy-on-write cloning, which only
	// works within one filesystem. Across volumes the same install silently
	// becomes real copies — gigabytes per worktree instead of megabytes.
	if cfg.Deps != nil && cfg.Deps.Manager != "" {
		wtDir := filepath.Join(root, cfg.WorktreeDir)
		if store, err := storePath(cfg.Deps.Manager); err != nil || store == "" {
			ui.Warn("could not locate the %s store — skipping the copy-on-write check", cfg.Deps.Manager)
		} else if same, err := platform.SameVolume(store, wtDir); err != nil {
			ui.Warn("copy-on-write check failed: %v", err)
		} else if !same {
			ui.Err("the %s store (%s) is on a different volume from %s", cfg.Deps.Manager, store, wtDir)
			fmt.Println("    Installs cannot clone or hardlink across volumes, so every worktree")
			fmt.Println("    would pay full size. Move the store onto this volume.")
			problems++
		} else {
			ui.Ok("%s store shares a volume with %s — installs stay copy-on-write", cfg.Deps.Manager, cfg.WorktreeDir)
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
