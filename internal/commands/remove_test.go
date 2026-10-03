package commands

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"flowtree/internal/gitx"
	"flowtree/internal/testutil"
)

// harness runs commands against a real repo with captured output.
type harness struct {
	t      *testing.T
	repo   string
	cwd    string
	env    map[string]string
	stdout bytes.Buffer
	stderr bytes.Buffer
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	repo := testutil.NewRepo(t)
	return &harness{t: t, repo: repo, cwd: repo, env: map[string]string{}}
}

func (h *harness) git(args ...string) string { return testutil.Git(h.t, h.repo, args...) }

// wt is the default worktree path for a branch (<repo>/../worktrees/<branch>).
func (h *harness) wt(branch string) string {
	return filepath.Join(filepath.Dir(h.repo), "worktrees", filepath.FromSlash(branch))
}

func (h *harness) run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	h.stdout.Reset()
	h.stderr.Reset()
	d := Deps{
		Run:    gitx.Exec,
		Stdin:  strings.NewReader(""),
		Stdout: &h.stdout,
		Stderr: &h.stderr,
		Getenv: func(k string) string { return h.env[k] },
		Cwd:    func() (string, error) { return h.cwd, nil },
		IsTTY:  func() bool { return false },
	}
	root := NewRoot(d, "test")
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	return h.stdout.String(), err
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func branchExists(h *harness, b string) bool {
	return gitx.BranchExists(gitx.In(context.Background(), h.repo), b)
}

func TestRemoveWorktreeOnly(t *testing.T) {
	h := newHarness(t)
	h.git("worktree", "add", "-q", "-b", "feat/x", h.wt("feat/x"))

	out, err := h.run(t, "remove", "feat/x")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Removed worktree 'feat/x'") {
		t.Errorf("output: %s", out)
	}
	if exists(h.wt("feat/x")) {
		t.Error("worktree directory still exists")
	}
	if !branchExists(h, "feat/x") {
		t.Error("branch must survive without --with-branch")
	}
}

func TestRemoveWithBranchMergedAndUnmerged(t *testing.T) {
	h := newHarness(t)
	h.git("worktree", "add", "-q", "-b", "merged", h.wt("merged"))
	h.git("worktree", "add", "-q", "-b", "unmerged", h.wt("unmerged"))
	testutil.WriteFile(t, filepath.Join(h.wt("unmerged"), "f.txt"), "x")
	testutil.Git(t, h.wt("unmerged"), "add", "-A")
	testutil.Git(t, h.wt("unmerged"), "commit", "-q", "-m", "work")

	if _, err := h.run(t, "remove", "--with-branch", "merged"); err != nil {
		t.Fatal(err)
	}
	if branchExists(h, "merged") {
		t.Error("merged branch should be deleted")
	}

	_, err := h.run(t, "remove", "--with-branch", "unmerged")
	if err == nil || !strings.Contains(err.Error(), "--force-branch") {
		t.Fatalf("want unmerged-branch error, got %v", err)
	}
	if !branchExists(h, "unmerged") || exists(h.wt("unmerged")) {
		t.Error("worktree should be gone but branch kept")
	}

	h.git("worktree", "add", "-q", h.wt("again"), "unmerged")
	if _, err := h.run(t, "remove", "--with-branch", "--force-branch", "unmerged"); err != nil {
		t.Fatal(err)
	}
	if branchExists(h, "unmerged") {
		t.Error("--force-branch should delete the branch")
	}
}

func TestRemoveDirtyNeedsForce(t *testing.T) {
	h := newHarness(t)
	h.git("worktree", "add", "-q", "-b", "dirty", h.wt("dirty"))
	testutil.WriteFile(t, filepath.Join(h.wt("dirty"), "untracked.txt"), "x")

	if _, err := h.run(t, "remove", "dirty"); err == nil {
		t.Fatal("expected failure on dirty worktree")
	}
	if _, err := h.run(t, "remove", "--force", "dirty"); err != nil {
		t.Fatal(err)
	}
	if exists(h.wt("dirty")) {
		t.Error("worktree should be removed with --force")
	}
}

func TestRemoveRefusals(t *testing.T) {
	h := newHarness(t)
	h.git("worktree", "add", "-q", "-b", "cur", h.wt("cur"))

	cases := []struct {
		name string
		args []string
		cwd  string
		want string
	}{
		{"no args", []string{"remove"}, h.repo, "worktree name required"},
		{"main", []string{"remove", "@"}, h.repo, "main worktree"},
		{"unknown", []string{"remove", "nope"}, h.repo, "not found"},
		{"force-branch alone", []string{"remove", "--force-branch", "cur"}, h.repo, "requires --with-branch"},
		{"standing inside", []string{"remove", "cur"}, h.wt("cur"), "you are inside it"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h.cwd = c.cwd
			_, err := h.run(t, c.args...)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want error containing %q, got %v", c.want, err)
			}
		})
	}
	if !exists(h.wt("cur")) {
		t.Error("refused removals must leave the worktree")
	}
}

func TestRemoveCurrentWithCdFile(t *testing.T) {
	h := newHarness(t)
	h.git("worktree", "add", "-q", "-b", "cur", h.wt("cur"))
	cdfile := filepath.Join(t.TempDir(), "cd")
	h.env[CdFileEnv] = cdfile
	h.cwd = h.wt("cur")

	if _, err := h.run(t, "remove", "cur"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(cdfile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != h.repo {
		t.Errorf("cd file = %q, want %q", got, h.repo)
	}
}

func TestRemoveSeveralReportsEachFailure(t *testing.T) {
	h := newHarness(t)
	h.git("worktree", "add", "-q", "-b", "a", h.wt("a"))

	_, err := h.run(t, "remove", "a", "ghost")
	if err == nil || !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("want error about ghost, got %v", err)
	}
	if exists(h.wt("a")) {
		t.Error("a should still be removed despite ghost failing")
	}
}
