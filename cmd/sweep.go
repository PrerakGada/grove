package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/PrerakGada/grove/internal/config"
	"github.com/PrerakGada/grove/internal/platform"
	"github.com/PrerakGada/grove/internal/registry"
	"github.com/PrerakGada/grove/internal/ui"
	"github.com/PrerakGada/grove/internal/worktree"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var (
	sweepBase       string
	sweepDry        bool
	sweepYes        bool
	sweepKeepBranch bool
	sweepKeepDB     bool
	sweepRunning    bool
)

var sweepCmd = &cobra.Command{
	Use:     "sweep",
	Aliases: []string{"clean", "tidy"},
	Short:   "Delete every worktree that is finished: clean, merged, nothing ahead",
	Long: `Removes the worktrees that are provably done with — no uncommitted or
untracked files, no commit that is not already in the base branch — and frees
their slots, databases and branches.

A worktree is swept only when every one of these holds:

  · it is not the main checkout, and not the directory you are standing in
  · it is not locked with ` + "`git worktree lock`" + `
  · it is not on the base branch itself, or any other protected branch
  · ` + "`git status`" + ` is clean — no modified, staged or untracked files
  · its HEAD is an ancestor of the base branch, so it is merged and has
    nothing ahead; deleting it cannot remove a commit the base does not have
  · nothing is listening on its ports (override with --include-running)

Anything failing a check is listed with the reason and left alone. Nothing is
deleted until you confirm, and --dry-run stops before that.

Squash and rebase merges are deliberately NOT treated as merged: their commits
genuinely do not exist in the base branch, so this command cannot prove the work
survives. Those are reported as such — delete them yourself with ` + "`grove delete`" + `.`,
	Args: cobra.NoArgs,
	RunE: runSweep,
}

func init() {
	sweepCmd.Flags().StringVar(&sweepBase, "base", "",
		"branch to measure merged against (default: the remote's default branch, else main)")
	sweepCmd.Flags().BoolVar(&sweepDry, "dry-run", false, "show what would be deleted and stop")
	sweepCmd.Flags().BoolVarP(&sweepYes, "yes", "y", false, "do not ask for confirmation")
	sweepCmd.Flags().BoolVar(&sweepKeepBranch, "keep-branch", false, "keep the local branches")
	sweepCmd.Flags().BoolVar(&sweepKeepDB, "keep-db", false, "keep the database volumes")
	sweepCmd.Flags().BoolVar(&sweepRunning, "include-running", false,
		"also sweep worktrees whose services are currently listening")
	rootCmd.AddCommand(sweepCmd)
}

// candidate is one worktree's verdict. A nil Skip means it will be deleted.
type candidate struct {
	wt    worktree.Worktree
	skip  string // why it is being left alone; empty means sweep it
	ahead int    // commits in this worktree that the base does not have
	note  string // extra detail worth printing, e.g. a suspected squash merge
}

