package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/PrerakGada/grove/internal/config"
	"github.com/PrerakGada/grove/internal/dbx"
	"github.com/PrerakGada/grove/internal/provision"
	"github.com/PrerakGada/grove/internal/runner"
	"github.com/PrerakGada/grove/internal/ui"
	"github.com/spf13/cobra"
)

var (
	upAll     bool
	upEvery   bool
	upTimeout int
)

var upCmd = &cobra.Command{
	Use:   "up [name] [service ...]",
	Short: "Start a worktree's services in the background",
	Long: `Starts services detached, so no terminal is held and many worktrees can run
at once. Which services start depends on the worktree's mode: a frontend
worktree runs frontends only; a fullstack one also starts its database and its
own backend.

The service list is remembered, so a later bare ` + "`grove up <name>`" + ` starts the
same set.`,
	RunE: runUp,
}

func init() {
	upCmd.Flags().BoolVar(&upAll, "all", false, "every worktree (restores everything after a reboot)")
	upCmd.Flags().BoolVar(&upEvery, "every-service", false, "start every configured service")
	upCmd.Flags().IntVar(&upTimeout, "timeout", 45, "seconds to wait for a service to bind")
	rootCmd.AddCommand(upCmd)
}

func runUp(cmd *cobra.Command, args []string) error {
	cfg, root, err := loadConfig()
	if err != nil {
		return err
	}

	names, services, err := splitNamesAndServices(cfg, root, args, upAll)
	if err != nil {
		return err
	}

	for _, name := range names {
		wtPath, slot, err := requireWorktree(cfg, root, name)
		if err != nil {
			ui.Warn("%v", err)
			continue
		}
		mode := provision.ReadMarker(wtPath, config.ModeFile)
		if mode == "" {
			mode = provision.ModeFrontend
		}

		want := services
		if len(want) == 0 && !upEvery {
			want = runner.RememberServices(wtPath, nil)
		}
		if upEvery || len(want) == 0 {
			for _, s := range cfg.Frontends() {
				want = append(want, s.Name)
			}
		}
		if len(services) > 0 {
			runner.RememberServices(wtPath, services)
		}

		fmt.Println()
		ui.Info("starting %q (%s) — %v", name, mode, want)

		// A database and a backend only belong to a worktree that owns them.
		if mode == provision.ModeFullstack {
			if cfg.HasDB() {
				env, err := dbx.NewEnv(cfg, root, wtPath, name, slot)
				switch {
				case err != nil:
					ui.Warn("  %v", err)
				case !fileExists(wtPath + "/" + dbx.ComposeFile):
					ui.Warn("  no database yet — run: grove db %s", name)
				default:
					if err := env.Up(); err != nil {
						ui.Warn("  database failed to start: %v", err)
					} else {
						ui.Ok("  db up on %d", env.Port)
					}
				}
			}
			for _, s := range cfg.Backends() {
				startOne(cfg, wtPath, s, slot)
			}
		}

		for _, n := range want {
			s, ok := cfg.Service(n)
			if !ok {
				ui.Warn("  unknown service %q", n)
				continue
			}
			if s.Kind == "backend" {
				continue // already handled above, and only in fullstack mode
			}
			startOne(cfg, wtPath, s, slot)
		}
	}

	fmt.Println()
	ui.Info("Detached — close this terminal freely.  Status: grove ps   Logs: grove logs <name> <service>")
	return nil
}

func startOne(cfg *config.Config, wtPath string, s config.Service, slot int) {
	port := runner.Port(s, slot)
	started, err := runner.Start(wtPath, s, port)
	if err != nil {
		ui.Warn("  %s failed to start: %v", s.Name, err)
		return
	}
	if !started {
		ui.Info("  %s already up on %d", s.Name, port)
		return
	}
	timeout := time.Duration(upTimeout) * time.Second
	if s.ReadySeconds > 0 {
		timeout = time.Duration(s.ReadySeconds) * time.Second
	} else if s.Kind == "backend" {
		timeout *= 2 // backends compile before they bind
	}
	if err := runner.WaitListening(wtPath, s.Name, port, timeout); err != nil {
		ui.Warn("  %s %v", s.Name, err)
		return
	}
	ui.Ok("  %s up on %d", s.Name, port)
}

// splitNamesAndServices separates worktree names from service names, so
// `grove up mywt web api` reads naturally.
func splitNamesAndServices(cfg *config.Config, root string, args []string, all bool) ([]string, []string, error) {
	var names, services []string
	for _, a := range args {
		if _, ok := cfg.Service(a); ok {
			services = append(services, a)
		} else {
			names = append(names, a)
		}
	}
	if all {
		found, err := allWorktreeNames(cfg, root)
		if err != nil {
			return nil, nil, err
		}
		names = found
	}
	if len(names) == 0 {
		return nil, nil, fmt.Errorf("which worktree? pass a name, or --all")
	}
	return names, services, nil
}

func allWorktreeNames(cfg *config.Config, root string) ([]string, error) {
	entries, err := os.ReadDir(root + "/" + cfg.WorktreeDir)
	if err != nil {
		return nil, fmt.Errorf("no worktrees yet")
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
