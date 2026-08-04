package cmd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/PrerakGada/grove/internal/config"
	"github.com/PrerakGada/grove/internal/platform"
	"github.com/PrerakGada/grove/internal/registry"
	"github.com/PrerakGada/grove/internal/ui"
	"github.com/PrerakGada/grove/internal/worktree"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List this project's worktrees",
	Long: `Shows every git worktree with its slot, mode, dependency isolation,
uncommitted change count, and which of its services are currently listening.

The main checkout is slot 0 and is listed first.`,
	RunE: runList,
}

func init() { rootCmd.AddCommand(listCmd) }

func runList(cmd *cobra.Command, args []string) error {
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

	// Grove's registry wins over anything read from the filesystem.
	for i := range wts {
		if proj == nil || wts[i].IsMain {
			continue
		}
		if s, ok := proj.Slot(wts[i].Name); ok {
			wts[i].Slot, wts[i].HasSlot, wts[i].LegacySlot = s, true, false
		}
	}

	sort.SliceStable(wts, func(i, j int) bool {
		if wts[i].IsMain != wts[j].IsMain {
			return wts[i].IsMain
		}
		if wts[i].HasSlot != wts[j].HasSlot {
			return wts[i].HasSlot
		}
		if wts[i].Slot != wts[j].Slot {
			return wts[i].Slot < wts[j].Slot
		}
		return wts[i].Name < wts[j].Name
	})

	// Cells are plain text and widths come from the data. Styling inside a
	// padded column would count the escape sequence toward the width and skew
	// every row after it.
	legacy, unslotted := 0, 0
	rows := make([][]string, 0, len(wts))
	for _, w := range wts {
		slot := "-"
		if w.HasSlot {
			slot = strconv.Itoa(w.Slot)
			if w.LegacySlot {
				slot += "*"
				legacy++
			}
		} else {
			unslotted++
		}

		branch := w.Branch
		if w.Detached {
			branch = "(detached)"
		}

		name := w.Name
		if w.IsMain {
			name += " (main)"
		}

		dirty := "-"
		if w.Changes >= 0 {
			dirty = strconv.Itoa(w.Changes)
		}

		rows = append(rows, []string{
			name, branch, slot, orDash(w.Mode), dirty, orDash(w.Deps), runningOn(cfg, w),
		})
	}

	header := []string{"WORKTREE", "BRANCH", "SLOT", "MODE", "DIRTY", "DEPS", "RUNNING"}
	width := make([]int, len(header))
	for i, h := range header {
		width[i] = len(h)
	}
	for _, r := range rows {
		for i, c := range r {
			if len(c) > width[i] {
				width[i] = len(c)
			}
		}
	}

	line := func(cells []string) string {
		var b strings.Builder
		b.WriteString("  ")
		for i, c := range cells {
			if i == len(cells)-1 {
				b.WriteString(c) // last column is free to run long
				break
			}
			b.WriteString(c)
			b.WriteString(strings.Repeat(" ", width[i]-len(c)+2))
		}
		return strings.TrimRight(b.String(), " ")
	}

	fmt.Println()
	fmt.Printf("%s  %s\n", ui.Title.Render(cfg.Project), ui.Dim.Render(root))
	fmt.Println()
	fmt.Println(ui.Dim.Render(line(header)))
	for _, r := range rows {
		fmt.Println(line(r))
	}

	fmt.Println()
	if legacy > 0 {
		ui.Warn("%d worktree(s) marked * hold a slot from the previous tool's .slots file, "+
			"not from %s", legacy, registry.Home())
	}
	if unslotted > 0 {
		ui.Warn("%d worktree(s) have no slot, so Grove cannot resolve their ports", unslotted)
	}
	return nil
}

// runningOn reports which of a worktree's services are currently listening.
// Without a slot there are no ports to check.
func runningOn(cfg *config.Config, w worktree.Worktree) string {
	if !w.HasSlot {
		return "-"
	}
	var up []string
	for _, s := range cfg.Services {
		if len(platform.Listeners(s.Band+w.Slot)) > 0 {
			up = append(up, fmt.Sprintf("%s:%d", s.Name, s.Band+w.Slot))
		}
	}
	if cfg.HasDB() && len(platform.Listeners(cfg.DB.Band+w.Slot)) > 0 {
		up = append(up, fmt.Sprintf("db:%d", cfg.DB.Band+w.Slot))
	}
	if len(up) == 0 {
		return "—"
	}
	return strings.Join(up, " ")
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
