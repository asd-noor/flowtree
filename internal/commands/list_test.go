package commands

import (
	"strings"
	"testing"

	"flowtree/internal/gitx"
)

func TestRenderTable(t *testing.T) {
	wts := []gitx.Worktree{
		{Path: "/r/repo", Branch: "main", Head: "0123456789abcdef", Main: true},
		{Path: "/r/worktrees/feat/x", Branch: "feat/x", Head: "fedcba9876543210"},
		{Path: "/elsewhere/y", Head: "aaaaaaaabbbb"},
	}
	got := renderTable(wts, "/r/worktrees/feat/x/sub", "/r/worktrees")
	for _, want := range []string{
		"PATH", "BRANCH", "HEAD",
		"@ (main worktree)   main",
		"feat/x*",
		"01234567",
		"/elsewhere/y",
		"detached",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("table missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "main worktree)*") {
		t.Errorf("main must not be marked current:\n%s", got)
	}
	if strings.Contains(got, "0123456789") {
		t.Errorf("HEAD not shortened:\n%s", got)
	}
}

func TestCurrentIndexPicksDeepest(t *testing.T) {
	wts := []gitx.Worktree{{Path: "/r/repo", Main: true}, {Path: "/r/repo/wt"}}
	if got := currentIndex(wts, "/r/repo/wt/a"); got != 1 {
		t.Errorf("got %d, want 1", got)
	}
	if got := currentIndex(wts, "/nowhere"); got != -1 {
		t.Errorf("got %d, want -1", got)
	}
}

func TestRenderNames(t *testing.T) {
	wts := []gitx.Worktree{
		{Path: "/r/repo", Branch: "main", Main: true},
		{Path: "/r/worktrees/feat/x", Branch: "feat/x"},
	}
	if got, want := renderNames(wts), "repo@main(root worktree)\nfeat/x\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestListCommand(t *testing.T) {
	h := newHarness(t)
	h.git("worktree", "add", "-q", "-b", "feat/x", h.wt("feat/x"))

	out, _ := h.run(t, "list")
	for _, want := range []string{"@ (main worktree)*", "main", "feat/x"} {
		if !strings.Contains(out, want) {
			t.Errorf("list output missing %q:\n%s", want, out)
		}
	}

	out, _ = h.run(t, "list", "-q")
	if want := "repo@main(root worktree)\nfeat/x\n"; out != want {
		t.Errorf("list -q = %q, want %q", out, want)
	}
}
