// Command grove is Grove — parallel dev environments built on git worktrees.
//
// Grove gives each worktree reserved ports, its own dependencies, its own
// database and a background runner, so many branches can run simultaneously
// without one terminal each. Project shape lives in .grove.json (committed);
// machine-local allocation lives in ~/.grove.
package main

import "github.com/PrerakGada/grove/cmd"

func main() { cmd.Execute() }