func runSweep(cmd *cobra.Command, args []string) error {
	cfg, root, err := loadConfig()
	if err != nil {
		return err
	}

	base, err := resolveBase(root, sweepBase)
	if err != nil {
		return err
	}
	baseTip, err := revParse(root, base)
	if err != nil {
		return fmt.Errorf("no branch or commit named %q — pass one that exists: "+
			"grove sweep --base origin/main", base)
	}

	wts, err := worktree.List(root, cfg)
	if err != nil {
		return err
	}
	if proj := loadProject(cfg); proj != nil {
		for i := range wts {
			if wts[i].IsMain {
				continue
			}
			if s, ok := proj.Slot(wts[i].Name); ok {
				wts[i].Slot, wts[i].HasSlot, wts[i].LegacySlot = s, true, false
			}
		}
	}

	protected := protectedBranches(root, cfg, base, wts)
	cwd, _ := os.Getwd()

	cands := make([]candidate, 0, len(wts))
	for _, w := range wts {
		cands = append(cands, judge(cfg, root, w, baseTip, base, protected, cwd))
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].wt.Name < cands[j].wt.Name })

	var sweepable, kept []candidate
	for _, c := range cands {
		if c.skip == "" {
			sweepable = append(sweepable, c)
		} else if !c.wt.IsMain {
			kept = append(kept, c)
		}
	}

	fmt.Println()
	fmt.Printf("%s  %s\n", ui.Title.Render(cfg.Project),
		ui.Dim.Render("merged against "+base+" ("+short(baseTip)+")"))
	fmt.Println()

	if len(kept) > 0 {
		tbl := ui.NewTable("KEEPING", "BRANCH", "REASON")
		for _, c := range kept {
			tbl.Row(c.wt.Name, branchLabel(c.wt), c.skip)
		}
		tbl.Print()
		for _, c := range kept {
			if c.note != "" {
				fmt.Printf("    %s\n", ui.Dim.Render(c.wt.Name+": "+c.note))
			}
		}
		fmt.Println()
	}

	if len(sweepable) == 0 {
		ui.Ok("nothing to sweep — no worktree is both clean and fully merged into %s", base)
		return nil
	}

	tbl := ui.NewTable("SWEEPING", "BRANCH", "SLOT", "PATH")
	tbl.Dim(3)
	for _, c := range sweepable {
		slot := "-"
		if c.wt.HasSlot {
			slot = strconv.Itoa(c.wt.Slot)
		}
		tbl.Row(c.wt.Name, branchLabel(c.wt), slot, c.wt.Path)
	}
	tbl.Print()
	fmt.Println()

	what, its := "worktree", "its"
	if len(sweepable) > 1 {
		what, its = "worktrees", "their"
	}
	if sweepKeepBranch {
		ui.Info("%d %s, %s slots and %s databases. Branches kept.", len(sweepable), what, its, its)
	} else {
		ui.Info("%d %s, %s slots, %s databases and %s branches.",
			len(sweepable), what, its, its, its)
	}

	if sweepDry {
		fmt.Println()
		ui.Info("Dry run — nothing deleted.")
		return nil
	}

	if !sweepYes {
		if !interactive() {
			return fmt.Errorf("refusing to delete %d %s without confirmation — "+
				"re-run with --yes, or --dry-run to see the list only", len(sweepable), what)
		}
		var proceed bool
		fmt.Println()
		if err := huh.NewConfirm().
			Title(fmt.Sprintf("Delete these %d %s?", len(sweepable), what)).
			Description("Every one is clean and already in " + base + ". This cannot be undone.").
			Value(&proceed).Run(); err != nil {
			// Not having a terminal is the common case here — a CI job, or a
			// pipe. huh's own message names /dev/tty, which does not tell
			// anyone what to do about it.
			return fmt.Errorf("no terminal to confirm on — re-run with --yes, "+
				"or --dry-run to see the list only (%v)", err)
		}
		if !proceed {
			return fmt.Errorf("aborted — nothing deleted")
		}
	}

	var failed []string
	for _, c := range sweepable {
		fmt.Println()
		ui.Info("%s", ui.Title.Render(c.wt.Name))
		if err := removeWorktree(cfg, root, c.wt.Name, c.wt.Path,
			sweepKeepBranch, sweepKeepDB); err != nil {
			ui.Err("%s: %v", c.wt.Name, err)
			failed = append(failed, c.wt.Name)
		}
	}

	fmt.Println()
	ui.Ok("swept %d of %d", len(sweepable)-len(failed), len(sweepable))
	if len(failed) > 0 {
		return fmt.Errorf("could not remove: %s", strings.Join(failed, ", "))
	}
	return nil
}

// judge applies every safety check in turn, cheapest and most absolute first.
// The order matters only for which reason gets reported; all of them must pass.
func judge(cfg *config.Config, root string, w worktree.Worktree, baseTip, base string,
	protected map[string]bool, cwd string) candidate {

	c := candidate{wt: w}
	switch {
	case w.IsMain:
		c.skip = "the main checkout"
		return c
	case w.Locked:
		c.skip = "locked"
		if w.LockReason != "" {
			c.skip += ": " + w.LockReason
		}
		return c
	case w.Prunable:
		c.skip = "stale — run grove prune"
		return c
	}

	// Removing the directory the shell is sitting in leaves that shell in a
	// path that no longer exists, and every later command in it fails oddly.
	if under(cwd, w.Path) {
		c.skip = "you are inside it"
		return c
	}

	// A worktree kept on the base branch itself — which `grove track` exists to
	// make, for deploys — passes the merged test trivially, since its HEAD *is*
	// the base. Without this it would be the first thing swept, every time.
	if w.Branch != "" && protected[w.Branch] {
		c.skip = "protected branch"
		return c
	}

	switch {
	case w.Changes < 0:
		c.skip = "could not read its git status"
		return c
	case w.Changes > 0:
		c.skip = fmt.Sprintf("%d uncommitted change(s)", w.Changes)
		return c
	}

	head := w.Head
	if head == "" {
		if h, err := revParse(w.Path, "HEAD"); err == nil {
			head = h
		} else {
			c.skip = "could not read its HEAD"
			return c
		}
	}

	// The whole safety argument reduces to this: if HEAD is an ancestor of the
	// base branch, every commit here is already reachable from the base, so
	// removing the worktree and its branch cannot drop a commit. It is both
	// halves of the question at once — "merged into base" and "nothing ahead
	// of base" are the same fact stated twice.
	if !isAncestor(root, head, baseTip) {
		c.ahead = countRange(root, baseTip, head)
		c.skip = fmt.Sprintf("%d commit(s) not in %s", c.ahead, base)
		if w.Branch != "" && squashMerged(root, w.Branch, baseTip) {
			c.note = "its changes appear to be in " + base + " already via a squash or rebase " +
				"merge, but the commits themselves are not — check, then: grove delete " + w.Name
		}
		return c
	}

	// A worktree sitting exactly on the base tip with nothing of its own is a
	// checkout of the base branch, not a finished piece of work — which is the
	// deploy checkout `grove track` exists to make. It passes the merged test
	// by definition, so without this it is swept every single time, whatever
	// its branch happens to be called.
	if head == baseTip {
		c.skip = "a checkout of " + base + ", nothing of its own"
		return c
	}

	// Detached HEADs get here only when that commit is in the base, so nothing
	// is lost; there is no branch to delete either.

	if !sweepRunning && w.HasSlot {
		if up := listening(cfg, w.Slot); up != "" {
			c.skip = "still running on " + up
			return c
		}
	}
	return c
}

