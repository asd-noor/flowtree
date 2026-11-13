package flow

import (
	"fmt"
	"strings"

	"flowtree/internal/board"
	"flowtree/internal/gitx"
)

func loadBoard(e *Env) (board.Board, error) {
	path, err := BoardPath(e)
	if err != nil {
		return board.Board{}, err
	}
	return board.Load(path)
}

// TaskBranch is the regular or hotfix branch of task id; the task must exist and its branch be open.
func TaskBranch(e *Env, id int) (string, error) {
	b, err := loadBoard(e)
	if err != nil {
		return "", err
	}
	c, ok := b.Card(id)
	switch {
	case !ok:
		return "", fmt.Errorf("no task #%d", id)
	case c.Branch == "":
		return "", fmt.Errorf("task #%d has no branch yet; start it first", id)
	case !gitx.BranchExists(e.MainGit(), c.Branch):
		return "", fmt.Errorf("the branch '%s' of task #%d does not exist", c.Branch, id)
	}
	return c.Branch, nil
}

// WorkRef is the work branch of subtask sub of task id; it must exist.
func WorkRef(e *Env, id int, sub string) (string, error) {
	if sub == "" {
		return "", fmt.Errorf("give the subtask too: <n>/<subtask>")
	}
	parent, err := TaskBranch(e, id)
	if err != nil {
		return "", err
	}
	branch := e.S.WorkBranch(parent, sub)
	if !gitx.BranchExists(e.MainGit(), branch) {
		return "", fmt.Errorf("task #%d has no work branch '%s'", id, branch)
	}
	return branch, nil
}

// checkParentHasTask refuses a parent branch that has no card, so a work branch is never created
// that the board cannot place.
func checkParentHasTask(e *Env, parent string) error {
	b, err := loadBoard(e)
	if err != nil {
		return err
	}
	if _, ok := b.ByBranch(parent); !ok {
		return fmt.Errorf("'%s' has no task; run `flowtree task sync` first", parent)
	}
	return nil
}

// ParentTasks lists the tasks whose regular or hotfix branch is open: what a work branch can go under.
func ParentTasks(e *Env) ([]board.Card, error) {
	b, err := loadBoard(e)
	if err != nil {
		return nil, err
	}
	var out []board.Card
	for _, c := range b.Cards {
		if c.Branch != "" && e.S.IsParent(c.Branch) && gitx.BranchExists(e.MainGit(), c.Branch) {
			out = append(out, c)
		}
	}
	return out, nil
}

// WorkRefs lists the "<n>/<subtask>" references of open work branches that start with prefix.
func WorkRefs(e *Env, prefix string) []string {
	b, err := loadBoard(e)
	if err != nil {
		return nil
	}
	var out []string
	for _, c := range b.Cards {
		for _, s := range c.Subtasks {
			if ref := fmt.Sprintf("%d/%s", c.ID, s.Name); strings.HasPrefix(ref, prefix) && gitx.BranchExists(e.MainGit(), s.Branch) {
				out = append(out, ref)
			}
		}
	}
	return out
}

// WorkArg is how commands name a work branch: "<n>/<subtask>", or the bare subtask name when the
// board has no task for its parent.
func WorkArg(e *Env, branch string) string {
	parent := e.S.ParentOf(branch)
	name := strings.TrimPrefix(branch, e.S.WorkPrefix+parent+"/")
	b, err := loadBoard(e)
	if err != nil {
		return name
	}
	c, ok := b.ByBranch(parent)
	if !ok {
		return name
	}
	return fmt.Sprintf("%d/%s", c.ID, name)
}
