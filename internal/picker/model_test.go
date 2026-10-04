package picker

import (
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func key(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	case "ctrl+n":
		return tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl}
	case "ctrl+p":
		return tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl}
	case "ctrl+u":
		return tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}
	}
	return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
}

// press feeds keys to the model and returns the result and the last command.
func press(m model, keys ...string) (model, tea.Cmd) {
	var cmd tea.Cmd
	for _, k := range keys {
		next, c := m.Update(key(k))
		m, cmd = next.(model), c
	}
	return m, cmd
}

func rows3() []Row {
	return []Row{{Key: "alpha"}, {Key: "beta"}, {Key: "gamma"}}
}

func TestSelectWithEnter(t *testing.T) {
	m, cmd := press(newModel(rows3(), ""), "down", "enter")
	if m.chosen == nil || m.chosen.Key != "beta" || cmd == nil {
		t.Fatalf("chosen = %+v, cmd nil = %v", m.chosen, cmd == nil)
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("enter should quit the program")
	}
}

func TestCancel(t *testing.T) {
	for _, k := range []string{"esc", "ctrl+c"} {
		m, cmd := press(newModel(rows3(), ""), k)
		if m.chosen != nil || !m.done || cmd == nil {
			t.Errorf("%s: chosen=%v done=%v", k, m.chosen, m.done)
		}
	}
}

func TestCursorClamps(t *testing.T) {
	m, _ := press(newModel(rows3(), ""), "up", "up")
	if m.cursor != 0 {
		t.Errorf("cursor above the top: %d", m.cursor)
	}
	m, _ = press(m, "down", "down", "down", "down", "ctrl+n")
	if m.cursor != 2 {
		t.Errorf("cursor below the bottom: %d", m.cursor)
	}
	m, _ = press(m, "ctrl+p")
	if m.cursor != 1 {
		t.Errorf("ctrl+p: %d", m.cursor)
	}
}

func TestTypingFiltersAndResetsCursor(t *testing.T) {
	m, _ := press(newModel(rows3(), ""), "down", "down", "g")
	if m.query != "g" || m.cursor != 0 {
		t.Fatalf("query %q cursor %d", m.query, m.cursor)
	}
	if got := keys(m.visible()); len(got) != 1 || got[0] != "gamma" {
		t.Errorf("visible = %v", got)
	}
	m, _ = press(m, "a", "m")
	if row, _ := m.current(); row.Key != "gamma" {
		t.Errorf("current = %q", row.Key)
	}
	m, _ = press(m, "backspace", "backspace", "backspace")
	if m.query != "" || len(m.visible()) != 3 {
		t.Errorf("after backspaces query %q, visible %d", m.query, len(m.visible()))
	}
	m, _ = press(m, "a", "ctrl+u")
	if m.query != "" {
		t.Errorf("ctrl+u should clear, got %q", m.query)
	}
}

func TestPrefilledQuery(t *testing.T) {
	m := newModel(rows3(), "bet")
	if got := keys(m.visible()); len(got) != 1 || got[0] != "beta" {
		t.Errorf("visible = %v", got)
	}
	m, _ = press(m, "enter")
	if m.chosen == nil || m.chosen.Key != "beta" {
		t.Error("enter should choose the only match")
	}
}

func TestEnterWithNoMatchDoesNothing(t *testing.T) {
	m, cmd := press(newModel(rows3(), "zzz"), "enter")
	if m.chosen != nil || m.done || cmd != nil {
		t.Error("no match: enter must not finish")
	}
}

func TestPreviewIsLazyAndCached(t *testing.T) {
	calls := map[string]int{}
	mk := func(k string) Row {
		return Row{Key: k, Preview: func() string { calls[k]++; return "preview of " + k }}
	}
	m := newModel([]Row{mk("a"), mk("b")}, "")
	if calls["a"] != 1 || calls["b"] != 0 {
		t.Fatalf("only the highlighted row is previewed: %v", calls)
	}
	m, _ = press(m, "down", "up", "down", "up")
	if calls["a"] != 1 || calls["b"] != 1 {
		t.Errorf("each preview computed once: %v", calls)
	}
	if !strings.Contains(render(m), "preview of a") {
		t.Errorf("view lacks the preview:\n%s", render(m))
	}
}

func TestViewUsesAlternateScreen(t *testing.T) {
	if !newModel(rows3(), "").View().AltScreen {
		t.Error("picker view should use the alternate screen")
	}
}

func TestViewLayouts(t *testing.T) {
	rows := []Row{{Key: "alpha", Preview: func() string { return "P1\nP2" }}, {Key: "beta"}}
	m := newModel(rows, "")
	narrow := render(m)
	if !strings.Contains(narrow, "> █") || !strings.Contains(narrow, "▸ alpha") || !strings.Contains(narrow, "─") {
		t.Errorf("narrow layout:\n%s", narrow)
	}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	wide := render(next.(model))
	lines := strings.Split(wide, "\n")
	if len(lines) < 2 || !strings.Contains(lines[1], "▸ alpha") || !strings.Contains(lines[1], "│ P1") {
		t.Errorf("wide layout puts the preview beside the list:\n%s", wide)
	}
}

func TestViewTruncatesAndWindows(t *testing.T) {
	var rows []Row
	for i := 0; i < 50; i++ {
		rows = append(rows, Row{Key: strings.Repeat("x", 200) + string(rune('a'+i%26))})
	}
	m := newModel(rows, "")
	for i := 0; i < 30; i++ {
		m, _ = press(m, "down")
	}
	out := render(m)
	lines := strings.Split(out, "\n")
	if len(lines) > maxListLines+3 {
		t.Errorf("list not windowed: %d lines", len(lines))
	}
	if !strings.Contains(out, "▸") {
		t.Error("the cursor row must stay visible after scrolling")
	}
	for _, l := range lines {
		if len([]rune(l)) > defaultWidth {
			t.Errorf("line not truncated: %d columns", len([]rune(l)))
		}
	}
}

func TestNoMatchView(t *testing.T) {
	if out := render(newModel(rows3(), "zzz")); !strings.Contains(out, "(no match)") {
		t.Errorf("view = %q", out)
	}
}

// slowReader hands out its script one byte at a time, then blocks, so the program does not see EOF.
type slowReader struct {
	mu     sync.Mutex
	script []byte
	block  chan struct{}
}

func (r *slowReader) Read(p []byte) (int, error) {
	r.mu.Lock()
	if len(r.script) == 0 {
		r.mu.Unlock()
		<-r.block
		return 0, io.EOF
	}
	p[0], r.script = r.script[0], r.script[1:]
	r.mu.Unlock()
	time.Sleep(5 * time.Millisecond)
	return 1, nil
}

func TestPickScripted(t *testing.T) {
	rd := &slowReader{script: []byte("be\r"), block: make(chan struct{})}
	defer close(rd.block)
	var out strings.Builder
	type result struct {
		row Row
		err error
	}
	done := make(chan result, 1)
	go func() {
		row, err := Pick(t.Context(), rows3(), "", rd, &out)
		done <- result{row, err}
	}()
	select {
	case r := <-done:
		if r.err != nil || r.row.Key != "beta" {
			t.Fatalf("got %+v %v", r.row, r.err)
		}
	case <-time.After(10 * time.Second):
		t.Skip("bubbletea did not process scripted input without a terminal")
	}
}

func TestPickNoRows(t *testing.T) {
	if _, err := Pick(t.Context(), nil, "", strings.NewReader(""), io.Discard); err == nil {
		t.Error("expected an error")
	}
}
