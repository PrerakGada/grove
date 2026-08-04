// Package detect inspects a repository and proposes a Grove configuration.
//
// Detection is a starting point, never an authority: `wt init` shows everything
// found here and lets the answer be corrected. The goal is that a typical
// JavaScript monorepo needs confirmation rather than authoring.
package detect

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Result struct {
	Root       string
	Project    string
	PackageMgr string
	InstallCmd string
	Workspaces []string
	Services   []Service
	DB         *DB
	EnvFile    string
	EnvKeys    []string
	EnvValues  map[string]string
	// ServiceEnv maps a service name to its own env file, when it has one
	// instead of sharing a root file.
	ServiceEnv map[string]string
	// DepSteps are per-sub-project install commands, used when the repository
	// has no single root package manager.
	DepSteps []DepStep
	// TaskRunner and Recipes describe a `just`-style command layer, if present.
	TaskRunner string
	Recipes    []Recipe
	// Polyglot is true when services were found by scanning sub-projects
	// rather than by reading a workspace manifest.
	Polyglot    bool
	HasPrisma   bool
	PrismaOwner string
	PrismaDir   string
	WorktreeDir string
}

// DepStep is one directory that installs its own dependencies.
type DepStep struct {
	Dir     string
	Manager string
	Install string
}

type Service struct {
	Name string
	Dir  string
	Kind string // frontend | backend
	// Framework is informational: next, vite, nest, express, flutter, unknown.
	Framework string
	Cmd       string
	PortVia   string // env | flag
	PortKey   string
	// RunScript is the wrapper script a project uses to start itself, if any.
	RunScript string
	// CurrentPort is the port the project uses today, if one could be found.
	// It seeds the suggested band so existing habits survive the migration.
	CurrentPort int
}

type DB struct {
	Driver    string
	Image     string
	User      string
	Password  string
	Name      string
	HostPort  int
	Compose   string
	Container string
	// Profiles maps a profile name (seed / dev / prod) to the command that
	// produces its dump the first time it is asked for.
	Profiles map[string]string
}

