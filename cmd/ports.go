package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/PrerakGada/grove/internal/config"
	"github.com/PrerakGada/grove/internal/registry"
	"github.com/PrerakGada/grove/internal/ui"
	"github.com/spf13/cobra"
)

var portsGlobal bool

var portsCmd = &cobra.Command{
	Use:   "ports",
	Short: "Show the port allocation table",
	Long: `Prints this project's bands and the ports each slot resolves to.

Slot 0 is the main checkout, so a service's band is also the port it uses
outside any worktree, and the trailing digits of any port are always the slot.`,
	RunE: runPorts,
}

func init() {
	portsCmd.Flags().BoolVarP(&portsGlobal, "global", "g", false,
		"show every project registered on this machine")
	rootCmd.AddCommand(portsCmd)
}

func runPorts(cmd *cobra.Command, args []string) error {
	if portsGlobal {
		return printGlobalPorts()
	}

	cfg, root, err := loadConfig()
	if err != nil {
		return err
	}
	reg, err := registry.Load()
	if err != nil {
		return err
	}
	proj := reg.Projects[cfg.Project]

	fmt.Println()
	fmt.Printf("%s  %s\n", ui.Title.Render(cfg.Project), ui.Dim.Render(root))
	fmt.Println()

	svcs := cfg.Services
	header := fmt.Sprintf("  %-10s %-8s", "", "band")
	for _, s := range svcs {
		header += fmt.Sprintf(" %-9s", s.Name)
	}
	if cfg.HasDB() {
		header += fmt.Sprintf(" %-9s", "db")
	}
	fmt.Println(ui.Dim.Render(header))

	line := func(label string, slot int) {
		row := fmt.Sprintf("  %-10s %-8d", label, slot)
		for _, s := range svcs {
			row += fmt.Sprintf(" %-9d", s.Band+slot)
		}
		if cfg.HasDB() {
			row += fmt.Sprintf(" %-9d", cfg.DB.Band+slot)
		}
		fmt.Println(row)
	}

	line("main", 0)
	if proj != nil {
		names := make([]string, 0, len(proj.Slots))
		for n := range proj.Slots {
			names = append(names, n)
		}
		sort.Slice(names, func(i, j int) bool { return proj.Slots[names[i]] < proj.Slots[names[j]] })
		for _, n := range names {
			line(n, proj.Slots[n])
		}
	}

	fmt.Println()
	used := 0
	if proj != nil {
		used = len(proj.Slots)
	}
	ui.Info("%d of %d slots in use. Bands: %s", used, cfg.MaxSlots, bandList(cfg.Bands()))
	return nil
}

func printGlobalPorts() error {
	reg, err := registry.Load()
	if err != nil {
		return err
	}
	if len(reg.Projects) == 0 {
		ui.Info("No projects registered yet. Run `grove init` in a repository.")
		return nil
	}

	names := make([]string, 0, len(reg.Projects))
	for n := range reg.Projects {
		names = append(names, n)
	}
	sort.Strings(names)

	fmt.Println()
	fmt.Println(ui.Title.Render("Registered projects") + "  " + ui.Dim.Render(registry.Home()))
	fmt.Println()
	fmt.Printf(ui.Dim.Render("  %-16s %-6s %-8s %s")+"\n", "project", "slots", "max", "bands")
	for _, n := range names {
		p := reg.Projects[n]
		fmt.Printf("  %-16s %-6d %-8d %s\n", n, len(p.Slots), p.MaxSlots, bandList(p.Bands))
	}
	fmt.Println()
	for _, n := range names {
		fmt.Printf("  %s %s\n", ui.Key.Render(n), ui.Dim.Render(reg.Projects[n].Root))
	}
	fmt.Println()
	return nil
}

func jsonPreview(cfg *config.Config) (string, error) {
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, line := range strings.Split(string(b), "\n") {
		sb.WriteString("  " + line + "\n")
	}
	return sb.String(), nil
}