// resolveBase picks the branch to measure against. The remote's default branch
// is preferred over the local one: a branch merged into origin/main is merged
// wherever it matters, while a local main can be arbitrarily stale, which would
// make this command refuse work that is plainly finished.
func resolveBase(root, want string) (string, error) {
	if want != "" {
		return want, nil
	}
	if out, err := exec.Command("git", "-C", root, "symbolic-ref", "--short", "-q",
		"refs/remotes/origin/HEAD").Output(); err == nil {
		if s := strings.TrimSpace(string(out)); s != "" {
			return s, nil
		}
	}
	for _, ref := range []string{"origin/main", "origin/master", "main", "master"} {
		if _, err := revParse(root, ref); err == nil {
			return ref, nil
		}
	}
	return "", fmt.Errorf("no base branch found — pass one with --base")
}

// protectedBranches is the set that is never swept whatever its history says:
// the base itself, the usual long-lived names, and whatever the main checkout
// has checked out.
func protectedBranches(root string, cfg *config.Config, base string,
	wts []worktree.Worktree) map[string]bool {

	p := map[string]bool{
		"main": true, "master": true, "develop": true, "dev": true,
		"staging": true, "production": true, "release": true,
	}
	p[strings.TrimPrefix(base, "origin/")] = true
	p[base] = true
	for _, w := range wts {
		if w.IsMain && w.Branch != "" {
			p[w.Branch] = true
		}
	}
	return p
}

func loadProject(cfg *config.Config) *registry.Project {
	reg, err := registry.Load()
	if err != nil {
		return nil
	}
	return reg.Projects[cfg.Project]
}

func listening(cfg *config.Config, slot int) string {
	var up []string
	for _, s := range cfg.Services {
		if len(platform.Listeners(s.Band+slot)) > 0 {
			up = append(up, fmt.Sprintf("%s:%d", s.Name, s.Band+slot))
		}
	}
	if cfg.HasDB() && len(platform.Listeners(cfg.DB.Band+slot)) > 0 {
		up = append(up, fmt.Sprintf("db:%d", cfg.DB.Band+slot))
	}
	return strings.Join(up, " ")
}

func isAncestor(root, commit, of string) bool {
	return exec.Command("git", "-C", root, "merge-base", "--is-ancestor", commit, of).Run() == nil
}

// squashMerged reports whether the branch's whole diff against the base is
// already contained in the base — the signature of a squash or rebase merge,
// where the content landed but the commits did not.
func squashMerged(root, branch, baseTip string) bool {
	mb, err := exec.Command("git", "-C", root, "merge-base", baseTip, branch).Output()
	if err != nil {
		return false
	}
	tree, err := exec.Command("git", "-C", root, "rev-parse", branch+"^{tree}").Output()
	if err != nil {
		return false
	}
	// A commit object made from the branch's tree on top of the merge base:
	// if cherry-pick sees it as already applied, the content is upstream.
	commit, err := exec.Command("git", "-C", root, "commit-tree", strings.TrimSpace(string(tree)),
		"-p", strings.TrimSpace(string(mb)), "-m", "grove sweep probe").Output()
	if err != nil {
		return false
	}
	out, err := exec.Command("git", "-C", root, "cherry", baseTip,
		strings.TrimSpace(string(commit))).Output()
	if err != nil {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(string(out)), "-")
}

func countRange(root, from, to string) int {
	out, err := exec.Command("git", "-C", root, "rev-list", "--count", from+".."+to).Output()
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0
	}
	return n
}

func revParse(dir, ref string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--verify", "-q", ref+"^{commit}").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// under reports whether p is inside dir, comparing resolved paths so a symlink
// into a worktree still counts as being in it.
func under(p, dir string) bool {
	if p == "" {
		return false
	}
	rp, err := filepath.EvalSymlinks(p)
	if err != nil {
		rp = p
	}
	rd, err := filepath.EvalSymlinks(dir)
	if err != nil {
		rd = dir
	}
	rel, err := filepath.Rel(rd, rp)
	if err != nil {
		return false
	}
	return rel == "." || !strings.HasPrefix(rel, "..")
}

func branchLabel(w worktree.Worktree) string {
	if w.Detached {
		return "(detached)"
	}
	if w.Branch == "" {
		return "-"
	}
	return w.Branch
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// interactive reports whether there is a terminal to prompt on. Without one,
// huh cannot ask and would either block or return a zero value — and a zero
// value here would mean "no", silently doing nothing in a script.
func interactive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
