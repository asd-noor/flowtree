package commands

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"flowtree/internal/board"
	"flowtree/internal/flow"
	"flowtree/internal/gitx"
	"flowtree/internal/picker"
)

// newTopicCmd builds `flowtree feature|release|hotfix` with its subcommands.
func newTopicCmd(k flow.Kind) Constructor {
	return func(d Deps) *cobra.Command {
		cmd := &cobra.Command{
			Use:   topicName(k),
			Short: topicShort(k),
			Args:  cobra.NoArgs,
			RunE:  func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
		}
		for _, sub := range topicSubs(k) {
			cmd.AddCommand(sub(k, d))
		}
		if k == flow.Regular {
			cmd.AddCommand(newDiffSub(k, d), newRebaseSub(k, d), newPullSub(k, d))
		}
		return cmd
	}
}

// topicSubs are the subcommands of a kind; work branches are local only, so they cannot be
// published or tracked.
func topicSubs(k flow.Kind) []func(flow.Kind, Deps) *cobra.Command {
	subs := []func(flow.Kind, Deps) *cobra.Command{newListSub, newStartSub, newFinishSub, newCheckoutSub, newDeleteSub}
	switch k {
	case flow.Work:
		return subs
	case flow.Regular, flow.Hotfix:
		subs = append(subs, newReviewSub)
	}
	return append(subs, newPublishSub, newTrackSub)
}

func topicShort(k flow.Kind) string {
	switch k {
	case flow.Work:
		return "Work branches: local subtasks of a regular or hotfix branch, rebased onto it when finished"
	case flow.Regular:
		return "Regular branches: start from develop, merge back into develop with a merge commit"
	case flow.Release:
		return "Release branches: start from develop, merge into master (tagged) and back into develop"
	}
	return "Hotfix branches: start from master, merge into master (tagged) and back into develop"
}

func nameArg(k flow.Kind) string {
	switch k {
	case flow.Regular:
		return "<name>"
	case flow.Work:
		return "<n>/<subtask>"
	}
	return "<version>"
}

// planFunc builds the steps of a command once the flow environment is known.
type planFunc func(*flow.Env) ([]flow.Step, error)

// runPlan opens the repository, builds the plan and runs it, or only prints it for --dry-run.
func runPlan(ctx context.Context, d Deps, dry bool, plan planFunc) error {
	e, err := d.OpenFlow(ctx)
	if err != nil {
		return err
	}
	steps, err := plan(e)
	if err != nil {
		return err
	}
	if dry {
		d.Printf("Dry run; would do:\n%s\n", flow.Describe(steps))
		return nil
	}
	return flow.Run(e, steps)
}

// withFlow opens the flow environment and runs fn with it.
func withFlow(ctx context.Context, d Deps, fn func(*flow.Env) error) error {
	e, err := d.OpenFlow(ctx)
	if err != nil {
		return err
	}
	return fn(e)
}

func optArg(args []string, i int) string {
	if i < len(args) {
		return args[i]
	}
	return ""
}

// ---- completion

func completeTopics(k flow.Kind, d Deps) Completer {
	return func(cmd *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		e, err := d.OpenFlow(cmd.Context())
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		if k == flow.Work {
			return flow.WorkRefs(e, prefix), cobra.ShellCompDirectiveNoFileComp
		}
		return flow.Names(e, k, prefix), cobra.ShellCompDirectiveNoFileComp
	}
}

// completeRemoteTopics suggests topic branches that exist only on the remote.
func completeRemoteTopics(k flow.Kind, d Deps) Completer {
	return func(cmd *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
		e, err := d.OpenFlow(cmd.Context())
		if err != nil || len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		refs, _ := e.MainGit().Lines("for-each-ref", "--format=%(refname:short)", "refs/remotes/"+e.S.Remote)
		var out []string
		for _, ref := range refs {
			b := strings.TrimPrefix(ref, e.S.Remote+"/")
			if strings.HasPrefix(b, e.S.Prefix(k)) && !gitx.BranchExists(e.MainGit(), b) && strings.HasPrefix(e.S.Name(k, b), prefix) {
				out = append(out, e.S.Name(k, b))
			}
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}

// ---- list

func newListSub(k flow.Kind, d Deps) *cobra.Command {
	var verbose bool
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   fmt.Sprintf("List %s branches", k),
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withFlow(cmd.Context(), d, func(e *flow.Env) error { return printList(d, e, k, verbose) })
		},
	}
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "show how each branch relates to its base")
	return cmd
}

