package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/PrerakGada/grove/internal/config"
	"github.com/PrerakGada/grove/internal/detect"
	"github.com/PrerakGada/grove/internal/registry"
	"github.com/PrerakGada/grove/internal/ui"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var (
	initYes   bool
	initForce bool
	initDry   bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Detect this repository and write .wt.json",
	Long: `Inspects the repository — package manager, workspaces, dev scripts, how each
service takes its port, Prisma, docker-compose — proposes a configuration, and
writes .wt.json after you confirm it.

Port bands are reserved in ~/.wt so they cannot collide with another project on
this machine, or with well-known defaults like 3000, 5173 and 5432.`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolVarP(&initYes, "yes", "y", false, "accept everything detected without prompting")
	initCmd.Flags().BoolVar(&initForce, "force", false, "overwrite an existing .wt.json")
	initCmd.Flags().BoolVar(&initDry, "dry-run", false, "print the proposed config without writing it")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	root, err := repoRoot()
	if err != nil {
		return err
	}

	if _, err := os.Stat(filepath.Join(root, config.FileName)); err == nil && !initForce && !initDry {
		return fmt.Errorf("%s already exists in %s (use --force to overwrite, or edit it directly)",
			config.FileName, root)
	}

	det, err := detect.Run(root)
	if err != nil {
		return err
	}
	printDetection(det)
	if len(det.Services) == 0 {
		return fmt.Errorf("no runnable services found in %s\n"+
			"Grove looks for workspaces whose package.json has a \"dev\" script.\n"+
			"Write %s by hand if this project starts differently.", root, config.FileName)
	}

	reg, err := registry.Load()
	if err != nil {
		return err
	}

	cfg, err := buildConfig(det, reg)
	if err != nil {
		return err
	}

	if !initYes {
		if err := promptConfig(cfg, det, reg); err != nil {
			return err
		}
	}

	if err := cfg.Validate(); err != nil {
		return err
	}
	hard := reg.ProjectConflicts(cfg.Project, cfg.Bands(), cfg.MaxSlots)
	soft := reg.ReservedConflicts(cfg.Bands(), cfg.MaxSlots)
	for _, c := range soft {
		ui.Warn("%s", c)
	}
	if len(soft) > 0 {
		fmt.Printf("    %s\n", ui.Dim.Render(
			"Only bites at that one slot. Lower maxSlots or move the band to avoid it."))
	}
	if len(hard) > 0 {
		for _, c := range hard {
			ui.Err("%s", c)
		}
		if initYes || initDry {
			return fmt.Errorf("port bands collide with another project — fix them and re-run")
		}
		var proceed bool
		if err := huh.NewConfirm().
			Title("Reserve these bands anyway?").
			Description("Two projects owning one port means whichever starts second fails to bind.").
			Value(&proceed).Run(); err != nil {
			return err
		}
		if !proceed {
			return fmt.Errorf("aborted — adjust the bands and run `%s init` again", Name())
		}
	}

	if initDry {
		b, _ := jsonPreview(cfg)
		fmt.Println()
		fmt.Println(ui.Title.Render("Proposed " + config.FileName))
		fmt.Println()
		fmt.Println(b)
		ui.Info("Dry run — nothing written.")
		return nil
	}

	if err := cfg.Save(root); err != nil {
		return err
	}
	if err := registry.Update(func(r *registry.Registry) error {
		r.Register(cfg.Project, root, cfg.Bands(), cfg.MaxSlots)
		return nil
	}); err != nil {
		return err
	}

	fmt.Println()
	ui.Ok("wrote %s", filepath.Join(root, config.FileName))
	ui.Ok("reserved bands %s in %s", bandList(cfg.Bands()), registry.Home())
	fmt.Println()
	ui.Info("Next: `%s create <name>` to provision a worktree, `%s ports` to see the table.", Name(), Name())
	return nil
}

