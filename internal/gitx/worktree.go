package gitx

import (
	"strings"
)

// Worktree is one entry of `git worktree list --porcelain`.
type Worktree struct {
	Path   string
	Head   string
	Branch string // short name; empty when detached
	Main   bool
	Bare   bool
}

// ParseWorktrees parses porcelain output; the first entry is the main worktree.
func ParseWorktrees(out string) []Worktree {
	blocks := strings.Split(strings.TrimSpace(out), "\n\n")
	wts := make([]Worktree, 0, len(blocks))
	for _, b := range blocks {
		if strings.TrimSpace(b) == "" {
			continue
		}
		wts = append(wts, parseBlock(b))
	}
	if len(wts) > 0 {
		wts[0].Main = true
	}
	return wts
}

func parseBlock(block string) Worktree {
	var wt Worktree
	for _, line := range strings.Split(block, "\n") {
		key, val, _ := strings.Cut(strings.TrimSpace(line), " ")
		switch key {
		case "worktree":
			wt.Path = val
		case "HEAD":
			wt.Head = val
		case "branch":
			wt.Branch = strings.TrimPrefix(val, "refs/heads/")
		case "bare":
			wt.Bare = true
		}
	}
	return wt
}

// Worktrees lists the repository's worktrees.
func Worktrees(g Git) ([]Worktree, error) {
	out, err := g("worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return ParseWorktrees(out), nil
}

// MainWorktree returns the main worktree.
func MainWorktree(g Git) (Worktree, error) {
	wts, err := Worktrees(g)
	if err != nil || len(wts) == 0 {
		return Worktree{}, err
	}
	return wts[0], nil
}

// WorktreeOf returns the worktree that has branch checked out.
func WorktreeOf(wts []Worktree, branch string) (Worktree, bool) {
	for _, w := range wts {
		if w.Branch == branch {
			return w, true
		}
	}
	return Worktree{}, false
}

// AddOpts describes `git worktree add`.
type AddOpts struct {
	Path      string
	NewBranch string // -b
	Track     bool   // --track, for a remote Commitish
	Commitish string
}

// Args renders the git arguments.
func (o AddOpts) Args() []string {
	args := []string{"worktree", "add"}
	if o.Track {
		args = append(args, "--track")
	}
	if o.NewBranch != "" {
		args = append(args, "-b", o.NewBranch)
	}
	args = append(args, o.Path)
	if o.Commitish != "" {
		args = append(args, o.Commitish)
	}
	return args
}

// AddWorktree creates a worktree.
func AddWorktree(g Git, o AddOpts) error {
	_, err := g(o.Args()...)
	return err
}

// RemoveWorktree removes a worktree.
func RemoveWorktree(g Git, path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	_, err := g(append(args, path)...)
	return err
}

// Toplevel is the root of the worktree containing g's directory.
func Toplevel(g Git) (string, error) { return g("rev-parse", "--show-toplevel") }
