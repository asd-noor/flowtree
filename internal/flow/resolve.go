package flow

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"flowtree/internal/gitx"
)

// ErrNoSuchBranch is what a topic name that matches no local branch wraps; callers that may fall back
// to a remote branch test for it with errors.Is, so other failures (ambiguity, git errors) are not hidden.
var ErrNoSuchBranch = errors.New("no such branch")

// noSuchBranchError keeps the user-facing text while matching ErrNoSuchBranch.
type noSuchBranchError struct{ branch string }

func (e noSuchBranchError) Error() string {
	return fmt.Sprintf("branch '%s' does not exist and is required", e.branch)
}

func (e noSuchBranchError) Is(target error) bool { return target == ErrNoSuchBranch }

// ResolveTopic turns what the user typed (or nothing) into a full branch name of kind k.
// A name may omit the prefix; feature names may also be a unique prefix of the name, and work names
// may be the bare subtask name (see resolveWork). With no name, the branch of the worktree the command
// runs in is used.
func ResolveTopic(e *Env, k Kind, name string) (string, error) {
	if name == "" {
		return currentTopic(e, k)
	}
	branch := e.S.Branch(k, name)
	g := e.MainGit()
	if gitx.BranchExists(g, branch) {
		return branch, nil
	}
	switch k {
	case Feature:
		return uniquePrefixMatch(g, branch)
	case Work:
		return resolveWork(e, name)
	}
	return "", noSuchBranchError{branch}
}

// resolveWork finds a work branch by its bare name ("api"): the subtask of the parent the working
// directory belongs to, else the only work branch of that name.
func resolveWork(e *Env, name string) (string, error) {
	if parent, ok := CurrentParent(e); ok && gitx.BranchExists(e.MainGit(), e.S.WorkBranch(parent, name)) {
		return e.S.WorkBranch(parent, name), nil
	}
	all, err := gitx.LocalBranches(e.MainGit(), e.S.WorkPrefix)
	if err != nil {
		return "", err
	}
	matches := slices.DeleteFunc(all, func(b string) bool { return !strings.HasSuffix(b, "/"+name) })
	switch len(matches) {
	case 0:
		return "", noSuchBranchError{e.S.Branch(Work, name)}
	case 1:
		return matches[0], nil
	}
	return "", fmt.Errorf("multiple work branches are named '%s'; give the parent too:\n  %s", name, strings.Join(matches, "\n  "))
}

// CurrentParent is the feature or hotfix the working directory belongs to: the branch of its worktree,
// or the parent of the work branch checked out there.
func CurrentParent(e *Env) (string, bool) {
	b := currentBranch(e)
	if k, ok := e.S.KindOf(b); ok && k == Work {
		b = e.S.ParentOf(b)
	}
	return b, e.S.IsParent(b)
}

// Children lists the open work branches of a feature or hotfix.
func Children(e *Env, parent string) ([]string, error) {
	return gitx.LocalBranches(e.MainGit(), e.S.WorkPrefix+parent+"/")
}

// checkNoChildren refuses to finish or delete a parent while work branches of it are open.
func checkNoChildren(e *Env, k Kind, branch string) error {
	if k != Feature && k != Hotfix {
		return nil
	}
	kids, err := Children(e, branch)
	if err != nil || len(kids) == 0 {
		return err
	}
	return fmt.Errorf("'%s' still has open work branches; finish or delete them first:\n  %s", branch, strings.Join(kids, "\n  "))
}

func uniquePrefixMatch(g gitx.Git, branch string) (string, error) {
	all, err := gitx.LocalBranches(g, "")
	if err != nil {
		return "", err
	}
	matches := slices.DeleteFunc(all, func(b string) bool { return !strings.HasPrefix(b, branch) })
	switch len(matches) {
	case 0:
		return "", noSuchBranchError{branch}
	case 1:
		return matches[0], nil
	}
	return "", fmt.Errorf("multiple branches match prefix '%s':\n  %s", branch, strings.Join(matches, "\n  "))
}

// currentTopic is the branch checked out in the worktree containing the working directory.
func currentTopic(e *Env, k Kind) (string, error) {
	wts, err := e.Worktrees()
	if err != nil {
		return "", err
	}
	if w, ok := Containing(wts, e.Cwd); ok && strings.HasPrefix(w.Branch, e.S.Prefix(k)) {
		return w.Branch, nil
	}
	return "", fmt.Errorf("the current HEAD is no %s branch; please specify a <name> argument", k)
}

// Containing returns the deepest worktree whose directory contains path.
func Containing(wts []gitx.Worktree, path string) (gitx.Worktree, bool) {
	var best gitx.Worktree
	found := false
	for _, w := range wts {
		if within(w.Path, path) && (!found || len(w.Path) > len(best.Path)) {
			best, found = w, true
		}
	}
	return best, found
}

func within(dir, path string) bool {
	return path == dir || strings.HasPrefix(path, strings.TrimRight(dir, "/")+"/")
}

// Open lists the branches of kind k, without their prefix.
func Open(e *Env, k Kind) ([]string, error) {
	branches, err := gitx.LocalBranches(e.MainGit(), e.S.Prefix(k))
	if err != nil {
		return nil, err
	}
	names := make([]string, len(branches))
	for i, b := range branches {
		names[i] = e.S.Name(k, b)
	}
	return names, nil
}