func printDetection(d *detect.Result) {
	fmt.Println()
	fmt.Println(ui.Title.Render("Detected"))
	fmt.Println()
	row := func(k, v string) {
		if v == "" {
			v = ui.Dim.Render("none")
		}
		fmt.Printf("  %-14s %s\n", ui.Key.Render(k), v)
	}
	row("root", d.Root)
	row("package manager", d.PackageMgr)
	row("workspaces", strconv.Itoa(len(d.Workspaces)))
	row("env file", d.EnvFile)
	if d.HasPrisma {
		row("prisma", d.PrismaDir)
	}
	if d.DB != nil {
		row("database", fmt.Sprintf("%s (%s, host port %d)", d.DB.Driver, d.DB.Image, d.DB.HostPort))
	}
	fmt.Println()
	fmt.Println("  " + ui.Key.Render("services"))
	for _, s := range d.Services {
		port := ui.Dim.Render("port not found")
		if s.CurrentPort > 0 {
			port = fmt.Sprintf("currently :%d", s.CurrentPort)
		}
		via := s.PortKey
		if s.PortVia == "flag" {
			via = s.PortKey + " (flag)"
		}
		fmt.Printf("    %-12s %-9s %-8s %-14s %s\n",
			s.Name, s.Kind, s.Framework, via, port)
	}
	fmt.Println()
}

