package flow

import (
	"path/filepath"

	"flowtree/internal/board"
)

// BoardPath is the board file of the repository, in the git directory every worktree shares.
func BoardPath(e *Env) (string, error) {
	dir, err := e.MainGit()("rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(e.Main, dir)
	}
	return board.Path(dir), nil
}

// Target is the branch a topic branch is finished into: develop for a feature, master for a hotfix,
// the parent for a work branch. Other branches have none.
func (s Settings) Target(branch string) string {
	k, ok := s.KindOf(branch)
	if !ok || k == Release {
		return ""
	}
	return s.BaseOf(k, branch)
}

// BranchFacts says whether branch exists and whether it is merged into its target. Once the branch is
// gone, the last known tip stands in for it. A tip equal to start has no commits of its own and so
// never counts as merged.
func BranchFacts(e *Env, branch, start, lastTip string) board.Facts {
	g := e.MainGit()
	tip, exists := Rev(g, "refs/heads/"+branch)
	known := lastTip
	if exists {
		known = tip
	}
	target := e.S.Target(branch)
	merged := known != "" && known != start && target != "" && g.Ok("merge-base", "--is-ancestor", known, target)
	return board.Facts{Exists: exists, Tip: tip, Merged: merged}
}

// CardStatus is the column of a card.
func CardStatus(e *Env, c board.Card) board.Status {
	if c.Branch == "" {
		return board.StatusOf(c, board.Facts{})
	}
	return board.StatusOf(c, BranchFacts(e, c.Branch, c.Start, c.Tip))
}

// SubtaskStatus is the column of a subtask.
func SubtaskStatus(e *Env, s board.Subtask) board.Status {
	return board.SubtaskStatus(BranchFacts(e, s.Branch, s.Start, s.Tip))
}
