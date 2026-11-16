package worktree_test

import (
	"testing"

	"flowtree/internal/gitx"
	"flowtree/internal/worktree"
)

var wts = []gitx.Worktree{
	{Path: "/code/api", Branch: "main", Main: true},
	{Path: "/wt/api/regular/login", Branch: "regular/login"},
	{Path: "/wt/api/odd", Branch: "regular/other"},
}

func TestDisplay(t *testing.T) {
	want := []string{"api@main(root worktree)", "regular/login", "odd@regular/other"}
	for i, w := range wts {
		if got := worktree.Display(w); got != want[i] {
			t.Errorf("%d: %s", i, got)
		}
	}
}

func TestResolve(t *testing.T) {
	for query, path := range map[string]string{
		"": "/code/api", "@": "/code/api", "main": "/code/api",
		"regular/login": "/wt/api/regular/login", "odd": "/wt/api/odd", "regular/other": "/wt/api/odd",
		"/wt/api/odd": "/wt/api/odd", "odd@regular/other": "/wt/api/odd",
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
