package flow_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"flowtree/internal/config"
	"flowtree/internal/flow"
	"flowtree/internal/gitx"
	"flowtree/internal/testutil"
)

// setup returns an Env on a fresh repo whose production branch is "main", plus its path.
func setup(t *testing.T) (*flow.Env, string) {
	t.Helper()
	repo := testutil.NewRepo(t)
	s := flow.DefaultSettings()
	s.Master = "main"
	cfg := config.Defaults()
	fl := s.ToConfig()
	cfg.Flow = &fl
	e := &flow.Env{Ctx: context.Background(), Run: gitx.Exec, S: s, Config: cfg, Main: repo, Cwd: repo, Out: io.Discard, Err: io.Discard}
	if err := flow.EnsureBranches(e); err != nil {
		t.Fatal(err)
	}
	return e, repo
}

func commit(t *testing.T, dir, file, content string) {
	t.Helper()
	testutil.WriteFile(t, filepath.Join(dir, file), content)
	testutil.Git(t, dir, "add", "-A")
	testutil.Git(t, dir, "commit", "-q", "-m", "change "+file)
}

func TestEnsureBranches(t *testing.T) {
	e, repo := setup(t)
	g := e.MainGit()
	if !gitx.BranchExists(g, "develop") {
		t.Fatal("develop not created")
	}
	if got := gitx.CurrentBranch(g); got != "develop" {
		t.Errorf("main worktree on %q, want develop", got)
	}
	if err := flow.EnsureBranches(e); err != nil { // idempotent
		t.Fatal(err)
	}
	_ = repo
}

func TestEnsureBranchesNeedsCommit(t *testing.T) {
	dir := t.TempDir()
	testutil.Git(t, dir, "init", "-q", "-b", "main")
	e := &flow.Env{Ctx: context.Background(), Run: gitx.Exec, S: flow.DefaultSettings(), Main: dir, Out: io.Discard, Err: io.Discard}
	if err := flow.EnsureBranches(e); err == nil || !strings.Contains(err.Error(), "no commits") {
		t.Fatalf("got %v", err)
	}
}

func TestMergeIntoModes(t *testing.T) {
	for _, tc := range []struct {
		mode    flow.MergeMode
		parents int
	}{{flow.FastForward, 1}, {flow.NoFF, 2}, {flow.Squash, 1}} {
		e, repo := setup(t)
		wt, err := flow.CreateWorktree(e, "work/a", gitx.AddOpts{NewBranch: "work/a", Commitish: "develop"})
		if err != nil {
			t.Fatal(err)
		}
		commit(t, wt, "a.txt", "a")
		commit(t, wt, "b.txt", "b")
		if err := flow.MergeInto(e, "develop", "work/a", tc.mode, ""); err != nil {
			t.Fatalf("mode %d: %v", tc.mode, err)
		}
		// develop is checked out in the main worktree, so the merge ran there.
		if _, err := os.Stat(filepath.Join(repo, "b.txt")); err != nil {
			t.Errorf("mode %d: merge result missing in main worktree", tc.mode)
		}
		parents := strings.Fields(testutil.Git(t, repo, "rev-list", "--parents", "-n1", "develop"))
		if len(parents)-1 != tc.parents {
			t.Errorf("mode %d: head has %d parents, want %d", tc.mode, len(parents)-1, tc.parents)
		}
	}
}

func TestMergeIntoUsesThrowawayWorktree(t *testing.T) {
	e, repo := setup(t)
	testutil.Git(t, repo, "checkout", "-q", "main") // develop is now checked out nowhere
	wt, _ := flow.CreateWorktree(e, "work/a", gitx.AddOpts{NewBranch: "work/a", Commitish: "develop"})
	commit(t, wt, "a.txt", "a")
	if err := flow.MergeInto(e, "develop", "work/a", flow.FastForward, ""); err != nil {
		t.Fatal(err)
	}
	if got := testutil.Git(t, repo, "rev-parse", "develop"); got != testutil.Git(t, repo, "rev-parse", "work/a") {
		t.Error("develop was not fast-forwarded")
	}
	wts, _ := e.Worktrees()
	if len(wts) != 2 {
		t.Errorf("throwaway worktree left behind: %+v", wts)
	}
	if testutil.Git(t, repo, "status", "--porcelain") != "" {
		t.Error("main worktree touched")
	}
}

func TestMergeConflictChangesNothing(t *testing.T) {
	e, repo := setup(t)
	wt, _ := flow.CreateWorktree(e, "work/a", gitx.AddOpts{NewBranch: "work/a", Commitish: "develop"})
	commit(t, wt, "README.md", "from work\n")
	commit(t, repo, "README.md", "from develop\n") // main worktree is on develop
	before := testutil.Git(t, repo, "rev-parse", "develop")
	err := flow.MergeInto(e, "develop", "work/a", flow.NoFF, "")
	if err == nil || !strings.Contains(err.Error(), "Nothing was changed") {
		t.Fatalf("got %v", err)
	}
	if testutil.Git(t, repo, "rev-parse", "develop") != before || testutil.Git(t, repo, "status", "--porcelain") != "" {
		t.Error("conflicting merge left state behind")
	}
}

