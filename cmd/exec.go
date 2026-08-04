package cmd

import (
	"os/exec"
	"strings"
)

// runCapture runs a command and returns its trimmed stdout.
func runCapture(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
