package flow

import (
	"fmt"
	"path/filepath"
	"time"

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

// boardStep updates the board as part of a plan. A failure only warns: git has already changed by
// then, and the board must never make a flow command fail or roll back.
func boardStep(desc string, fn func(e *Env, b board.Board) (board.Board, error)) []Step {
	return One("board: "+desc, func(e *Env) error {
		if err := updateBoard(e, func(b board.Board) (board.Board, error) { return fn(e, b) }); err != nil {
			e.Warn("could not update the board: %v", err)
		}
		return nil
	})
}

func updateBoard(e *Env, fn func(board.Board) (board.Board, error)) error {
	path, err := BoardPath(e)
	if err != nil {
		return err
	}
	return board.Update(path, fn)
}

func tipOf(e *Env, branch string) string {
	tip, _ := Rev(e.MainGit(), "refs/heads/"+branch)
	return tip
}

// linkStep ties a new feature or hotfix branch to task id, or to a new task named after it.
func linkStep(task int, branch, title string) []Step {
	desc := "add a task for " + branch
	if task > 0 {
		desc = fmt.Sprintf("link task #%d to %s", task, branch)
	}
	return boardStep(desc, func(e *Env, b board.Board) (board.Board, error) {
		start := tipOf(e, branch)
		if task > 0 {
			return b.Link(task, branch, start)
		}
		return b.Ensure(branch, title, start, time.Now().UTC()), nil
	})
}

// subtaskStep records a new work branch on its parent's task, adding that task if the parent has none.
func subtaskStep(name, branch, parent string) []Step {
	return boardStep("add subtask "+name+" to the task of "+parent, func(e *Env, b board.Board) (board.Board, error) {
		start := tipOf(e, branch)
		pstart, _ := e.MainGit()("merge-base", parent, e.S.Target(parent))
		b = b.Ensure(parent, e.S.Name(kindOf(e.S, parent), parent), pstart, time.Now().UTC())
		return b.AddSubtask(parent, board.Subtask{Name: name, Branch: branch, Start: start, Tip: start}), nil
	})
}

// trackStep records branch's tip, so the board can still tell merged from abandoned once it is gone.
func trackStep(branch string) []Step {
	return boardStep("record the tip of "+branch, func(e *Env, b board.Board) (board.Board, error) {
		return b.Track(branch, tipOf(e, branch)), nil
	})
}

func kindOf(s Settings, branch string) Kind {
	k, _ := s.KindOf(branch)
	return k
}

// checkTask refuses -T for a task that does not exist or already has a branch, before anything is created.
func checkTask(e *Env, task int) error {
	if task == 0 {
		return nil
	}
	path, err := BoardPath(e)
	if err != nil {
		return err
	}
	b, err := board.Load(path)
	if err != nil {
		return err
	}
	c, ok := b.Card(task)
	switch {
	case !ok:
		return fmt.Errorf("no task #%d", task)
	case c.Branch != "":
		return fmt.Errorf("task #%d already belongs to '%s'", task, c.Branch)
	}
	return nil
}

// Review marks the task of a feature or hotfix as waiting for review at the branch's current tip, or
// with undo clears the mark. New commits move the task back to doing on their own.
func Review(e *Env, k Kind, name string, undo bool) (string, error) {
	branch, err := ResolveTopic(e, k, name)
	if err != nil {
		return "", err
	}
	tip := ""
	if !undo {
		tip = tipOf(e, branch)
	}
	return branch, updateBoard(e, func(b board.Board) (board.Board, error) {
		start, _ := e.MainGit()("merge-base", branch, e.S.Target(branch))
		b = b.Ensure(branch, e.S.Name(k, branch), start, time.Now().UTC())
		return b.Track(branch, tipOf(e, branch)).SetReview(branch, tip), nil
	})
}

// carriedReview keeps a review mark across a rebase that only moved the branch: when the mark was at
// the old tip, it moves to the new one. Any other mark is left alone, so real new work still shows.
func carriedReview(b board.Board, branch, oldTip, newTip string) board.Board {
	c, ok := b.ByBranch(branch)
	if !ok || c.ReviewTip == "" || c.ReviewTip != oldTip {
		return b.Track(branch, newTip)
	}
	return b.Track(branch, newTip).SetReview(branch, newTip)
}
