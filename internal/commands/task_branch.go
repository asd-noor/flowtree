package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"flowtree/internal/flow"
)

const idOptional = "[<n>]"

// hotfixOnlyFinish are the finish flags a regular task has no use for.
var hotfixOnlyFinish = []string{"fetch", "squash", "sign", "signingkey", "message", "messagefile", "notag", "nobackmerge", "tagname"}

// idArg reads the optional task number; 0 means "the task of the working directory".
func idArg(args []string) (int, error) {
	if len(args) == 0 {
		return 0, nil
	}
	return parseID(args[0])
}

// withTask resolves the task named by args (see flow.ResolveTask) and runs fn with it.
func withTask(ctx context.Context, d Deps, args []string, fn func(*flow.Env, flow.Task) error) error {
	id, err := idArg(args)
	if err != nil {
		return err
	}
	return withFlow(ctx, d, func(e *flow.Env) error {
		t, err := flow.ResolveTask(e, id)
		if err != nil {
			return err
		}
		return fn(e, t)
	})
}

// planTask is runPlan for the steps of a resolved task.
func planTask(ctx context.Context, d Deps, args []string, dry bool, plan func(*flow.Env, flow.Task) ([]flow.Step, error)) error {
	id, err := idArg(args)
	if err != nil {
		return err
	}
	return runPlan(ctx, d, dry, func(e *flow.Env) ([]flow.Step, error) {
		t, err := flow.ResolveTask(e, id)
		if err != nil {
			return nil, err
		}
		return plan(e, t)
	})
}

func regularOnly(verb string, t flow.Task) error {
	if t.Kind != flow.Regular {
		return fmt.Errorf("%s is for regular tasks; task #%d is a hotfix", verb, t.Card.ID)
	}
	return nil
}

func newTaskReviewCmd(d Deps) *cobra.Command {
	var undo bool
	cmd := &cobra.Command{
		Use:   "review [--undo] " + idOptional,
		Short: "Mark the task as ready for review; new commits move it back to doing",
		Long: "Records the branch's current tip on the task, which then shows in review. A new commit moves\n" +
			"the task back to doing by itself; run review again when it is ready. --undo clears the mark.\n" +
			"Without <n> it is the task of the worktree you are in (or of a work worktree's parent).",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withTask(cmd.Context(), d, args, func(e *flow.Env, t flow.Task) error {
				branch, err := flow.Review(e, t.Kind, t.Name(e), undo)
				if err == nil && undo {
					d.Printf("The review mark of '%s' was cleared\n", branch)
				} else if err == nil {
					d.Printf("'%s' is ready for review\n", branch)
				}
				return err
			})
		},
	}
	cmd.Flags().BoolVar(&undo, "undo", false, "clear the review mark")
	return cmd
}

func newTaskDiffCmd(d Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "diff " + idOptional,
		Short: "Show the changes of the task's branch since it left its base",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withTask(cmd.Context(), d, args, func(e *flow.Env, t flow.Task) error {
				out, err := flow.TaskDiff(e, t)
				if err == nil && out != "" {
					d.Printf("%s\n", out)
				}
				return err
			})
		},
	}
}

func newTaskPublishCmd(d Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "publish " + idOptional,
		Short: "Push the task's branch to the remote and set its upstream",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withTask(cmd.Context(), d, args, func(e *flow.Env, t flow.Task) error {
				return flow.Publish(e, t.Kind, t.Name(e))
			})
		},
	}
}

func newTaskRebaseCmd(d Deps) *cobra.Command {
	var o flow.RebaseOpts
	cmd := &cobra.Command{
		Use:   "rebase [-i] [-p] " + idOptional,
		Short: "Rebase the task's branch onto develop, in its worktree (regular tasks)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Attach = attachedGit(cmd.Context(), d)
			return planTask(cmd.Context(), d, args, false, func(e *flow.Env, t flow.Task) ([]flow.Step, error) {
				if err := regularOnly("rebase", t); err != nil {
					return nil, err
				}
				o.Name = t.Name(e)
				return flow.PlanRebase(e, o)
			})
		},
	}
	cmd.Flags().BoolVarP(&o.Interactive, "interactive", "i", false, "interactive rebase (needs a terminal)")
	cmd.Flags().BoolVarP(&o.Preserve, "preserve-merges", "p", false, "keep merges while rebasing")
	return cmd
}

