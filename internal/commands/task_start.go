package commands

import (
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"flowtree/internal/board"
	"flowtree/internal/flow"
)

func startArg(t board.Type) string {
	if t == board.Hotfix {
		return "<version>"
	}
	return "<name>"
}

func newTaskStartCmd(d Deps) *cobra.Command {
	var fetch, dry bool
	cmd := &cobra.Command{
		Use:   "start [-F] [--dry-run] <n> <name|version> [<base>]",
		Short: "Start the branch of an existing task, by its type",
		Long: "A regular task gets regular/<name> from develop; a hotfix task gets hotfix/<version> from master\n" +
			"(the version is required). <base> overrides the start point. The branch gets its own worktree and\n" +
			"the task is linked to it (and leaves the backlog). Refused when the task already has a branch.",
		Args: cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			return runPlan(cmd.Context(), d, dry, func(e *flow.Env) ([]flow.Step, error) {
				kind, err := flow.KindOfTask(e, id)
				if err != nil {
					return nil, err
				}
				return flow.PlanStart(e, flow.StartOpts{Kind: kind, Name: args[1], Base: optArg(args, 2), Task: id, Fetch: fetch})
			})
		},
		ValidArgsFunction: completeStartTasks(d),
	}
	cmd.Flags().BoolVarP(&fetch, "fetch", "F", false, "fetch the remote first")
	cmd.Flags().BoolVar(&dry, "dry-run", false, "print the steps without running them")
	return cmd
}

// completeStartTasks suggests the ids of tasks without a branch, for the first argument.
func completeStartTasks(d Deps) Completer {
	return func(cmd *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
		no := cobra.ShellCompDirectiveNoFileComp
		if len(args) > 0 {
			return nil, no
		}
		var out []string
		_ = withBoard(cmd.Context(), d, func(_ *flow.Env, path string) error {
			b, err := board.Load(path)
			for _, c := range b.Cards {
				if id := strconv.Itoa(c.ID); c.Branch == "" && strings.HasPrefix(id, prefix) {
					out = append(out, id+"\t"+c.Title)
				}
			}
			return err
		})
		return out, no
	}
}
