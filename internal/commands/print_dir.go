package commands

import (
	"github.com/spf13/cobra"

	"flowtree/internal/worktree"
)

// newPrintDirCmd is wtp's `cd`: it only prints, never changes anything.
func newPrintDirCmd(d Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "print-dir [<worktree>]",
		Short: "Print the absolute path of a worktree",
		Long: "Prints the absolute path of an existing worktree. With no argument, or \"@\", " +
			"prints the main worktree. Use `flowtree switch` to actually move there.",
		Example: "  cd \"$(flowtree print-dir feature/auth)\"",
		Args:    cobra.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return completeWorktrees(d)(cmd, args, prefix)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := d.OpenRepo(cmd.Context())
			if err != nil {
				return err
			}
			w, err := worktree.Resolve(repo.Worktrees, firstArg(args))
			if err != nil {
				return err
			}
			d.Printf("%s\n", w.Path)
			return nil
		},
	}
}

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}