// Root resolves the main worktree root for any path inside a repository —
// including a linked worktree or a subdirectory. --git-common-dir points at the
// main checkout's .git for every one of those cases, which is what makes this a
// single rule instead of three.
func Root(start string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	cmd.Dir = start
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	gitDir := strings.TrimSpace(string(out))
	if filepath.Base(gitDir) == ".git" {
		return filepath.Dir(gitDir), nil
	}
	// A bare repository, or a submodule whose git dir lives elsewhere. Fall
	// back to the toplevel of the checkout we were asked about.
	cmd = exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = start
	out, err = cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Run inspects the repository rooted at root.
func Run(root string) (*Result, error) {
	r := &Result{
		Root:        root,
		Project:     sanitise(filepath.Base(root)),
		WorktreeDir: "worktrees",
	}

	r.detectPackageManager()
	if pkg := readPackageJSON(filepath.Join(root, "package.json")); pkg != nil && pkg.Name != "" {
		// Prefer the declared package name over the directory name: a folder
		// called "monorepo" says nothing, "web" identifies the project.
		if n := sanitise(strings.TrimPrefix(pkg.Name, "@")); n != "" {
			r.Project = n
		}
	}
	r.detectWorkspaces()
	r.detectServices()
	r.detectJustfile()

	// Fall back to scanning sub-projects only when the workspace path found
	// nothing runnable — a polyglot repository has no root manifest to read.
	if len(r.Services) == 0 {
		r.Polyglot = true
		r.detectSubProjects()
		r.applyRecipes()
	} else if r.PackageMgr != "" {
		r.DepSteps = []DepStep{{Dir: ".", Manager: r.PackageMgr, Install: r.InstallCmd}}
	}

	r.detectPrisma()
	r.detectDB()
	r.detectEnv()
	r.portsFromEnv()

	sort.Slice(r.Services, func(i, j int) bool {
		// Backends first — they are what a fullstack worktree provisions.
		if (r.Services[i].Kind == "backend") != (r.Services[j].Kind == "backend") {
			return r.Services[i].Kind == "backend"
		}
		return r.Services[i].Name < r.Services[j].Name
	})
	return r, nil
}

func (r *Result) detectPackageManager() {
	switch {
	case exists(r.Root, "pnpm-lock.yaml"), exists(r.Root, "pnpm-workspace.yaml"):
		r.PackageMgr = "pnpm"
		r.InstallCmd = "pnpm install --config.confirmModulesPurge=false"
	case exists(r.Root, "bun.lockb"), exists(r.Root, "bun.lock"):
		r.PackageMgr = "bun"
		r.InstallCmd = "bun install"
	case exists(r.Root, "yarn.lock"):
		r.PackageMgr = "yarn"
		r.InstallCmd = "yarn install"
	case exists(r.Root, "package-lock.json"):
		r.PackageMgr = "npm"
		r.InstallCmd = "npm install"
	case exists(r.Root, "package.json"):
		r.PackageMgr = "npm"
		r.InstallCmd = "npm install"
	}
}

type pnpmWorkspace struct {
	Packages []string `yaml:"packages"`
}

func (r *Result) detectWorkspaces() {
	// pnpm workspaces
	if b, err := os.ReadFile(filepath.Join(r.Root, "pnpm-workspace.yaml")); err == nil {
		var w pnpmWorkspace
		if yaml.Unmarshal(b, &w) == nil {
			r.Workspaces = expandGlobs(r.Root, w.Packages)
			return
		}
	}
	// npm/yarn/bun workspaces
	if pkg := readPackageJSON(filepath.Join(r.Root, "package.json")); pkg != nil {
		if len(pkg.Workspaces) > 0 {
			r.Workspaces = expandGlobs(r.Root, pkg.Workspaces)
			return
		}
	}
	// Single-package repository.
	if exists(r.Root, "package.json") {
		r.Workspaces = []string{"."}
	}
}

type packageJSON struct {
	Name         string            `json:"name"`
	Scripts      map[string]string `json:"scripts"`
	Dependencies map[string]string `json:"dependencies"`
	DevDeps      map[string]string `json:"devDependencies"`
	Workspaces   []string          `json:"workspaces"`
}

var (
	flagPortRe = regexp.MustCompile(`(?:^|\s)(--port|-p)[= ]\s*\$?\{?([A-Z_]+)?:?-?(\d+)?`)
	// Matches both `process.env.X_PORT` and Vite's `env.X_PORT` after loadEnv.
	envPortRe = regexp.MustCompile(`\benv\.([A-Z][A-Z0-9_]*PORT)\b`)
	// Anchored on the `port:` key so a year or an unrelated constant elsewhere
	// in the config is not mistaken for the dev server port.
	vitePortRe = regexp.MustCompile(`\bport\s*:[^\n,]*?(\d{4,5})`)
)

func (r *Result) detectServices() {
	for _, ws := range r.Workspaces {
		dir := filepath.Join(r.Root, ws)
		pkg := readPackageJSON(filepath.Join(dir, "package.json"))
		if pkg == nil {
			continue
		}
		dev, ok := pkg.Scripts["dev"]
		if !ok || strings.TrimSpace(dev) == "" {
			continue // not a runnable service — a library workspace
		}

		s := Service{
			Name:      sanitise(filepath.Base(ws)),
			Dir:       ws,
			Framework: framework(pkg, dev),
			PortVia:   "env",
			PortKey:   "PORT",
		}
		s.Kind = "frontend"
		if isBackend(pkg, dev, s.Name) {
			s.Kind = "backend"
		}
		s.Cmd = runCmd(r.PackageMgr, ws)
		r.detectPortBinding(&s, dir, dev)
		r.Services = append(r.Services, s)
	}
}

// detectPortBinding works out how this service is told which port to use.
//
// Getting this wrong is silent and expensive: Next.js `-p` beats the PORT
// environment variable, so a project configured to set PORT alone will bind
// whatever is hardcoded in its dev script, in every worktree, forever.
func (r *Result) detectPortBinding(s *Service, dir, dev string) {
	// A port flag already in the dev script is authoritative.
	if m := flagPortRe.FindStringSubmatch(dev); m != nil {
		if m[2] != "" {
			// e.g. `next dev -p ${PORT:-3300}` — driven by an env var already.
			s.PortVia, s.PortKey = "env", m[2]
		} else {
			s.PortVia, s.PortKey = "flag", m[1]
		}
		if m[3] != "" {
			s.CurrentPort = atoi(m[3])
		}
		return
	}

	// Vite reads its port from the config file, often via a custom env var.
	for _, name := range []string{"vite.config.ts", "vite.config.js", "vite.config.mts"} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		if m := envPortRe.FindStringSubmatch(string(b)); m != nil {
			s.PortVia, s.PortKey = "env", m[1]
		}
		if m := vitePortRe.FindStringSubmatch(string(b)); m != nil {
			s.CurrentPort = atoi(m[1])
		}
		return
	}

	if s.Framework == "next" {
		// Next honours PORT when no -p is present.
		s.PortVia, s.PortKey = "env", "PORT"
	}
}

