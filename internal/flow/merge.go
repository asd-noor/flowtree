package flow

import (
	"fmt"
	"os"
	"path/filepath"

	"flowtree/internal/gitx"
)

// MergeMode is how a branch is merged.
type MergeMode int

// The merge modes.
const (
	FastForward MergeMode = iota // fast-forward when possible
	NoFF                         // always a merge commit
	Squash
)

// RunOn runs fn in a worktree of branch: the one that has it checked out (which must be clean),
// else a throwaway one that is removed afterwards. The user's checkout is never touched.
func RunOn(e *Env, branch string, fn func(gitx.Git) error) (err error) {
	w, ok, err := e.WorktreeOf(branch)
	if err != nil {
		return err
	}
	if ok {
		if err := RequireClean(e, w.Path); err != nil {
			return err
		}
		return fn(e.Git(w.Path))
	}
	return withThrowaway(e, branch, fn)
}

func withThrowaway(e *Env, branch string, fn func(gitx.Git) error) (err error) {
	tmp, err := os.MkdirTemp("", "flowtree-")
	if err != nil {
		return err
	}
	dir := filepath.Join(tmp, "wt")
	if _, err := e.MainGit()("worktree", "add", dir, branch); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	defer func() {
		_ = gitx.RemoveWorktree(e.MainGit(), dir, true)
		_ = os.RemoveAll(tmp)
	}()
	return fn(e.Git(dir))
}

// mergeIn merges source into the branch checked out in g; on failure it undoes everything.
func mergeIn(g gitx.Git, source string, mode MergeMode) error {
	switch mode {
	case Squash:
		return squashIn(g, source)
	case NoFF:
		return abortOnFail(g, "merge", "--no-ff", "--no-edit", source)
	}
	return abortOnFail(g, "merge", "--no-edit", source)
}

func abortOnFail(g gitx.Git, args ...string) error {
	if _, err := g(args...); err != nil {
		_ = g.Ok("merge", "--abort")
		return err
	}
	return nil
}

func squashIn(g gitx.Git, source string) error {
	_, err := g("merge", "--squash", source)
	if err == nil && !g.Ok("diff", "--cached", "--quiet") {
		_, err = g("commit", "-q", "--no-edit")
	}
	if err != nil {
		_ = g.Ok("reset", "-q", "--hard", "HEAD")
	}
	return err
}

// MergeInto merges source into target inside a worktree of target. A conflict aborts the merge
// and changes nothing. hint, when not empty, replaces the generic advice in the error.
func MergeInto(e *Env, target, source string, mode MergeMode, hint string) error {
	err := RunOn(e, target, func(g gitx.Git) error { return mergeIn(g, source, mode) })
	if err == nil {
		return nil
	}
	if hint == "" {
		hint = fmt.Sprintf("Fix it, then re-run the command: for conflicts run 'git merge %s' on the topic branch and resolve; "+
			"for untracked files move or delete them.", target)
	}
	return fmt.Errorf("could not merge '%s' into '%s': %w\nNothing was changed.\n%s", source, target, err, hint)
}
