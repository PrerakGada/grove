package detect

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// This file handles repositories that are not a single JavaScript workspace:
// a Flutter app beside a Node server beside a codegen package, each with its
// own toolchain, usually tied together by a task runner rather than by one
// package manager.
//
// The root-workspace path in detect.go stays the fast path. Polyglot scanning
// only runs when that finds nothing, so a normal monorepo is unaffected.

// skipDirs never contain a service worth starting.
var skipDirs = map[string]bool{
	"node_modules": true, "build": true, "dist": true, ".git": true,
	"docs": true, "scripts": true, "deploy": true, "assets": true,
	".dart_tool": true, "vendor": true, "target": true, "worktrees": true,
	"outputs": true, "coverage": true, ".next": true, ".turbo": true,
}

// detectSubProjects finds independently-installable projects one level down.
func (r *Result) detectSubProjects() {
	entries, err := os.ReadDir(r.Root)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") || skipDirs[e.Name()] {
			continue
		}
		dir := filepath.Join(r.Root, e.Name())

		switch {
		case fileExists(filepath.Join(dir, "pubspec.yaml")):
			r.addFlutterService(e.Name(), dir)
		case fileExists(filepath.Join(dir, "package.json")):
			r.addNodeSubProject(e.Name(), dir)
		}
	}
}

func (r *Result) addNodeSubProject(name, dir string) {
	pkg := readPackageJSON(filepath.Join(dir, "package.json"))
	if pkg == nil {
		return
	}
	mgr, install := managerFor(dir)
	if mgr != "" {
		r.DepSteps = append(r.DepSteps, DepStep{Dir: name, Manager: mgr, Install: install})
	}

	dev, ok := pkg.Scripts["dev"]
	if !ok || strings.TrimSpace(dev) == "" {
		return // installable, but nothing to run — a codegen or contracts package
	}
	s := Service{
		Name:      sanitise(name),
		Dir:       name,
		Framework: framework(pkg, dev),
		Kind:      "frontend",
		PortVia:   "env",
		PortKey:   "PORT",
		Cmd:       subProjectCmd(mgr, name),
	}
	if isBackend(pkg, dev, s.Name) {
		s.Kind = "backend"
	}
	r.detectPortBinding(&s, dir, dev)
	r.Services = append(r.Services, s)
}

// addFlutterService registers a Flutter project that can actually be run. A
// pubspec alone is not enough — a shared Dart package has one too — so this
// requires a platform directory or a lib/main.dart entrypoint.
func (r *Result) addFlutterService(name, dir string) {
	r.DepSteps = append(r.DepSteps, DepStep{
		Dir: name, Manager: "flutter", Install: "flutter pub get",
	})

	runnable := fileExists(filepath.Join(dir, "lib", "main.dart"))
	for _, p := range []string{"web", "android", "ios", "macos", "windows", "linux"} {
		if fileExists(filepath.Join(dir, p)) {
			runnable = true
		}
	}
	if !runnable {
		return
	}

	s := Service{
		Name:      sanitise(name),
		Dir:       name,
		Kind:      "frontend",
		Framework: "flutter",
		// Flutter's web dev server takes its port as a flag. Detection below
		// may replace this if the project wraps it in a run script.
		PortVia: "flag",
		PortKey: "--web-port",
		Cmd:     "flutter run -d web-server --web-hostname localhost",
	}
	r.detectRunScript(&s, dir)
	r.Services = append(r.Services, s)
}

// shellDefaultRe matches `PORT="${SOME_PORT:-25445}"` — the usual way a run
// script exposes an overridable port.
var shellDefaultRe = regexp.MustCompile(`\$\{([A-Z][A-Z0-9_]*)(?:_PORT|PORT)?:-(\d{2,5})\}`)

// webPortFlagRe matches an explicit --web-port in a run script.
var webPortFlagRe = regexp.MustCompile(`--web-port[= ]+"?\$?\{?([A-Z_]+)?\}?"?\s*(\d{2,5})?`)

