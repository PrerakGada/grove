//go:build linux

package platform

import (
	"os"
	"os/exec"
	"strconv"
)

// ProcCwd returns a process's working directory. Linux exposes this directly,
// which makes the working-directory sweep both cheaper and more reliable here
// than on macOS.
func ProcCwd(pid int) (string, error) {
	return os.Readlink("/proc/" + strconv.Itoa(pid) + "/cwd")
}

// CloneDir copies a directory using filesystem reflinks where available
// (btrfs, XFS), falling back to a real copy elsewhere.
func CloneDir(src, dst string) error {
	if err := exec.Command("cp", "-R", "--reflink=auto", src, dst).Run(); err == nil {
		return nil
	}
	return exec.Command("cp", "-R", src, dst).Run()
}
