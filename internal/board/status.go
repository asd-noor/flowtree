package board

// Status is a card's column. It is never stored: it follows from git, plus the review marker.
type Status string

// The columns, in board order.
const (
	Backlog   Status = "backlog"
	Todo      Status = "todo"
	Doing     Status = "doing"
	Review    Status = "review"
	Done      Status = "done"
	Abandoned Status = "abandoned"
)

// Columns lists every status in board order.
var Columns = []Status{Backlog, Todo, Doing, Review, Done, Abandoned}

// Facts is what git says about a branch.
type Facts struct {
	Exists bool   // the branch exists
	Tip    string // its tip, when it exists
	Merged bool   // its tip (or the last known tip, once gone) has commits of its own and is in its target
}

// StatusOf is the column of a card whose branch has the given facts.
func StatusOf(c Card, f Facts) Status {
	switch {
	case c.Branch == "" && c.Backlog:
		return Backlog
	case c.Branch == "":
		return Todo
	case f.Merged:
		return Done
	case !f.Exists:
		return Abandoned
	case c.ReviewTip != "" && c.ReviewTip == f.Tip:
		return Review
	}
	return Doing
}

// SubtaskStatus is the column of a subtask: doing, done or abandoned.
func SubtaskStatus(f Facts) Status {
	switch {
	case f.Merged:
		return Done
	case !f.Exists:
		return Abandoned
	}
	return Doing
}
