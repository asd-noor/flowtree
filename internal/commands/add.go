package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"flowtree/internal/gitx"
	"flowtree/internal/hooks"
)

type addFlags struct {
	newBranch string
	exec      string
	quiet     bool
}

// addPlan is everything `add` is about to do, decided before anything is touched.
type addPlan struct {
	branch string // branch the worktree ends up on; names the directory
	path   string
	git    gitx.AddOpts
}

func newAddCmd(d Deps) *cobra.Command {
	var f addFlags
	cmd := &cobra.Command{
		Use:   "add <branch> | -b <new-branch> [<commitish>]",
		Short: "Create a worktree for a branch",
		Long: `Create a worktree at <base dir>/<branch>. An existing branch is checked out;
one that exists only on a remote is tracked. With -b a new branch is created,
from the optional commitish.

Examples:
  flowtree add feature/auth                  existing (local or remote) branch
  flowtree add -b feature/new                new branch from HEAD
  flowtree add -b hotfix/urgent main         new branch from main
  flowtree add -b feature/x --exec "npm test"  run a command in the new worktree
  flowtree add -b feature/x --quiet          print only the new path`,
		Args:              cobra.MaximumNArgs(2),
		ValidArgsFunction: completeBranches(d),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(cmd.Context(), d, f, args)
		},
	}
	cmd.Flags().StringVarP(&f.newBranch, "branch", "b", "", "create a new branch")
	cmd.Flags().StringVar(&f.exec, "exec", "", "run a command in the new worktree, after hooks")
	cmd.Flags().BoolVarP(&f.quiet, "quiet", "q", false, "print only the worktree path on stdout")
	return cmd
}

func runAdd(ctx context.Context, d Deps, f addFlags, args []string) error {
	repo, err := d.OpenRepo(ctx)
	if err != nil {
		return err
	}
	plan, err := planAdd(repo, f, args)
	if err != nil {
		return err
	}
	if err := gitx.AddWorktree(repo.Git, plan.git); err != nil {
		return fmt.Errorf("could not create the worktree: %w", err)
	}
	status := statusWriter(d, f.quiet)
	runHooks(ctx, status, repo, plan)
	if err := runExec(ctx, d, status, f, plan); err != nil {
		return fmt.Errorf("worktree was created at '%s', but --exec command failed: %w", plan.path, err)
	}
	writeCdFile(d, plan.path)
	return report(d, f, plan)
}

// planAdd validates the request and works out the git invocation; it changes nothing.
func planAdd(repo Repo, f addFlags, args []string) (addPlan, error) {
	build := planExisting
	if f.newBranch != "" {
		build = planNew
	}
	plan, err := build(repo, f, args)
	if err != nil {
		return addPlan{}, err
	}
	plan.path = repo.Config.WorktreePath(repo.Main, plan.branch)
	plan.git.Path = plan.path
	return plan, checkTarget(repo, plan)
}

// planNew is `add -b <new> [<commitish>]`.
func planNew(repo Repo, f addFlags, args []string) (addPlan, error) {
	if len(args) > 1 {
		return addPlan{}, fmt.Errorf("too many arguments: with -b, give at most one commitish")
	}
	if gitx.BranchExists(repo.Git, f.newBranch) {
		return addPlan{}, fmt.Errorf("branch '%s' already exists\n  To check it out in a worktree: flowtree add %s", f.newBranch, f.newBranch)
	}
	// With -b, the positional argument is the start point, not the branch.
	opts := gitx.AddOpts{NewBranch: f.newBranch}
	if len(args) == 1 {
		opts.Commitish = args[0]
	}
	return addPlan{branch: f.newBranch, git: opts}, nil
}

// planExisting is `add <branch>`: a local branch, or one tracked from a single remote.
func planExisting(repo Repo, _ addFlags, args []string) (addPlan, error) {
	if len(args) != 1 {
		return addPlan{}, fmt.Errorf("expected one branch name (a second argument is only valid with -b)\n  Usage: flowtree add <branch> | flowtree add -b <new-branch> [<commitish>]")
	}
	name := args[0]
	ref, err := gitx.ResolveBranch(repo.Git, name)
	if err != nil {
		return addPlan{}, hintNotFound(err, name)
	}
	if !ref.Remote {
		return addPlan{branch: name, git: gitx.AddOpts{Commitish: name}}, nil
	}
	return addPlan{branch: name, git: gitx.AddOpts{Track: true, NewBranch: name, Commitish: ref.Name}}, nil
}

func hintNotFound(err error, name string) error {
	var be *gitx.BranchError
	if errors.As(err, &be) && len(be.Remotes) == 0 {
		return fmt.Errorf("%w\n  To create it: flowtree add -b %s [<commitish>]", err, name)
	}
	return err
}

// checkTarget refuses the cases git would fail on, with a hint instead of git's wording.
func checkTarget(repo Repo, plan addPlan) error {
	if w, ok := gitx.WorktreeOf(repo.Worktrees, plan.branch); ok {
		return fmt.Errorf("branch '%s' is already checked out at %s\n  To go there: cd %s", plan.branch, w.Path, w.Path)
	}
	if _, err := os.Lstat(plan.path); err == nil {
		return fmt.Errorf("path '%s' already exists\n  Remove it, or pick another branch name", plan.path)
	}
	return nil
}

func statusWriter(d Deps, quiet bool) io.Writer {
	if quiet {
		return d.Stderr
	}
	return d.Stdout
}

// runHooks reports a failing hook as a warning: the worktree exists, which is the point.
func runHooks(ctx context.Context, status io.Writer, repo Repo, plan addPlan) {
	if !repo.Config.HasHooks() {
		return
	}
	_, _ = fmt.Fprintln(status, "\nExecuting post-create hooks...")
	if err := hooks.RunPostCreate(ctx, status, repo.Config, repo.Main, plan.path); err != nil {
		_, _ = fmt.Fprintf(status, "Warning: Hook execution failed: %v\n", err)
		return
	}
	_, _ = fmt.Fprintln(status, "✓ All hooks executed successfully")
}

func runExec(ctx context.Context, d Deps, status io.Writer, f addFlags, plan addPlan) error {
	if strings.TrimSpace(f.exec) == "" {
		return nil
	}
	_, _ = fmt.Fprintf(status, "\nExecuting --exec command: %s\n", f.exec)
	cmd := exec.CommandContext(ctx, "sh", "-c", f.exec)
	cmd.Dir = plan.path
	cmd.Stdin, cmd.Stdout, cmd.Stderr = d.Stdin, status, status
	if err := cmd.Run(); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(status, "✓ --exec command completed")
	return nil
}

func report(d Deps, f addFlags, plan addPlan) error {
	if f.quiet {
		d.Printf("%s\n", plan.path)
		return nil
	}
	d.Printf("✅ Worktree created successfully!\n\n📁 Location: %s\n🌿 Branch: %s\n", plan.path, plan.branch)
	return nil
}
