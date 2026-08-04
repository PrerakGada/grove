// Package config defines the per-repository Grove configuration (`.grove.json`).
//
// The schema exists to keep every project-specific fact — service names, port
// bands, package manager, database driver, env keys — out of the tool itself.
// Grove's own logic (slots, ports, process control, symlinks) is generic.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// FileName is the per-repository config, committed to git so a team shares one
// definition. Machine-specific state (band assignment, slot table) lives in the
// user's Grove home instead — see package registry.
const FileName = ".grove.json"

// Per-worktree marker files. They live in the worktree root, are gitignored,
// and record what was provisioned there so later commands do not have to guess.
const (
	// ModeFile holds "frontend" or "fullstack".
	ModeFile = ".grove-mode"
	// DepsFile holds "real" (own dependencies) or "light" (shared with main).
	DepsFile = ".grove-deps"
	// DBProfileFile holds the database profile this worktree was seeded from.
	DBProfileFile = ".grove-db-profile"
	// RunDir holds the background runner's per-service pid and log files.
	RunDir = ".grove-run"
)

// SchemaVersion is bumped only on breaking schema changes.
const SchemaVersion = 1

type Config struct {
	Version      int       `json:"version"`
	Project      string    `json:"project"`
	WorktreeDir  string    `json:"worktreeDir"`
	BranchPrefix string    `json:"branchPrefix"`
	MaxSlots     int       `json:"maxSlots"`
	Services     []Service `json:"services"`
	Deps         *Deps     `json:"deps,omitempty"`
	Env          *Env      `json:"env,omitempty"`
	DB           *DB       `json:"db,omitempty"`
	Links        []Link    `json:"links,omitempty"`
	Hooks        *Hooks    `json:"hooks,omitempty"`

	// path is where this config was loaded from; not serialised.
	path string
}

// Service is one long-running dev process. Each owns a 100-port band and the
// worktree's slot is a +1 offset into it, so the main checkout (slot 0) sits at
// the band itself and the last digits of any port are always the slot.
type Service struct {
	Name string `json:"name"`
	// Kind is "frontend" or "backend". A frontend runs in every mode; a backend
	// only runs when the worktree owns one (fullstack mode).
	Kind string      `json:"kind"`
	Band int         `json:"band"`
	Cmd  string      `json:"cmd"`
	Port PortBinding `json:"port"`
	// ReadySeconds is how long to wait for the port to bind before reporting
	// the service as slow rather than started. 0 means use the default.
	ReadySeconds int `json:"readySeconds,omitempty"`
}

// PortBinding says how a service is told which port to use. Both forms exist in
// the wild and getting it wrong is silent: Next.js `-p` overrides the PORT env
// var, so a project that only sets PORT will bind the wrong port forever.
type PortBinding struct {
	Via string `json:"via"` // "env" or "flag"
	Key string `json:"key"` // e.g. "PORT", or "--port" / "-p"
}

type Deps struct {
	// Steps run in order to materialise a new worktree's dependencies. A
	// single-toolchain repository has one; a polyglot one has several, because
	// a Flutter app and a Node server cannot be installed by the same command.
	Steps []DepStep `json:"steps,omitempty"`
	// Workspaces are subdirectories that carry their own dependency directory.
	Workspaces []string `json:"workspaces,omitempty"`
	// Shareable allows the `light` escape hatch, which symlinks the main
	// checkout's dependencies instead of installing. Set false for projects
	// where sharing corrupts state (generated clients, bundler caches).
	Shareable bool `json:"shareable"`
	// Seed paths are copy-on-write cloned from the main checkout after install,
	// so an expensive generated artifact costs ~0 bytes until it is regenerated.
	Seed []string `json:"seed,omitempty"`
}

type DepStep struct {
	// Dir is relative to the worktree root; "." is the root itself.
	Dir     string `json:"dir"`
	Manager string `json:"manager"`
	Install string `json:"install"`
}

// Managers lists the distinct package managers this project installs with.
func (d *Deps) Managers() []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range d.Steps {
		if s.Manager != "" && !seen[s.Manager] {
			seen[s.Manager] = true
			out = append(out, s.Manager)
		}
	}
	return out
}

// Env lists every environment file a worktree needs rewritten. A repository
// with one root .env has a single entry; a polyglot one has the file that
// belongs to each sub-project, since there is no shared root to write to.
type Env struct {
	Files []EnvFile `json:"files"`
}

type EnvFile struct {
	// Path is relative to the worktree root.
	Path string `json:"path"`
	// Inherit is "copy" (own file, seeded from the main checkout), "link"
	// (symlink to the main checkout's), or "none".
	Inherit string `json:"inherit"`
	// Set applies in every mode; Fullstack applies only when the worktree owns
	// its backend and database.
	//
	// These are computed key->value assignments, never textual substitution of
	// the main checkout's current values. That distinction matters: the bash
	// predecessor sed-ed the main repo's literal port numbers, so the day those
	// ports changed, every worktree silently kept pointing at the main backend.
	Set       map[string]string `json:"set,omitempty"`
	Fullstack map[string]string `json:"fullstack,omitempty"`
}

