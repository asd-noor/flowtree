package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"flowtree/internal/gitx"
	"flowtree/internal/picker"
	"flowtree/internal/worktree"
)

// newPrintDirCmd is wtp's `cd`: it only prints, never changes anything.
func newPrintDirCmd(d Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "print-dir [<worktree>]",
		Short: "Print the absolute path of a worktree",
		Long: "Prints the absolute path of an existing worktree. With no argument, or \"@\", " +
			"prints the main worktree; on a terminal, with no argument, a picker over the existing worktrees\n" +
			"opens instead. Use `flowtree switch` to actually move there.",
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
			query, err := chooseWorktree(cmd.Context(), d, firstArg(args), repo.Worktrees)
			if err != nil {
				return err
			}
			w, err := worktree.Resolve(repo.Worktrees, query)
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

// chooseWorktree returns name, or on a terminal with no name the worktree picked from wts.
func chooseWorktree(ctx context.Context, d Deps, name string, wts []gitx.Worktree) (string, error) {
	if name != "" || !d.IsTTY() {
		return name, nil
	}
	row, err := d.pick(ctx, worktreeRows(ctx, d, wts), "")
	return row.Key, err
}

// worktreeRows are picker rows for worktrees; the main one is "@" and comes first.
func worktreeRows(ctx context.Context, d Deps, wts []gitx.Worktree) []picker.Row {
	return mapSlice(wts, func(w gitx.Worktree) picker.Row {
		key := worktree.Display(w)
		label := key
		if w.Main {
			key, label = worktree.Main, fmt.Sprintf("@  [%s]", branchOrDetached(w))
		}
		return picker.Row{Key: key, Label: label, Preview: func() string {
			return dirPreview(gitx.Bind(ctx, d.Run, w.Path), w.Path, w.Branch)
		}}
	})
}

func branchOrDetached(w gitx.Worktree) string {
	if w.Branch == "" {
		return "detached"
	}
	return w.Branch
}
