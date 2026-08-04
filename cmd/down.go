package cmd

import (
	"fmt"

	"github.com/PrerakGada/grove/internal/dbx"
	"github.com/PrerakGada/grove/internal/runner"
	"github.com/PrerakGada/grove/internal/ui"
	"github.com/spf13/cobra"
)

var (
	downAll    bool
	downKeepDB bool
)

var downCmd = &cobra.Command{
	Use:   "down [name ...]",
	Short: "Stop a worktree's services",
	Long: `Stops the recorded processes, anything holding the slot's ports, and anything
whose working directory is inside the worktree.

All three passes are needed. Dev servers fork children that outlive the process
in the pid file, and a server started before a port change — or by hand — holds
a port the slot no longer owns, so it survives both of the first two.`,
	RunE: runDown,
}

func init() {
	downCmd.Flags().BoolVar(&downAll, "all", false, "every worktree")
	downCmd.Flags().BoolVar(&downKeepDB, "keep-db", false, "leave the database running")
	rootCmd.AddCommand(downCmd)
}

func runDown(cmd *cobra.Command, args []string) error {
	cfg, root, err := loadConfig()
	if err != nil {
		return err
	}

	names := args
	if downAll {
		names, err = allWorktreeNames(cfg, root)
		if err != nil {
			return err
		}
	}
	if len(names) == 0 {
		return fmt.Errorf("which worktree? pass a name, or --all")
	}

	for _, name := range names {
		wtPath, slot, err := requireWorktree(cfg, root, name)
		if err != nil {
			ui.Warn("%v", err)
			continue
		}
		ui.Info("stopping %q ...", name)
		runner.Stop(cfg, wtPath, slot)

		if cfg.HasDB() && !downKeepDB && fileExists(wtPath+"/"+dbx.ComposeFile) {
			if env, err := dbx.NewEnv(cfg, root, wtPath, name, slot); err == nil {
				_ = env.Stop()
			}
		}
		ui.Ok("  %q stopped", name)
	}
	return nil
}
