package flow

import (
	"fmt"
	"strings"

	"flowtree/internal/gitx"
)

// CleanupOpts say what Cleanup keeps.
type CleanupOpts struct {
	KeepLocal  bool // keep the local branch
	KeepRemote bool // keep the branch on the remote
	Force      bool // remove the worktree even when it has untracked files
}

// Cleanup removes a topic branch's worktree, its local branch and its remote branch. A release
// checked out in the main worktree is not removed: the main worktree goes back to develop.
func Cleanup(e *Env, branch string, o CleanupOpts) error {
	removed, err := removeCheckout(e, branch, o.Force)
	if err != nil || !removed {
		return err
	}
	g := e.MainGit()
	if !o.KeepLocal {
		if _, err := g("branch", "-q", "-D", branch); err != nil {
			return err
		}
		e.Say("- Branch '%s' has been locally deleted", branch)
	}
	if !o.KeepRemote && e.HasRemote() && gitx.RemoteBranchExists(g, e.S.Remote, branch) {
		if _, err := g("push", "-q", e.S.Remote, "--delete", branch); err != nil {
			e.Warn("could not delete '%s' from %s: %v", branch, e.S.Remote, err)
		} else {
			e.Say("- Branch '%s' in remote '%s' has been deleted", branch, e.S.Remote)
		}
	}
	return nil
}

// removeCheckout frees the branch: removes its worktree or moves the main worktree back to
// develop. It reports false (with no error) when the worktree could not be removed, in which
// case the branch is kept.
func removeCheckout(e *Env, branch string, force bool) (bool, error) {
	w, ok, err := e.WorktreeOf(branch)
	if err != nil || !ok {
		return true, err
	}
	if w.Main {
		if _, err := e.MainGit()("checkout", "-q", e.S.Develop); err != nil {
			return false, fmt.Errorf("cannot leave '%s' in the main worktree; '%s' is busy: %w", branch, e.S.Develop, err)
		}
		e.Say("- The main worktree is back on '%s'", e.S.Develop)
		return true, nil
	}
	if err := gitx.RemoveWorktree(e.MainGit(), w.Path, force); err != nil {
		e.Warn("could not remove worktree '%s' (untracked files?). Branch '%s' kept.", w.Path, branch)
		e.Warn("Use -D to force removal.")
		return false, nil
	}
	e.Say("- Worktree '%s' has been removed", w.Path)
	if inside(e.Cwd, w.Path) {
		e.cd(e.Main)
	}
	return true, nil
}

func inside(path, dir string) bool { return path == dir || strings.HasPrefix(path, dir+"/") }
