package commands

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"flowtree/internal/flow"
	"flowtree/internal/gitx"
	"flowtree/internal/picker"
)

func newSwitchCmd(d Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "switch [QUERY]",
		Short: "Go to where a branch lives",
		Long: "Prints the directory of a branch and, with shell integration, cd's there.\n\n" +
			"Rows: '@' (the main worktree), every work/hotfix worktree, master/develop/release branches\n" +
			"(selecting one moves the main worktree onto it; it must be clean) and work/hotfix branches\n" +
			"without a worktree (selecting one creates it).\n\n" +
			"A query matches case-insensitively: exact name first, then substring, then fuzzy. When exactly\n" +
			"one row matches it is used at once; otherwise a picker opens with the query prefilled.",
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: completeSwitch(d),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withFlow(cmd.Context(), d, func(e *flow.Env) error {
				return runSwitch(cmd.Context(), d, e, strings.Join(args, " "))
			})
		},
	}
}

func runSwitch(ctx context.Context, d Deps, e *flow.Env, query string) error {
	targets, err := e.SwitchTargets()
	if err != nil {
		return err
	}
	rows := switchRows(e, targets)
	row, ok := picker.Resolve(rows, query)
	if !ok {
		if row, err = d.pick(ctx, rows, query); err != nil {
			return err
		}
	}
	i := slices.IndexFunc(targets, func(t flow.SwitchTarget) bool { return t.Key == row.Key })
	if i < 0 {
		return fmt.Errorf("no such target '%s'", row.Key)
	}
	dir, err := flow.Switch(e, targets[i])
	if err != nil {
		return err
	}
	d.Printf("%s\n", dir)
	return nil
}

func completeSwitch(d Deps) Completer {
	return func(cmd *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
		e, err := d.OpenFlow(cmd.Context())
		if err != nil || len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		targets, _ := e.SwitchTargets()
		keys := make([]string, len(targets))
		for i, t := range targets {
			keys[i] = t.Key
		}
		return withPrefix(keys, prefix), cobra.ShellCompDirectiveNoFileComp
	}
}

func switchRows(e *flow.Env, targets []flow.SwitchTarget) []picker.Row {
	return mapSlice(targets, func(t flow.SwitchTarget) picker.Row {
		return picker.Row{Key: t.Key, Label: t.Label, Alt: t.Alt, Preview: switchPreview(e, t)}
	})
}

// switchPreview shows where a target is and what is in it, or what selecting it will do.
func switchPreview(e *flow.Env, t flow.SwitchTarget) func() string {
	return func() string {
		if t.Dir != "" {
			return dirPreview(e.Git(t.Dir), t.Dir, t.Branch)
		}
		if t.Action == flow.MoveMain {
			return fmt.Sprintf("The main worktree will be moved onto '%s'.\n\n%s", t.Branch, logPreview(e.MainGit(), t.Branch))
		}
		return fmt.Sprintf("'%s' has no worktree yet; one will be created.\n\n%s", t.Branch, logPreview(e.MainGit(), t.Branch))
	}
}

// dirPreview is the path, `git status -sb` and the last commits of a worktree.
func dirPreview(g gitx.Git, dir, branch string) string {
	status, _ := g("-c", "color.ui=always", "status", "-sb")
	return fmt.Sprintf("%s\n\n%s\n\n%s", dir, status, logPreview(g, branch))
}

func logPreview(g gitx.Git, branch string) string {
	ref := branch
	if ref == "" {
		ref = "HEAD"
	}
	out, _ := g("-c", "color.ui=always", "log", "--oneline", "--decorate", "-n", "20", ref, "--")
	return out
}