func (r *Result) detectPrisma() {
	names := []string{"prisma/schema", "prisma/schema.prisma", "prisma"}
	for _, p := range names {
		if exists(r.Root, p) {
			r.HasPrisma = true
			r.PrismaDir = p
			return
		}
	}
	// A polyglot repository keeps Prisma inside whichever sub-project owns
	// persistence, so record the owner too — migrations must run from there.
	for _, step := range r.DepSteps {
		if step.Dir == "." {
			continue
		}
		for _, p := range names {
			if exists(r.Root, filepath.Join(step.Dir, p)) {
				r.HasPrisma = true
				r.PrismaOwner = step.Dir
				r.PrismaDir = p
				return
			}
		}
	}
}

type composeFile struct {
	Services map[string]struct {
		Image         string            `yaml:"image"`
		ContainerName string            `yaml:"container_name"`
		Environment   map[string]string `yaml:"environment"`
		Ports         []string          `yaml:"ports"`
	} `yaml:"services"`
}

func (r *Result) detectDB() {
	candidates := []string{
		"docker-compose.yml", "docker-compose.yaml",
		"prisma/docker-compose.yml", "compose.yml", "compose.yaml",
	}
	// In a polyglot repository the database belongs to whichever sub-project
	// owns persistence, not to the root, so search those too.
	for _, step := range r.DepSteps {
		if step.Dir == "." {
			continue
		}
		for _, n := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml"} {
			candidates = append(candidates, filepath.Join(step.Dir, n))
		}
	}

	for _, rel := range candidates {
		b, err := os.ReadFile(filepath.Join(r.Root, rel))
		if err != nil {
			continue
		}
		var cf composeFile
		if yaml.Unmarshal(b, &cf) != nil {
			continue
		}
		for _, svc := range cf.Services {
			if !strings.Contains(svc.Image, "postgres") {
				continue
			}
			// Values are read only from the compose file, which is committed,
			// and shell substitutions resolve to their inline default. The
			// project's .env is deliberately not consulted for credentials:
			// .grove.json is meant to be committed, so it must never pick up a
			// real secret that the repository keeps out of git.
			db := &DB{
				Driver:    "docker-postgres",
				Image:     svc.Image,
				Compose:   rel,
				Container: svc.ContainerName,
				User:      shellDefault(svc.Environment["POSTGRES_USER"]),
				Password:  shellDefault(svc.Environment["POSTGRES_PASSWORD"]),
				Name:      shellDefault(svc.Environment["POSTGRES_DB"]),
			}
			for _, p := range svc.Ports {
				db.HostPort = hostPort(p)
			}
			// POSTGRES_DB is often just the bootstrap database. When it is,
			// prefer another *_DB_NAME the compose file declares.
			if db.Name == "" || db.Name == "postgres" {
				for k, v := range svc.Environment {
					if k == "POSTGRES_DB" || !strings.HasSuffix(k, "DB_NAME") {
						continue
					}
					if n := shellDefault(v); n != "" && n != "postgres" {
						db.Name = n
						break
					}
				}
			}
			db.Profiles = r.detectDBProfiles()
			r.DB = db
			return
		}
	}
}

