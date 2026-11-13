package board

import (
	"strings"
	"testing"
	"time"
)

func TestReconcile(t *testing.T) {
	b, _ := Empty().Add(Card{Title: "old name", Branch: "regular/old", Tip: "t1"})
	b, _ = b.Add(Card{Title: "kept", Branch: "regular/kept", Tip: "k0"})
	refs := []Ref{
		{Branch: "regular/new", Tip: "t1", Title: "new"},   // regular/old renamed
		{Branch: "regular/kept", Tip: "k1", Title: "kept"}, // moved on
		{Branch: "hotfix/1.0.1", Tip: "h1", Start: "m0", Title: "1.0.1"},
		{Branch: "work/regular/kept/x", Tip: "w1", Start: "k1", Title: "x", Parent: "regular/kept"},
		{Branch: "work/regular/gone/y", Tip: "w2", Title: "y", Parent: "regular/gone"}, // orphan: skipped
	}
	got, changes := Reconcile(b, refs, time.Time{})
	want := []string{
		"relinked task #1 from regular/old to regular/new",
		"added task #3 for hotfix/1.0.1",
		"added subtask x to task #2",
	}
	if strings.Join(changes, "\n") != strings.Join(want, "\n") {
		t.Errorf("changes:\n%s", strings.Join(changes, "\n"))
	}
	if c, _ := got.Card(1); c.Branch != "regular/new" || c.Title != "old name" {
		t.Errorf("relinked card: %+v", c)
	}
	if c, _ := got.Card(2); c.Tip != "k1" || len(c.Subtasks) != 1 || c.Subtasks[0].Tip != "w1" {
		t.Errorf("tracked card: %+v", c)
	}
	if again, more := Reconcile(got, refs, time.Time{}); len(more) != 0 || len(again.Cards) != 3 {
		t.Errorf("a second run must change nothing: %v", more)
	}
}
