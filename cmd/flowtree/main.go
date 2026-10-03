// Command flowtree manages git worktrees with git-flow style branching.
package main

import (
	"fmt"
	"os"

	"flowtree/internal/commands"
)

// Set with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := commands.NewRoot(commands.RealDeps(), version).Execute(); err != nil {
		if code, ok := commands.ExitCode(err); ok {
			os.Exit(code)
		}
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
