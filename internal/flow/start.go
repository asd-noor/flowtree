package flow

import (
	"errors"
	"fmt"
	"strings"

	"flowtree/internal/gitx"
)

// StartOpts describe `<kind> start`.
type StartOpts struct {
	Kind   Kind
	Name   string
	Base   string // defaults to the kind's base branch
	Parent int    // work only: the task whose branch the subtask belongs to (0: from the working directory)
	Task   int    // regular and hotfix only: the board task the branch is for (0: a new task)
	Desc   string // work only: the subtask's description on the board
	Fetch  bool
}

// ErrNoParent is returned by ResolveParent when no parent was given and none follows from the working
// directory; a caller on a terminal may then offer a choice.
var ErrNoParent = errors.New("no parent given (-P <task number>) and the working directory is not in a regular or hotfix worktree")

// ResolveParent is the parent a new work branch belongs to: the branch of task id, else the regular or
// hotfix the working directory belongs to, which must have a task.
func ResolveParent(e *Env, task int) (string, error) {
	if task > 0 {
		return TaskBranch(e, task)
	}
	p, ok := CurrentParent(e)
	if !ok {
		return "", ErrNoParent
	}
	return p, checkParentHasTask(e, p)
}

func missingName(k Kind) error {
	if k == Regular || k == Work {
		return fmt.Errorf("missing argument <name>")
	}
	return fmt.Errorf("missing argument <version>")
}

// PlanStart checks what can be checked now and returns the steps that create the branch.
func PlanStart(e *Env, o StartOpts) ([]Step, error) {
	if o.Name == "" {
		return nil, missingName(o.Kind)
	}
	if o.Kind == Work {
		return planStartWork(e, o)
	}
	branch := e.S.Branch(o.Kind, o.Name)
	name := e.S.Name(o.Kind, branch)
	base := firstNonEmpty(o.Base, e.S.Base(o.Kind))
	if err := checkStartable(e, o.Kind, name, branch, base); err != nil {
		return nil, err
	}
	if o.Task > 0 && o.Kind == Release {
		return nil, fmt.Errorf("releases have no task; -T is for regular and hotfix branches")
	}
	if err := checkTask(e, o.Task, o.Kind); err != nil {
		return nil, err
	}
	return Plan(
		If(o.Fetch, One("fetch "+e.S.Remote, Fetch)),
		One("check "+e.S.Remote+" state", func(e *Env) error { return checkStartRemote(e, o.Kind, branch, base, o.Base == "") }),
		createSteps(o.Kind, name, branch, base),
		If(o.Kind != Release, linkStep(o.Task, branch, name)),
	), nil
}

// planStartWork starts a local subtask off the tip of its parent; it never touches the remote.
func planStartWork(e *Env, o StartOpts) ([]Step, error) {
	if strings.Contains(o.Name, "/") {
		return nil, fmt.Errorf("a work name must not contain '/' (got '%s'); the task goes in -P", o.Name)
	}
	parent, err := ResolveParent(e, o.Parent)
	if err != nil {
		return nil, err
	}
	branch := e.S.WorkBranch(parent, o.Name)
	if err := checkStartable(e, Work, o.Name, branch, parent); err != nil {
		return nil, err
	}
	return Plan(createSteps(Work, o.Name, branch, parent), subtaskStep(o.Name, branch, parent, o.Desc)), nil
}

// createSteps create the branch and its worktree, then print a summary.
func createSteps(k Kind, name, branch, base string) []Step {
	var path string
	return Plan(
		One(createDesc(branch, base), func(e *Env) (err error) {
			path, err = createTopic(e, branch, base)
			return err
		}),
		One("print summary", func(e *Env) error { printStartSummary(e, k, name, branch, base, path); return nil }),
	)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func checkStartable(e *Env, k Kind, name, branch, base string) error {
	g := e.MainGit()
	if k == Release || k == Hotfix {
		if err := checkNoneOpen(e); err != nil {
			return err
		}
		if gitx.TagExists(g, name) {
			return fmt.Errorf("tag '%s' already exists; pick another name", name)
		}
	}
	if _, ok := Rev(g, base); !ok {
		return fmt.Errorf("base '%s' does not exist", base)
	}
	if gitx.BranchExists(g, branch) {
		return fmt.Errorf("branch '%s' already exists; pick another name", branch)
	}
	return nil
}

// checkNoneOpen enforces one open release or hotfix at a time: a hotfix fixes a shipped release,
// so the two never overlap.
func checkNoneOpen(e *Env) error {
	for _, k := range []Kind{Release, Hotfix} {
		open, err := Open(e, k)
		if err != nil {
			return err
		}
		if len(open) > 0 {
			return fmt.Errorf("there is an existing %s branch '%s'; finish that one first", k, open[0])
		}
	}
	return nil
}

func checkStartRemote(e *Env, k Kind, branch, base string, defaultBase bool) error {
	if e.HasRemote() && gitx.RemoteBranchExists(e.MainGit(), e.S.Remote, branch) {
		return fmt.Errorf("branch '%s' already exists on %s; use 'flowtree %s track %s'", branch, e.S.Remote, k, e.S.Name(k, branch))
	}
	if defaultBase {
		return CheckSync(e, base)
	}
	return nil
}

func createDesc(branch, base string) string {
	return fmt.Sprintf("create branch %s from %s in a new worktree", branch, base)
}

func createTopic(e *Env, branch, base string) (string, error) {
	path, err := CreateWorktree(e, branch, gitx.AddOpts{NewBranch: branch, Commitish: base})
	if err == nil {
		e.cd(path)
	}
	return path, err
}

func printStartSummary(e *Env, k Kind, name, branch, base, path string) {
	e.Say("\nSummary of actions:\n- A new branch '%s' was created, based on '%s'\n- %s\n", branch, base, whereIs(path))
	e.Say("Now, start committing on your %s. When done, use:\n\n     flowtree %s finish %s\n", k, k, finishArg(e, k, name, branch))
}

// PlanTrack returns the steps that check out a branch that exists on the remote.
func PlanTrack(e *Env, k Kind, name string) ([]Step, error) {
	if name == "" {
		return nil, fmt.Errorf("missing argument <name>")
	}
	branch := e.S.Branch(k, name)
	if gitx.BranchExists(e.MainGit(), branch) {
		return nil, fmt.Errorf("local branch '%s' already exists", branch)
	}
	var path string
	return Plan(
		One("fetch "+e.S.Remote, Fetch),
		One("check that "+branch+" exists on "+e.S.Remote, func(e *Env) error {
			if !gitx.RemoteBranchExists(e.MainGit(), e.S.Remote, branch) {
				return fmt.Errorf("could not fetch branch '%s' from remote '%s'", branch, e.S.Remote)
			}
			return nil
		}),
		One("check out "+branch, func(e *Env) (err error) {
			path, err = trackBranch(e, branch)
			return err
		}),
		One("print summary", func(e *Env) error {
			e.Say("\nSummary of actions:\n- A new remote tracking branch '%s' was created", branch)
			e.Say("- %s", whereIs(path))
			return nil
		}),
	), nil
}

func trackBranch(e *Env, branch string) (string, error) {
	path, err := worktreeForExisting(e, branch)
	if err == nil {
		e.cd(path)
	}
	return path, err
}

func finishArg(e *Env, k Kind, name, branch string) string {
	if k == Work {
		return WorkArg(e, branch)
	}
	return name
}

func whereIs(path string) string {
	return fmt.Sprintf("A worktree for it was created at '%s'", path)
}
