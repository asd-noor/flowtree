package commands

import (
	"github.com/spf13/cobra"

	"flowtree/internal/flow"
)

func newRelocateCmd(d Deps) *cobra.Command {
	var dry bool
	cmd := &cobra.Command{
		Use:   "relocate [-n]",
		Short: "Move topic worktrees to <base dir>/<branch> after the base directory changed",
		Long: "Moves work and hotfix worktrees to where the base directory says they belong. Locked worktrees,\n" +
			"missing directories and occupied targets are skipped with a note. It refuses to run while\n" +
			"you are inside a worktree it would move.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withFlow(cmd.Context(), d, func(e *flow.Env) error { return runRelocate(d, e, dry) })
		},
	}
	cmd.Flags().BoolVarP(&dry, "dry-run", "n", false, "show what would move without moving it")
	return cmd
}

func runRelocate(d Deps, e *flow.Env, dry bool) error {
	plan, err := flow.PlanRelocateRepo(e)
	if err != nil {
		return err
	}
	if len(plan.Moves) == 0 {
		d.Printf("Nothing to relocate; topic worktrees are already under %s.\n", e.Config.ResolveBaseDir(e.Main))
		return nil
	}
	steps := flow.RelocateSteps(plan)
	if dry {
		d.Printf("Would move:\n%s\n", flow.Describe(steps))
		return nil
	}
	return flow.Run(e, steps)
}
