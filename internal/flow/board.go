package flow

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"flowtree/internal/board"
	"flowtree/internal/gitx"
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

// Target is the branch a topic branch is finished into: develop for a regular branch, master for a hotfix,
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

// TypeOf is the type of a card: the stored one, else read from its branch prefix (a hotfix branch is a
// hotfix, anything else, including no branch and old feature/ branches, is regular).
func TypeOf(s Settings, c board.Card) board.Type {
	switch {
	case c.Type != "":
		return c.Type
	case kindOf(s, c.Branch) == Hotfix:
		return board.Hotfix
	}
	return board.Regular
}

// typeOfKind is the task type of a branch kind.
func typeOfKind(k Kind) board.Type {
	if k == Hotfix {
		return board.Hotfix
	}
	return board.Regular
}

// linkStep ties a new regular or hotfix branch to task id.
func linkStep(task int, branch string) []Step {
	return boardStep(fmt.Sprintf("link task #%d to %s", task, branch), func(e *Env, b board.Board) (board.Board, error) {
		return b.Link(task, branch, tipOf(e, branch), typeOfKind(kindOf(e.S, branch)))
	})
}

// subtaskStep records a new work branch on its parent's task.
func subtaskStep(name, branch, parent, desc string) []Step {
	return boardStep("add subtask "+name+" to the task of "+parent, func(e *Env, b board.Board) (board.Board, error) {
		start := tipOf(e, branch)
		return b.AddSubtask(parent, board.Subtask{Name: name, Branch: branch, Description: desc, Start: start, Tip: start}), nil
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

// checkTask refuses a task that does not exist, already has a branch or is of the other type than
// the branch kind k, before anything is created.
func checkTask(e *Env, task int, k Kind) error {
	b, err := loadBoard(e)
	if err != nil {
		return err
	}
	c, ok := b.Card(task)
	switch {
	case !ok:
		return fmt.Errorf("no task #%d", task)
	case c.Branch != "":
		return fmt.Errorf("task #%d already belongs to '%s'", task, c.Branch)
	case TypeOf(e.S, c) != typeOfKind(k):
		return fmt.Errorf("task #%d is a %s task, not a %s", task, TypeOf(e.S, c), typeOfKind(k))
	}
	return nil
}

// Review marks the task of a regular or hotfix (which must have one) as waiting for review at the branch's current tip, or
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
		if _, ok := b.ByBranch(branch); !ok {
			return b, noTaskFor(branch)
		}
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

// syncRefs lists the regular, hotfix and work branches as Reconcile wants them.
func syncRefs(e *Env) ([]board.Ref, error) {
	var refs []board.Ref
	for _, k := range []Kind{Regular, Hotfix, Work} {
		branches, err := gitx.LocalBranches(e.MainGit(), e.S.Prefix(k))
		if err != nil {
			return nil, err
		}
		for _, br := range branches {
			refs = append(refs, refOf(e, k, br))
		}
	}
	return refs, nil
}

func refOf(e *Env, k Kind, branch string) board.Ref {
	start, _ := e.MainGit()("merge-base", branch, e.S.Target(branch))
	r := board.Ref{Branch: branch, Tip: tipOf(e, branch), Start: start, Title: e.S.Name(k, branch)}
	if k != Work {
		r.Type = typeOfKind(k)
	} else {
		r.Parent = e.S.ParentOf(branch)
		r.Title = strings.TrimPrefix(branch, e.S.WorkPrefix+r.Parent+"/")
	}
	return r
}

// Sync reconciles the board with the branches as they are, for changes made with plain git. With dry
// it only reports what it would change.
func Sync(e *Env, dry bool) ([]string, error) {
	refs, err := syncRefs(e)
	if err != nil {
		return nil, err
	}
	path, err := BoardPath(e)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if dry {
		b, err := board.Load(path)
		_, changes := board.Reconcile(b, refs, now)
		return changes, err
	}
	var changes []string
	err = board.Update(path, func(b board.Board) (board.Board, error) {
		b, changes = board.Reconcile(b, refs, now)
		return b, nil
	})
	return changes, err
}
