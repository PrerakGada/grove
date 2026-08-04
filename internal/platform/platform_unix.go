//go:build !windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Listeners returns the pids listening on a TCP port.
func Listeners(port int) []int {
	out, err := exec.Command("lsof", "-ti", "tcp:"+strconv.Itoa(port), "-sTCP:LISTEN").Output()
	if err != nil {
		return nil // lsof exits non-zero when nothing matches
	}
	return parsePIDs(string(out))
}

// Children returns the direct child pids of pid.
func Children(pid int) []int {
	out, err := exec.Command("pgrep", "-P", strconv.Itoa(pid)).Output()
	if err != nil {
		return nil
	}
	return parsePIDs(string(out))
}

// KillTree signals a process and all its descendants, deepest first.
func KillTree(pid int, sig Signal) {
	if pid <= 0 {
		return
	}
	for _, kid := range Children(pid) {
		KillTree(kid, sig)
	}
	Kill(pid, sig)
}

// Kill signals a single process, ignoring "already gone".
func Kill(pid int, sig Signal) {
	s := syscall.SIGTERM
	if sig == SigKill {
		s = syscall.SIGKILL
	}
	_ = syscall.Kill(pid, s)
}

// ProcsMatching returns pids whose command line matches any of the patterns.
func ProcsMatching(patterns []string) []int {
	if len(patterns) == 0 {
		return nil
	}
	out, err := exec.Command("pgrep", "-f", strings.Join(patterns, "|")).Output()
	if err != nil {
		return nil
	}
	return parsePIDs(string(out))
}

// ProcsUnder returns pids from the candidate set whose working directory is
// inside dir. This is the sweep that pid files and port scans both miss.
func ProcsUnder(dir string, candidates []int) []int {
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		real = dir
	}
	var out []int
	for _, pid := range candidates {
		cwd, err := ProcCwd(pid)
		if err != nil || cwd == "" {
			continue // processes we cannot inspect are simply skipped
		}
		if cwd == real || strings.HasPrefix(cwd, real+string(os.PathSeparator)) {
			out = append(out, pid)
		}
	}
	return out
}

// SpawnDetached starts cmd in dir with stdout+stderr redirected to logPath,
// fully detached so it survives the parent exiting. Returns the child pid.
func SpawnDetached(dir, cmdline, logPath string, env []string) (int, error) {
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return 0, err
	}
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, err
	}
	defer log.Close()

	c := exec.Command("/bin/sh", "-c", cmdline)
	c.Dir = dir
	c.Stdout = log
	c.Stderr = log
	c.Stdin = nil
	c.Env = env
	// Setsid detaches from the controlling terminal and starts a new process
	// group, so closing the launching terminal does not signal the child and
	// the whole group stays killable as a unit.
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := c.Start(); err != nil {
		return 0, err
	}
	pid := c.Process.Pid
	// Reap the direct child asynchronously; it reparents to init/launchd.
	go func() { _ = c.Wait() }()
	return pid, nil
}

// Alive reports whether a pid exists.
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}

// SameVolume reports whether two paths live on the same filesystem, walking up
// to the nearest existing ancestor for paths not yet created.
func SameVolume(a, b string) (bool, error) {
	da, err := deviceOf(a)
	if err != nil {
		return false, err
	}
	db, err := deviceOf(b)
	if err != nil {
		return false, err
	}
	return da == db, nil
}

func deviceOf(p string) (uint64, error) {
	probe := p
	for {
		if _, err := os.Stat(probe); err == nil {
			break
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return 0, fmt.Errorf("no existing ancestor of %s", p)
		}
		probe = parent
	}
	fi, err := os.Stat(probe)
	if err != nil {
		return 0, err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("cannot read device id of %s", probe)
	}
	return uint64(st.Dev), nil
}

func parsePIDs(s string) []int {
	var out []int
	for _, line := range strings.Fields(s) {
		if n, err := strconv.Atoi(line); err == nil && n > 0 {
			out = append(out, n)
		}
	}
	return out
}