func buildConfig(d *detect.Result, reg *registry.Registry) (*config.Config, error) {
	cfg := &config.Config{
		Version:      config.SchemaVersion,
		Project:      d.Project,
		WorktreeDir:  d.WorktreeDir,
		BranchPrefix: "feat/",
		MaxSlots:     99,
	}

	// Band assignment runs in two passes. Services whose current port is known
	// claim the matching band first, so adopting Grove does not move ports that
	// already work; only the leftovers get an arbitrary free band.
	//
	// One pass would let whichever service happened to be first take a band
	// another service already uses, shuffling every port in the project.
	bands := map[string]int{}
	taken := map[int]bool{}
	// A band the project already uses is kept even if it contains a well-known
	// default port — moving working ports is worse than reporting the overlap,
	// which `grove doctor` does. Bands chosen automatically avoid them outright.
	keepable := func(b int) bool {
		return !taken[b] && len(reg.ProjectConflicts(cfg.Project, []int{b}, cfg.MaxSlots)) == 0
	}
	free := func(b int) bool {
		return keepable(b) && len(reg.ReservedConflicts([]int{b}, cfg.MaxSlots)) == 0
	}

	type want struct {
		key  string
		pref int
	}
	var wants []want
	for _, s := range d.Services {
		wants = append(wants, want{s.Name, s.CurrentPort})
	}
	if d.DB != nil {
		wants = append(wants, want{"\x00db", d.DB.HostPort})
	}

	for _, w := range wants {
		if w.pref <= 0 {
			continue
		}
		b := (w.pref / registry.BandStep) * registry.BandStep
		if b >= 1024 && keepable(b) {
			taken[b] = true
			bands[w.key] = b
		}
	}
	for _, w := range wants {
		if _, done := bands[w.key]; done {
			continue
		}
		for b := registry.SearchLow; b <= registry.SearchHigh; b += registry.BandStep {
			if free(b) {
				taken[b] = true
				bands[w.key] = b
				break
			}
		}
		if _, done := bands[w.key]; !done {
			return nil, fmt.Errorf("ran out of free port bands between %d and %d",
				registry.SearchLow, registry.SearchHigh)
		}
	}

	for _, s := range d.Services {
		cfg.Services = append(cfg.Services, config.Service{
			Name: s.Name,
			Kind: s.Kind,
			Band: bands[s.Name],
			Cmd:  s.Cmd,
			Port: config.PortBinding{Via: s.PortVia, Key: s.PortKey},
		})
	}

	if d.PackageMgr != "" {
		cfg.Deps = &config.Deps{
			Manager:    d.PackageMgr,
			Install:    d.InstallCmd,
			Workspaces: d.Workspaces,
			Shareable:  true,
		}
		if d.HasPrisma {
			// A generated Prisma client is expensive to rebuild and cheap to
			// clone, so seed it copy-on-write into each new worktree.
			cfg.Deps.Seed = []string{"node_modules/.prisma/client"}
			// Sharing dependencies would put every worktree's generated client
			// at the same path, so one `prisma generate` rewrites it for all.
			cfg.Deps.Shareable = false
		}
	}

	if d.EnvFile != "" {
		cfg.Env = &config.Env{
			File:      d.EnvFile,
			Inherit:   "copy",
			Set:       map[string]string{},
			Fullstack: map[string]string{},
		}
		for _, s := range cfg.Services {
			// A shared env file can hold only one value per key, so a port key
			// used by more than one service (the generic PORT is the usual
			// case) cannot live here. The runner injects those per process at
			// launch instead — which is the only correct place for them.
			if s.Port.Via == "env" && s.Kind != "backend" && d.KeyIsUnique(s.Port.Key) {
				cfg.Env.Set[s.Port.Key] = "{{port." + s.Name + "}}"
			}
		}
		if be := firstBackend(cfg); be != "" {
			for _, s := range cfg.Services {
				if s.Name == be && s.Port.Via == "env" {
					cfg.Env.Fullstack[s.Port.Key] = "{{port." + be + "}}"
				}
			}
			for _, k := range d.EnvKeysMatching("API_URL", "BACKEND_URL") {
				cfg.Env.Fullstack[k] = "http://localhost:{{port." + be + "}}/"
			}
		}
	}

	if d.DB != nil {
		cfg.DB = &config.DB{
			Driver:    d.DB.Driver,
			Band:      bands["\x00db"],
			Image:     orDefault(d.DB.Image, "postgres:16"),
			User:      orDefault(d.DB.User, "postgres"),
			Password:  orDefault(d.DB.Password, "postgres"),
			Name:      dbTemplate(d.DB.Name, cfg.Project),
			Container: "postgres-" + cfg.Project + "-wt{{slot}}",
			Volume:    "pgdata-" + cfg.Project + "-wt{{slot}}",
			DumpDir:   "prisma/dumps",
			Profiles:  d.DB.Profiles,
			Verify:    config.Verify{MinTables: 10, MinRows: 1},
		}
		if cfg.DB.Profiles == nil {
			cfg.DB.Profiles = map[string]string{}
		}
		// Prefer a seed profile as the default: it is the cheapest to create
		// and the safest thing to hand a new worktree.
		for _, p := range []string{"seed", "dev", "prod"} {
			if _, ok := cfg.DB.Profiles[p]; ok {
				cfg.DB.Default = p
				break
			}
		}
		if !d.HasPrisma {
			cfg.DB.DumpDir = "db/dumps"
		} else {
			cfg.DB.Migrate = "npx prisma migrate deploy --schema=" + d.PrismaDir
			cfg.DB.MigrateDir = backendDir(d)
		}
		if cfg.Env != nil {
			cfg.Env.Fullstack["DATABASE_URL"] = fmt.Sprintf(
				"postgresql://%s:%s@localhost:{{port.db}}/{{db.name}}",
				cfg.DB.User, cfg.DB.Password)
		}
	}

	return cfg, nil
}

