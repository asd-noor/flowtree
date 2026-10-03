package flow

import (
	"errors"
	"fmt"

	"flowtree/internal/gitx"
)

// Attached runs git in dir with the user's terminal attached; interactive rebases need it.
type Attached func(dir string, args ...string) error

// Diff returns the changes a work branch made since it left develop.
func Diff(e *Env, name string) (string, error) {
	branch, err := ResolveTopic(e, Work, name)
	if err != nil {
		return "", err
	}
	g := e.MainGit()
	base, err := g("merge-base", e.S.Develop, branch)
	if err != nil {
		return "", err
	}
	return g("diff", base+".."+branch)
}

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

// PlanRebase returns the steps that rebase a work branch onto develop inside its worktree.
func PlanRebase(e *Env, o RebaseOpts) ([]Step, error) {
	if o.Interactive && o.Attach == nil {
		return nil, fmt.Errorf("an interactive rebase needs a terminal")
	}
	branch, err := ResolveTopic(e, Work, o.Name)
	if err != nil {
		return nil, err
	}
	var dir string
	return Plan(
		One("find or create the worktree of "+branch, func(e *Env) (err error) {
			dir, err = EnsureWorktree(e, branch)
			return err
		}),
		One(fmt.Sprintf("rebase %s onto %s", branch, e.S.Develop), func(e *Env) error {
			e.Say("Will try to rebase '%s'...", e.S.Name(Work, branch))
			return runRebase(e, o, dir)
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

// PlanPull returns the steps that fast-forward a work branch from a remote, or create it
// (tracking the remote branch, with a worktree) when it does not exist locally.
func PlanPull(e *Env, remote, name string) ([]Step, error) {
	if remote == "" {
		return nil, fmt.Errorf("missing argument <remote>")
	}
	branch, err := pullBranch(e, name)
	if err != nil {
		return nil, err
	}
	return Plan(
		One("fetch "+remote, func(e *Env) error {
			_, err := e.MainGit()("fetch", "-q", remote)
			return err
		}),
		One(fmt.Sprintf("update %s from %s", branch, remote), func(e *Env) error { return pullInto(e, remote, branch) }),
	), nil
}

// pullBranch is the existing branch the name resolves to, else the full name as typed (it may exist
// only on the remote). Only "no such local branch" falls back; ambiguity and git failures are reported.
func pullBranch(e *Env, name string) (string, error) {
	branch, err := ResolveTopic(e, Work, name)
	switch {
	case err == nil:
		return branch, nil
	case name != "" && errors.Is(err, ErrNoSuchBranch):
		return e.S.Branch(Work, name), nil
	}
	return "", err
}

func pullInto(e *Env, remote, branch string) error {
	g := e.MainGit()
	if gitx.BranchExists(g, branch) {
		return fastForward(e, remote, branch)
	}
	if !g.Ok("show-ref", "--verify", "--quiet", "refs/remotes/"+remote+"/"+branch) {
		return fmt.Errorf("branch '%s' not found on '%s'", branch, remote)
	}
	if _, err := g("branch", "--track", branch, remote+"/"+branch); err != nil {
		return err
	}
	dir, err := EnsureWorktree(e, branch)
	if err == nil {
		e.Say("Created branch '%s' tracking '%s/%s' in '%s'", branch, remote, branch, dir)
		e.cd(dir)
	}
	return err
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
