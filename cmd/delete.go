package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/PrerakGada/grove/internal/dbx"
	"github.com/PrerakGada/grove/internal/runner"
	"github.com/PrerakGada/grove/internal/ui"
	"github.com/PrerakGada/grove/internal/worktree"
	"github.com/spf13/cobra"
)

var (
	deleteKeepBranch bool
	deleteKeepDB     bool
	deleteForce      bool
)

var deleteCmd = &cobra.Command{
	Use:     "delete <name>",
	Aliases: []string{"rm"},
	Short:   "Remove a worktree, its branch, its database and its slot",
	Long: `Stops anything running, removes the worktree, deletes its database volume,
frees its slot, and deletes the branch.

Refuses to run when the worktree has uncommitted changes or unpushed commits
unless --force is given, because git worktree removal discards them.`,
	Args: cobra.ExactArgs(1),
	RunE: runDelete,
}

func init() {
	deleteCmd.Flags().BoolVar(&deleteKeepBranch, "keep-branch", false, "keep the local branch")
	deleteCmd.Flags().BoolVar(&deleteKeepDB, "keep-db", false, "keep the database volume")
	deleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false,
		"delete even with uncommitted changes or unpushed commits")
	rootCmd.AddCommand(deleteCmd)
}

func runDelete(cmd *cobra.Command, args []string) error {
	cfg, root, err := loadConfig()
	if err != nil {
		return err
	}
	name := args[0]
	wtPath := worktreePath(cfg, root, name)
	if _, err := os.Stat(wtPath); err != nil {
		return fmt.Errorf("no worktree named %q", name)
	}

	// Removal is irreversible, so check for work that would be destroyed first.
	// The bash predecessor removed with --force unconditionally and deleted the
	// branch by default, which is a good way to lose a day.
	if !deleteForce {
		wts, _ := worktree.List(root, cfg)
		for _, w := range wts {
			if w.Name != name {
				continue
			}
			if w.Changes > 0 {
				return fmt.Errorf("%q has %d uncommitted change(s) — commit them, or re-run with --force",
					name, w.Changes)
			}
			if w.Branch != "" {
				if n := unpushed(root, w.Branch); n > 0 {
					return fmt.Errorf("%q has %d unpushed commit(s) on %s — push them, "+
						"or re-run with --force --keep-branch", name, n, w.Branch)
				}
			}
		}
	}

	branch := branchOf(root, wtPath)

	slot, hasSlot := slotOf(cfg, name)
	if hasSlot {
		ui.Info("stopping anything still running ...")
		runner.Stop(cfg, wtPath, slot)
	}

	if cfg.HasDB() && !deleteKeepDB && hasSlot {
		if env, err := dbx.NewEnv(cfg, root, wtPath, name, slot); err == nil {
			env.Destroy()
			ui.Ok("database container and volume removed")
		}
	}

	rm := exec.Command("git", "-C", root, "worktree", "remove", "--force", wtPath)
	rm.Stdout, rm.Stderr = os.Stdout, os.Stderr
	if err := rm.Run(); err != nil {
		return fmt.Errorf("git worktree remove failed: %w", err)
	}
	ui.Ok("worktree removed")

	releaseSlot(cfg, name)
	ui.Ok("slot freed")

	if !deleteKeepBranch && branch != "" {
		if err := exec.Command("git", "-C", root, "branch", "-D", branch).Run(); err != nil {
			ui.Warn("could not delete branch %s — delete it manually if you meant to", branch)
		} else {
			ui.Ok("branch %s deleted", branch)
		}
	}
	return nil
}

func branchOf(root, wtPath string) string {
	out, err := exec.Command("git", "-C", wtPath, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	b := strings.TrimSpace(string(out))
	if b == "HEAD" {
		return "" // detached
	}
	return b
}

// unpushed counts commits on a branch that no remote has, so deleting it cannot
// silently drop work. A branch with no upstream counts all its commits since
// the default branch.
func unpushed(root, branch string) int {
	out, err := exec.Command("git", "-C", root, "rev-list", "--count",
		branch, "--not", "--remotes").Output()
	if err != nil {
		return 0
	}
	n := 0
	for _, c := range strings.TrimSpace(string(out)) {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