// File returns the entry for a path, creating it if absent.
func (e *Env) File(path string) *EnvFile {
	for i := range e.Files {
		if e.Files[i].Path == path {
			return &e.Files[i]
		}
	}
	e.Files = append(e.Files, EnvFile{
		Path:      path,
		Inherit:   "copy",
		Set:       map[string]string{},
		Fullstack: map[string]string{},
	})
	return &e.Files[len(e.Files)-1]
}

type DB struct {
	Driver    string `json:"driver"` // "docker-postgres" or "none"
	Band      int    `json:"band"`
	Image     string `json:"image"`
	User      string `json:"user"`
	Password  string `json:"password"`
	Name      string `json:"name"`      // template
	Container string `json:"container"` // template
	Volume    string `json:"volume"`    // template
	// DumpDir holds one .sql per profile. The first worktree to ask for a
	// profile creates it via Profiles[name]; every later one restores the file.
	DumpDir    string            `json:"dumpDir"`
	Profiles   map[string]string `json:"profiles"`
	Default    string            `json:"default"`
	Migrate    string            `json:"migrate,omitempty"`
	MigrateDir string            `json:"migrateDir,omitempty"`
	Verify     Verify            `json:"verify"`
}

// Verify is the post-restore proof that a database actually holds data. A
// silently-empty database was the single most common worktree failure; these
// thresholds turn it into a hard error at provisioning time.
type Verify struct {
	MinTables int `json:"minTables"`
	MinRows   int `json:"minRows"`
}

// Link is a symlink recreated inside every worktree. Most exist because they
// are gitignored in the main checkout and so are absent from a fresh one.
type Link struct {
	From string `json:"from"`
	To   string `json:"to"`
	// ForEach expands the entry once per dependency workspace, exposing the
	// workspace path as {{ws}}.
	ForEach string `json:"forEach,omitempty"`
	// Optional entries are skipped when the source does not exist.
	Optional bool `json:"optional,omitempty"`
}

type Hooks struct {
	PostCreate []string `json:"postCreate,omitempty"`
	PostDB     []string `json:"postDB,omitempty"`
}

// Path returns where this config was loaded from.
func (c *Config) Path() string { return c.path }

// Root returns the repository root the config belongs to.
func (c *Config) Root() string { return filepath.Dir(c.path) }

// Service looks up a service by name.
func (c *Config) Service(name string) (Service, bool) {
	for _, s := range c.Services {
		if s.Name == name {
			return s, true
		}
	}
	return Service{}, false
}

// Frontends returns services that run in every mode.
func (c *Config) Frontends() []Service {
	var out []Service
	for _, s := range c.Services {
		if s.Kind != "backend" {
			out = append(out, s)
		}
	}
	return out
}

// Backends returns services that only run when the worktree owns them.
func (c *Config) Backends() []Service {
	var out []Service
	for _, s := range c.Services {
		if s.Kind == "backend" {
			out = append(out, s)
		}
	}
	return out
}

// HasDB reports whether this project provisions databases at all.
func (c *Config) HasDB() bool {
	return c.DB != nil && c.DB.Driver != "" && c.DB.Driver != "none"
}

// Bands lists every port band the project claims, database included. Used to
// reserve a non-overlapping range in the global registry.
func (c *Config) Bands() []int {
	var out []int
	for _, s := range c.Services {
		out = append(out, s.Band)
	}
	if c.HasDB() {
		out = append(out, c.DB.Band)
	}
	return out
}

// Load reads the config at root/.grove.json.
func Load(root string) (*Config, error) {
	p := filepath.Join(root, FileName)
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("no %s in %s — run `grove init` here first", FileName, root)
		}
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	c.path = p
	c.applyDefaults()
	return &c, nil
}

// Save writes the config to root/.grove.json.
func (c *Config) Save(root string) error {
	c.Version = SchemaVersion
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	p := filepath.Join(root, FileName)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		return err
	}
	c.path = p
	return nil
}

func (c *Config) applyDefaults() {
	if c.WorktreeDir == "" {
		c.WorktreeDir = "worktrees"
	}
	if c.BranchPrefix == "" {
		c.BranchPrefix = "feat/"
	}
	if c.MaxSlots == 0 {
		c.MaxSlots = 99
	}
	if c.Env != nil {
		for i := range c.Env.Files {
			if c.Env.Files[i].Inherit == "" {
				c.Env.Files[i].Inherit = "copy"
			}
		}
	}
	if c.DB != nil {
		if c.DB.Verify.MinTables == 0 {
			c.DB.Verify.MinTables = 1
		}
		if c.DB.Default == "" {
			for k := range c.DB.Profiles {
				c.DB.Default = k
				break
			}
		}
	}
	for i := range c.Services {
		if c.Services[i].Kind == "" {
			c.Services[i].Kind = "frontend"
		}
		if c.Services[i].Port.Via == "" {
			c.Services[i].Port.Via = "env"
		}
		if c.Services[i].Port.Key == "" {
			c.Services[i].Port.Key = "PORT"
		}
	}
}
