package flow_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"flowtree/internal/flow"
	"flowtree/internal/gitx"
	"flowtree/internal/testutil"
)

func keys(ts []flow.SwitchTarget) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.Key
	}
	return out
}

func TestBuildSwitchTargets(t *testing.T) {
	s := flow.DefaultSettings()
	wts := []gitx.Worktree{
		{Path: "/r", Branch: "develop", Main: true},
		{Path: "/w/work/a", Branch: "work/a"},
		{Path: "/w/hotfix/1", Branch: "hotfix/1"},
	}
	branches := []string{"develop", "master", "release/1.0", "work/a", "work/b", "hotfix/1", "random"}
	ts := flow.BuildSwitchTargets(s, "/r", wts, branches)
	if got, want := strings.Join(keys(ts), " "), "@ work/a hotfix/1 master release/1.0 work/b"; got != want {
		t.Fatalf("keys = %q, want %q", got, want)
	}
	main := ts[0]
	if main.Alt != "develop" || main.Dir != "/r" || main.Action != flow.GoDir || !strings.Contains(main.Label, "[develop]") {
		t.Errorf("main row: %+v", main)
	}
	for _, c := range []struct {
		key    string
		action flow.SwitchAction
	}{{"work/a", flow.GoDir}, {"master", flow.MoveMain}, {"release/1.0", flow.MoveMain}, {"work/b", flow.Ensure}} {
		for _, tg := range ts {
			if tg.Key == c.key && tg.Action != c.action {
				t.Errorf("%s: action %d, want %d", c.key, tg.Action, c.action)
			}
		}
	}
}

func TestBuildSwitchTargetsStrayTopicInMain(t *testing.T) {
	s := flow.DefaultSettings()
	wts := []gitx.Worktree{{Path: "/r", Branch: "work/x", Main: true}}
	ts := flow.BuildSwitchTargets(s, "/r", wts, []string{"work/x", "develop"})
	if got, want := strings.Join(keys(ts), " "), "@ work/x develop"; got != want {
		t.Fatalf("keys = %q, want %q", got, want)
	}
	if ts[0].Alt != "" || ts[1].Action != flow.Ensure || !strings.Contains(ts[1].Label, "will be moved out") {
		t.Errorf("stray row not offered for repair: %+v", ts)
	}
}

func TestBuildSwitchTargetsDetachedMain(t *testing.T) {
	ts := flow.BuildSwitchTargets(flow.DefaultSettings(), "/r", []gitx.Worktree{{Path: "/r", Main: true}}, nil)
	if len(ts) != 1 || !strings.Contains(ts[0].Label, "detached") || ts[0].Alt != "" {
		t.Errorf("%+v", ts)
	}
}

func targetNamed(t *testing.T, e *flow.Env, key string) flow.SwitchTarget {
	t.Helper()
	ts, err := e.SwitchTargets()
	if err != nil {
		t.Fatal(err)
	}
	for _, tg := range ts {
		if tg.Key == key {
			return tg
		}
	}
	t.Fatalf("no target %q in %v", key, keys(ts))
	return flow.SwitchTarget{}
}

func recordCd(e *flow.Env) *string {
	var got string
	e.CdTo = func(p string) { got = p }
	return &got
}

func TestSwitchMovesMainOntoMaster(t *testing.T) {
	e, repo := setup(t) // main worktree on develop
	cd := recordCd(e)
	dir, err := flow.Switch(e, targetNamed(t, e, "main"))
	if err != nil || dir != repo || *cd != repo {
		t.Fatalf("%s %v (cd %q)", dir, err, *cd)
	}
	if got := gitx.CurrentBranch(e.MainGit()); got != "main" {
		t.Errorf("main worktree on %q", got)
	}
}

func TestSwitchMoveMainNeedsCleanTree(t *testing.T) {
	e, repo := setup(t)
	testutil.WriteFile(t, filepath.Join(repo, "README.md"), "dirty\n")
	if _, err := flow.Switch(e, targetNamed(t, e, "main")); err == nil {
		t.Fatal("expected refusal on a dirty main worktree")
	}
	if got := gitx.CurrentBranch(e.MainGit()); got != "develop" {
		t.Errorf("main worktree on %q", got)
	}
}

func TestSwitchCreatesMissingWorktree(t *testing.T) {
	e, repo := setup(t)
	testutil.Git(t, repo, "branch", "work/x", "develop")
	cd := recordCd(e)
	dir, err := flow.Switch(e, targetNamed(t, e, "work/x"))
	if err != nil || !strings.HasSuffix(dir, "worktrees/work/x") || *cd != dir {
		t.Fatalf("%s %v (cd %q)", dir, err, *cd)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Error("worktree not created")
	}
	if again := targetNamed(t, e, "work/x"); again.Action != flow.GoDir || again.Dir != dir {
		t.Errorf("an existing worktree should be a plain GoDir row: %+v", again)
	}
}

func TestSwitchRepairsStrayTopicInMain(t *testing.T) {
	e, repo := setup(t)
	testutil.Git(t, repo, "checkout", "-q", "-b", "work/x", "develop")
	dir, err := flow.Switch(e, targetNamed(t, e, "work/x"))
	if err != nil || dir == repo {
		t.Fatalf("%s %v", dir, err)
	}
	if got := gitx.CurrentBranch(e.MainGit()); got != "develop" {
		t.Errorf("main worktree on %q, want develop", got)
	}
}

func TestSwitchGoDir(t *testing.T) {
	e, repo := setup(t)
	cd := recordCd(e)
	if dir, err := flow.Switch(e, targetNamed(t, e, "@")); err != nil || dir != repo || *cd != repo {
		t.Fatalf("%s %v", dir, err)
	}
}
