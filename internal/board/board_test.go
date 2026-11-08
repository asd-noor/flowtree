package board

import (
	"path/filepath"
	"sync"
	"testing"
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
