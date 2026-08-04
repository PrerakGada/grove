package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/PrerakGada/grove/internal/config"
	"github.com/PrerakGada/grove/internal/detect"
	"github.com/PrerakGada/grove/internal/provision"
	"github.com/PrerakGada/grove/internal/registry"
	"github.com/PrerakGada/grove/internal/runner"
	"github.com/PrerakGada/grove/internal/ui"
	"github.com/spf13/cobra"
)

var devCmd = &cobra.Command{
	Use:   "dev [service ...]",
	Short: "Run services in the foreground, on this worktree's ports",
	Long: `Runs from wherever you are: Grove works out which worktree the current
directory belongs to and uses that slot's ports.

This replaces the per-worktree dev script the previous tool generated — the
ports come from the registry rather than from a file that has to be kept in
sync with it.`,
	RunE: runDev,
}

func init() { rootCmd.AddCommand(devCmd) }

func runDev(cmd *cobra.Command, args []string) error {
	cfg, root, err := loadConfig()
	if err != nil {
		return err
	}

	name, wtPath, slot, err := currentWorktree(cfg, root)
	if err != nil {
		return err
	}

	want := args
	if len(want) == 0 {
		if fs := cfg.Frontends(); len(fs) > 0 {
			want = []string{fs[0].Name}
		} else {
			return fmt.Errorf("which service? %s", strings.Join(serviceNames(cfg), ", "))
		}
	}

	mode := provision.ReadMarker(wtPath, config.ModeFile)
	var cmds []*exec.Cmd
	for _, n := range want {
		s, ok := cfg.Service(n)
		if !ok {
			return fmt.Errorf("unknown service %q — have: %s", n, strings.Join(serviceNames(cfg), ", "))
		}
		if s.Kind == "backend" && mode != provision.ModeFullstack && name != "" {
			return fmt.Errorf("%q is a frontend worktree, so it shares the main backend.\n"+
				"  Give it its own with: grove setup %s --fullstack && grove db %s", name, name, name)
		}
		port := runner.Port(s, slot)
		c := exec.Command("/bin/sh", "-c", runner.Command(s, port))
		c.Dir = wtPath
		c.Stdout, c.Stderr, c.Stdin = os.Stdout, os.Stderr, os.Stdin
		cmds = append(cmds, c)
		ui.Info("%s → http://localhost:%d", s.Name, port)
	}

	// Each process gets its own environment, which is why they cannot simply be
	// handed to a package manager's parallel runner: several services read the
	// same variable name and would collide on one shared value.
	errs := make(chan error, len(cmds))
	for _, c := range cmds {
		go func(c *exec.Cmd) { errs <- c.Run() }(c)
	}
	fmt.Println()
	err = <-errs
	for _, c := range cmds {
		if c.Process != nil {
			_ = c.Process.Kill()
		}
	}
	return err
}

// currentWorktree identifies which worktree the working directory is inside.
// Returns an empty name for the main checkout, which is slot 0.
func currentWorktree(cfg *config.Config, root string) (name, path string, slot int, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", 0, err
	}
	real, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		real = cwd
	}

	base := filepath.Join(root, cfg.WorktreeDir)
	baseReal, err := filepath.EvalSymlinks(base)
	if err != nil {
		baseReal = base
	}
	if rel, err := filepath.Rel(baseReal, real); err == nil &&
		!strings.HasPrefix(rel, "..") && rel != "." {
		name = strings.Split(rel, string(os.PathSeparator))[0]
		path = filepath.Join(base, name)
		s, ok := slotOf(cfg, name)
		if !ok {
			return name, path, 0, fmt.Errorf("%q has no slot — run: grove setup %s", name, name)
		}
		return name, path, s, nil
	}
	return "", root, 0, nil
}

func serviceNames(cfg *config.Config) []string {
	var out []string
	for _, s := range cfg.Services {
		out = append(out, s.Name)
	}
	return out
}

var (
	_ = registry.Home
	_ = detect.Root
)
