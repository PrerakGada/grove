// Package cmd holds Grove's command-line surface.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/PrerakGada/grove/internal/config"
	"github.com/PrerakGada/grove/internal/detect"
	"github.com/spf13/cobra"
)

// Version is set at build time by GoReleaser.
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "wt",
	Short: "Grove — parallel dev environments from git worktrees",
	Long: `Grove gives every git worktree a fully provisioned dev environment:
reserved ports, its own dependencies, its own database, and a background
runner, so many branches can run at once without a terminal each.

Project shape lives in .wt.json (committed). Machine state lives in ~/.wt.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	Version:       Version,
}

// Execute runs the CLI.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "\033[31m✗\033[0m %v\n", err)
		os.Exit(1)
	}
}

// repoRoot resolves the main checkout root from the current directory, so every
// command works identically from the main repository, a linked worktree, or any
// subdirectory of either.
func repoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root, err := detect.Root(cwd)
	if err != nil {
		return "", fmt.Errorf("not inside a git repository (%s)", cwd)
	}
	return root, nil
}

// loadConfig resolves the repository root and reads its Grove config.
func loadConfig() (*config.Config, string, error) {
	root, err := repoRoot()
	if err != nil {
		return nil, "", err
	}
	cfg, err := config.Load(root)
	if err != nil {
		return nil, root, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, root, err
	}
	return cfg, root, nil
}

// worktreePath returns where a named worktree lives.
func worktreePath(cfg *config.Config, root, name string) string {
	return filepath.Join(root, cfg.WorktreeDir, name)
}
