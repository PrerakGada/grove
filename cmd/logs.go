package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/PrerakGada/grove/internal/runner"
	"github.com/spf13/cobra"
)

var (
	logsFollow bool
	logsLines  int
)

var logsCmd = &cobra.Command{
	Use:   "logs <name> <service>",
	Short: "Show a service's captured output",
	Args:  cobra.ExactArgs(2),
	RunE:  runLogs,
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "follow the log")
	logsCmd.Flags().IntVarP(&logsLines, "lines", "n", 200, "lines to show")
	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
	cfg, root, err := loadConfig()
	if err != nil {
		return err
	}
	name, svc := args[0], args[1]
	if _, ok := cfg.Service(svc); !ok {
		return fmt.Errorf("unknown service %q", svc)
	}
	wtPath, _, err := requireWorktree(cfg, root, name)
	if err != nil {
		return err
	}

	log := runner.LogFile(wtPath, svc)
	if _, err := os.Stat(log); err != nil {
		return fmt.Errorf("no log for %s in %q yet — has it been started? (grove up %s %s)",
			svc, name, name, svc)
	}

	tailArgs := []string{"-n", strconv.Itoa(logsLines)}
	if logsFollow {
		tailArgs = append(tailArgs, "-f")
	}
	tailArgs = append(tailArgs, log)

	c := exec.Command("tail", tailArgs...)
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	return c.Run()
}
