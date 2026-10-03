package flow

import (
	"fmt"
	"strings"

	"flowtree/internal/gitx"
)

// Entry is one branch in a `<kind> list`.
type Entry struct {
	Name    string
	Branch  string
	Current bool   // checked out in the worktree the command runs in
	State   string // only filled in when asked for
}

// List returns the branches of kind k; verbose adds how each relates to its base.
func List(e *Env, k Kind, verbose bool) ([]Entry, error) {
	branches, err := gitx.LocalBranches(e.MainGit(), e.S.Prefix(k))
	if err != nil {
		return nil, err
	}
	current := currentBranch(e)
	entries := make([]Entry, len(branches))
	for i, b := range branches {
		entries[i] = Entry{Name: e.S.Name(k, b), Branch: b, Current: b == current}
		if verbose {
			entries[i].State = stateOf(e.MainGit(), b, e.S.Base(k))
		}
	}
	return entries, nil
}

func currentBranch(e *Env) string {
	wts, err := e.Worktrees()
	if err != nil {
		return ""
	}
	w, _ := Containing(wts, e.Cwd)
	return w.Branch
}

// stateOf says how branch stands against base, as `git flow list -v` does.
func stateOf(g gitx.Git, branch, base string) string {
	tip, _ := Rev(g, branch)
	baseTip, _ := Rev(g, base)
	mb, _ := g("merge-base", base, branch)
	switch {
	case tip == baseTip:
		return "(no commits yet)"
	case mb == tip:
		return fmt.Sprintf("(is merged into %s, can be deleted)", base)
	case mb == baseTip:
		return fmt.Sprintf("(based on latest %s)", base)
	}
	return "(may be rebased)"
}

// Publish pushes a topic branch to the remote and sets its upstream.
func Publish(e *Env, k Kind, name string) error {
	if !e.HasRemote() {
		return fmt.Errorf("could not push: no remote named '%s'", e.S.Remote)
	}
	branch, err := ResolveTopic(e, k, name)
	if err != nil {
		return err
	}
	if err := Fetch(e); err != nil {
		return err
	}
	g := e.MainGit()
	if gitx.RemoteBranchExists(g, e.S.Remote, branch) {
		return fmt.Errorf("branch '%s' already exists on %s", branch, e.S.Remote)
	}
	if _, err := g("push", "-u", e.S.Remote, branch+":"+branch); err != nil {
		return fmt.Errorf("could not push branch '%s': %w", branch, err)
	}
	e.Say("\nSummary of actions:\n- The remote branch '%s' was created or updated\n- The local branch '%s' was configured to track the remote branch", branch, branch)
	return nil
}

// Checkout returns the directory of a topic branch, creating its worktree (or switching the main
// worktree to a release) when needed, and asks the shell to cd there.
func Checkout(e *Env, k Kind, name string) (string, error) {
	branch, err := ResolveTopic(e, k, name)
	if err != nil {
		return "", err
	}
	dir, err := TopicDir(e, k, branch)
	if err == nil {
		e.cd(dir)
	}
	return dir, err
}

// DeleteOpts describe `<kind> delete`.
type DeleteOpts struct {
	Kind   Kind
	Name   string
	Force  bool // delete even when unmerged; also force worktree removal
	Remote bool // also delete the branch on the remote
}

// PlanDelete returns the steps that remove a topic branch without merging it.
func PlanDelete(e *Env, o DeleteOpts) ([]Step, error) {
	if o.Name == "" {
		return nil, fmt.Errorf("missing argument <name>")
	}
	branch, err := ResolveTopic(e, o.Kind, o.Name)
	if err != nil {
		return nil, err
	}
	base := e.S.Base(o.Kind)
	if !o.Force && !e.MainGit().Ok("merge-base", "--is-ancestor", branch, base) {
		return nil, fmt.Errorf("branch '%s' has not been merged into '%s'; use -f to force the deletion", branch, base)
	}
	co := CleanupOpts{KeepRemote: !o.Remote, Force: o.Force}
	return One("remove worktree and branch "+branch, func(e *Env) error {
		e.Say("Summary of actions:")
		return Cleanup(e, branch, co)
	}), nil
}

// Names lists topic names of kind k that start with prefix, for completion.
func Names(e *Env, k Kind, prefix string) []string {
	open, _ := Open(e, k)
	return filterPrefix(open, prefix)
}

func filterPrefix(names []string, prefix string) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		if strings.HasPrefix(n, prefix) {
			out = append(out, n)
		}
	}
	return out
}
