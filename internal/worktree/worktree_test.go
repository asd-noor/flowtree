package worktree_test

import (
	"testing"

	"flowtree/internal/gitx"
	"flowtree/internal/worktree"
)

var wts = []gitx.Worktree{
	{Path: "/code/api", Branch: "main", Main: true},
	{Path: "/wt/api/work/login", Branch: "work/login"},
	{Path: "/wt/api/odd", Branch: "work/other"},
}

func TestDisplay(t *testing.T) {
	want := []string{"api@main(root worktree)", "work/login", "odd@work/other"}
	for i, w := range wts {
		if got := worktree.Display(w); got != want[i] {
			t.Errorf("%d: %s", i, got)
		}
	}
}

func TestResolve(t *testing.T) {
	for query, path := range map[string]string{
		"": "/code/api", "@": "/code/api", "main": "/code/api",
		"work/login": "/wt/api/work/login", "odd": "/wt/api/odd", "work/other": "/wt/api/odd",
		"/wt/api/odd": "/wt/api/odd", "odd@work/other": "/wt/api/odd",
	} {
		w, err := worktree.Resolve(wts, query)
		if err != nil || w.Path != path {
			t.Errorf("%q: %+v %v", query, w, err)
		}
	}
	if _, err := worktree.Resolve(wts, "nope"); err == nil {
		t.Error("expected error")
	}
}
