// Package runner starts and stops a worktree's services in the background.
//
// Detached processes are the whole point: one terminal per worktree does not
// scale past two or three, and the reason to isolate worktrees at all is to
// keep many alive at once.
package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/PrerakGada/grove/internal/config"
	"github.com/PrerakGada/grove/internal/platform"
)

// procPatterns match the process names dev servers run under. Used by the
// working-directory sweep, which is the only pass that catches a server whose
// port the slot no longer owns.
var procPatterns = []string{"node", "vite", "pnpm", "npm", "yarn", "bun", "tsx", "next", "dart", "flutter"}

// RunDir is where pid and log files live inside a worktree.
func RunDir(wtPath string) string { return filepath.Join(wtPath, config.RunDir) }

func pidFile(wtPath, svc string) string { return filepath.Join(RunDir(wtPath), svc+".pid") }

// LogFile is a service's captured output.
func LogFile(wtPath, svc string) string { return filepath.Join(RunDir(wtPath), svc+".log") }

// ProductsFile remembers which services were last asked for, so a bare
// `grove up <name>` keeps doing what was asked the first time.
func ProductsFile(wtPath string) string { return filepath.Join(RunDir(wtPath), "services") }

// Port resolves a service's port for a slot.
func Port(s config.Service, slot int) int { return s.Band + slot }

// Command builds the shell command for a service, injecting its port the way
// that service actually accepts it.
//
// The distinction matters and fails silently when wrong: a CLI port flag
// overrides the environment variable, so a project told only through the
// environment binds whatever its script hardcodes — in every worktree.
func Command(s config.Service, port int) string {
	switch s.Port.Via {
	case "flag":
		return fmt.Sprintf("%s %s %d", s.Cmd, s.Port.Key, port)
	default:
		return fmt.Sprintf("env %s=%d %s", s.Port.Key, port, s.Cmd)
	}
}

// Start launches a service unless its port is already served. Returns whether
// it was started (false means it was already up).
func Start(wtPath string, s config.Service, port int) (bool, error) {
	if len(platform.Listeners(port)) > 0 {
		return false, nil
	}
	if err := os.MkdirAll(RunDir(wtPath), 0o755); err != nil {
		return false, err
	}
	pid, err := platform.SpawnDetached(wtPath, Command(s, port), LogFile(wtPath, s.Name), os.Environ())
	if err != nil {
		return false, err
	}
	return true, os.WriteFile(pidFile(wtPath, s.Name), []byte(strconv.Itoa(pid)), 0o644)
}

// WaitListening waits for a service to bind, giving up after timeout. It
// reports early if the process died, so a crash is not reported as "slow".
func WaitListening(wtPath, svc string, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if len(platform.Listeners(port)) > 0 {
			return nil
		}
		if pid := readPID(wtPath, svc); pid > 0 && !platform.Alive(pid) {
			return fmt.Errorf("exited early — see %s", LogFile(wtPath, svc))
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("not listening on %d after %s — still booting? tail %s",
		port, timeout, LogFile(wtPath, svc))
}

// Stop shuts a worktree's services down in three passes.
//
// All three are needed, and the third was learned the hard way:
//  1. the recorded process trees;
//  2. anything holding the slot's current ports, because dev servers fork
//     children that outlive the parent named in the pid file;
//  3. anything whose working directory is inside the worktree, because passes
//     1 and 2 only cover the ports the slot owns *now* — a server started
//     before a port change, by hand, or by a raw package script, survives both.
func Stop(cfg *config.Config, wtPath string, slot int) {
	// 1. recorded process trees
	entries, _ := os.ReadDir(RunDir(wtPath))
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".pid") {
			continue
		}
		svc := strings.TrimSuffix(e.Name(), ".pid")
		if pid := readPID(wtPath, svc); pid > 0 {
			platform.KillTree(pid, platform.SigTerm)
		}
		_ = os.Remove(pidFile(wtPath, svc))
	}

	// 2. the slot's ports
	ports := []int{}
	for _, s := range cfg.Services {
		ports = append(ports, Port(s, slot))
	}
	for _, sig := range []platform.Signal{platform.SigTerm, platform.SigKill} {
		hit := false
		for _, p := range ports {
			for _, pid := range platform.Listeners(p) {
				platform.Kill(pid, sig)
				hit = true
			}
		}
		if hit && sig == platform.SigTerm {
			time.Sleep(2 * time.Second)
		}
	}

	// 3. the working-directory sweep
	for _, sig := range []platform.Signal{platform.SigTerm, platform.SigKill} {
		victims := platform.ProcsUnder(wtPath, platform.ProcsMatching(procPatterns))
		for _, pid := range victims {
			platform.Kill(pid, sig)
		}
		if len(victims) > 0 && sig == platform.SigTerm {
			time.Sleep(2 * time.Second)
		}
	}
}

// Status is a service's live state.
type Status struct {
	Service string
	Port    int
	Up      bool
	PID     int
}

// Statuses reports every configured service for a worktree.
func Statuses(cfg *config.Config, wtPath string, slot int) []Status {
	var out []Status
	for _, s := range cfg.Services {
		p := Port(s, slot)
		pids := platform.Listeners(p)
		st := Status{Service: s.Name, Port: p, Up: len(pids) > 0}
		if st.Up {
			st.PID = pids[0]
		}
		out = append(out, st)
	}
	return out
}

// RememberServices records the requested service names, or returns the last
// set when none are given.
func RememberServices(wtPath string, names []string) []string {
	f := ProductsFile(wtPath)
	if len(names) > 0 {
		_ = os.MkdirAll(RunDir(wtPath), 0o755)
		_ = os.WriteFile(f, []byte(strings.Join(names, " ")), 0o644)
		return names
	}
	if b, err := os.ReadFile(f); err == nil {
		if s := strings.Fields(string(b)); len(s) > 0 {
			return s
		}
	}
	return nil
}

func readPID(wtPath, svc string) int {
	b, err := os.ReadFile(pidFile(wtPath, svc))
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	return n
}
