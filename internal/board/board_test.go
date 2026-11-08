package board

import (
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestStatusOf(t *testing.T) {
	linked := Card{Branch: "feature/a"}
	for _, c := range []struct {
		name string
		card Card
		f    Facts
		want Status
	}{
		{"no branch", Card{}, Facts{}, Todo},
		{"parked", Card{Backlog: true}, Facts{}, Backlog},
		{"branch exists", linked, Facts{Exists: true, Tip: "t1"}, Doing},
		{"review asked at this tip", Card{Branch: "feature/a", ReviewTip: "t1"}, Facts{Exists: true, Tip: "t1"}, Review},
		{"new commits after review", Card{Branch: "feature/a", ReviewTip: "t1"}, Facts{Exists: true, Tip: "t2"}, Doing},
		{"merged wins over review", Card{Branch: "feature/a", ReviewTip: "t1"}, Facts{Exists: true, Tip: "t1", Merged: true}, Done},
		{"merged and deleted", linked, Facts{Merged: true}, Done},
		{"deleted unmerged", linked, Facts{}, Abandoned},
	} {
		if got := StatusOf(c.card, c.f); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
}

func TestAddAndWithDoNotShareState(t *testing.T) {
	b0 := Empty()
	b1, c := b0.Add(Card{Title: "one"})
	b2, err := b1.With(c.ID, func(c Card) Card { c.Title = "changed"; return c })
	if err != nil || c.ID != 1 || b1.NextID != 2 {
		t.Fatalf("%v %+v %+v", err, c, b1)
	}
	if got, _ := b1.Card(1); got.Title != "one" || len(b0.Cards) != 0 {
		t.Error("earlier boards must not change")
	}
	if got, _ := b2.Card(1); got.Title != "changed" {
		t.Errorf("got %q", got.Title)
	}
	if _, err := b2.With(9, func(c Card) Card { return c }); err == nil {
		t.Error("a missing id is an error")
	}
}

func TestUpdateUnderParallelWriters(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if b, err := Load(path); err != nil || b.NextID != 1 || len(b.Cards) != 0 {
		t.Fatalf("missing file: %+v %v", b, err)
	}
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := Update(path, func(b Board) (Board, error) { b, _ = b.Add(Card{Title: "x"}); return b, nil }); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	b, err := Load(path)
	if err != nil || len(b.Cards) != 20 || b.NextID != 21 {
		t.Fatalf("lost writes: %d cards, next %d, %v", len(b.Cards), b.NextID, err)
	}
}

func TestLifecycleOperations(t *testing.T) {
	b, _ := Empty().Add(Card{Title: "t", Backlog: true})
	b, err := b.Link(1, "feature/a", "s0")
	if c, _ := b.Card(1); err != nil || c.Branch != "feature/a" || c.Start != "s0" || c.Backlog {
		t.Fatalf("link: %+v %v", c, err)
	}
	if _, err := b.Link(1, "feature/b", "s"); err == nil {
		t.Error("a linked task cannot take another branch")
	}
	if b2 := b.Ensure("feature/a", "dup", "s", time.Time{}); len(b2.Cards) != 1 {
		t.Error("Ensure must reuse the linked card")
	}
	b = b.AddSubtask("feature/a", Subtask{Name: "x", Branch: "work/feature/a/x"})
	b = b.AddSubtask("feature/a", Subtask{Name: "x", Branch: "work/feature/a/x"})
	b = b.Track("work/feature/a/x", "w1").Track("feature/a", "t1").SetReview("feature/a", "t1")
	c, _ := b.Card(1)
	if len(c.Subtasks) != 1 || c.Subtasks[0].Tip != "w1" || c.Tip != "t1" || c.ReviewTip != "t1" {
		t.Fatalf("%+v", c)
	}
}