// skipScript filters out database scripts that tear down, switch, or test
// rather than produce a populated database.
var skipScript = regexp.MustCompile(`stop|clean|reset|use|reuse|test|unchanged|no-dump|safe|stress`)

// detectDBProfiles guesses how this project populates a database, by reading
// the scripts it already has. Three profiles are recognised — prod, seed and
// dev — and the shortest matching script name wins, so `db:setup:prod` is
// preferred over `db:setup:prod:safe`.
func (r *Result) detectDBProfiles() map[string]string {
	out := map[string]string{}
	// A polyglot repository has no workspace list; its installable directories
	// are the dependency steps instead.
	dirs := r.Workspaces
	if len(dirs) == 0 {
		for _, s := range r.DepSteps {
			dirs = append(dirs, s.Dir)
		}
	}
	for _, ws := range dirs {
		pkg := readPackageJSON(filepath.Join(r.Root, ws, "package.json"))
		if pkg == nil {
			continue
		}
		names := make([]string, 0, len(pkg.Scripts))
		for n := range pkg.Scripts {
			names = append(names, n)
		}
		// A profile command has to produce a reusable dump, not just mutate a
		// live database. Scripts that mention setup/baseline/dump do that;
		// a bare `seed:demo` runs a seeder against whatever is connected. Rank
		// the dump-producing ones first, then prefer the shortest name so
		// `db:setup:prod` wins over `db:setup:prod:safe`.
		sort.Slice(names, func(i, j int) bool {
			ri, rj := dumpRank(names[i]), dumpRank(names[j])
			if ri != rj {
				return ri < rj
			}
			if len(names[i]) != len(names[j]) {
				return len(names[i]) < len(names[j])
			}
			return names[i] < names[j]
		})

		for _, n := range names {
			if !strings.HasPrefix(n, "db:") && !strings.HasPrefix(n, "seed") {
				continue
			}
			if skipScript.MatchString(n) {
				continue
			}
			var profile string
			switch {
			case strings.Contains(n, "prod"):
				profile = "prod"
			case strings.Contains(n, "seed") || strings.Contains(n, "baseline"):
				profile = "seed"
			case strings.Contains(n, "setup") || strings.Contains(n, "dump"):
				profile = "dev"
			default:
				continue
			}
			if _, taken := out[profile]; !taken {
				out[profile] = r.profileCmd(ws, n)
			}
		}
	}
	return out
}

// profileCmd builds the command that runs a database script.
//
// A workspace member is addressed through the root package manager's filter; a
// standalone sub-project has its own manager and no workspace to filter on, so
// it has to be entered directly.
func (r *Result) profileCmd(dir, script string) string {
	if len(r.Workspaces) > 0 {
		return scriptCmd(r.PackageMgr, dir, script)
	}
	mgr := "npm"
	for _, s := range r.DepSteps {
		if s.Dir == dir && s.Manager != "" {
			mgr = s.Manager
		}
	}
	return "cd " + dir + " && " + mgr + " run " + script
}

// dumpRank orders candidate profile scripts, lowest first.
func dumpRank(name string) int {
	switch {
	case strings.Contains(name, "dump"), strings.Contains(name, "baseline"):
		return 0
	case strings.Contains(name, "setup"):
		return 1
	default:
		return 2
	}
}

// scriptCmd builds the command that runs a package script in a workspace.
func scriptCmd(mgr, ws, script string) string {
	if ws == "." {
		switch mgr {
		case "yarn":
			return "yarn " + script
		case "bun":
			return "bun run " + script
		default:
			return mgr + " run " + script
		}
	}
	switch mgr {
	case "pnpm":
		return "pnpm --filter ./" + ws + " run " + script
	case "yarn":
		return "yarn workspace " + filepath.Base(ws) + " " + script
	case "bun":
		return "bun run --cwd " + ws + " " + script
	default:
		return "npm run " + script + " --workspace " + ws
	}
}

