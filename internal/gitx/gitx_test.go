package gitx_test

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"flowtree/internal/gitx"
	"flowtree/internal/testutil"
)

func TestParseWorktrees(t *testing.T) {
	out := "worktree /r\nHEAD aaa\nbranch refs/heads/main\n\nworktree /w/x\nHEAD bbb\nbranch refs/heads/work/x\n\nworktree /w/d\nHEAD ccc\ndetached\n"
	wts := gitx.ParseWorktrees(out)
	want := []gitx.Worktree{
		{Path: "/r", Head: "aaa", Branch: "main", Main: true},
		{Path: "/w/x", Head: "bbb", Branch: "work/x"},
		{Path: "/w/d", Head: "ccc"},
	}
	if !slices.Equal(wts, want) {
		t.Fatalf("got %+v\nwant %+v", wts, want)
	}
}

func TestAddOptsArgs(t *testing.T) {
	cases := []struct {
		o    gitx.AddOpts
		want []string
	}{
		{gitx.AddOpts{Path: "/p", Commitish: "b"}, []string{"worktree", "add", "/p", "b"}},
		{gitx.AddOpts{Path: "/p", NewBranch: "n", Commitish: "main"}, []string{"worktree", "add", "-b", "n", "/p", "main"}},
		{gitx.AddOpts{Path: "/p", NewBranch: "n", Track: true, Commitish: "origin/n"}, []string{"worktree", "add", "--track", "-b", "n", "/p", "origin/n"}},
	}
	for _, c := range cases {
		if got := c.o.Args(); !slices.Equal(got, c.want) {
			t.Errorf("got %v want %v", got, c.want)
		}
	}
}

func TestResolveBranch(t *testing.T) {
	repo, remote := testutil.NewRepoWithRemote(t)
	testutil.Git(t, repo, "branch", "local")
	testutil.Git(t, repo, "push", "-q", "origin", "local:remote-only")
	testutil.Git(t, repo, "fetch", "-q")
	other := filepath.Join(filepath.Dir(remote), "other.git")
	testutil.Git(t, repo, "init", "-q", "--bare", other)
	testutil.Git(t, repo, "remote", "add", "upstream", other)
	testutil.Git(t, repo, "push", "-q", "upstream", "local:remote-only")
	testutil.Git(t, repo, "fetch", "-q", "upstream")

	g := gitx.In(context.Background(), repo)
	if r, err := gitx.ResolveBranch(g, "local"); err != nil || r.Remote || r.Name != "local" {
		t.Errorf("local: %+v %v", r, err)
	}
	_, err := gitx.ResolveBranch(g, "remote-only")
	if be, ok := err.(*gitx.BranchError); !ok || !slices.Equal(be.Remotes, []string{"origin", "upstream"}) {
		t.Errorf("ambiguous: %v", err)
	}
	if _, err := gitx.ResolveBranch(g, "nope"); err == nil {
		t.Error("expected not found")
	}
	if !gitx.BranchExists(g, "main") || gitx.BranchExists(g, "ghost") {
		t.Error("BranchExists")
	}
}

func TestWorktreeLifecycle(t *testing.T) {
	repo := testutil.NewRepo(t)
	g := gitx.In(context.Background(), repo)
	path := filepath.Join(filepath.Dir(repo), "wt", "feat")
	if err := gitx.AddWorktree(g, gitx.AddOpts{Path: path, NewBranch: "feat", Commitish: "main"}); err != nil {
		t.Fatal(err)
	}
	wts, err := gitx.Worktrees(g)
	if err != nil || len(wts) != 2 || !wts[0].Main {
		t.Fatalf("%+v %v", wts, err)
	}
	if w, ok := gitx.WorktreeOf(wts, "feat"); !ok || w.Path != path {
		t.Errorf("WorktreeOf: %+v", w)
	}
	if err := gitx.RemoveWorktree(g, path, false); err != nil {
		t.Fatal(err)
	}
	if err := gitx.DeleteBranch(g, "feat", false); err != nil {
		t.Fatal(err)
	}
}
