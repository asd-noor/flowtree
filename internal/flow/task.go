package flow

import (
	"errors"
	"fmt"

	"flowtree/internal/board"
	"flowtree/internal/gitx"
)

// ErrNotInTask is what resolving a task without a number wraps outside a task's worktree.
var ErrNotInTask = errors.New("not in a task's worktree; give the task number")

// Task is a task that has an open regular or hotfix branch, with the kind of that branch.
type Task struct {
	Card   board.Card
	Branch string
	Kind   Kind
}

// Name is the branch's name without its prefix, as the topic functions take it.
func (t Task) Name(e *Env) string { return e.S.Name(t.Kind, t.Branch) }

// KindOfCard is the branch kind of a card, from its effective type.
func KindOfCard(s Settings, c board.Card) Kind {
	if TypeOf(s, c) == board.Hotfix {
		return Hotfix
	}
	return Regular
}

// KindOfTask is the branch kind task id starts, so a missing task is reported before anything else.
func KindOfTask(e *Env, id int) (Kind, error) {
	b, err := loadBoard(e)
	if err != nil {
		return Regular, err
	}
	c, ok := b.Card(id)
	if !ok {
		return Regular, fmt.Errorf("no task #%d", id)
	}
	return KindOfCard(e.S, c), nil
}

// ResolveTask finds the task id and its open branch. With id 0 it is the task of the regular or
// hotfix branch of the working directory's worktree, or of the parent of a work worktree.
func ResolveTask(e *Env, id int) (Task, error) {
	b, err := loadBoard(e)
	if err != nil {
		return Task{}, err
	}
	c, err := taskCard(e, b, id)
	if err != nil {
		return Task{}, err
	}
	switch {
	case c.Branch == "":
		return Task{}, fmt.Errorf("task #%d has no branch yet; start it first", c.ID)
	case !gitx.BranchExists(e.MainGit(), c.Branch):
		return Task{}, fmt.Errorf("the branch '%s' of task #%d does not exist", c.Branch, c.ID)
	}
	return Task{Card: c, Branch: c.Branch, Kind: KindOfCard(e.S, c)}, nil
}

func taskCard(e *Env, b board.Board, id int) (board.Card, error) {
	if id > 0 {
		c, ok := b.Card(id)
		if !ok {
			return c, fmt.Errorf("no task #%d", id)
		}
		return c, nil
	}
	branch, ok := CurrentParent(e)
	if !ok {
		return board.Card{}, ErrNotInTask
	}
	c, ok := b.ByBranch(branch)
	if !ok {
		return c, noTaskFor(branch)
	}
	return c, nil
}

func noTaskFor(branch string) error {
	return fmt.Errorf("no task for '%s'; run `flowtree task sync`", branch)
}

// TaskDiff returns the changes the task's branch made since it left its base.
func TaskDiff(e *Env, t Task) (string, error) {
	g := e.MainGit()
	base, err := g("merge-base", e.S.Target(t.Branch), t.Branch)
	if err != nil {
		return "", err
	}
	return g("diff", base+".."+t.Branch)
}

// PlanTaskTrack checks out a branch that exists on the remote and links it to task id, which must
// exist, have no branch and be of the branch's type. Nothing is created before those checks pass.
func PlanTaskTrack(e *Env, id int, branch string) ([]Step, error) {
	k, ok := e.S.KindOf(branch)
	if !ok || (k != Regular && k != Hotfix) {
		return nil, fmt.Errorf("'%s' is not a regular or hotfix branch (expected %s... or %s...)", branch, e.S.RegularPrefix, e.S.HotfixPrefix)
	}
	if err := checkTask(e, id, k); err != nil {
		return nil, err
	}
	track, err := PlanTrack(e, k, e.S.Name(k, branch))
	if err != nil {
		return nil, err
	}
	return Plan(track, linkStep(id, branch)), nil
}

// finishHint is the command that resumes finishing the branch of kind k named ver.
func finishHint(e *Env, k Kind, ver string) string {
	if k == Release {
		return "flowtree release finish " + ver
	}
	if b, err := loadBoard(e); err == nil {
		if c, ok := b.ByBranch(e.S.Branch(k, ver)); ok {
			return fmt.Sprintf("flowtree task finish %d", c.ID)
		}
	}
	return "flowtree task finish <n>"
}
