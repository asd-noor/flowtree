package flow

import (
	"fmt"

	"flowtree/internal/board"
)

// Attached runs git in dir with the user's terminal attached; interactive rebases need it.
type Attached func(dir string, args ...string) error

// RebaseOpts describe `work rebase`.
type RebaseOpts struct {
	Name        string
	Interactive bool
	Preserve    bool
	Attach      Attached // required when Interactive
}

func (o RebaseOpts) args(onto string) []string {
	args := []string{"rebase"}
	if o.Interactive {
		args = append(args, "-i")
	}
	if o.Preserve {
		args = append(args, "--rebase-merges")
	}
	return append(args, onto)
}

// PlanRebase returns the steps that rebase a regular branch onto develop inside its worktree.
func PlanRebase(e *Env, o RebaseOpts) ([]Step, error) {
	if o.Interactive && o.Attach == nil {
		return nil, fmt.Errorf("an interactive rebase needs a terminal")
	}
	branch, err := ResolveTopic(e, Regular, o.Name)
	if err != nil {
		return nil, err
	}
	var dir, before string
	return Plan(
		One("find or create the worktree of "+branch, func(e *Env) (err error) {
			dir, err = EnsureWorktree(e, branch)
			return err
		}),
		One(fmt.Sprintf("rebase %s onto %s", branch, e.S.Develop), func(e *Env) error {
			e.Say("Will try to rebase '%s'...", e.S.Name(Regular, branch))
			before = tipOf(e, branch)
			return runRebase(e, o, dir)
		}),
		boardStep("keep the review mark of "+branch+" across the rebase", func(e *Env, b board.Board) (board.Board, error) {
			return carriedReview(b, branch, before, tipOf(e, branch)), nil
		}),
	), nil
}

func runRebase(e *Env, o RebaseOpts, dir string) error {
	args := o.args(e.S.Develop)
	if o.Interactive {
		return o.Attach(dir, args...)
	}
	out, err := e.Git(dir)(args...)
	if out != "" {
		e.Say("%s", out)
	}
	return err
}

// PlanPull returns the steps that fast-forward a regular branch from the remote.
func PlanPull(e *Env, name string) ([]Step, error) {
	branch, err := ResolveTopic(e, Regular, name)
	if err != nil {
		return nil, err
	}
	remote := e.S.Remote
	return Plan(
		One("fetch "+remote, func(e *Env) error {
			_, err := e.MainGit()("fetch", "-q", remote)
			return err
		}),
		One(fmt.Sprintf("update %s from %s", branch, remote), func(e *Env) error { return fastForward(e, remote, branch) }),
	), nil
}

func fastForward(e *Env, remote, branch string) error {
	dir, err := EnsureWorktree(e, branch)
	if err != nil {
		return err
	}
	out, err := e.Git(dir)("pull", "--ff-only", remote, branch)
	if out != "" {
		e.Say("%s", out)
	}
	return err
}
