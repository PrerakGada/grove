package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/PrerakGada/grove/internal/config"
	"github.com/PrerakGada/grove/internal/ui"
	"github.com/spf13/cobra"
)

var trackFlags provisionFlags

var trackCmd = &cobra.Command{
	Use:   "track <branch> [name]",
	Short: "Check out an existing branch into a provisioned worktree",
	Long: `Checks out a branch that already exists — locally or only on the remote —
and provisions it exactly as ` + "`create`" + ` does.

Use this to review someone else's branch, or to keep a checkout of the default
branch for deploys. ` + "`create`" + ` always makes a new branch, so it is the wrong
command for an existing one.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runTrack,
}

func init() {
	trackFlags.register(trackCmd)
	rootCmd.AddCommand(trackCmd)
}

func runTrack(cmd *cobra.Command, args []string) error {
	cfg, root, err := loadConfig()
	if err != nil {
		return err
	}
	branch := args[0]

	// Default the directory name to the branch with its prefix stripped, so
	// `track feat/perf-sprint` lands in worktrees/perf-sprint.
	name := strings.TrimPrefix(branch, cfg.BranchPrefix)
	name = strings.ReplaceAll(name, "/", "-")
	if len(args) == 2 {
		name = args[1]
	}
	if !wtNameRe.MatchString(name) {
		return fmt.Errorf("invalid worktree name %q", name)
	}

	wtPath := worktreePath(cfg, root, name)
	if _, err := os.Stat(wtPath); err == nil {
		return fmt.Errorf("%s already exists", wtPath)
	}

	local := branchExists(root, branch)
	if !local {
		ui.Info("fetching %s from origin ...", branch)
		if err := exec.Command("git", "-C", root, "fetch", "origin",
			branch+":"+branch).Run(); err != nil {
			return fmt.Errorf("branch %q not found locally or on origin", branch)
		}
	} else {
		// A stale local copy is checked out as-is, deliberately: silently
		// moving someone's branch to the remote tip can discard unpushed work.
		if behind := behindRemote(root, branch); behind > 0 {
			ui.Warn("local %s is %d commit(s) behind origin — checking out the local copy", branch, behind)
			ui.Warn("to take the remote tip instead (only if it has no unpushed commits):")
			ui.Warn("  git branch -f %s origin/%s", branch, branch)
		}
	}

	slot, err := allocateSlot(cfg, root, name)
	if err != nil {
		return err
	}

	ui.Info("checking out %s into worktree %q", branch, name)
	add := exec.Command("git", "-C", root, "worktree", "add", wtPath, branch)
	add.Stdout, add.Stderr = os.Stdout, os.Stderr
	if err := add.Run(); err != nil {
		releaseSlot(cfg, name)
		return fmt.Errorf("git worktree add failed: %w", err)
	}

	return finishProvision(cfg, root, wtPath, name, slot, &trackFlags)
}

// behindRemote reports how many commits origin/<branch> is ahead of the local
// copy, or 0 when there is no remote branch or they match.
func behindRemote(root, branch string) int {
	out, err := exec.Command("git", "-C", root, "rev-list", "--count",
		branch+".."+"origin/"+branch).Output()
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

var _ = config.FileName
