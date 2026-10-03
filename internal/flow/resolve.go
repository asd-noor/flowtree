package flow

import (
	"fmt"
	"slices"
	"strings"

	"flowtree/internal/gitx"
)

// ResolveTopic turns what the user typed (or nothing) into a full branch name of kind k.
// A name may omit the prefix; work names may also be a unique prefix of the name. With no name,
// the branch of the worktree the command runs in is used.
func ResolveTopic(e *Env, k Kind, name string) (string, error) {
	if name == "" {
		return currentTopic(e, k)
	}
	branch := e.S.Branch(k, name)
	g := e.MainGit()
	if gitx.BranchExists(g, branch) {
		return branch, nil
	}
	if k == Work {
		return uniquePrefixMatch(g, branch)
	}
	return "", fmt.Errorf("branch '%s' does not exist and is required", branch)
}

func uniquePrefixMatch(g gitx.Git, branch string) (string, error) {
	all, err := gitx.LocalBranches(g, "")
	if err != nil {
		return "", err
	}
	matches := slices.DeleteFunc(all, func(b string) bool { return !strings.HasPrefix(b, branch) })
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("branch '%s' does not exist and is required", branch)
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
