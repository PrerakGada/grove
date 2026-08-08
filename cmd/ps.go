package cmd

import (
	"fmt"
	"strconv"

	"github.com/PrerakGada/grove/internal/config"
	"github.com/PrerakGada/grove/internal/provision"
	"github.com/PrerakGada/grove/internal/registry"
	"github.com/PrerakGada/grove/internal/runner"
	"github.com/PrerakGada/grove/internal/ui"
	"github.com/PrerakGada/grove/internal/worktree"
	"github.com/spf13/cobra"
)

var psAll bool

var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "Show what is running, per worktree and per service",
	RunE:  runPS,
}

func init() {
	psCmd.Flags().BoolVarP(&psAll, "all", "a", false, "include services that are not running")
	rootCmd.AddCommand(psCmd)
}

func runPS(cmd *cobra.Command, args []string) error {
	cfg, root, err := loadConfig()
	if err != nil {
		return err
	}
	reg, err := registry.Load()
	if err != nil {
		return err
	}
	proj := reg.Projects[cfg.Project]

	wts, err := worktree.List(root, cfg)
	if err != nil {
		return err
	}

	type row struct{ wt, svc, port, pid, state string }
	var rows []row
	running := 0

	for _, w := range wts {
		slot, ok := w.Slot, w.HasSlot
		if proj != nil && !w.IsMain {
			if s, found := proj.Slot(w.Name); found {
				slot, ok = s, true
			}
		}
		if !ok {
			continue
		}
		label := w.Name
		if w.IsMain {
			label += " (main)"
		}
		mode := provision.ReadMarker(w.Path, config.ModeFile)

		for _, st := range runner.Statuses(cfg, w.Path, slot) {
			// A frontend worktree does not own a backend, so a backend port
			// showing up there is the main checkout's, not this worktree's.
			if !w.IsMain && mode != provision.ModeFullstack {
				if s, found := cfg.Service(st.Service); found && s.Kind == "backend" {
					continue
				}
			}
			if st.Up {
				running++
			} else if !psAll {
				continue
			}
			state, pid := "stopped", "-"
			if st.Up {
				state, pid = "up", strconv.Itoa(st.PID)
			}
			rows = append(rows, row{label, st.Service, strconv.Itoa(st.Port), pid, state})
		}
	}

	fmt.Println()
	fmt.Printf("%s  %s\n", ui.Title.Render(cfg.Project), ui.Dim.Render(root))
	fmt.Println()

	tbl := ui.NewTable("WORKTREE", "SERVICE", "PORT", "PID", "STATE")

	// The header prints even with nothing running: an empty table is a state,
	// not an error. Rendering it through the same table keeps the empty and
	// populated forms from drifting apart.
	if len(rows) == 0 {
		tbl.Print()
		fmt.Println()
		ui.Info("Nothing running. Start one: grove up <name>")
		return nil
	}

	for _, r := range rows {
		tbl.Row(r.wt, r.svc, r.port, r.pid, r.state)
	}
	tbl.Print()
	fmt.Println()
	ui.Info("%d service(s) running", running)
	return nil
}
