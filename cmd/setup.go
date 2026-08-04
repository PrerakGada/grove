package cmd

import (
	"fmt"
	"os"

	"github.com/PrerakGada/grove/internal/config"
	"github.com/PrerakGada/grove/internal/provision"
	"github.com/PrerakGada/grove/internal/ui"
	"github.com/spf13/cobra"
)

var (
	setupFlags    provisionFlags
	setupNoInstal bool
)

var setupCmd = &cobra.Command{
	Use:   "setup <name>",
	Short: "Re-provision an existing worktree",
	Long: `Rewrites a worktree's environment files, symlinks and markers, and can
convert its dependency isolation or its mode.

Use it to adopt a worktree Grove did not create, to repair one whose ports or
links broke, or to switch a shared-dependency worktree to its own.`,
	Args: cobra.ExactArgs(1),
	RunE: runSetup,
}

func init() {
	setupFlags.register(setupCmd)
	setupCmd.Flags().BoolVar(&setupNoInstal, "no-install", false,
		"only rewrite ports, env files and links")
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	cfg, root, err := loadConfig()
	if err != nil {
		return err
	}
	name := args[0]
	wtPath := worktreePath(cfg, root, name)
	if _, err := os.Stat(wtPath); err != nil {
		return fmt.Errorf("no worktree named %q", name)
	}

	slot, ok := slotOf(cfg, name)
	if !ok {
		slot, err = allocateSlot(cfg, root, name)
		if err != nil {
			return err
		}
		ui.Ok("allocated slot %d", slot)
	}

	// Unspecified flags keep whatever the worktree already is, so a repair does
	// not silently convert it.
	mode := setupFlags.mode()
	if !setupFlags.frontend && !setupFlags.fullstack {
		if m := provision.ReadMarker(wtPath, config.ModeFile); m != "" {
			mode = m
		}
	}
	deps := setupFlags.deps()
	if !setupFlags.sharedDeps && !setupFlags.ownDeps {
		if d := provision.ReadMarker(wtPath, config.DepsFile); d != "" {
			deps = d
		}
	}

	if err := provision.Setup(provision.Options{
		Config: cfg, MainRoot: root, Path: wtPath, Name: name, Slot: slot,
		Mode: mode, Deps: deps, SkipInstall: setupNoInstal,
	}); err != nil {
		return err
	}

	if setupFlags.wantDB(cfg) && setupFlags.dbProfile != "" {
		if err := provisionDB(cfg, root, wtPath, name, slot,
			setupFlags.profile(cfg), false, false); err != nil {
			return err
		}
	}

	fmt.Println()
	ui.Ok("%q re-provisioned", name)
	printPorts(cfg, slot)
	return nil
}
