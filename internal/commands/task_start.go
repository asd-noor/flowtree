package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"flowtree/internal/board"
	"flowtree/internal/flow"
)

// kindOfType is the branch kind a task of type t starts.
func kindOfType(t board.Type) flow.Kind {
	if t == board.Hotfix {
		return flow.Hotfix
	}
	return flow.Regular
}

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
				kind, err := taskKind(e, id)
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

// taskKind is the branch kind for task id, so a missing task is reported before anything else.
func taskKind(e *flow.Env, id int) (flow.Kind, error) {
	path, err := flow.BoardPath(e)
	if err != nil {
		return flow.Regular, err
	}
	b, err := board.Load(path)
	if err != nil {
		return flow.Regular, err
	}
	c, ok := b.Card(id)
	if !ok {
		return flow.Regular, fmt.Errorf("no task #%d", id)
	}
	return kindOfType(flow.TypeOf(e.S, c)), nil
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
