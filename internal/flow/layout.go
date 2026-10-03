package flow

import (
	"fmt"
	"os"
	"strings"

	"flowtree/internal/gitx"
)

// Rev resolves ref to a commit id.
func Rev(g gitx.Git, ref string) (string, bool) {
	out, err := g("rev-parse", "--verify", "-q", ref+"^{commit}")
	return out, err == nil && out != ""
}

// HasRemote reports whether the configured remote exists.
func (e *Env) HasRemote() bool { return e.MainGit().Ok("remote", "get-url", e.S.Remote) }

// RequireClean fails when dir has uncommitted changes to tracked files.
func RequireClean(e *Env, dir string) error {
	out, err := e.Git(dir)("status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return err
	}
	if out != "" {
		return fmt.Errorf("working tree '%s' contains unstaged or uncommitted changes. Aborting", dir)
	}
	return nil
}

// Fetch updates the remote-tracking branches.
func Fetch(e *Env) error {
	if !e.HasRemote() {
		return fmt.Errorf("could not fetch: no remote named '%s'", e.S.Remote)
	}
	if _, err := e.MainGit()("fetch", "-q", e.S.Remote); err != nil {
		return fmt.Errorf("could not fetch from %s: %w", e.S.Remote, err)
	}
	return nil
}

// CheckSync refuses to go on when a long-lived branch is behind or has diverged from its remote.
func CheckSync(e *Env, branch string) error {
	g := e.MainGit()
	if !e.HasRemote() || !gitx.RemoteBranchExists(g, e.S.Remote, branch) {
		return nil
	}
	remote := e.S.Remote + "/" + branch
	l, _ := Rev(g, branch)
	r, _ := Rev(g, remote)
	switch {
	case l == r, g.Ok("merge-base", "--is-ancestor", r, l):
		return nil
	case g.Ok("merge-base", "--is-ancestor", l, r):
		return fmt.Errorf("branch '%s' is behind '%s'; update it first (e.g. 'git pull' in its worktree)", branch, remote)
	}
	return fmt.Errorf("branches '%s' and '%s' have diverged", branch, remote)
}

// SwitchMain runs `git checkout args...` in the main worktree, which must be clean.
func SwitchMain(e *Env, args ...string) error {
	if err := RequireClean(e, e.Main); err != nil {
		return err
	}
	if _, err := e.MainGit()(append([]string{"checkout", "-q"}, args...)...); err != nil {
		return fmt.Errorf("could not switch the main worktree (git checkout %s): %w", strings.Join(args, " "), err)
	}
	return nil
}

// CreateWorktree adds a worktree for branch under the base directory and runs the post-create hooks.
func CreateWorktree(e *Env, branch string, o gitx.AddOpts) (string, error) {
	o.Path = e.Config.WorktreePath(e.Main, branch)
	if _, err := os.Stat(o.Path); err == nil {
		return "", fmt.Errorf("cannot create a worktree for '%s': '%s' already exists", branch, o.Path)
	}
	if err := gitx.AddWorktree(e.MainGit(), o); err != nil {
		return "", err
	}
	e.runHooks(o.Path)
	return o.Path, nil
}

// worktreeForExisting adds a worktree for an existing local or remote-only branch.
func worktreeForExisting(e *Env, branch string) (string, error) {
	ref, err := gitx.ResolveBranch(e.MainGit(), branch)
	if err != nil {
		return "", err
	}
	if !ref.Remote {
		return CreateWorktree(e, branch, gitx.AddOpts{Commitish: branch})
	}
	return CreateWorktree(e, branch, gitx.AddOpts{NewBranch: branch, Track: true, Commitish: ref.Name})
}

// EvacuateMain moves the main worktree off a topic branch that was checked out there by hand.
func EvacuateMain(e *Env, branch string) error {
	to := e.S.Develop
	if k, ok := e.S.KindOf(branch); ok {
		to = e.S.EvacuateTarget(k)
	}
	if err := SwitchMain(e, to); err != nil {
		return err
	}
	e.Note("Note: '%s' was checked out in the main worktree; it is back on '%s' and '%s' gets its own worktree.", branch, to, branch)
	return nil
}

// EnsureWorktree returns the directory of a work or hotfix branch, creating the worktree if the
// branch has none. Such branches never live in the main worktree.
func EnsureWorktree(e *Env, branch string) (string, error) {
	w, ok, err := e.WorktreeOf(branch)
	if err != nil {
		return "", err
	}
	if ok && !w.Main {
		return w.Path, nil
	}
	if ok {
		if err := EvacuateMain(e, branch); err != nil {
			return "", err
		}
	}
	return worktreeForExisting(e, branch)
}

// TopicDir is where a branch of kind k lives: its worktree, or for releases the main worktree,
// which is switched onto the branch if it is not there already.
func TopicDir(e *Env, k Kind, branch string) (string, error) {
	if !k.LivesInMain() {
		return EnsureWorktree(e, branch)
	}
	w, ok, err := e.WorktreeOf(branch)
	if err != nil || ok {
		return w.Path, err
	}
	return e.Main, SwitchMain(e, branch)
}