func printList(d Deps, e *flow.Env, k flow.Kind, verbose bool) error {
	entries, err := flow.List(e, k, verbose)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		d.Printf("%s", noBranches(k))
		return nil
	}
	d.Printf("%s", renderEntries(entries))
	return nil
}

func noBranches(k flow.Kind) string {
	if k == flow.Regular || k == flow.Hotfix {
		return fmt.Sprintf("No %s branches exist.\n\nYou can add a task and start its branch:\n\n    flowtree task add <title>\n    flowtree task start <n> %s\n\n", k, nameArg(k))
	}
	return fmt.Sprintf("No %s branches exist.\n\nYou can start a new %s branch:\n\n    flowtree %s start %s\n\n", k, k, k, nameArg(k))
}

// renderEntries formats list entries; the current branch is starred.
func renderEntries(entries []flow.Entry) string {
	width := 0
	for _, en := range entries {
		width = max(width, len(en.Name))
	}
	var b strings.Builder
	for _, en := range entries {
		mark := "  "
		if en.Current {
			mark = "* "
		}
		if en.State == "" {
			fmt.Fprintf(&b, "%s%s\n", mark, en.Name)
		} else {
			fmt.Fprintf(&b, "%s%-*s %s\n", mark, width, en.Name, en.State)
		}
	}
	return b.String()
}

// ---- start / track / checkout / publish

func newStartSub(k flow.Kind, d Deps) *cobra.Command {
	if k == flow.Work {
		return newWorkStartSub(d)
	}
	o := flow.StartOpts{Kind: k}
	var dry bool
	cmd := &cobra.Command{
		Use:   "start [-F] [--dry-run] " + nameArg(k) + " [<base>]",
		Short: fmt.Sprintf("Start a new %s branch", k),
		Long: fmt.Sprintf("Create a %s branch from <base> (default %s) and give it its own worktree.",
			k, map[bool]string{true: "master", false: "develop"}[k == flow.Hotfix]),
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Name, o.Base = args[0], optArg(args, 1)
			return runPlan(cmd.Context(), d, dry, func(e *flow.Env) ([]flow.Step, error) { return flow.PlanStart(e, o) })
		},
	}
	cmd.Flags().BoolVarP(&o.Fetch, "fetch", "F", false, "fetch the remote first")
	cmd.Flags().BoolVar(&dry, "dry-run", false, "print the steps without running them")
	if k != flow.Release {
		cmd.Flags().IntVarP(&o.Task, "task", "T", 0, "the board task this branch is for (default: a new task named after it)")
	}
	return cmd
}

func newWorkStartSub(d Deps) *cobra.Command {
	o := flow.StartOpts{Kind: flow.Work}
	var dry bool
	var parent string
	cmd := &cobra.Command{
		Use:   "start [-P <n>] [-d <brief>] [--dry-run] <subtask>",
		Short: "Start a work branch: a local subtask of a regular or hotfix",
		Long: "Create work/<parent>/<subtask> from the tip of the branch of task <n> and give it its own worktree.\n" +
			"Without -P the parent is the regular or hotfix branch whose worktree you are in; elsewhere, on a\n" +
			"terminal, a picker offers the tasks that have an open regular or hotfix branch.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Name = args[0]
			return runPlan(cmd.Context(), d, dry, func(e *flow.Env) ([]flow.Step, error) {
				task, err := chooseParent(cmd.Context(), d, e, parent)
				if err != nil {
					return nil, err
				}
				o.Parent = task
				return flow.PlanStart(e, o)
			})
		},
	}
	cmd.Flags().StringVarP(&parent, "parent", "P", "", "the number of the task whose regular or hotfix branch it belongs to (e.g. 12 or #12)")
	cmd.Flags().StringVarP(&o.Desc, "description", "d", "", "the subtask's brief on the board (Markdown)")
	cmd.Flags().BoolVar(&dry, "dry-run", false, "print the steps without running them")
	return cmd
}

// chooseParent is the task number of the parent: the one given (-P), else the working directory's
// (0), or, on a terminal when neither works, a task picked from those with an open regular or hotfix branch.
func chooseParent(ctx context.Context, d Deps, e *flow.Env, given string) (int, error) {
	if given != "" {
		return parseID(given)
	}
	_, err := flow.ResolveParent(e, 0)
	if !errors.Is(err, flow.ErrNoParent) || !d.IsTTY() {
		return 0, err
	}
	open, err := flow.ParentTasks(e)
	if err != nil || len(open) == 0 {
		return 0, errors.Join(flow.ErrNoParent, err)
	}
	row, err := d.pick(ctx, mapSlice(open, parentRow), "")
	if err != nil {
		return 0, err
	}
	return parseID(row.Key)
}

