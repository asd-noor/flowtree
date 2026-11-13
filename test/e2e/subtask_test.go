package e2e

import (
	"path/filepath"
	"strings"
	"testing"

	"flowtree/internal/testutil"
)

func TestWorkSubtaskLifecycle(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "feature", "start", "login").MustSucceed(t)
	fwt := worktreeDir(repo, "regular/login")
	commitFile(t, fwt, "base.txt")

	Run(t, repo, "work", "start", "-P", "1", "api").MustSucceed(t)
	Run(t, fwt, "work", "start", "ui").MustSucceed(t) // parent inferred from the feature worktree
	api, ui := worktreeDir(repo, "work/regular/login/api"), worktreeDir(repo, "work/regular/login/ui")
	if !exists(api) || !exists(ui) {
		t.Fatalf("work worktrees missing:\n%s", branches(t, repo))
	}
	commitFile(t, api, "api.txt")
	commitFile(t, fwt, "on-feature.txt") // the parent moves on: finish must rebase

	if res := Run(t, repo, "feature", "finish", "login").MustFail(t); !strings.Contains(res.Stderr, "open work branches") {
		t.Errorf("feature finish with open subtasks: stderr = %q", res.Stderr)
	}

	Run(t, api, "work", "finish", "#1/api").MustSucceed(t)
	if out := testutil.Git(t, repo, "log", "--merges", "--oneline", "regular/login"); out != "" {
		t.Errorf("work finish must rebase and fast-forward the parent, found merges:\n%s", out)
	}
	if !exists(fwt+"/api.txt") || exists(api) || strings.Contains(branches(t, repo), "work/regular/login/api") {
		t.Error("the parent worktree should have the subtask's commit; the subtask should be gone")
	}

	Run(t, repo, "work", "delete", "-f", "1/ui").MustSucceed(t)
	Run(t, repo, "feature", "finish", "login").MustSucceed(t)
	if !exists(repo + "/api.txt") {
		t.Error("the feature, with its subtask, should be on develop")
	}
}

func TestWorkStartNeedsAParent(t *testing.T) {
	repo := flowRepo(t)
	if res := Run(t, repo, "work", "start", "x").MustFail(t); !strings.Contains(res.Stderr, "no parent given") {
		t.Errorf("stderr = %q", res.Stderr)
	}
	Run(t, repo, "feature", "start", "a").MustSucceed(t)
	Run(t, repo, "task", "add", "Later").MustSucceed(t) // task 2, no branch
	for arg, want := range map[string]string{
		"regular/a": "not a task number",
		"9":         "no task #9",
		"2":         "task #2 has no branch yet; start it first",
	} {
		if res := Run(t, repo, "work", "start", "-P", arg, "x").MustFail(t); !strings.Contains(res.Stderr, want) {
			t.Errorf("-P %s: stderr = %q, want %q", arg, res.Stderr, want)
		}
	}
	if res := Run(t, repo, "work", "start", "-P", "1", "x/y").MustFail(t); !strings.Contains(res.Stderr, "must not contain '/'") {
		t.Errorf("stderr = %q", res.Stderr)
	}
}

func TestWorkStartRefusesParentWithoutTask(t *testing.T) {
	repo := flowRepo(t)
	testutil.Git(t, repo, "branch", "regular/raw", "develop") // plain git: no card
	wt := filepath.Join(t.TempDir(), "raw")
	testutil.Git(t, repo, "worktree", "add", wt, "regular/raw")
	res := Run(t, wt, "work", "start", "x").MustFail(t)
	if !strings.Contains(res.Stderr, "'regular/raw' has no task; run `flowtree task sync` first") {
		t.Errorf("stderr = %q", res.Stderr)
	}
	if strings.Contains(branches(t, repo), "work/") {
		t.Errorf("nothing should have been created:\n%s", branches(t, repo))
	}
}

func TestWorkByTaskAndSubtask(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "hotfix", "start", "1.0.1").MustSucceed(t) // task 1
	Run(t, repo, "feature", "start", "a").MustSucceed(t)    // task 2
	Run(t, repo, "work", "start", "-P", "#1", "fix").MustSucceed(t)
	Run(t, repo, "work", "start", "-P", "2", "fix").MustSucceed(t)
	hwt := worktreeDir(repo, "hotfix/1.0.1")
	commitFile(t, worktreeDir(repo, "work/hotfix/1.0.1/fix"), "fix.txt")

	for _, arg := range []string{"fix", "1", "9/fix", "1/nope"} {
		if res := Run(t, repo, "work", "finish", arg).MustFail(t); res.Stderr == "" {
			t.Errorf("finish %s should be refused", arg)
		}
	}
	if res := Run(t, repo, "work", "finish").MustFail(t); !strings.Contains(res.Stderr, "accepts 1 arg") {
		t.Errorf("the argument is required; stderr = %q", res.Stderr)
	}
	if out := Run(t, repo, "work", "checkout", "2/fix").MustSucceed(t).Stdout; !strings.Contains(out, "work/regular/a/fix") {
		t.Errorf("checkout printed %q", out)
	}
	Run(t, hwt, "work", "finish", "1/fix").MustSucceed(t)
	if !exists(hwt + "/fix.txt") {
		t.Error("the hotfix should have the subtask's commit")
	}
	Run(t, repo, "work", "delete", "-f", "2/fix").MustSucceed(t)
	if strings.Contains(branches(t, repo), "work/") {
		t.Errorf("work branches should be gone:\n%s", branches(t, repo))
	}
	if res := Run(t, repo, "work", "publish", "1/fix").MustFail(t); !strings.Contains(res.Stderr, "unknown command") {
		t.Errorf("work branches are local only; stderr = %q", res.Stderr)
	}
}
