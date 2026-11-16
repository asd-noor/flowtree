package board

import (
	"fmt"
	"slices"
	"strings"
)

// NormalizeNeeds sorts ids and drops duplicates; an empty list becomes nil so it is not stored.
func NormalizeNeeds(ids []int) []int {
	out := slices.Clone(ids)
	slices.Sort(out)
	out = slices.Compact(out)
	if len(out) == 0 {
		return nil
	}
	return out
}

// CheckNeeds refuses a needs list for task id that names an unknown task, the task itself, or closes a
// cycle (the error shows it, e.g. "#5 -> #6 -> #5"). id may be a task not on the board yet.
func (b Board) CheckNeeds(id int, needs []int) error {
	for _, n := range needs {
		if _, ok := b.Card(n); !ok {
			return fmt.Errorf("no task #%d to need", n)
		}
		if n == id {
			return fmt.Errorf("task #%d cannot need itself", id)
		}
	}
	next := b.withNeeds(id, needs)
	seen := map[int]bool{}
	for _, n := range needs {
		if path := next.pathTo(n, id, seen); path != nil {
			return fmt.Errorf("dependency cycle: %s", cycleText(append([]int{id}, path...)))
		}
	}
	return nil
}

// withNeeds is the board with id needing needs, adding a bare card when id is not on the board yet.
func (b Board) withNeeds(id int, needs []int) Board {
	if next, err := b.With(id, func(c Card) Card { c.Needs = needs; return c }); err == nil {
		return next
	}
	return Board{NextID: b.NextID, Cards: append(slices.Clone(b.Cards), Card{ID: id, Needs: needs})}
}

// pathTo is a path of needs from one task to target (both ends included), or nil. seen holds the tasks
// already searched from, which cannot reach target, so each task is walked once.
func (b Board) pathTo(from, target int, seen map[int]bool) []int {
	if from == target {
		return []int{from}
	}
	if seen[from] {
		return nil
	}
	seen[from] = true
	c, _ := b.Card(from)
	for _, n := range c.Needs {
		if p := b.pathTo(n, target, seen); p != nil {
			return append([]int{from}, p...)
		}
	}
	return nil
}

func cycleText(ids []int) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("#%d", id)
	}
	return strings.Join(parts, " -> ")
}

// SetNeeds replaces the needs of task id with the checked, normalized list.
func (b Board) SetNeeds(id int, needs []int) (Board, error) {
	needs = NormalizeNeeds(needs)
	if err := b.CheckNeeds(id, needs); err != nil {
		return b, err
	}
	return b.With(id, func(c Card) Card { c.Needs = needs; return c })
}

// NeededBy lists the ids of the tasks that need id; it is derived, never stored.
func (b Board) NeededBy(id int) []int {
	var out []int
	for _, c := range b.Cards {
		if slices.Contains(c.Needs, id) {
			out = append(out, c.ID)
		}
	}
	return out
}

// Unfinished lists the needs of c that are not done, by the statuses status gives.
func Unfinished(c Card, status func(id int) Status) []int {
	return slices.DeleteFunc(slices.Clone(c.Needs), func(id int) bool { return status(id) == Done })
}

// Blocked says whether c is still open and a need of it is not done. A need that was abandoned still
// blocks; a card that is itself done or abandoned never is.
func Blocked(c Card, status func(id int) Status) bool {
	if own := status(c.ID); own == Done || own == Abandoned {
		return false
	}
	return len(Unfinished(c, status)) > 0
}

// Ready says whether c, whose own column is own, is in todo with every need done.
func Ready(c Card, own Status, status func(id int) Status) bool {
	return own == Todo && !Blocked(c, status)
}