func newTaskPullCmd(d Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "pull " + idOptional,
		Short: "Fast-forward the task's branch from the remote (regular tasks)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return planTask(cmd.Context(), d, args, false, func(e *flow.Env, t flow.Task) ([]flow.Step, error) {
				if err := regularOnly("pull", t); err != nil {
					return nil, err
				}
				return flow.PlanPull(e, t.Name(e))
			})
		},
	}
}

func newTaskCheckoutCmd(d Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "checkout <n>",
		Short: "Print the directory of the task's branch, creating its worktree if needed",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withTask(cmd.Context(), d, args, func(e *flow.Env, t flow.Task) error {
				dir, err := flow.Checkout(e, t.Kind, t.Name(e))
				if err == nil {
					d.Printf("%s\n", dir)
				}
				return err
			})
		},
	}
}

func newTaskDeleteCmd(d Deps) *cobra.Command {
	o := flow.DeleteOpts{}
	var dry bool
	cmd := &cobra.Command{
		Use:   "delete [-f] [-r] [--dry-run] <n>",
		Short: "Delete the task's branch and its worktree without merging it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return planTask(cmd.Context(), d, args, dry, func(e *flow.Env, t flow.Task) ([]flow.Step, error) {
				o.Kind, o.Name = t.Kind, t.Name(e)
				return flow.PlanDelete(e, o)
			})
		},
	}
	cmd.Flags().BoolVarP(&o.Force, "force", "f", false, "delete even when not merged; remove the worktree even with untracked files")
	cmd.Flags().BoolVarP(&o.Remote, "remote", "r", false, "also delete the branch on the remote")
	cmd.Flags().BoolVar(&dry, "dry-run", false, "print the steps without running them")
	return cmd
}

func newTaskFinishCmd(d Deps) *cobra.Command {
	var o flow.FinishOpts
	var dry, keep bool
	cmd := &cobra.Command{
		Use:   "finish [flags] <n>",
		Short: "Merge the task's branch (regular: into develop; hotfix: into master, tagged, and back into develop) and clean up",
		Long: "Regular tasks take --push, --keep, --keeplocal, --keepremote, --force-delete and --dry-run.\n" +
			"Hotfix tasks also take --fetch, --squash, --sign, --signingkey, --message, --messagefile, --notag,\n" +
			"--nobackmerge and --tagname. A flag the task's type does not take is an error.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if keep {
				o.Cleanup.KeepLocal, o.Cleanup.KeepRemote = true, true
			}
			if o.Message != "" && o.MessageFile != "" {
				return fmt.Errorf("use either --message or --messagefile, not both")
			}
			return planTask(cmd.Context(), d, args, dry, func(e *flow.Env, t flow.Task) ([]flow.Step, error) {
				if err := checkFinishFlags(cmd.Flags(), t); err != nil {
					return nil, err
				}
				warnUnfinishedNeeds(e, t.Card.ID)
				o.Kind, o.Name = t.Kind, t.Name(e)
				return flow.PlanFinish(e, o)
			})
		},
	}
	bindCommonFinish(cmd.Flags(), &o, &keep, &dry)
	bindReleaseFinish(cmd.Flags(), &o, flow.Hotfix)
	return cmd
}

// checkFinishFlags refuses a flag that was set but does not fit the task's type.
func checkFinishFlags(f *pflag.FlagSet, t flow.Task) error {
	if t.Kind != flow.Regular {
		return nil
	}
	for _, name := range hotfixOnlyFinish {
		if f.Changed(name) {
			return fmt.Errorf("--%s is for hotfix tasks; task #%d is a regular task", name, t.Card.ID)
		}
	}
	return nil
}

func newTaskTrackCmd(d Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "track <n> <remote branch>",
		Short: "Check out a branch that exists on the remote and link it to an existing task",
		Long: "The task must exist, have no branch and be of the branch's type (regular/... or hotfix/...).\n" +
			"No task is created.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			return runPlan(cmd.Context(), d, false, func(e *flow.Env) ([]flow.Step, error) {
				return flow.PlanTaskTrack(e, id, args[1])
			})
		},
	}
}

// taskBranchCmds are the lifecycle verbs of a task's branch.
func taskBranchCmds(d Deps) []*cobra.Command {
	return []*cobra.Command{newTaskReviewCmd(d), newTaskDiffCmd(d), newTaskPublishCmd(d), newTaskRebaseCmd(d),
		newTaskPullCmd(d), newTaskCheckoutCmd(d), newTaskFinishCmd(d), newTaskDeleteCmd(d), newTaskTrackCmd(d)}
}
