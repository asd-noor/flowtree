package commands

import (
	"strings"

	"github.com/spf13/cobra"

	"flowtree/internal/gitx"
	"flowtree/internal/worktree"
)

// Completer is cobra's ValidArgsFunction.
type Completer = func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective)

// completion builds a Completer from a name source. Errors yield no suggestions: a completion
// must never print an error into the user's prompt.
func completion(d Deps, names func(Repo) []string) Completer {
	return func(cmd *cobra.Command, _ []string, prefix string) ([]string, cobra.ShellCompDirective) {
		repo, err := d.OpenRepo(cmd.Context())
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return withPrefix(names(repo), prefix), cobra.ShellCompDirectiveNoFileComp
	}
}

// completeWorktrees suggests "@" and the display name of every worktree.
func completeWorktrees(d Deps) Completer {
	return completion(d, func(r Repo) []string {
		out := []string{worktree.Main}
		for _, w := range r.Worktrees {
			if !w.Main {
				out = append(out, worktree.Display(w))
			}
		}
		return out
	})
}

// completeBranches suggests local branches plus remote-only ones (as plain branch names).
func completeBranches(d Deps) Completer {
	return completion(d, func(r Repo) []string {
		local, _ := gitx.LocalBranches(r.Git, "")
		remote, _ := r.Git.Lines("for-each-ref", "--format=%(refname:short)", "refs/remotes")
		seen := map[string]bool{}
		var out []string
		for _, b := range local {
			seen[b] = true
			out = append(out, b)
		}
		for _, ref := range remote {
			_, b, ok := strings.Cut(ref, "/")
			if ok && b != "HEAD" && !seen[b] {
				seen[b] = true
				out = append(out, b)
			}
		}
		return out
	})
}

func withPrefix(names []string, prefix string) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		if strings.HasPrefix(n, prefix) {
			out = append(out, n)
		}
	}
	return out
}
