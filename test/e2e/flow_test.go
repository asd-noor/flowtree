package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"flowtree/internal/testutil"
)

// flowRepo is a repository set up with `init --flow` (production branch "main").
func flowRepo(t *testing.T) string {
	t.Helper()
	repo := testutil.NewRepo(t)
	Run(t, repo, "init", "--flow", "-d").MustSucceed(t)
	return repo
}

// flowRepoWithRemote also has a bare origin carrying main and develop.
func flowRepoWithRemote(t *testing.T) (repo, remote string) {
	t.Helper()
	repo, remote = testutil.NewRepoWithRemote(t)
	Run(t, repo, "init", "--flow", "-d").MustSucceed(t)
	testutil.Git(t, repo, "push", "-q", "origin", "develop")
	return repo, remote
}

func worktreeDir(repo, branch string) string {
	return filepath.Join(filepath.Dir(repo), "worktrees", filepath.FromSlash(branch))
}

func commitFile(t *testing.T, dir, name string) {
	t.Helper()
	testutil.WriteFile(t, filepath.Join(dir, name), name+"\n")
	testutil.Git(t, dir, "add", "-A")
	testutil.Git(t, dir, "commit", "-q", "-m", "add "+name)
}

func branches(t *testing.T, repo string) string {
	return testutil.Git(t, repo, "branch", "--format=%(refname:short)")
}

func exists(path string) bool { _, err := os.Stat(path); return err == nil }

func current(t *testing.T, dir string) string {
	return strings.TrimSpace(testutil.Git(t, dir, "branch", "--show-current"))
}

func TestWorkLifecycle(t *testing.T) {
	repo := flowRepo(t)
	res := Run(t, repo, "work", "start", "login").MustSucceed(t)
	wt := worktreeDir(repo, "work/login")
	if !exists(wt) || !strings.Contains(res.Stdout, wt) {
		t.Fatalf("worktree not created at %s:\n%s", wt, res.Stdout)
	}
	if got := Run(t, repo, "work", "list", "-v").MustSucceed(t).Stdout; !strings.Contains(got, "login") || !strings.Contains(got, "no commits yet") {
		t.Errorf("list: %q", got)
	}
	commitFile(t, wt, "login.txt")

	cdfile := filepath.Join(t.TempDir(), "cd")
	res = RunEnv(t, wt, []string{"FLOWTREE_CDFILE=" + cdfile}, "work", "finish").MustSucceed(t) // name inferred from cwd
	if exists(wt) || strings.Contains(branches(t, repo), "work/login") {
		t.Errorf("worktree or branch left behind:\n%s\n%s", res.Stdout, branches(t, repo))
	}
	if !exists(filepath.Join(repo, "login.txt")) || current(t, repo) != "develop" {
		t.Error("work was not merged into develop in the main worktree")
	}
	if cd, _ := os.ReadFile(cdfile); strings.TrimSpace(string(cd)) != repo {
		t.Errorf("shell should be sent to the main worktree, got %q", cd)
	}
}

func TestWorkFinishModes(t *testing.T) {
	for flag, parents := range map[string]int{"": 1, "--no-ff": 2, "-S": 1} {
		repo := flowRepo(t)
		Run(t, repo, "work", "start", "a").MustSucceed(t)
		wt := worktreeDir(repo, "work/a")
		commitFile(t, wt, "a1")
		commitFile(t, wt, "a2")
		args := []string{"work", "finish", "a"}
		if flag != "" {
			args = append(args, flag)
		}
		Run(t, repo, args...).MustSucceed(t)
		got := len(strings.Fields(testutil.Git(t, repo, "rev-list", "--parents", "-n1", "develop"))) - 1
		if got != parents {
			t.Errorf("%q: develop head has %d parents, want %d", flag, got, parents)
		}
	}
}

func TestWorkFinishRebase(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "work", "start", "a").MustSucceed(t)
	wt := worktreeDir(repo, "work/a")
	commitFile(t, wt, "a")
	commitFile(t, repo, "on-develop") // develop moved on
	Run(t, repo, "work", "finish", "a", "-r").MustSucceed(t)
	if out := testutil.Git(t, repo, "log", "--merges", "--oneline", "develop"); out != "" {
		t.Errorf("rebased finish should be linear:\n%s", out)
	}
	if !exists(filepath.Join(repo, "a")) || !exists(filepath.Join(repo, "on-develop")) {
		t.Error("both changes must be on develop")
	}
}