func TestMergeRefusesDirtyTarget(t *testing.T) {
	e, repo := setup(t)
	wt, _ := flow.CreateWorktree(e, "work/a", gitx.AddOpts{NewBranch: "work/a", Commitish: "develop"})
	commit(t, wt, "a.txt", "a")
	testutil.WriteFile(t, filepath.Join(repo, "README.md"), "dirty\n")
	if err := flow.MergeInto(e, "develop", "work/a", flow.FastForward, ""); err == nil {
		t.Fatal("expected refusal")
	}
}

func TestCheckSync(t *testing.T) {
	repo, _ := testutil.NewRepoWithRemote(t)
	s := flow.DefaultSettings()
	s.Master = "main"
	e := &flow.Env{Ctx: context.Background(), Run: gitx.Exec, S: s, Config: config.Defaults(), Main: repo, Cwd: repo, Out: io.Discard, Err: io.Discard}
	if err := flow.CheckSync(e, "main"); err != nil {
		t.Fatal(err)
	}
	commit(t, repo, "x", "x") // ahead of the remote: fine
	if err := flow.CheckSync(e, "main"); err != nil {
		t.Fatalf("ahead should pass: %v", err)
	}
	testutil.Git(t, repo, "push", "-q")
	testutil.Git(t, repo, "reset", "-q", "--hard", "HEAD~1") // now behind
	if err := flow.CheckSync(e, "main"); err == nil || !strings.Contains(err.Error(), "behind") {
		t.Fatalf("behind: %v", err)
	}
	commit(t, repo, "y", "y") // diverged
	if err := flow.CheckSync(e, "main"); err == nil || !strings.Contains(err.Error(), "diverged") {
		t.Fatalf("diverged: %v", err)
	}
}

func TestEnsureWorktreeEvacuatesMain(t *testing.T) {
	e, repo := setup(t)
	testutil.Git(t, repo, "checkout", "-q", "-b", "work/x", "develop")
	dir, err := flow.EnsureWorktree(e, "work/x")
	if err != nil {
		t.Fatal(err)
	}
	if dir == repo || !strings.HasSuffix(dir, "worktrees/work/x") {
		t.Errorf("worktree at %s", dir)
	}
	if got := gitx.CurrentBranch(e.MainGit()); got != "develop" {
		t.Errorf("main on %q, want develop", got)
	}
}

func TestTopicDirReleaseSwitchesMain(t *testing.T) {
	e, repo := setup(t)
	testutil.Git(t, repo, "branch", "release/1.0", "develop")
	dir, err := flow.TopicDir(e, flow.Release, "release/1.0")
	if err != nil || dir != repo {
		t.Fatalf("%s %v", dir, err)
	}
	if got := gitx.CurrentBranch(e.MainGit()); got != "release/1.0" {
		t.Errorf("main on %q", got)
	}
}

func TestResolveTopic(t *testing.T) {
	e, repo := setup(t)
	for _, b := range []string{"work/login", "work/logout", "work/signup", "hotfix/1.0.1"} {
		testutil.Git(t, repo, "branch", b, "develop")
	}
	for name, want := range map[string]string{"login": "work/login", "work/signup": "work/signup", "sig": "work/signup"} {
		if got, err := flow.ResolveTopic(e, flow.Work, name); err != nil || got != want {
			t.Errorf("%q: %q %v", name, got, err)
		}
	}
	if _, err := flow.ResolveTopic(e, flow.Work, "log"); err == nil || !strings.Contains(err.Error(), "multiple") {
		t.Errorf("ambiguous prefix: %v", err)
	}
	if _, err := flow.ResolveTopic(e, flow.Hotfix, "9"); err == nil {
		t.Error("hotfix has no prefix matching")
	}
	if got, err := flow.ResolveTopic(e, flow.Hotfix, "1.0.1"); err != nil || got != "hotfix/1.0.1" {
		t.Errorf("%q %v", got, err)
	}
	if _, err := flow.ResolveTopic(e, flow.Work, ""); err == nil { // cwd is the main worktree on develop
		t.Error("no name outside a topic worktree should fail")
	}
	wt, _ := flow.EnsureWorktree(e, "work/login")
	e.Cwd = filepath.Join(wt, "sub")
	if got, err := flow.ResolveTopic(e, flow.Work, ""); err != nil || got != "work/login" {
		t.Errorf("inferred from cwd: %q %v", got, err)
	}
}

func TestSettings(t *testing.T) {
	s := flow.DefaultSettings()
	if got := s.Branch(flow.Release, "1.0"); got != "release/1.0" || s.Branch(flow.Release, "release/1.0") != got {
		t.Error(got)
	}
	if s.Base(flow.Hotfix) != "master" || s.Base(flow.Work) != "develop" || s.Name(flow.Work, "work/x") != "x" {
		t.Error("base/name")
	}
	bad := s
	bad.Develop = bad.Master
	if bad.Validate() == nil {
		t.Error("master == develop must be rejected")
	}
	if _, err := flow.FromConfig(config.Defaults()); err == nil {
		t.Error("config without a flow section must be rejected")
	}
}
