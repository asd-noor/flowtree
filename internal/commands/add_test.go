package commands

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"flowtree/internal/config"
	"flowtree/internal/gitx"
	"flowtree/internal/testutil"
)

// addEnv is a repo plus buffers; worktrees go to <tmp>/worktrees (the default ../worktrees).
type addEnv struct {
	repo        string
	out, errOut *bytes.Buffer
	deps        Deps
}

func newAddEnv(t *testing.T, repo string, env map[string]string) addEnv {
	t.Helper()
	e := addEnv{repo: repo, out: &bytes.Buffer{}, errOut: &bytes.Buffer{}}
	e.deps = Deps{
		Run:    gitx.Exec,
		Stdin:  strings.NewReader(""),
		Stdout: e.out,
		Stderr: e.errOut,
		Getenv: func(k string) string { return env[k] },
		Cwd:    func() (string, error) { return repo, nil },
		IsTTY:  func() bool { return false },
	}
	return e
}

func (e addEnv) add(t *testing.T, args ...string) error {
	t.Helper()
	root := NewRoot(e.deps, "test")
	root.SetArgs(append([]string{"add"}, args...))
	return root.ExecuteContext(context.Background())
}

func (e addEnv) wtPath(branch string) string {
	return filepath.Join(filepath.Dir(e.repo), "worktrees", filepath.FromSlash(branch))
}

func (e addEnv) branches(t *testing.T) string {
	t.Helper()
	return testutil.Git(t, e.repo, "branch", "--format=%(refname:short)")
}

func TestAddNewBranch(t *testing.T) {
	e := newAddEnv(t, testutil.NewRepo(t), nil)
	if err := e.add(t, "-b", "feature/x"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(e.wtPath("feature/x")); err != nil {
		t.Fatalf("worktree missing: %v", err)
	}
	if !strings.Contains(e.branches(t), "feature/x") {
		t.Error("branch not created")
	}
	for _, want := range []string{"Worktree created", e.wtPath("feature/x"), "feature/x"} {
		if !strings.Contains(e.out.String(), want) {
			t.Errorf("stdout missing %q:\n%s", want, e.out)
		}
	}
}

func TestAddNewBranchFromCommitish(t *testing.T) {
	repo := testutil.NewRepo(t)
	testutil.Git(t, repo, "tag", "v0")
	testutil.WriteFile(t, filepath.Join(repo, "b.txt"), "b")
	testutil.Git(t, repo, "add", "-A")
	testutil.Git(t, repo, "commit", "-q", "-m", "second")
	e := newAddEnv(t, repo, nil)
	if err := e.add(t, "-b", "old", "v0"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(e.wtPath("old"), "b.txt")); err == nil {
		t.Error("branch should start at v0, which has no b.txt")
	}
}

func TestAddExistingLocalBranch(t *testing.T) {
	repo := testutil.NewRepo(t)
	testutil.Git(t, repo, "branch", "topic")
	e := newAddEnv(t, repo, nil)
	if err := e.add(t, "topic"); err != nil {
		t.Fatal(err)
	}
	if got := testutil.Git(t, e.wtPath("topic"), "branch", "--show-current"); strings.TrimSpace(got) != "topic" {
		t.Errorf("worktree on %q", got)
	}
}

func TestAddTracksRemoteOnlyBranch(t *testing.T) {
	repo, _ := testutil.NewRepoWithRemote(t)
	testutil.Git(t, repo, "push", "-q", "origin", "main:refs/heads/remote-only")
	testutil.Git(t, repo, "fetch", "-q", "origin")
	e := newAddEnv(t, repo, nil)
	if err := e.add(t, "remote-only"); err != nil {
		t.Fatal(err)
	}
	up := testutil.Git(t, e.wtPath("remote-only"), "rev-parse", "--abbrev-ref", "remote-only@{upstream}")
	if strings.TrimSpace(up) != "origin/remote-only" {
		t.Errorf("upstream = %q", up)
	}
}

func TestAddQuietPrintsOnlyPath(t *testing.T) {
	e := newAddEnv(t, testutil.NewRepo(t), nil)
	if err := e.add(t, "-b", "q", "--quiet"); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(e.out.String()); got != e.wtPath("q") {
		t.Errorf("stdout = %q, want %q", got, e.wtPath("q"))
	}
}

func TestAddWritesCdFile(t *testing.T) {
	cd := filepath.Join(t.TempDir(), "cd")
	e := newAddEnv(t, testutil.NewRepo(t), map[string]string{CdFileEnv: cd})
	if err := e.add(t, "-b", "c"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(cd); string(b) != e.wtPath("c") {
		t.Errorf("cd file = %q", b)
	}
}

func TestAddErrors(t *testing.T) {
	repo := testutil.NewRepo(t)
	testutil.Git(t, repo, "branch", "taken")
	e := newAddEnv(t, repo, nil)
	if err := e.add(t, "taken"); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"new branch already exists", []string{"-b", "taken"}, "already exists"},
		{"branch already checked out", []string{"taken"}, "already checked out"},
		{"checked out in main", []string{"main"}, "already checked out"},
		{"unknown branch hints -b", []string{"ghost"}, "flowtree add -b ghost"},
		{"second arg without -b", []string{"a", "b"}, "only valid with -b"},
		{"no arguments", nil, "expected one branch name"},
		{"two commitishes with -b", []string{"-b", "n", "a", "b"}, "too many arguments"},
	}
	for _, c := range cases {
		err := e.add(t, c.args...)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want containing %q", c.name, err, c.want)
		}
	}
}