func TestStartRefusals(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "work", "start", "x").MustSucceed(t)
	if res := Run(t, repo, "work", "start", "x").MustFail(t); !strings.Contains(res.Stderr, "already exists") {
		t.Errorf("stderr = %q", res.Stderr)
	}
	Run(t, repo, "release", "start", "1.0.0").MustSucceed(t)
	if res := Run(t, repo, "release", "start", "1.1.0").MustFail(t); !strings.Contains(res.Stderr, "finish that one first") {
		t.Errorf("stderr = %q", res.Stderr)
	}
	if res := Run(t, repo, "work", "start").MustFail(t); res.Stderr == "" {
		t.Error("missing name should be reported")
	}
}

func TestFlowNeedsInit(t *testing.T) {
	repo := testutil.NewRepo(t)
	Run(t, repo, "init", "-d").MustSucceed(t)
	if res := Run(t, repo, "work", "list").MustFail(t); !strings.Contains(res.Stderr, "init --flow") {
		t.Errorf("stderr = %q", res.Stderr)
	}
}

func TestReleaseLifecycle(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "release", "start", "1.0.0").MustSucceed(t)
	if current(t, repo) != "release/1.0.0" {
		t.Fatalf("main worktree should be on the release, is on %q", current(t, repo))
	}
	commitFile(t, repo, "CHANGELOG.md")
	Run(t, repo, "release", "finish", "-m", "Release 1.0.0").MustSucceed(t)

	if current(t, repo) != "develop" || strings.Contains(branches(t, repo), "release/1.0.0") {
		t.Errorf("main should be back on develop with the release deleted; on %q\n%s", current(t, repo), branches(t, repo))
	}
	if got := testutil.Git(t, repo, "tag", "-l", "1.0.0"); !strings.Contains(got, "1.0.0") {
		t.Fatal("tag missing")
	}
	if got := testutil.Git(t, repo, "cat-file", "-t", "1.0.0"); strings.TrimSpace(got) != "tag" {
		t.Errorf("tag should be annotated, is %q", got)
	}
	for _, b := range []string{"main", "develop"} {
		if out := testutil.Git(t, repo, "ls-tree", "--name-only", b); !strings.Contains(out, "CHANGELOG.md") {
			t.Errorf("%s lacks the release changes", b)
		}
	}
	Run(t, repo, "release", "list").MustSucceed(t) // no release left; just must not fail
}

func TestReleaseVersionTagPrefixAndFlags(t *testing.T) {
	repo := testutil.NewRepo(t)
	Run(t, repo, "init", "--flow", "-d", "-t", "v").MustSucceed(t)
	Run(t, repo, "release", "start", "2.0.0").MustSucceed(t)
	commitFile(t, repo, "x")
	Run(t, repo, "release", "finish", "-n", "-b", "-k").MustSucceed(t) // no tag, no back-merge, keep branch
	if testutil.Git(t, repo, "tag", "-l") != "" {
		t.Error("-n must not tag")
	}
	if !strings.Contains(branches(t, repo), "release/2.0.0") {
		t.Error("-k must keep the branch")
	}
	if current(t, repo) != "develop" {
		t.Error("main worktree should be back on develop")
	}
	if strings.Contains(testutil.Git(t, repo, "ls-tree", "--name-only", "develop"), "x") {
		t.Error("-b must not merge into develop")
	}
}

func TestReleaseTagGetsPrefix(t *testing.T) {
	repo := testutil.NewRepo(t)
	Run(t, repo, "init", "--flow", "-d", "-t", "v").MustSucceed(t)
	Run(t, repo, "release", "start", "2.0.0").MustSucceed(t)
	commitFile(t, repo, "x")
	Run(t, repo, "release", "finish", "-m", "m").MustSucceed(t)
	if !strings.Contains(testutil.Git(t, repo, "tag", "-l"), "v2.0.0") {
		t.Error("tag should carry the version tag prefix")
	}
}

