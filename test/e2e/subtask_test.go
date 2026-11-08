package e2e

import (
	"strings"
	"testing"

	"flowtree/internal/testutil"
)

func TestWorkSubtaskLifecycle(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "feature", "start", "login").MustSucceed(t)
	fwt := worktreeDir(repo, "feature/login")
	commitFile(t, fwt, "base.txt")

	Run(t, repo, "work", "start", "-P", "feature/login", "api").MustSucceed(t)
	Run(t, fwt, "work", "start", "ui").MustSucceed(t) // parent inferred from the feature worktree
	api, ui := worktreeDir(repo, "work/feature/login/api"), worktreeDir(repo, "work/feature/login/ui")
	if !exists(api) || !exists(ui) {
		t.Fatalf("work worktrees missing:\n%s", branches(t, repo))
	}
	commitFile(t, api, "api.txt")
	commitFile(t, fwt, "on-feature.txt") // the parent moves on: finish must rebase

	if res := Run(t, repo, "feature", "finish", "login").MustFail(t); !strings.Contains(res.Stderr, "open work branches") {
		t.Errorf("feature finish with open subtasks: stderr = %q", res.Stderr)
	}

	Run(t, api, "work", "finish").MustSucceed(t) // name inferred from cwd
	if out := testutil.Git(t, repo, "log", "--merges", "--oneline", "feature/login"); out != "" {
		t.Errorf("work finish must rebase and fast-forward the parent, found merges:\n%s", out)
	}
	if !exists(fwt+"/api.txt") || exists(api) || strings.Contains(branches(t, repo), "work/feature/login/api") {
		t.Error("the parent worktree should have the subtask's commit; the subtask should be gone")
	}

	Run(t, repo, "work", "delete", "-f", "ui").MustSucceed(t) // bare name, unique across parents
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
	Run(t, repo, "release", "start", "1.0.0").MustSucceed(t)
	for _, parent := range []string{"release/1.0.0", "feature/missing", "develop"} {
		if res := Run(t, repo, "work", "start", "-P", parent, "x").MustFail(t); res.Stderr == "" {
			t.Errorf("-P %s should be refused", parent)
		}
	}
	Run(t, repo, "feature", "start", "a").MustSucceed(t)
	if res := Run(t, repo, "work", "start", "-P", "feature/a", "x/y").MustFail(t); !strings.Contains(res.Stderr, "must not contain '/'") {
		t.Errorf("stderr = %q", res.Stderr)
	}
}

func TestWorkUnderHotfixAndAmbiguousNames(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "hotfix", "start", "1.0.1").MustSucceed(t)
	Run(t, repo, "feature", "start", "a").MustSucceed(t)
	Run(t, repo, "work", "start", "-P", "hotfix/1.0.1", "fix").MustSucceed(t)
	Run(t, repo, "work", "start", "-P", "feature/a", "fix").MustSucceed(t)
	if res := Run(t, repo, "work", "finish", "fix").MustFail(t); !strings.Contains(res.Stderr, "multiple work branches") {
		t.Errorf("stderr = %q", res.Stderr)
	}
	hwt := worktreeDir(repo, "hotfix/1.0.1")
	commitFile(t, worktreeDir(repo, "work/hotfix/1.0.1/fix"), "fix.txt")
	Run(t, hwt, "work", "finish", "fix").MustSucceed(t) // inside the hotfix worktree "fix" means its subtask
	if !exists(hwt + "/fix.txt") {
		t.Error("the hotfix should have the subtask's commit")
	}
	if res := Run(t, repo, "work", "publish", "fix").MustFail(t); !strings.Contains(res.Stderr, "unknown command") {
		t.Errorf("work branches are local only; stderr = %q", res.Stderr)
	}
}