func parentRow(c board.Card) picker.Row {
	return picker.Row{Key: strconv.Itoa(c.ID), Label: fmt.Sprintf("#%d %s (%s)", c.ID, c.Title, c.Branch)}
}

// workBranch is the work branch named by "<n>/<subtask>".
func workBranch(e *flow.Env, ref string) (string, error) {
	id, sub, err := parseRef(ref)
	if err != nil {
		return "", err
	}
	return flow.WorkRef(e, id, sub)
}

func newTrackSub(k flow.Kind, d Deps) *cobra.Command {
	return &cobra.Command{
		Use:               "track " + nameArg(k),
		Short:             fmt.Sprintf("Check out a %s branch that exists on the remote", k),
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeRemoteTopics(k, d),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPlan(cmd.Context(), d, false, func(e *flow.Env) ([]flow.Step, error) { return flow.PlanTrack(e, k, args[0]) })
		},
	}
}

func newCheckoutSub(k flow.Kind, d Deps) *cobra.Command {
	return &cobra.Command{
		Use:               "checkout " + optional(k, nameArg(k)),
		Short:             fmt.Sprintf("Print the directory of a %s branch, creating its worktree if needed", k),
		Args:              topicArgs(k),
		ValidArgsFunction: completeTopics(k, d),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withFlow(cmd.Context(), d, func(e *flow.Env) error {
				name, err := resolveArg(cmd.Context(), d, e, k, optArg(args, 0))
				if err != nil {
					return err
				}
				dir, err := flow.Checkout(e, k, name)
				if err == nil {
					d.Printf("%s\n", dir)
				}
				return err
			})
		},
	}
}

func newPublishSub(k flow.Kind, d Deps) *cobra.Command {
	return &cobra.Command{
		Use:               "publish [" + nameArg(k) + "]",
		Short:             fmt.Sprintf("Push a %s branch to the remote and set its upstream", k),
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeTopics(k, d),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withFlow(cmd.Context(), d, func(e *flow.Env) error { return flow.Publish(e, k, optArg(args, 0)) })
		},
	}
}