// shellDefault resolves `${VAR:-fallback}` to its fallback and leaves anything
// else untouched. Compose files routinely write ports and credentials that way.
var shellSubstRe = regexp.MustCompile(`^\$\{[A-Za-z_][A-Za-z0-9_]*:-([^}]*)\}$`)

func shellDefault(s string) string {
	s = strings.TrimSpace(s)
	if m := shellSubstRe.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return s
}

// hostPort extracts the host side of a compose port mapping.
//
// Splitting on the first colon is wrong: `${DB_PORT:-25446}:5432` contains a
// colon inside the shell substitution, so the container port has to be taken
// off the end instead. An optional bind address may also precede the host port.
func hostPort(spec string) int {
	s := strings.Trim(strings.TrimSpace(spec), `"'`)
	if i := strings.LastIndex(s, ":"); i >= 0 {
		s = s[:i]
	}
	if n := atoi(shellDefault(s)); n > 0 {
		return n
	}
	// Still not a number — an "ip:port" form. Take the trailing segment.
	if i := strings.LastIndex(s, ":"); i >= 0 {
		return atoi(shellDefault(s[i+1:]))
	}
	return 0
}

func (r *Result) detectEnv() {
	r.EnvValues = map[string]string{}
	r.ServiceEnv = map[string]string{}

	// The root env file, if there is one.
	for _, name := range []string{".env", ".env.local", ".env.development"} {
		if r.readEnvFile(name, "") {
			r.EnvFile = name
			break
		}
	}

	// Polyglot repositories keep an env file per sub-project instead. Record
	// which service owns which file, because each has to be rewritten
	// separately — there is no shared root file to put the ports in.
	//
	// Symlinks have to be resolved first. A monorepo commonly links every
	// workspace's .env back to one root file, and treating those as distinct
	// would have each service read the same values and then write conflicting
	// ones into a single file.
	claimed := map[string]bool{}
	if r.EnvFile != "" {
		claimed[realPath(filepath.Join(r.Root, r.EnvFile))] = true
	}
	for i := range r.Services {
		s := &r.Services[i]
		if s.Dir == "" || s.Dir == "." {
			continue
		}
		for _, name := range []string{".env", ".env.local", ".env.development"} {
			rel := filepath.Join(s.Dir, name)
			real := realPath(filepath.Join(r.Root, rel))
			if _, err := os.Stat(filepath.Join(r.Root, rel)); err != nil {
				continue
			}
			if claimed[real] {
				break // shares a file already accounted for — not its own
			}
			if r.readEnvFile(rel, s.Name) {
				claimed[real] = true
				r.ServiceEnv[s.Name] = rel
			}
			break
		}
	}
}

// realPath resolves symlinks, falling back to the input when it cannot.
func realPath(p string) string {
	if rp, err := filepath.EvalSymlinks(p); err == nil {
		return rp
	}
	return p
}

// readEnvFile records a file's keys and values, returning whether it existed.
// Values are scoped per service so two sub-projects can both define PORT.
func (r *Result) readEnvFile(rel, service string) bool {
	b, err := os.ReadFile(filepath.Join(r.Root, rel))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if service == "" {
			r.EnvKeys = append(r.EnvKeys, k)
			r.EnvValues[k] = v
		} else {
			r.EnvValues[service+"\x00"+k] = v
		}
	}
	return true
}

// envValueFor returns a key's value as seen by one service: its own env file
// first, then the root one.
func (r *Result) envValueFor(service, key string) (string, bool) {
	if v, ok := r.EnvValues[service+"\x00"+key]; ok {
		return v, true
	}
	v, ok := r.EnvValues[key]
	return v, ok
}

