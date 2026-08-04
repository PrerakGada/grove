//go:build darwin

package platform

import (
	"os/exec"
	"strconv"
	"strings"
)

// ProcCwd returns a process's working directory.
//
// macOS has no /proc, so this shells out to lsof. Processes owned by other
// users, and short-lived ones, simply fail here — callers treat that as "not
// under this directory" rather than an error.
func ProcCwd(pid int) (string, error) {
	out, err := exec.Command("lsof", "-a", "-p", strconv.Itoa(pid), "-d", "cwd", "-Fn").Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "n") {
			return strings.TrimPrefix(line, "n"), nil
		}
	}
	return "", nil
}

// CloneDir copies a directory using APFS clonefile, so the copy shares blocks
// with the original and costs approximately nothing until one side is written.
func CloneDir(src, dst string) error {
	if err := exec.Command("cp", "-Rc", src, dst).Run(); err == nil {
		return nil
	}
	return exec.Command("cp", "-R", src, dst).Run()
}
