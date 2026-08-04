package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/PrerakGada/grove/internal/config"
	"github.com/PrerakGada/grove/internal/dbx"
	"github.com/PrerakGada/grove/internal/provision"
	"github.com/PrerakGada/grove/internal/ui"
	"github.com/spf13/cobra"
)

var (
	dbEmpty  bool
	dbReseed bool
)

var dbCmd = &cobra.Command{
	Use:   "db <name> [profile]",
	Short: "Provision a worktree's database",
	Long: `Starts the worktree's container, restores the requested profile the first
time, applies pending migrations, and then proves the database is not empty.

This is the only path to a populated database, deliberately. --empty is the one
way to get a blank one.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runDB,
}

func init() {
	dbCmd.Flags().BoolVar(&dbEmpty, "empty", false, "start a blank database on purpose")
	dbCmd.Flags().BoolVar(&dbReseed, "reseed", false, "wipe and re-restore (destroys data)")
	rootCmd.AddCommand(dbCmd)
}

func runDB(cmd *cobra.Command, args []string) error {
	cfg, root, err := loadConfig()
	if err != nil {
		return err
	}
	if !cfg.HasDB() {
		return fmt.Errorf("no database configured in %s", config.FileName)
	}
	name := args[0]
	wtPath, slot, err := requireWorktree(cfg, root, name)
	if err != nil {
		return err
	}

	profile := ""
	if len(args) == 2 {
		profile = args[1]
	}
	// Precedence: explicit argument, then what this worktree already uses,
	// then the project default.
	if profile == "" {
		profile = provision.ReadMarker(wtPath, config.DBProfileFile)
	}
	if profile == "" {
		profile = cfg.DB.Default
	}

	return provisionDB(cfg, root, wtPath, name, slot, profile, dbEmpty, dbReseed)
}

func provisionDB(cfg *config.Config, root, wtPath, name string, slot int,
	profile string, empty, reseed bool) error {

	env, err := dbx.NewEnv(cfg, root, wtPath, name, slot)
	if err != nil {
		return err
	}

	// Owning a database only makes sense alongside owning a backend.
	if provision.ReadMarker(wtPath, config.ModeFile) != provision.ModeFullstack {
		ui.Info("switching %q to fullstack mode (own backend and database)", name)
		if err := provision.Setup(provision.Options{
			Config: cfg, MainRoot: root, Path: wtPath, Name: name, Slot: slot,
			Mode: provision.ModeFullstack, SkipInstall: true,
		}); err != nil {
			return err
		}
	}

	if err := env.WriteCompose(); err != nil {
		return err
	}

	fresh := !env.VolumeExists()
	if reseed {
		if !isInteractive() {
			return fmt.Errorf("refusing to --reseed non-interactively — it destroys data")
		}
		fmt.Printf("Wipe %s and re-restore the %s dump? [y/N] ", env.DBName, profile)
		r := bufio.NewReader(os.Stdin)
		ans, _ := r.ReadString('\n')
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(ans)), "y") {
			ui.Info("keeping existing data")
			reseed = false
		} else {
			env.Destroy()
			fresh = true
		}
	}

	// Resolve the dump before starting anything, so a missing one fails fast.
	var dump string
	if !empty && fresh {
		d, err := env.EnsureDump(profile)
		if err != nil {
			return err
		}
		dump = d
	}

	ui.Info("starting database for %q on port %d ...", name, env.Port)
	if err := env.Up(); err != nil {
		return err
	}
	if err := env.WaitReady(60 * time.Second); err != nil {
		return err
	}

	if empty {
		writeProfile(wtPath, "empty")
		ui.Warn("started an EMPTY database on purpose (--empty)")
		ui.Ok("database up: %s on localhost:%d", env.DBName, env.Port)
		return nil
	}

	if fresh {
		ui.Info("restoring the %s dump into %s ...", profile, env.DBName)
		if err := env.Restore(dump); err != nil {
			return err
		}
	} else {
		ui.Info("reusing the existing %s database", profile)
	}

	ui.Info("applying pending migrations ...")
	if err := env.Migrate(); err != nil {
		return err
	}
	if err := env.AssertPopulated(profile); err != nil {
		return err
	}

	writeProfile(wtPath, profile)
	ui.Ok("%q is serving %s data — %s on localhost:%d", name, profile, env.DBName, env.Port)
	return nil
}

func writeProfile(wtPath, profile string) {
	_ = os.WriteFile(wtPath+"/"+config.DBProfileFile, []byte(profile+"\n"), 0o644)
}

func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