func TestAddRefusesExistingPath(t *testing.T) {
	e := newAddEnv(t, testutil.NewRepo(t), nil)
	if err := os.MkdirAll(e.wtPath("busy"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := e.add(t, "-b", "busy")
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("err = %v", err)
	}
	if strings.Contains(e.branches(t), "busy") {
		t.Error("branch must not be created when the path is taken")
	}
}

func TestAddAmbiguousRemoteBranch(t *testing.T) {
	repo, _ := testutil.NewRepoWithRemote(t)
	other := filepath.Join(filepath.Dir(repo), "other.git")
	testutil.Git(t, filepath.Dir(repo), "init", "-q", "--bare", "-b", "main", other)
	testutil.Git(t, repo, "remote", "add", "upstream", other)
	testutil.Git(t, repo, "push", "-q", "origin", "main:refs/heads/shared")
	testutil.Git(t, repo, "push", "-q", "upstream", "main:refs/heads/shared")
	testutil.Git(t, repo, "fetch", "-q", "--all")
	e := newAddEnv(t, repo, nil)
	err := e.add(t, "shared")
	if err == nil || !strings.Contains(err.Error(), "multiple remotes") {
		t.Errorf("err = %v", err)
	}
}

func TestAddRunsHooksThenExec(t *testing.T) {
	repo := testutil.NewRepo(t)
	testutil.WriteFile(t, filepath.Join(repo, ".env"), "K=1")
	if err := config.SaveHooks(gitx.In(context.Background(), repo), repo, config.Hooks{PostCreate: []config.Hook{
		{Type: config.HookCopy, From: ".env", To: ".env"},
	}}); err != nil {
		t.Fatal(err)
	}
	e := newAddEnv(t, repo, nil)
	if err := e.add(t, "-b", "h", "--exec", "cat .env > exec.txt"); err != nil {
		t.Fatalf("%v\n%s", err, e.out)
	}
	for _, f := range []string{".env", "exec.txt"} {
		if b, err := os.ReadFile(filepath.Join(e.wtPath("h"), f)); err != nil || string(b) != "K=1" {
			t.Errorf("%s = %q, err = %v", f, b, err)
		}
	}
	if !strings.Contains(e.out.String(), "All hooks executed successfully") {
		t.Errorf("stdout:\n%s", e.out)
	}
}

func TestAddHookFailureIsAWarning(t *testing.T) {
	repo := testutil.NewRepo(t)
	if err := config.SaveHooks(gitx.In(context.Background(), repo), repo, config.Hooks{PostCreate: []config.Hook{
		{Type: config.HookCommand, Command: "exit 1"},
	}}); err != nil {
		t.Fatal(err)
	}
	e := newAddEnv(t, repo, nil)
	if err := e.add(t, "-b", "w", "--quiet"); err != nil {
		t.Fatalf("hook failure must not fail add: %v", err)
	}
	if !strings.Contains(e.errOut.String(), "Warning: Hook execution failed") {
		t.Errorf("stderr:\n%s", e.errOut)
	}
	if got := strings.TrimSpace(e.out.String()); got != e.wtPath("w") {
		t.Errorf("quiet stdout polluted: %q", got)
	}
}

func TestAddExecFailureKeepsWorktree(t *testing.T) {
	e := newAddEnv(t, testutil.NewRepo(t), nil)
	err := e.add(t, "-b", "x", "--exec", "exit 2")
	if err == nil || !strings.Contains(err.Error(), "--exec command failed") {
		t.Fatalf("err = %v", err)
	}
	if _, statErr := os.Stat(e.wtPath("x")); statErr != nil {
		t.Error("worktree should remain")
	}
}

func TestAddFromLinkedWorktreeUsesMainBaseDir(t *testing.T) {
	repo := testutil.NewRepo(t)
	first := newAddEnv(t, repo, nil)
	if err := first.add(t, "-b", "one"); err != nil {
		t.Fatal(err)
	}
	inner := newAddEnv(t, first.wtPath("one"), nil)
	if err := inner.add(t, "-b", "two"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(first.wtPath("two")); err != nil {
		t.Errorf("expected %s: %v", first.wtPath("two"), err)
	}
}

func TestAddCompletesBranches(t *testing.T) {
	repo := testutil.NewRepo(t)
	testutil.Git(t, repo, "branch", "topic")
	e := newAddEnv(t, repo, nil)
	root := NewRoot(e.deps, "test")
	root.SetArgs([]string{"__complete", "add", "to"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(e.out.String(), "topic") {
		t.Errorf("completions:\n%s", e.out)
	}
}