func TestHotfixLifecycleWithPush(t *testing.T) {
	repo, remote := flowRepoWithRemote(t)
	Run(t, repo, "hotfix", "start", "1.0.1").MustSucceed(t)
	wt := worktreeDir(repo, "hotfix/1.0.1")
	if !exists(wt) {
		t.Fatal("hotfix worktree missing")
	}
	commitFile(t, wt, "fix.txt")
	Run(t, wt, "hotfix", "finish", "-p", "-m", "fix").MustSucceed(t)

	if exists(wt) || strings.Contains(branches(t, repo), "hotfix/1.0.1") {
		t.Error("hotfix worktree or branch left behind")
	}
	for _, b := range []string{"main", "develop"} {
		if !strings.Contains(testutil.Git(t, remote, "ls-tree", "--name-only", b), "fix.txt") {
			t.Errorf("remote %s lacks the hotfix", b)
		}
	}
	if !strings.Contains(testutil.Git(t, remote, "tag", "-l"), "1.0.1") {
		t.Error("tag was not pushed")
	}
}

func TestFinishConflictChangesNothing(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "work", "start", "a").MustSucceed(t)
	wt := worktreeDir(repo, "work/a")
	testutil.WriteFile(t, filepath.Join(wt, "README.md"), "from work\n")
	testutil.Git(t, wt, "commit", "-q", "-am", "work edit")
	testutil.WriteFile(t, filepath.Join(repo, "README.md"), "from develop\n")
	testutil.Git(t, repo, "commit", "-q", "-am", "develop edit")
	before := testutil.Git(t, repo, "rev-parse", "develop")

	res := Run(t, repo, "work", "finish", "a", "--no-ff").MustFail(t)
	if !strings.Contains(res.Stderr, "Nothing was changed") {
		t.Errorf("stderr = %q", res.Stderr)
	}
	if testutil.Git(t, repo, "rev-parse", "develop") != before || testutil.Git(t, repo, "status", "--porcelain", "--untracked-files=no") != "" || !exists(wt) {
		t.Error("state changed after a failed finish")
	}
}

func TestFinishRefusesDirtyWorktree(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "work", "start", "a").MustSucceed(t)
	wt := worktreeDir(repo, "work/a")
	testutil.WriteFile(t, filepath.Join(wt, "README.md"), "dirty\n")
	if res := Run(t, repo, "work", "finish", "a").MustFail(t); !strings.Contains(res.Stderr, "uncommitted") {
		t.Errorf("stderr = %q", res.Stderr)
	}
}

func TestDryRunChangesNothing(t *testing.T) {
	repo := flowRepo(t)
	res := Run(t, repo, "work", "start", "a", "--dry-run").MustSucceed(t)
	if !strings.Contains(res.Stdout, "Dry run") || strings.Contains(branches(t, repo), "work/a") || exists(worktreeDir(repo, "work/a")) {
		t.Errorf("start --dry-run changed things or printed nothing:\n%s", res.Stdout)
	}
	Run(t, repo, "work", "start", "b").MustSucceed(t)
	commitFile(t, worktreeDir(repo, "work/b"), "b")
	Run(t, repo, "work", "finish", "b", "--dry-run").MustSucceed(t)
	if !exists(worktreeDir(repo, "work/b")) || exists(filepath.Join(repo, "b")) {
		t.Error("finish --dry-run changed things")
	}
	Run(t, repo, "work", "delete", "b", "-f", "--dry-run").MustSucceed(t)
	if !strings.Contains(branches(t, repo), "work/b") {
		t.Error("delete --dry-run changed things")
	}
}

func TestDelete(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "work", "start", "a").MustSucceed(t)
	wt := worktreeDir(repo, "work/a")
	commitFile(t, wt, "a")
	if res := Run(t, repo, "work", "delete", "a").MustFail(t); !strings.Contains(res.Stderr, "not been merged") {
		t.Errorf("stderr = %q", res.Stderr)
	}
	Run(t, repo, "work", "delete", "-f", "a").MustSucceed(t)
	if exists(wt) || strings.Contains(branches(t, repo), "work/a") {
		t.Error("delete -f should remove worktree and branch")
	}
}

