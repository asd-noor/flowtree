// Package worktree resolves user-facing worktree names ("@", a branch, a directory name).
package worktree

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"flowtree/internal/gitx"
)

// Main is the name for the main worktree.
const Main = "@"

// Display is the name shown in listings and completions:
// "<repo>@<branch>(root worktree)" for main, the branch when the directory
// name or path suffix says the same, else "<dir>@<branch>".
func Display(w gitx.Worktree) string {
	switch {
	case w.Main:
		return fmt.Sprintf("%s@%s(root worktree)", filepath.Base(w.Path), orHead(w))
	case w.Branch == "":
		return filepath.Base(w.Path)
	case strings.HasSuffix(filepath.ToSlash(w.Path), "/"+w.Branch), filepath.Base(w.Path) == w.Branch:
		return w.Branch
	default:
		return filepath.Base(w.Path) + "@" + w.Branch
	}
}

func orHead(w gitx.Worktree) string {
	if w.Branch != "" {
		return w.Branch
	}
	return "HEAD"
}

// Names are all the strings that identify a worktree, most specific first.
func Names(w gitx.Worktree) []string {
	names := []string{Display(w), w.Path}
	if w.Branch != "" {
		names = append(names, w.Branch)
	}
	if w.Main {
		return append(names, Main)
	}
	return append(names, filepath.Base(w.Path))
}

// Resolve finds the worktree a query refers to. "" and "@" mean the main worktree.
func Resolve(wts []gitx.Worktree, query string) (gitx.Worktree, error) {
	if len(wts) == 0 {
		return gitx.Worktree{}, fmt.Errorf("no worktrees found")
	}
	if query == "" || query == Main {
		return wts[0], nil
	}
	for _, step := range []func(gitx.Worktree) []string{Display1, branchOf, dirOf} {
		if hits := filterWT(wts, func(w gitx.Worktree) bool { return slices.Contains(step(w), query) }); len(hits) == 1 {
			return hits[0], nil
		} else if len(hits) > 1 {
			return gitx.Worktree{}, ambiguous(query, hits)
		}
	}
	return gitx.Worktree{}, fmt.Errorf("worktree '%s' not found; run 'flowtree list' to see them", query)
}

// Display1 matches the display name or the absolute path.
func Display1(w gitx.Worktree) []string { return []string{Display(w), w.Path} }

func branchOf(w gitx.Worktree) []string {
	if w.Branch == "" {
		return nil
	}
	return []string{w.Branch}
}

func dirOf(w gitx.Worktree) []string { return []string{filepath.Base(w.Path)} }

func ambiguous(query string, hits []gitx.Worktree) error {
	paths := make([]string, len(hits))
	for i, h := range hits {
		paths[i] = h.Path
	}
	return fmt.Errorf("'%s' matches several worktrees: %s", query, strings.Join(paths, ", "))
}

func filterWT(wts []gitx.Worktree, keep func(gitx.Worktree) bool) []gitx.Worktree {
	return slices.DeleteFunc(slices.Clone(wts), func(w gitx.Worktree) bool { return !keep(w) })
}