func newReviewSub(k flow.Kind, d Deps) *cobra.Command {
	var undo bool
	cmd := &cobra.Command{
		Use:   "review [--undo] [" + nameArg(k) + "]",
		Short: fmt.Sprintf("Mark the %s's task as ready for review; new commits move it back to doing", k),
		Long: "Records the branch's current tip on its board task, which then shows in review. A new commit moves\n" +
			"the task back to doing by itself; run review again when it is ready. --undo clears the mark.",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeTopics(k, d),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withFlow(cmd.Context(), d, func(e *flow.Env) error {
				branch, err := flow.Review(e, k, optArg(args, 0), undo)
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

// ---- delete

func newDeleteSub(k flow.Kind, d Deps) *cobra.Command {
	o := flow.DeleteOpts{Kind: k}
	var dry bool
	cmd := &cobra.Command{
		Use:               "delete [-f] [-r] [--dry-run] " + nameArg(k),
		Short:             fmt.Sprintf("Delete a %s branch and its worktree without merging it", k),
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeTopics(k, d),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPlan(cmd.Context(), d, dry, func(e *flow.Env) ([]flow.Step, error) {
				name, err := resolveArg(cmd.Context(), d, e, k, args[0])
				o.Name = name
				if err != nil {
					return nil, err
				}
				return flow.PlanDelete(e, o)
			})
		},
	}
	cmd.Flags().BoolVarP(&o.Force, "force", "f", false, "delete even when not merged; remove the worktree even with untracked files")
	cmd.Flags().BoolVarP(&o.Remote, "remote", "r", false, "also delete the branch on the remote")
	cmd.Flags().BoolVar(&dry, "dry-run", false, "print the steps without running them")
	return cmd
}

// ---- finish

func newFinishSub(k flow.Kind, d Deps) *cobra.Command {
	o := flow.FinishOpts{Kind: k}
	var dry, keep bool
	cmd := &cobra.Command{
		Use:               "finish [flags] " + optional(k, nameArg(k)),
		Short:             finishShort(k),
		Args:              topicArgs(k),
		ValidArgsFunction: completeTopics(k, d),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Name = optArg(args, 0)
			if keep {
				o.Cleanup.KeepLocal, o.Cleanup.KeepRemote = true, true
			}
			if o.Message != "" && o.MessageFile != "" {
				return fmt.Errorf("use either --message or --messagefile, not both")
			}
			return runPlan(cmd.Context(), d, dry, func(e *flow.Env) ([]flow.Step, error) {
				name, err := resolveArg(cmd.Context(), d, e, k, o.Name)
				if err != nil {
					return nil, err
				}
				o.Name = name
				return flow.PlanFinish(e, o)
			})
		},
	}
	bindCommonFinish(cmd.Flags(), &o, &keep, &dry)
	switch k {
	case flow.Regular:
		cmd.Flags().BoolVar(&o.Push, "push", false, "push develop afterwards")
	case flow.Release, flow.Hotfix:
		bindReleaseFinish(cmd.Flags(), &o, k)
	}
	return cmd
}

func finishShort(k flow.Kind) string {
	if k == flow.Work {
		return "Rebase a work branch onto its parent, fast-forward the parent and clean up"
	}
	if k == flow.Regular {
		return "Merge a regular branch into develop and clean up"
	}
	return fmt.Sprintf("Merge a %s into master, tag it, merge the tag into develop and clean up", k)
}

func bindCommonFinish(f *pflag.FlagSet, o *flow.FinishOpts, keep, dry *bool) {
	f.BoolVarP(keep, "keep", "k", false, "keep the local and remote branch (the worktree is still removed)")
	f.BoolVar(&o.Cleanup.KeepLocal, "keeplocal", false, "keep the local branch")
	f.BoolVar(&o.Cleanup.KeepRemote, "keepremote", false, "keep the remote branch")
	f.BoolVarP(&o.Cleanup.Force, "force-delete", "D", false, "remove the worktree even when it has untracked files")
	f.BoolVar(dry, "dry-run", false, "print the steps without running them")
}

func bindReleaseFinish(f *pflag.FlagSet, o *flow.FinishOpts, k flow.Kind) {
	f.BoolVarP(&o.Fetch, "fetch", "F", false, "fetch the remote first")
	f.BoolVarP(&o.Squash, "squash", "S", false, "squash the branch into one commit")
	f.BoolVarP(&o.Push, "push", "p", false, "push develop, master and the tag afterwards")
	f.BoolVarP(&o.Sign, "sign", "s", false, "sign the tag")
	f.StringVarP(&o.SignKey, "signingkey", "u", "", "sign the tag with this key")
	f.StringVarP(&o.Message, "message", "m", "", "tag message")
	f.StringVarP(&o.MessageFile, "messagefile", "f", "", "read the tag message from a file")
	f.BoolVarP(&o.NoTag, "notag", "n", false, "do not tag")
	f.BoolVarP(&o.NoBackmerge, "nobackmerge", "b", false, "do not merge back into develop")
	f.StringVarP(&o.TagName, "tagname", "T", "", "use this tag name instead of <version>")
	if k == flow.Release {
		f.BoolVar(&o.FFMaster, "ff-master", false, "fast-forward master instead of creating a merge commit")
	}
}

// topicArgs: work branches are addressed by task and subtask, so their argument is required.
func topicArgs(k flow.Kind) cobra.PositionalArgs {
	if k == flow.Work {
		return cobra.ExactArgs(1)
	}
	return cobra.MaximumNArgs(1)
}

func optional(k flow.Kind, arg string) string {
	if k == flow.Work {
		return arg
	}
	return "[" + arg + "]"
}

// resolveArg is the topic branch a command argument names: "<n>/<subtask>" for work, else see chooseTopic.
func resolveArg(ctx context.Context, d Deps, e *flow.Env, k flow.Kind, arg string) (string, error) {
	if k == flow.Work {
		return workBranch(e, arg)
	}
	return chooseTopic(ctx, d, e, k, arg)
}

// chooseTopic is name, or, on a terminal when no name was given and the working directory is not
// inside a worktree of kind k, the open branch of that kind picked from a list.
func chooseTopic(ctx context.Context, d Deps, e *flow.Env, k flow.Kind, name string) (string, error) {
	if name != "" || !d.IsTTY() {
		return name, nil
	}
	if _, err := flow.ResolveTopic(e, k, ""); err == nil {
		return "", nil // inferred from the working directory
	}
	open, _ := flow.Open(e, k)
	if len(open) == 0 {
		return "", nil // let ResolveTopic report what is wrong
	}
	row, err := d.pick(ctx, mapSlice(open, func(n string) picker.Row { return picker.Row{Key: n, Label: n} }), "")
	return row.Key, err
}

// topicName is the command group of a kind. Regular branches keep the legacy `feature` group until
// it is removed.
func topicName(k flow.Kind) string {
	if k == flow.Regular {
		return "feature"
	}
	return k.String()
}
