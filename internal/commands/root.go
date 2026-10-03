package commands

import (
	"github.com/spf13/cobra"
)

// Constructor builds one command from the shared dependencies.
type Constructor func(Deps) *cobra.Command

// Constructors lists every top-level command; keep it sorted.
var Constructors = []Constructor{
	newAddCmd,
	newExecCmd,
	newInitCmd,
	newListCmd,
	newPrintDirCmd,
	newRemoveCmd,
	newShellInitCmd,
}

// NewRoot assembles the root command.
func NewRoot(d Deps, version string) *cobra.Command {
	root := &cobra.Command{
		Use:           "flowtree",
		Short:         "Git worktrees with opinionated git-flow branching",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(d.Stdout)
	root.SetErr(d.Stderr)
	root.SetIn(d.Stdin)
	for _, c := range Constructors {
		root.AddCommand(c(d))
	}
	return root
}
