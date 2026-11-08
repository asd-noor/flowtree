package boardui

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"flowtree/internal/board"
)

func card(id int, title string, s board.Status) Card {
	return Card{Card: board.Card{ID: id, Title: title}, Status: s}
}

func sample() []Card {
	return []Card{
		card(1, "a", board.Todo), card(2, "b", board.Doing), card(3, "c", board.Todo),
		card(4, "d", board.Done),
	}
}

func key(m tea.Model, s string) model {
	code := map[string]rune{"left": tea.KeyLeft, "right": tea.KeyRight, "up": tea.KeyUp, "down": tea.KeyDown,
		"enter": tea.KeyEnter, "tab": tea.KeyTab, "esc": tea.KeyEscape}[s]
	if code == 0 {
		code = rune(s[0])
	}
	next, _ := m.Update(tea.KeyPressMsg{Code: code, Text: map[bool]string{true: s, false: ""}[len(s) == 1]})
	return next.(model)
}

func ids(cs []Card) []int {
	out := []int{}
	for _, c := range cs {
		out = append(out, c.ID)
	}
	return out
}

func TestGroupingKeepsBoardOrder(t *testing.T) {
	got := ids(cardsIn(sample(), board.Todo))
	if len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Fatalf("todo = %v", got)
	}
}

func TestNavigation(t *testing.T) {
	m := newModel(Source{}, sample())
	m = key(key(m, "l"), "l") // backlog -> todo -> doing
	if c, _ := m.selected(); c.ID != 2 {
		t.Fatalf("doing selected %d", c.ID)
	}
	m = key(m, "h")
	m = key(key(m, "j"), "j") // clamps at the last todo card
	if c, _ := m.selected(); c.ID != 3 {
		t.Fatalf("todo selected %d", c.ID)
	}
	m = key(m, "k")
	if c, _ := m.selected(); c.ID != 1 {
		t.Fatalf("selected %d", c.ID)
	}
	if m = key(key(key(key(key(key(m, "h"), "h"), "h"), "l"), "l"), "l"); m.col != 3 {
		t.Fatalf("col %d", m.col)
	}
}

func TestColumnChangeClampsRow(t *testing.T) {
	m := newModel(Source{}, sample())
	m = key(m, "l")
	m = key(key(m, "j"), "j") // todo row 1
	m = key(m, "l")           // doing has one card
	if m.row != 0 {
		t.Fatalf("row %d", m.row)
	}
}

func TestDetailToggle(t *testing.T) {
	m := newModel(Source{}, sample())
	if m = key(m, "enter"); !m.detail {
		t.Fatal("enter should open")
	}
	if m = key(m, "tab"); m.detail {
		t.Fatal("tab should close")
	}
}

func TestReloadKeepsSelectionById(t *testing.T) {
	m := key(key(newModel(Source{}, sample()), "l"), "j") // card 3
	moved := []Card{card(3, "c", board.Doing), card(1, "a", board.Todo), card(2, "b", board.Doing)}
	m = m.loaded(loadedMsg{cards: moved})
	if c, _ := m.selected(); c.ID != 3 || board.Columns[m.col] != board.Doing {
		t.Fatalf("selected %+v in %s", c.ID, board.Columns[m.col])
	}
	m = m.loaded(loadedMsg{cards: []Card{card(1, "a", board.Todo)}})
	if _, ok := m.selected(); ok && m.col != 2 {
		t.Fatal("selection must stay in range")
	}
	if m = m.loaded(loadedMsg{cards: nil}); m.row != 0 {
		t.Fatalf("row %d", m.row)
	}
}

func TestReloadErrorKeepsCards(t *testing.T) {
	m := newModel(Source{}, sample()).loaded(loadedMsg{err: os.ErrInvalid})
	if len(m.cards) != 4 || m.notice == "" {
		t.Fatalf("%d cards, notice %q", len(m.cards), m.notice)
	}
}

func TestQuitKeys(t *testing.T) {
	for _, k := range []string{"q", "esc"} {
		m := newModel(Source{}, sample())
		var cmd tea.Cmd
		if k == "esc" {
			_, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		} else {
			_, cmd = m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
		}
		if cmd == nil {
			t.Errorf("%s should quit", k)
		}
	}
}

func TestTickReloadsEvenWithoutAFileChange(t *testing.T) {
	loads := 0
	src := Source{Load: func() ([]Card, error) { loads++; return sample(), nil }}
	msg, ok := reload(src)().(loadedMsg)
	if !ok || loads != 1 || len(msg.cards) != 4 {
		t.Fatalf("%+v loads=%d", msg, loads)
	}
	if _, cmd := newModel(src, nil).Update(tickMsg(time.Now())); cmd == nil {
		t.Error("a tick must schedule a reload")
	}
}

func TestRenderShowsColumnsAndDetail(t *testing.T) {
	c := card(7, "Login", board.Todo)
	c.Human, c.Description, c.Branch = true, "Body text", "feature/login"
	c.Subtasks = []board.Subtask{{Name: "api", Branch: "work/login/api"}}
	c.SubStatus = []board.Status{board.Doing}
	m := newModel(Source{}, []Card{c})
	m.width = 120
	m = key(key(m, "l"), "enter")
	out := render(m)
	for _, want := range []string{"BACKLOG (0)", "[TODO (1)]", ">#7 Login [h]", "branch: feature/login", "Body text", "doing", "work/login/api"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if n := len(strings.Split(out, "\n")); n > m.height {
		t.Errorf("%d lines for height %d", n, m.height)
	}
}
