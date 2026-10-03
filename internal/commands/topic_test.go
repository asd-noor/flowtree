package commands

import (
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
