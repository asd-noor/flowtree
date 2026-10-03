package flow

import (
	"fmt"

	"flowtree/internal/gitx"
)

// StartOpts describe `<kind> start`.
type StartOpts struct {
	Kind  Kind
	Name  string
	Base  string // defaults to the kind's base branch
	Fetch bool
}

func missingName(k Kind) error {
	if k == Work {
		return fmt.Errorf("missing argument <name>")
	}
	return fmt.Errorf("missing argument <version>")
}

// PlanStart checks what can be checked now and returns the steps that create the branch.
func PlanStart(e *Env, o StartOpts) ([]Step, error) {
	if o.Name == "" {
		return nil, missingName(o.Kind)
	}
	branch := e.S.Branch(o.Kind, o.Name)
	name := e.S.Name(o.Kind, branch)
	base := firstNonEmpty(o.Base, e.S.Base(o.Kind))
	if err := checkStartable(e, o.Kind, name, branch, base); err != nil {
		return nil, err
	}
	var path string
	return Plan(
		If(o.Fetch, One("fetch "+e.S.Remote, Fetch)),
		One("check "+e.S.Remote+" state", func(e *Env) error { return checkStartRemote(e, o.Kind, branch, base, o.Base == "") }),
		One(createDesc(o.Kind, branch, base), func(e *Env) (err error) {
			path, err = createTopic(e, o.Kind, branch, base)
			return err
		}),
		One("print summary", func(e *Env) error { printStartSummary(e, o.Kind, name, branch, base, path); return nil }),
	), nil
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
	if k != Work {
		if err := checkNoneOpen(e, k); err != nil {
			return err
		}
		if gitx.TagExists(g, e.S.VersionTag+name) {
			return fmt.Errorf("tag '%s' already exists; pick another name", e.S.VersionTag+name)
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

// checkNoneOpen enforces one open release (or hotfix) at a time.
func checkNoneOpen(e *Env, k Kind) error {
	open, err := Open(e, k)
	if err != nil || len(open) == 0 {
		return err
	}
	return fmt.Errorf("there is an existing %s branch '%s'; finish that one first", k, open[0])
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

func createDesc(k Kind, branch, base string) string {
	if k.LivesInMain() {
		return fmt.Sprintf("create branch %s from %s and switch the main worktree to it", branch, base)
	}
	return fmt.Sprintf("create branch %s from %s in a new worktree", branch, base)
}

func createTopic(e *Env, k Kind, branch, base string) (string, error) {
	var path string
	if k.LivesInMain() {
		if err := SwitchMain(e, "--no-track", "-b", branch, base); err != nil {
			return "", err
		}
		path = e.Main
	} else {
		var err error
		path, err = CreateWorktree(e, branch, gitx.AddOpts{NewBranch: branch, Commitish: base})
		if err != nil {
			return "", err
		}
	}
	e.cd(path)
	return path, nil
}

func printStartSummary(e *Env, k Kind, name, branch, base, path string) {
	where := fmt.Sprintf("- A worktree for it was created at '%s'", path)
	if k.LivesInMain() {
		where = fmt.Sprintf("- The main worktree (%s) is now on branch '%s'", path, branch)
	}
	e.Say("\nSummary of actions:\n- A new branch '%s' was created, based on '%s'\n%s\n", branch, base, where)
	e.Say("Now, start committing on your %s. When done, use:\n\n     flowtree %s finish %s\n", k, k, name)
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
			path, err = trackBranch(e, k, branch)
			return err
		}),
		One("print summary", func(e *Env) error {
			e.Say("\nSummary of actions:\n- A new remote tracking branch '%s' was created", branch)
			e.Say("- %s", whereIs(k, branch, path))
			return nil
		}),
	), nil
}

func trackBranch(e *Env, k Kind, branch string) (string, error) {
	path, err := checkoutTracked(e, k, branch)
	if err == nil {
		e.cd(path)
	}
	return path, err
}

func checkoutTracked(e *Env, k Kind, branch string) (string, error) {
	if !k.LivesInMain() {
		return worktreeForExisting(e, branch)
	}
	return e.Main, SwitchMain(e, "--track", e.S.Remote+"/"+branch)
}

func whereIs(k Kind, branch, path string) string {
	if k.LivesInMain() {
		return fmt.Sprintf("The main worktree (%s) is now on branch '%s'", path, branch)
	}
	return fmt.Sprintf("A worktree for it was created at '%s'", path)
}
