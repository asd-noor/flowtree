package e2e

import (
	"strings"
	"testing"

	"flowtree/internal/testutil"
)

func TestTaskStartRegularAndHotfix(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "task", "add", "Login").MustSucceed(t)
	Run(t, repo, "task", "add", "--type", "hotfix", "Crash").MustSucceed(t)

	out := Run(t, repo, "task", "start", "#1", "login").MustSucceed(t).Stdout
	if !strings.Contains(out, "flowtree task finish 1") {
		t.Errorf("no next-step hint:\n%s", out)
	}
	if v := tasks(t, repo)[1]; v.Branch != "regular/login" || v.Status != "doing" || len(tasks(t, repo)) != 2 {
		t.Errorf("regular: %+v", v)
	}
	if !exists(worktreeDir(repo, "regular/login")) {
		t.Error("worktree missing")
	}

	Run(t, repo, "task", "start", "2", "1.0.1").MustSucceed(t)
	if v := tasks(t, repo)[2]; v.Branch != "hotfix/1.0.1" || v.Type != "hotfix" {
		t.Errorf("hotfix: %+v", v)
	}
	if !exists(worktreeDir(repo, "hotfix/1.0.1")) {
		t.Error("hotfix worktree missing")
	}
}

func TestTaskStartBaseBacklogAndDryRun(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "task", "add", "--backlog", "Parked").MustSucceed(t)
	if v := tasks(t, repo)[1]; v.Status != "backlog" {
		t.Fatalf("status = %s", v.Status)
	}
	before := branches(t, repo)
	if dry := Run(t, repo, "task", "start", "--dry-run", "1", "p").MustSucceed(t).Stdout; !strings.Contains(dry, "regular/p") {
		t.Errorf("dry run:\n%s", dry)
	}
	if branches(t, repo) != before || tasks(t, repo)[1].Branch != "" {
		t.Error("dry run changed something")
	}

	testutil.Git(t, repo, "branch", "base", "main")
	Run(t, repo, "task", "start", "1", "p", "base").MustSucceed(t)
	if v := tasks(t, repo)[1]; v.Branch != "regular/p" || v.Status != "doing" {
		t.Errorf("after start: %+v", v)
	}
}

func TestTaskStartRefusals(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "task", "add", "A").MustSucceed(t)
	Run(t, repo, "task", "add", "B").MustSucceed(t)
	Run(t, repo, "task", "add", "--type", "hotfix", "C").MustSucceed(t)
	Run(t, repo, "task", "start", "1", "a").MustSucceed(t)
	testutil.Git(t, repo, "branch", "regular/b")
	testutil.Git(t, repo, "tag", "1.0.1")

	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"9", "x"}, "no task #9"},
		{[]string{"1", "other"}, "already belongs"},
		{[]string{"2", "b"}, "already exists"},
		{[]string{"3", "1.0.1"}, "tag"},
		{[]string{"3"}, "accepts between 2 and 3"},
	} {
		res := Run(t, repo, append([]string{"task", "start"}, c.args...)...).MustFail(t)
		if !strings.Contains(res.Stderr, c.want) {
			t.Errorf("%v: stderr = %q, want %q", c.args, res.Stderr, c.want)
		}
	}
	if len(tasks(t, repo)) != 3 {
		t.Error("task start created a card")
	}
}

func TestTaskStartRemoteBranchPointsToTrack(t *testing.T) {
	repo, _ := flowRepoWithRemote(t)
	Run(t, repo, "task", "add", "A").MustSucceed(t)
	testutil.Git(t, repo, "push", "-q", "origin", "develop:regular/a")
	res := Run(t, repo, "task", "start", "1", "a").MustFail(t)
	if !strings.Contains(res.Stderr, "flowtree task track 1 regular/a") {
		t.Errorf("stderr = %q", res.Stderr)
	}
}
