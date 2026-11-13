package commands

import (
	"flowtree/internal/board"
	"testing"

	"flowtree/internal/flow"
)

func TestRenderEntries(t *testing.T) {
	got := renderEntries([]flow.Entry{
		{Name: "a", Current: true, State: "(no commits yet)"},
		{Name: "longer", State: "(based on latest develop)"},
	})
	want := "* a      (no commits yet)\n  longer (based on latest develop)\n"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
	if got := renderEntries([]flow.Entry{{Name: "x"}}); got != "  x\n" {
		t.Errorf("plain: %q", got)
	}
}

func TestParentRow(t *testing.T) {
	r := parentRow(board.Card{ID: 12, Title: "Login", Branch: "regular/login"})
	if r.Key != "12" || r.Label != "#12 Login (regular/login)" {
		t.Errorf("row = %+v", r)
	}
}