// detectRunScript reads a project's run.sh, which is where a Flutter project
// usually hides its real port handling.
func (r *Result) detectRunScript(s *Service, dir string) {
	for _, name := range []string{"run.sh", "dev.sh", "start.sh"} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		body := string(b)
		s.RunScript = filepath.Join(s.Dir, name)
		s.Cmd = "./" + s.RunScript

		if m := shellDefaultRe.FindStringSubmatch(body); m != nil {
			s.PortVia, s.PortKey = "env", m[1]
			s.CurrentPort = atoi(m[2])
			return
		}
		if m := webPortFlagRe.FindStringSubmatch(body); m != nil && m[2] != "" {
			s.CurrentPort = atoi(m[2])
		}
		return
	}
}

// managerFor identifies the package manager governing a directory.
func managerFor(dir string) (manager, install string) {
	switch {
	case fileExists(filepath.Join(dir, "pnpm-lock.yaml")),
		fileExists(filepath.Join(dir, "pnpm-workspace.yaml")):
		return "pnpm", "pnpm install --config.confirmModulesPurge=false"
	case fileExists(filepath.Join(dir, "bun.lockb")), fileExists(filepath.Join(dir, "bun.lock")):
		return "bun", "bun install"
	case fileExists(filepath.Join(dir, "yarn.lock")):
		return "yarn", "yarn install"
	case fileExists(filepath.Join(dir, "package-lock.json")):
		return "npm", "npm install"
	case fileExists(filepath.Join(dir, "package.json")):
		return "npm", "npm install"
	}
	return "", ""
}

func subProjectCmd(mgr, dir string) string {
	switch mgr {
	case "yarn":
		return "cd " + dir + " && yarn dev"
	case "bun":
		return "cd " + dir + " && bun run dev"
	case "":
		return "cd " + dir + " && npm run dev"
	default:
		return "cd " + dir + " && " + mgr + " run dev"
	}
}

// ---------------------------------------------------------------------------
// justfile
// ---------------------------------------------------------------------------

var (
	recipeRe  = regexp.MustCompile(`^([a-z][a-z0-9_-]*):\s*$`)
	cdDirRe   = regexp.MustCompile(`\bcd\s+([a-zA-Z0-9_.-]+)`)
	pathDirRe = regexp.MustCompile(`\./([a-zA-Z0-9_-]+)/`)
)

// Recipe is one task-runner target.
type Recipe struct {
	Name string
	Body string
	// Dir is the sub-project the recipe operates on, if one is evident.
	Dir string
}

// detectJustfile parses recipe names and bodies.
//
// A task runner is a strong signal in a polyglot repository: it is where the
// project already records how each piece is started, which beats guessing from
// a manifest.
func (r *Result) detectJustfile() {
	b, err := os.ReadFile(filepath.Join(r.Root, "justfile"))
	if err != nil {
		b, err = os.ReadFile(filepath.Join(r.Root, "Justfile"))
		if err != nil {
			return
		}
	}
	r.TaskRunner = "just"

	var cur *Recipe
	flush := func() {
		if cur == nil {
			return
		}
		cur.Body = strings.TrimSpace(cur.Body)
		if cur.Body != "" {
			if m := cdDirRe.FindStringSubmatch(cur.Body); m != nil {
				cur.Dir = m[1]
			} else if m := pathDirRe.FindStringSubmatch(cur.Body); m != nil {
				cur.Dir = m[1]
			}
			r.Recipes = append(r.Recipes, *cur)
		}
		cur = nil
	}

	for _, line := range strings.Split(string(b), "\n") {
		if m := recipeRe.FindStringSubmatch(line); m != nil {
			flush()
			cur = &Recipe{Name: m[1]}
			continue
		}
		if cur == nil {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			cur.Body += strings.TrimSpace(line) + "\n"
			continue
		}
		flush()
	}
	flush()
}

// applyRecipes replaces detected commands with the project's own task-runner
// targets where one clearly covers the same directory. `just api` is both more
// correct and more legible than a reconstructed `cd backend && pnpm dev`.
func (r *Result) applyRecipes() {
	if r.TaskRunner == "" {
		return
	}
	for i := range r.Services {
		s := &r.Services[i]
		for _, rec := range r.Recipes {
			if rec.Dir == "" || rec.Dir != s.Dir {
				continue
			}
			// Only adopt recipes that look like "run this thing", not helpers
			// such as api-stop, db-up, api-migrate.
			if strings.ContainsAny(rec.Name, "-") {
				continue
			}
			s.Cmd = r.TaskRunner + " " + rec.Name
			break
		}
	}
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
