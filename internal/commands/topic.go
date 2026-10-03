package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"flowtree/internal/flow"
	"flowtree/internal/gitx"
	"flowtree/internal/picker"
)

// newTopicCmd builds `flowtree work|release|hotfix` with its subcommands.
func newTopicCmd(k flow.Kind) Constructor {
	return func(d Deps) *cobra.Command {
		cmd := &cobra.Command{
			Use:   k.String(),
			Short: topicShort(k),
			Args:  cobra.NoArgs,
			RunE:  func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
		}
		for _, sub := range []func(flow.Kind, Deps) *cobra.Command{
			newListSub, newStartSub, newFinishSub, newCheckoutSub, newPublishSub, newTrackSub, newDeleteSub,
		} {
			cmd.AddCommand(sub(k, d))
		}
		if k == flow.Work {
			cmd.AddCommand(newDiffSub(k, d), newRebaseSub(k, d), newPullSub(k, d))
		}
		return cmd
	}
}

func topicShort(k flow.Kind) string {
	switch k {
	case flow.Work:
		return "Work branches: start from develop, merge back into develop"
	case flow.Release:
		return "Release branches: start from develop, merge into master (tagged) and back into develop"
	}
	return "Hotfix branches: start from master, merge into master (tagged) and back into develop"
}

func nameArg(k flow.Kind) string {
	if k == flow.Work {
		return "<name>"
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
		d.Printf("No %s branches exist.\n\nYou can start a new %s branch:\n\n    flowtree %s start %s\n\n", k, k, k, nameArg(k))
		return nil
	}
	d.Printf("%s", renderEntries(entries))
	return nil
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
	o := flow.StartOpts{Kind: k}
	var dry bool
	cmd := &cobra.Command{
		Use:   "start [-F] [--dry-run] " + nameArg(k) + " [<base>]",
		Short: fmt.Sprintf("Start a new %s branch", k),
		Long: fmt.Sprintf("Create a %s branch from <base> (default %s) and %s.",
			k, map[bool]string{true: "master", false: "develop"}[k == flow.Hotfix],
			map[bool]string{true: "switch the main worktree onto it", false: "give it its own worktree"}[k.LivesInMain()]),
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Name, o.Base = args[0], optArg(args, 1)
			return runPlan(cmd.Context(), d, dry, func(e *flow.Env) ([]flow.Step, error) { return flow.PlanStart(e, o) })
		},
	}
	cmd.Flags().BoolVarP(&o.Fetch, "fetch", "F", false, "fetch the remote first")
	cmd.Flags().BoolVar(&dry, "dry-run", false, "print the steps without running them")
	return cmd
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
		Use:               "checkout [" + nameArg(k) + "]",
		Short:             fmt.Sprintf("Print the directory of a %s branch, creating its worktree if needed", k),
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeTopics(k, d),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withFlow(cmd.Context(), d, func(e *flow.Env) error {
				name, err := chooseTopic(cmd.Context(), d, e, k, optArg(args, 0))
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
			o.Name = args[0]
			return runPlan(cmd.Context(), d, dry, func(e *flow.Env) ([]flow.Step, error) { return flow.PlanDelete(e, o) })
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
		Use:               "finish [flags] [" + nameArg(k) + "]",
		Short:             finishShort(k),
		Args:              cobra.MaximumNArgs(1),
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
				name, err := chooseTopic(cmd.Context(), d, e, k, o.Name)
				if err != nil {
					return nil, err
				}
				o.Name = name
				return flow.PlanFinish(e, o)
			})
		},
	}
	bindCommonFinish(cmd.Flags(), &o, &keep, &dry)
	if k == flow.Work {
		bindWorkFinish(cmd.Flags(), &o)
	} else {
		bindReleaseFinish(cmd.Flags(), &o, k)
	}
	return cmd
}

func finishShort(k flow.Kind) string {
	if k == flow.Work {
		return "Merge a work branch into develop and clean up"
	}
	return fmt.Sprintf("Merge a %s into master, tag it, merge the tag into develop and clean up", k)
}

func bindCommonFinish(f *pflag.FlagSet, o *flow.FinishOpts, keep, dry *bool) {
	f.BoolVarP(&o.Fetch, "fetch", "F", false, "fetch the remote first")
	f.BoolVarP(keep, "keep", "k", false, "keep the local and remote branch (the worktree is still removed)")
	f.BoolVar(&o.Cleanup.KeepLocal, "keeplocal", false, "keep the local branch")
	f.BoolVar(&o.Cleanup.KeepRemote, "keepremote", false, "keep the remote branch")
	f.BoolVarP(&o.Cleanup.Force, "force-delete", "D", false, "remove the worktree even when it has untracked files")
	f.BoolVarP(&o.Squash, "squash", "S", false, "squash the branch into one commit")
	f.BoolVar(dry, "dry-run", false, "print the steps without running them")
}

func bindWorkFinish(f *pflag.FlagSet, o *flow.FinishOpts) {
	f.BoolVarP(&o.Rebase, "rebase", "r", false, "rebase onto develop before merging")
	f.BoolVarP(&o.Preserve, "preserve-merges", "p", false, "keep merges while rebasing")
	f.BoolVar(&o.NoFF, "no-ff", false, "always create a merge commit")
	f.BoolVar(&o.Push, "push", false, "push develop afterwards")
}

func bindReleaseFinish(f *pflag.FlagSet, o *flow.FinishOpts, k flow.Kind) {
	f.BoolVarP(&o.Push, "push", "p", false, "push develop, master and the tag afterwards")
	f.BoolVarP(&o.Sign, "sign", "s", false, "sign the tag")
	f.StringVarP(&o.SignKey, "signingkey", "u", "", "sign the tag with this key")
	f.StringVarP(&o.Message, "message", "m", "", "tag message")
	f.StringVarP(&o.MessageFile, "messagefile", "f", "", "read the tag message from a file")
	f.BoolVarP(&o.NoTag, "notag", "n", false, "do not tag")
	f.BoolVarP(&o.NoBackmerge, "nobackmerge", "b", false, "do not merge back into develop")
	f.StringVarP(&o.TagName, "tagname", "T", "", "use this tag name instead of <version tag prefix><version>")
	if k == flow.Release {
		f.BoolVar(&o.FFMaster, "ff-master", false, "fast-forward master instead of creating a merge commit")
	}
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
