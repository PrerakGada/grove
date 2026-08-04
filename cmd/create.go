package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/PrerakGada/grove/internal/config"
	"github.com/PrerakGada/grove/internal/provision"
	"github.com/PrerakGada/grove/internal/registry"
	"github.com/PrerakGada/grove/internal/ui"
	"github.com/spf13/cobra"
)

var wtNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

type provisionFlags struct {
	frontend   bool
	fullstack  bool
	sharedDeps bool
	ownDeps    bool
	noDB       bool
	dbProfile  string
}

func (f *provisionFlags) register(c *cobra.Command) {
	c.Flags().BoolVar(&f.frontend, "frontend", false,
		"share the main checkout's backend and database; no container of its own")
	c.Flags().BoolVar(&f.fullstack, "fullstack", false, "own backend and own database (default)")
	c.Flags().BoolVar(&f.sharedDeps, "shared-deps", false,
		"symlink the main checkout's dependencies instead of installing")
	c.Flags().BoolVar(&f.ownDeps, "own-deps", false, "force an installation of its own")
	c.Flags().BoolVar(&f.noDB, "no-db", false, "do not provision a database")
	c.Flags().StringVar(&f.dbProfile, "db", "", "database profile to seed from")
}

func (f *provisionFlags) mode() string {
	if f.frontend {
		return provision.ModeFrontend
	}
	return provision.ModeFullstack
}

func (f *provisionFlags) deps() string {
	if f.sharedDeps && !f.ownDeps {
		return provision.DepsLight
	}
	return provision.DepsReal
}

func (f *provisionFlags) wantDB(cfg *config.Config) bool {
	if f.noDB || f.frontend || !cfg.HasDB() {
		return false
	}
	return true
}

func (f *provisionFlags) profile(cfg *config.Config) string {
	if f.dbProfile != "" {
		return f.dbProfile
	}
	return cfg.DB.Default
}

var createFlags provisionFlags

var createCmd = &cobra.Command{
	Use:   "create <name> [base-branch]",
	Short: "Create a worktree on a new branch, fully provisioned",
	Long: `Creates a branch, checks it out into a worktree, allocates a slot, writes
its environment files, installs its dependencies, and provisions its database —
seeded and verified non-empty before the command returns.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runCreate,
}

func init() {
	createFlags.register(createCmd)
	rootCmd.AddCommand(createCmd)
}

func runCreate(cmd *cobra.Command, args []string) error {
	cfg, root, err := loadConfig()
	if err != nil {
		return err
	}
	name := args[0]
	if !wtNameRe.MatchString(name) {
		return fmt.Errorf("invalid worktree name %q", name)
	}
	base := "main"
	if len(args) == 2 {
		base = args[1]
	}

	branch := cfg.BranchPrefix + name
	wtPath := worktreePath(cfg, root, name)

	if _, err := os.Stat(wtPath); err == nil {
		return fmt.Errorf("%s already exists", wtPath)
	}

	// A branch of this name already existing almost always means the intent was
	// to check out that branch, not to fork a new one from it.
	if branchExists(root, branch) || branchExists(root, name) {
		existing := branch
		if !branchExists(root, branch) {
			existing = name
		}
		return fmt.Errorf("branch %q already exists — `create` always makes a new one.\n"+
			"  To check out the existing branch instead:\n    grove track %s%s",
			existing, existing, flagSuffix(&createFlags))
	}

	slot, err := allocateSlot(cfg, root, name)
	if err != nil {
		return err
	}

	ui.Info("creating worktree %q on branch %s (from %s)", name, branch, base)
	add := exec.Command("git", "-C", root, "worktree", "add", "-b", branch, wtPath, base)
	add.Stdout, add.Stderr = os.Stdout, os.Stderr
	if err := add.Run(); err != nil {
		releaseSlot(cfg, name)
		return fmt.Errorf("git worktree add failed: %w", err)
	}

	return finishProvision(cfg, root, wtPath, name, slot, &createFlags)
}

// finishProvision is the shared tail of create, track and setup.
func finishProvision(cfg *config.Config, root, wtPath, name string, slot int, f *provisionFlags) error {
	if err := provision.Setup(provision.Options{
		Config: cfg, MainRoot: root, Path: wtPath, Name: name,
		Slot: slot, Mode: f.mode(), Deps: f.deps(),
	}); err != nil {
		return err
	}

	if f.wantDB(cfg) {
		if err := provisionDB(cfg, root, wtPath, name, slot, f.profile(cfg), false, false); err != nil {
			return err
		}
	}

	fmt.Println()
	ui.Ok("%q is ready at %s", name, wtPath)
	printPorts(cfg, slot)
	fmt.Println()
	ui.Info("Start it: grove up %s", name)
	return nil
}

func printPorts(cfg *config.Config, slot int) {
	var parts []string
	for _, s := range cfg.Services {
		parts = append(parts, fmt.Sprintf("%s:%d", s.Name, s.Band+slot))
	}
	if cfg.HasDB() {
		parts = append(parts, fmt.Sprintf("db:%d", cfg.DB.Band+slot))
	}
	ui.Info("slot %d — %s", slot, strings.Join(parts, "  "))
}

func allocateSlot(cfg *config.Config, root, name string) (int, error) {
	var slot int
	err := registry.Update(func(r *registry.Registry) error {
		p := r.Projects[cfg.Project]
		if p == nil {
			r.Register(cfg.Project, root, cfg.Bands(), cfg.MaxSlots)
			p = r.Projects[cfg.Project]
		}
		p.MaxSlots = cfg.MaxSlots
		s, err := p.Allocate(name)
		slot = s
		return err
	})
	return slot, err
}

func releaseSlot(cfg *config.Config, name string) {
	_ = registry.Update(func(r *registry.Registry) error {
		if p := r.Projects[cfg.Project]; p != nil {
			p.Release(name)
		}
		return nil
	})
}

func slotOf(cfg *config.Config, name string) (int, bool) {
	reg, err := registry.Load()
	if err != nil {
		return 0, false
	}
	p := reg.Projects[cfg.Project]
	if p == nil {
		return 0, false
	}
	return p.Slot(name)
}

func branchExists(root, branch string) bool {
	return exec.Command("git", "-C", root, "show-ref", "--verify", "--quiet",
		"refs/heads/"+branch).Run() == nil
}

func flagSuffix(f *provisionFlags) string {
	var s string
	if f.frontend {
		s += " --frontend"
	}
	if f.sharedDeps {
		s += " --shared-deps"
	}
	if f.noDB {
		s += " --no-db"
	}
	if f.dbProfile != "" {
		s += " --db " + f.dbProfile
	}
	return s
}

// requireWorktree resolves a named worktree's path and slot.
func requireWorktree(cfg *config.Config, root, name string) (string, int, error) {
	p := worktreePath(cfg, root, name)
	if _, err := os.Stat(p); err != nil {
		return "", 0, fmt.Errorf("no worktree named %q in %s", name, filepath.Join(root, cfg.WorktreeDir))
	}
	slot, ok := slotOf(cfg, name)
	if !ok {
		return p, 0, fmt.Errorf("%q has no slot — run: grove setup %s", name, name)
	}
	return p, slot, nil
}