// portsFromEnv fills in ports for services whose port comes from the
// environment rather than their dev script — the usual shape for a backend.
func (r *Result) portsFromEnv() {
	for i := range r.Services {
		s := &r.Services[i]
		if s.CurrentPort > 0 || s.PortVia != "env" {
			continue
		}
		// A service with its own env file owns every key in it, so there is no
		// ambiguity to resolve.
		if own, hasOwn := r.ServiceEnv[s.Name]; hasOwn && own != "" {
			if v, ok := r.envValueFor(s.Name, s.PortKey); ok {
				s.CurrentPort = atoi(v)
			}
			continue
		}

		// Otherwise only trust a shared env file when exactly one service reads
		// that key. Several frontends commonly share the generic key PORT, and
		// the value in .env belongs to whichever service it was written for.
		//
		// The exception is the sole backend: a shared .env in a repository with
		// one server is written for that server, so its PORT is the backend's
		// even when frontends nominally read the same key.
		if !r.KeyIsUnique(s.PortKey) && !(s.Kind == "backend" && r.countBackends() == 1) {
			continue
		}
		if v, ok := r.EnvValues[s.PortKey]; ok {
			s.CurrentPort = atoi(v)
		}
	}
}

func (r *Result) countBackends() int {
	n := 0
	for _, s := range r.Services {
		if s.Kind == "backend" {
			n++
		}
	}
	return n
}

// KeyIsUnique reports whether exactly one service reads this env key.
func (r *Result) KeyIsUnique(key string) bool {
	n := 0
	for _, s := range r.Services {
		if s.PortVia == "env" && s.PortKey == key {
			n++
		}
	}
	return n == 1
}

// EnvKeysMatching returns detected env keys containing any of the substrings.
// Used to surface the port/URL keys that must be rewritten per worktree.
func (r *Result) EnvKeysMatching(subs ...string) []string {
	var out []string
	for _, k := range r.EnvKeys {
		for _, s := range subs {
			if strings.Contains(k, s) {
				out = append(out, k)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

func framework(pkg *packageJSON, dev string) string {
	all := map[string]string{}
	for k, v := range pkg.Dependencies {
		all[k] = v
	}
	for k, v := range pkg.DevDeps {
		all[k] = v
	}
	switch {
	case strings.Contains(dev, "next"), all["next"] != "":
		return "next"
	case strings.Contains(dev, "vite"), all["vite"] != "":
		return "vite"
	case all["@nestjs/core"] != "":
		return "nest"
	case all["express"] != "", all["fastify"] != "", all["hono"] != "":
		return "express"
	}
	return "unknown"
}

func isBackend(pkg *packageJSON, dev, name string) bool {
	switch framework(pkg, dev) {
	case "nest", "express":
		return true
	}
	return name == "backend" || name == "api" || name == "server"
}

func runCmd(mgr, ws string) string {
	if ws == "." {
		switch mgr {
		case "bun":
			return "bun run dev"
		case "yarn":
			return "yarn dev"
		default:
			return mgr + " run dev"
		}
	}
	switch mgr {
	case "pnpm":
		return "pnpm --filter ./" + ws + " run dev"
	case "yarn":
		return "yarn workspace " + filepath.Base(ws) + " dev"
	case "bun":
		return "bun run --cwd " + ws + " dev"
	default:
		return "npm run dev --workspace " + ws
	}
}

func expandGlobs(root string, globs []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, g := range globs {
		if strings.HasPrefix(g, "!") {
			continue
		}
		matches, err := filepath.Glob(filepath.Join(root, g))
		if err != nil {
			continue
		}
		for _, m := range matches {
			rel, err := filepath.Rel(root, m)
			if err != nil || rel == "" {
				continue
			}
			fi, err := os.Stat(m)
			if err != nil || !fi.IsDir() {
				continue
			}
			if !seen[rel] {
				seen[rel] = true
				out = append(out, rel)
			}
		}
	}
	sort.Strings(out)
	return out
}

func readPackageJSON(p string) *packageJSON {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var pkg packageJSON
	if json.Unmarshal(b, &pkg) != nil {
		return nil
	}
	return &pkg
}

func exists(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, rel))
	return err == nil
}

func sanitise(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == ' ' || r == '.':
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}
