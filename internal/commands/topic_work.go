package commands

import (
	"context"
	"os/exec"

	"github.com/spf13/cobra"

	"flowtree/internal/flow"
)

// attachedGit runs git in dir with the user's terminal attached (interactive rebase).
func attachedGit(ctx context.Context, d Deps) flow.Attached {
	return func(dir string, args ...string) error {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = dir
		cmd.Stdin, cmd.Stdout, cmd.Stderr = d.Stdin, d.Stdout, d.Stderr
		return cmd.Run()
	}
}

func newDiffSub(k flow.Kind, d Deps) *cobra.Command {
	return &cobra.Command{
		Use:               "diff [<name>]",
		Short:             "Show the changes of a work branch since it left develop",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeTopics(k, d),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withFlow(cmd.Context(), d, func(e *flow.Env) error {
				out, err := flow.Diff(e, optArg(args, 0))
				if err == nil && out != "" {
					d.Printf("%s\n", out)
				}
				return err
			})
		},
	}
}

func newRebaseSub(k flow.Kind, d Deps) *cobra.Command {
	var o flow.RebaseOpts
	cmd := &cobra.Command{
		Use:               "rebase [-i] [-p] [<name>]",
		Short:             "Rebase a work branch onto develop, in its worktree",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeTopics(k, d),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Name, o.Attach = optArg(args, 0), attachedGit(cmd.Context(), d)
			return runPlan(cmd.Context(), d, false, func(e *flow.Env) ([]flow.Step, error) { return flow.PlanRebase(e, o) })
		},
	}
	cmd.Flags().BoolVarP(&o.Interactive, "interactive", "i", false, "interactive rebase (needs a terminal)")
	cmd.Flags().BoolVarP(&o.Preserve, "preserve-merges", "p", false, "keep merges while rebasing")
	return cmd
}

func newPullSub(k flow.Kind, d Deps) *cobra.Command {
	return &cobra.Command{
		Use:               "pull <remote> [<name>]",
		Short:             "Fast-forward a work branch from a remote; create it when it does not exist locally",
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: completePull(k, d),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPlan(cmd.Context(), d, false, func(e *flow.Env) ([]flow.Step, error) {
				return flow.PlanPull(e, args[0], optArg(args, 1))
			})
		},
	}
}

// completePull suggests remotes for the first argument and topic names for the second.
func completePull(k flow.Kind, d Deps) Completer {
	topics := completeTopicsAt(k, d, 1)
	return func(cmd *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return topics(cmd, args, prefix)
		}
		e, err := d.OpenFlow(cmd.Context())
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		remotes, _ := e.MainGit().Lines("remote")
		return withPrefix(remotes, prefix), cobra.ShellCompDirectiveNoFileComp
	}
}

// completeTopicsAt completes topic names for the argument at index at (earlier ones are taken).
func completeTopicsAt(k flow.Kind, d Deps, at int) Completer {
	return func(cmd *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
		if len(args) != at {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		e, err := d.OpenFlow(cmd.Context())
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return flow.Names(e, k, prefix), cobra.ShellCompDirectiveNoFileComp
	}
}
