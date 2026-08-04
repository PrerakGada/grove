package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/PrerakGada/grove/internal/registry"
	"github.com/PrerakGada/grove/internal/ui"
	"github.com/spf13/cobra"
)

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Clean up stale worktree references and free their slots",
	Long: `Prunes git's own stale worktree records, then releases any slot whose
directory no longer exists.

Freeing the slot is the part that matters: a registry that only ever hands out
slots and never takes them back shrinks the pool every time a worktree is
deleted, until it runs out for no visible reason.`,
	RunE: runPrune,
}

func init() { rootCmd.AddCommand(pruneCmd) }

func runPrune(cmd *cobra.Command, args []string) error {
	cfg, root, err := loadConfig()
	if err != nil {
		return err
	}

	out, err := exec.Command("git", "-C", root, "worktree", "prune", "-v").CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree prune failed: %s", out)
	}
	if len(out) > 0 {
		ui.Ok("git: %s", out)
	} else {
		ui.Ok("no stale git worktree records")
	}

	var freed []string
	err = registry.Update(func(r *registry.Registry) error {
		p := r.Projects[cfg.Project]
		if p == nil {
			return nil
		}
		for name := range p.Slots {
			dir := filepath.Join(root, cfg.WorktreeDir, name)
			if _, err := os.Stat(dir); err != nil {
				freed = append(freed, name)
			}
		}
		for _, n := range freed {
			p.Release(n)
		}
		return nil
	})
	if err != nil {
		return err
	}

	sort.Strings(freed)
	if len(freed) == 0 {
		ui.Ok("no orphaned slots")
		return nil
	}
	for _, n := range freed {
		ui.Ok("freed the slot held by %q", n)
	}
	return nil
}