func TestPublishTrackCheckout(t *testing.T) {
	repo, remote := flowRepoWithRemote(t)
	Run(t, repo, "work", "start", "feat").MustSucceed(t)
	Run(t, repo, "work", "publish", "feat").MustSucceed(t)
	if !strings.Contains(testutil.Git(t, remote, "branch", "--list"), "work/feat") {
		t.Fatal("publish did not push")
	}
	Run(t, repo, "work", "finish", "feat", "-k", "--keepremote").MustSucceed(t) // branch kept, worktree removed
	wt := worktreeDir(repo, "work/feat")
	if exists(wt) {
		t.Fatal("finish -k should still remove the worktree")
	}
	res := Run(t, repo, "work", "checkout", "feat").MustSucceed(t) // recreates it
	if strings.TrimSpace(res.Stdout) != wt || !exists(wt) {
		t.Errorf("checkout printed %q, want %s", res.Stdout, wt)
	}
	Run(t, repo, "work", "finish", "feat").MustSucceed(t)
	if strings.Contains(testutil.Git(t, remote, "branch", "--list"), "work/feat") {
		t.Error("finish should delete the remote branch")
	}

	testutil.Git(t, repo, "push", "-q", "origin", "develop:work/other")
	Run(t, repo, "work", "track", "other").MustSucceed(t)
	if !exists(worktreeDir(repo, "work/other")) {
		t.Error("track should create a worktree")
	}
}

func TestReleaseCheckoutSwitchesMain(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "release", "start", "1.0.0").MustSucceed(t)
	testutil.Git(t, repo, "checkout", "-q", "develop")
	res := Run(t, repo, "release", "checkout").MustFail(t) // not on a release, no name
	if res.Stderr == "" {
		t.Error("expected an error")
	}
	Run(t, repo, "release", "checkout", "1.0.0").MustSucceed(t)
	if current(t, repo) != "release/1.0.0" {
		t.Errorf("main on %q", current(t, repo))
	}
}

func TestReleaseFinishResumesAfterBackMergeConflict(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "release", "start", "1.0.0").MustSucceed(t)
	testutil.WriteFile(t, filepath.Join(repo, "README.md"), "release notes\n")
	testutil.Git(t, repo, "commit", "-q", "-am", "release edit")
	// develop moves on with a conflicting edit while the release is open (done in a side worktree)
	side := filepath.Join(t.TempDir(), "side")
	testutil.Git(t, repo, "worktree", "add", "-q", side, "develop")
	testutil.WriteFile(t, filepath.Join(side, "README.md"), "develop notes\n")
	testutil.Git(t, side, "commit", "-q", "-am", "develop edit")
	testutil.Git(t, repo, "worktree", "remove", "--force", side)

	res := Run(t, repo, "release", "finish", "-m", "1.0.0").MustFail(t)
	if !strings.Contains(res.Stderr, "by hand") || !strings.Contains(testutil.Git(t, repo, "tag", "-l"), "1.0.0") {
		t.Fatalf("expected a back-merge failure after tagging; stderr = %q", res.Stderr)
	}

	// resolve by hand, as the error says: merge the tag into develop
	testutil.Git(t, repo, "worktree", "add", "-q", side, "develop")
	testutil.Git(t, side, "merge", "-q", "-X", "ours", "--no-edit", "1.0.0")
	testutil.Git(t, repo, "worktree", "remove", "--force", side)

	res = Run(t, repo, "release", "finish", "-m", "1.0.0").MustSucceed(t)
	if !strings.Contains(res.Stdout, "Resuming") {
		t.Errorf("stdout = %q", res.Stdout)
	}
	if current(t, repo) != "develop" || strings.Contains(branches(t, repo), "release/1.0.0") {
		t.Errorf("finish should complete: main on %q\n%s", current(t, repo), branches(t, repo))
	}
}

func TestFlowCompletion(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "work", "start", "login").MustSucceed(t)
	Run(t, repo, "work", "start", "signup").MustSucceed(t)
	out := Run(t, repo, "__complete", "work", "finish", "lo").MustSucceed(t).Stdout
	if !strings.Contains(out, "login") || strings.Contains(out, "signup") {
		t.Errorf("work finish completion: %q", out)
	}
	out = Run(t, repo, "__complete", "hotfix", "").MustSucceed(t).Stdout
	if !strings.Contains(out, "start") {
		t.Errorf("subcommand completion: %q", out)
	}
}
