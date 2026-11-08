package board

import (
	"fmt"
	"slices"
	"time"
)

// Ref is a branch as git has it now, for Reconcile.
type Ref struct {
	Branch string
	Tip    string
	Start  string // where it left its target, for a new task or subtask
	Title  string // the name a new task gets
	Parent string // set for a work branch: the branch whose task it is a subtask of
}

// Reconcile brings the board in line with branches made, renamed or moved with plain git: a task whose
// branch vanished is relinked to an untracked branch with the same tip (a rename), branches without a
// task or subtask get one, and every live branch's tip is recorded. It returns what changed.
func Reconcile(b Board, refs []Ref, now time.Time) (Board, []string) {
	var changes []string
	note := func(f string, a ...any) { changes = append(changes, fmt.Sprintf(f, a...)) }
	tops := slices.DeleteFunc(slices.Clone(refs), func(r Ref) bool { return r.Parent != "" })

	b = relinkRenamed(b, tops, note)
	for _, r := range tops {
		if _, ok := b.ByBranch(r.Branch); !ok {
			var c Card
			b, c = b.Add(Card{Title: r.Title, Branch: r.Branch, Start: r.Start, Tip: r.Tip, Created: now})
			note("added task #%d for %s", c.ID, r.Branch)
		}
	}
	for _, r := range refs {
		if r.Parent != "" && !hasSubtask(b, r.Branch) {
			if c, ok := b.ByBranch(r.Parent); ok {
				b = b.AddSubtask(r.Parent, Subtask{Name: r.Title, Branch: r.Branch, Start: r.Start, Tip: r.Tip})
				note("added subtask %s to task #%d", r.Title, c.ID)
			}
		}
	}
	for _, r := range refs {
		b = b.Track(r.Branch, r.Tip)
	}
	return b, changes
}

// relinkRenamed moves a task whose branch is gone to a branch that has no task and sits exactly at the
// task's last known tip.
func relinkRenamed(b Board, tops []Ref, note func(string, ...any)) Board {
	live := func(branch string) bool {
		return slices.ContainsFunc(tops, func(r Ref) bool { return r.Branch == branch })
	}
	for _, c := range b.Cards {
		if c.Branch == "" || c.Tip == "" || live(c.Branch) {
			continue
		}
		i := slices.IndexFunc(tops, func(r Ref) bool {
			_, linked := b.ByBranch(r.Branch)
			return r.Tip == c.Tip && !linked
		})
		if i < 0 {
			continue
		}
		old := c.Branch
		b, _ = b.With(c.ID, func(c Card) Card { c.Branch = tops[i].Branch; return c })
		note("relinked task #%d from %s to %s", c.ID, old, tops[i].Branch)
	}
	return b
}

func hasSubtask(b Board, branch string) bool {
	return slices.ContainsFunc(b.Cards, func(c Card) bool {
		return slices.ContainsFunc(c.Subtasks, func(s Subtask) bool { return s.Branch == branch })
	})
}
