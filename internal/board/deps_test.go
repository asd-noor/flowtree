package board

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func boardWith(needs map[int][]int, n int) Board {
	b := Empty()
	for i := 1; i <= n; i++ {
		b, _ = b.Add(Card{Needs: needs[i]})
	}
	return b
}

func TestCheckNeeds(t *testing.T) {
	b := boardWith(map[int][]int{6: {5}, 7: {6}}, 7)
	for _, c := range []struct {
		name  string
		id    int
		needs []int
		want  string // substring of the error; empty means valid
	}{
		{"valid", 7, []int{1, 2}, ""},
		{"dag", 5, []int{1}, ""},
		{"unknown", 1, []int{9}, "no task #9"},
		{"self", 1, []int{1}, "cannot need itself"},
		{"cycle of 2", 5, []int{6}, "#5 -> #6 -> #5"},
		{"cycle of 3", 5, []int{7}, "#5 -> #7 -> #6 -> #5"},
	} {
		err := b.CheckNeeds(c.id, c.needs)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s: %v", c.name, err)
		case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
			t.Errorf("%s: %v, want %q", c.name, err, c.want)
		}
	}
}

func TestSetNeedsNormalizesAndDoesNotMutate(t *testing.T) {
	b := boardWith(nil, 4)
	next, err := b.SetNeeds(4, []int{3, 1, 3, 2})
	if err != nil {
		t.Fatal(err)
	}
	if c, _ := next.Card(4); !slices.Equal(c.Needs, []int{1, 2, 3}) {
		t.Errorf("needs = %v", c.Needs)
	}
	if c, _ := b.Card(4); c.Needs != nil {
		t.Error("the original board changed")
	}
	if cleared, _ := next.SetNeeds(4, nil); func() bool { c, _ := cleared.Card(4); return c.Needs != nil }() {
		t.Error("empty list should clear")
	}
}

func TestNeededBy(t *testing.T) {
	b := boardWith(map[int][]int{2: {1}, 3: {1, 2}}, 3)
	if got := b.NeededBy(1); !slices.Equal(got, []int{2, 3}) {
		t.Errorf("needed by #1: %v", got)
	}
	if got := b.NeededBy(3); got != nil {
		t.Errorf("needed by #3: %v", got)
	}
}

func TestBlockedAndReady(t *testing.T) {
	st := map[int]Status{1: Done, 2: Doing, 3: Abandoned}
	look := func(id int) Status { return st[id] }
	for _, c := range []struct {
		name           string
		needs          []int
		own            Status
		blocked, ready bool
	}{
		{"no needs", nil, Todo, false, true},
		{"all done", []int{1}, Todo, false, true},
		{"one doing", []int{1, 2}, Todo, true, false},
		{"abandoned blocks", []int{3}, Todo, true, false},
		{"doing is not ready", nil, Doing, false, false},
		{"backlog is not ready", nil, Backlog, false, false},
		{"done is never blocked", []int{2}, Done, false, false},
		{"abandoned is never blocked", []int{2}, Abandoned, false, false},
	} {
		card := Card{ID: 9, Needs: c.needs}
		st[9] = c.own
		if got := Blocked(card, look); got != c.blocked {
			t.Errorf("%s: blocked = %v", c.name, got)
		}
		if got := Ready(card, c.own, look); got != c.ready {
			t.Errorf("%s: ready = %v", c.name, got)
		}
	}
}

func TestCheckNeedsForNewTaskAndWideGraph(t *testing.T) {
	b := boardWith(map[int][]int{2: {1}}, 2)
	if err := b.CheckNeeds(3, []int{2}); err != nil {
		t.Errorf("new task needing #2: %v", err)
	}
	// Layers of 4 tasks, each needing the whole previous layer: many paths, still fast.
	needs := map[int][]int{}
	for id := 5; id <= 60; id++ {
		layer := (id - 1) / 4
		for p := (layer-1)*4 + 1; p <= layer*4 && layer > 0; p++ {
			needs[id] = append(needs[id], p)
		}
	}
	wide := boardWith(needs, 60)
	if err := wide.CheckNeeds(1, []int{60}); err == nil || !strings.Contains(err.Error(), "dependency cycle") {
		t.Errorf("cycle through a wide graph: %v", err)
	}
}

func TestMissingNeedDoesNotBlock(t *testing.T) {
	look := func(id int) Status {
		if id == 7 {
			return Missing
		}
		return Done
	}
	c := Card{ID: 1, Needs: []int{2, 7}}
	if Blocked(c, look) || len(Unfinished(c, look)) != 0 {
		t.Errorf("a missing need blocks: %v", Unfinished(c, look))
	}
	deps := boardWith(nil, 2).Deps([]int{2, 7}, look)
	if deps[1] != (Dep{ID: 7, Status: Missing}) {
		t.Errorf("deps: %+v", deps)
	}
}

func TestReadTakesTheLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := Update(path, func(b Board) (Board, error) { b, _ = b.Add(Card{Title: "x"}); return b, nil }); err != nil {
		t.Fatal(err)
	}
	b, err := Read(path)
	if err != nil || len(b.Cards) != 1 {
		t.Fatalf("read: %v %+v", err, b)
	}
	if _, err := os.Stat(path + ".lock"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("lock left behind: %v", err)
	}
}