func promptConfig(cfg *config.Config, d *detect.Result, reg *registry.Registry) error {
	names := make([]string, 0, len(cfg.Services))
	for _, s := range cfg.Services {
		names = append(names, s.Name)
	}
	selected := append([]string(nil), names...)

	opts := make([]huh.Option[string], 0, len(names))
	for _, s := range cfg.Services {
		label := fmt.Sprintf("%-12s %s → band %d", s.Name, s.Kind, s.Band)
		opts = append(opts, huh.NewOption(label, s.Name).Selected(true))
	}

	maxSlots := strconv.Itoa(cfg.MaxSlots)
	wantDB := cfg.DB != nil

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Project name").
				Description("Identifies this repository in ~/.wt and reserves its port bands.").
				Value(&cfg.Project),
			huh.NewInput().
				Title("Worktree directory").
				Description("Relative to the repository root.").
				Value(&cfg.WorktreeDir),
			huh.NewInput().
				Title("Branch prefix").
				Description("`"+Name()+" create foo` makes this branch. Leave empty for no prefix.").
				Value(&cfg.BranchPrefix),
			huh.NewInput().
				Title("Max slots").
				Description("How many worktrees can run at once. Also the band spacing.").
				Value(&maxSlots).
				Validate(func(s string) error {
					n, err := strconv.Atoi(s)
					if err != nil || n < 1 || n > 999 {
						return fmt.Errorf("must be a number between 1 and 999")
					}
					return nil
				}),
		),
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Which of these are dev services?").
				Description("Unselected workspaces are treated as libraries and never started.").
				Options(opts...).
				Value(&selected),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Give each worktree its own database?").
				Description(dbDescription(d)).
				Value(&wantDB),
		).WithHideFunc(func() bool { return d.DB == nil }),
	)

	if err := form.Run(); err != nil {
		return err
	}

	cfg.MaxSlots, _ = strconv.Atoi(maxSlots)

	keep := map[string]bool{}
	for _, n := range selected {
		keep[n] = true
	}
	var kept []config.Service
	for _, s := range cfg.Services {
		if keep[s.Name] {
			kept = append(kept, s)
		}
	}
	cfg.Services = kept

	if !wantDB {
		cfg.DB = nil
		if cfg.Env != nil {
			delete(cfg.Env.Fullstack, "DATABASE_URL")
		}
	}

	// Bands and env rules reference service names; drop anything now orphaned.
	if cfg.Env != nil {
		for k, v := range cfg.Env.Set {
			if refsMissingService(v, cfg) {
				delete(cfg.Env.Set, k)
			}
		}
		for k, v := range cfg.Env.Fullstack {
			if refsMissingService(v, cfg) {
				delete(cfg.Env.Fullstack, k)
			}
		}
	}
	return nil
}

func dbDescription(d *detect.Result) string {
	if d.DB == nil {
		return ""
	}
	return fmt.Sprintf("Found %s in %s. Each worktree gets its own container, volume and port.",
		d.DB.Image, d.DB.Compose)
}

func refsMissingService(tmpl string, cfg *config.Config) bool {
	const prefix = "{{port."
	i := strings.Index(tmpl, prefix)
	if i < 0 {
		return false
	}
	rest := tmpl[i+len(prefix):]
	j := strings.Index(rest, "}}")
	if j < 0 {
		return false
	}
	name := strings.TrimSpace(rest[:j])
	if name == "db" {
		return !cfg.HasDB()
	}
	_, ok := cfg.Service(name)
	return !ok
}

func firstBackend(cfg *config.Config) string {
	for _, s := range cfg.Services {
		if s.Kind == "backend" {
			return s.Name
		}
	}
	return ""
}

func backendDir(d *detect.Result) string {
	for _, s := range d.Services {
		if s.Kind == "backend" {
			return s.Dir
		}
	}
	return "."
}

func dbTemplate(current, project string) string {
	if current == "" {
		return project + "_wt{{slot}}"
	}
	// Strip a trailing _local / _dev so the template reads naturally.
	base := strings.TrimSuffix(strings.TrimSuffix(current, "_local"), "_dev")
	return base + "_wt{{slot}}"
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func bandList(bands []int) string {
	sort.Ints(bands)
	parts := make([]string, len(bands))
	for i, b := range bands {
		parts[i] = strconv.Itoa(b)
	}
	return strings.Join(parts, ", ")
}
