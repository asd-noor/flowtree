package e2e

import (
	"encoding/json"
	"strings"
	"testing"

	"flowtree/internal/testutil"
)

type taskView struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Type     string `json:"type"`
	Branch   string `json:"branch"`
	Status   string `json:"status"`
	Subtasks []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	} `json:"subtasks"`
}

func tasks(t *testing.T, repo string) map[int]taskView {
	t.Helper()
	var vs []taskView
	if err := json.Unmarshal([]byte(Run(t, repo, "task", "list", "--json").MustSucceed(t).Stdout), &vs); err != nil {
		t.Fatal(err)
	}
	out := map[int]taskView{}
	for _, v := range vs {
		out[v.ID] = v
	}
	return out
}

func TestBoardFollowsTheFlow(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "task", "add", "Login page").MustSucceed(t)
	if dry := Run(t, repo, "task", "start", "--dry-run", "1", "login").MustSucceed(t).Stdout; !strings.Contains(dry, "board: link task #1") {
		t.Errorf("dry run should show the board step:\n%s", dry)
	}
	Run(t, repo, "task", "start", "1", "login").MustSucceed(t)
	if v := tasks(t, repo)[1]; v.Status != "doing" || v.Branch != "regular/login" {
		t.Fatalf("after start: %+v", v)
	}
	fwt := worktreeDir(repo, "regular/login")

	Run(t, fwt, "work", "start", "api").MustSucceed(t)
	api := worktreeDir(repo, "work/regular/login/api")
	commitFile(t, api, "api.txt")
	if v := tasks(t, repo)[1]; len(v.Subtasks) != 1 || v.Subtasks[0].Status != "doing" {
		t.Fatalf("subtask after work start: %+v", v)
	}
	Run(t, repo, "work", "finish", "1/api").MustSucceed(t)
	if v := tasks(t, repo)[1]; v.Subtasks[0].Status != "done" {
		t.Errorf("subtask after work finish: %+v", v)
	}

	Run(t, fwt, "task", "review").MustSucceed(t)
	if v := tasks(t, repo)[1]; v.Status != "review" {
		t.Errorf("after review: %s", v.Status)
	}
	commitFile(t, fwt, "fix.txt")
	if v := tasks(t, repo)[1]; v.Status != "doing" {
		t.Errorf("a new commit should move it back to doing: %s", v.Status)
	}
	Run(t, fwt, "task", "review").MustSucceed(t)
	Run(t, fwt, "task", "review", "--undo").MustSucceed(t)
	if v := tasks(t, repo)[1]; v.Status != "doing" {
		t.Errorf("after review --undo: %s", v.Status)
	}

	Run(t, repo, "task", "finish", "1").MustSucceed(t)
	if v := tasks(t, repo)[1]; v.Status != "done" {
		t.Errorf("after finish: %s", v.Status)
	}
}

func TestBoardRefusesBadLinksAndReleasesGetNoTask(t *testing.T) {
	repo := flowRepo(t)
	startRegular(t, repo, "other")
	Run(t, repo, "task", "delete", "-f", "1").MustSucceed(t)
	if v := tasks(t, repo)[1]; v.Status != "abandoned" {
		t.Errorf("deleted unmerged: %s", v.Status)
	}

	for _, args := range [][]string{{"9", "x"}, {"1", "y"}} {
		res := Run(t, repo, append([]string{"task", "start"}, args...)...).MustFail(t)
		if !strings.Contains(res.Stderr, "task #") || strings.Contains(branches(t, repo), "regular/"+args[1]) {
			t.Errorf("%v: stderr = %q; no branch may be created", args, res.Stderr)
		}
	}
	Run(t, repo, "release", "start", "1.0.0").MustSucceed(t)
	if len(tasks(t, repo)) != 1 {
		t.Error("a release gets no task")
	}
}

func TestReviewMarkSurvivesARebase(t *testing.T) {
	repo := flowRepo(t)
	startRegular(t, repo, "a")
	wt := worktreeDir(repo, "regular/a")
	commitFile(t, wt, "a.txt")
	Run(t, wt, "task", "review").MustSucceed(t)
	commitFile(t, repo, "on-develop.txt")
	Run(t, wt, "task", "rebase").MustSucceed(t)
	if v := tasks(t, repo)[1]; v.Status != "review" {
		t.Errorf("a rebase alone must keep the task in review: %s", v.Status)
	}
}

func TestSubtaskBriefAndPrompt(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "task", "add", "-d", "Email and password.", "Login").MustSucceed(t)
	Run(t, repo, "task", "start", "1", "login").MustSucceed(t)
	Run(t, repo, "work", "start", "-P", "1", "-d", "POST /login", "api").MustSucceed(t)
	Run(t, repo, "task", "edit", "1/api", "-d", "POST /login, returns a JWT").MustSucceed(t)
	if res := Run(t, repo, "task", "edit", "1/api", "--title", "x").MustFail(t); !strings.Contains(res.Stderr, "only a description") {
		t.Errorf("stderr = %q", res.Stderr)
	}
	if res := Run(t, repo, "task", "edit", "1/nope", "-d", "x").MustFail(t); !strings.Contains(res.Stderr, "no subtask") {
		t.Errorf("stderr = %q", res.Stderr)
	}
	out := Run(t, repo, "task", "prompt", "1/api").MustSucceed(t).Stdout
	for _, want := range []string{"Email and password.", "## Subtask: api", "returns a JWT", worktreeDir(repo, "work/regular/login/api")} {
		if !strings.Contains(out, want) {
			t.Errorf("prompt lacks %q:\n%s", want, out)
		}
	}
	if show := Run(t, repo, "task", "show", "1").MustSucceed(t).Stdout; !strings.Contains(show, "returns a JWT") {
		t.Errorf("show should list the subtask's description:\n%s", show)
	}
}

func TestTaskType(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "task", "add", "Login").MustSucceed(t)
	Run(t, repo, "task", "add", "--type", "hotfix", "Crash").MustSucceed(t)
	if got := tasks(t, repo); got[1].Type != "regular" || got[2].Type != "hotfix" {
		t.Fatalf("types after add: %+v", got)
	}

	// the version of a hotfix task becomes its branch: the type decides the kind, so no mismatch can be asked for
	Run(t, repo, "task", "start", "2", "1.0.1").MustSucceed(t)
	if res := Run(t, repo, "task", "edit", "2", "--type", "regular").MustFail(t); !strings.Contains(res.Stderr, "already has a branch") {
		t.Errorf("edit --type with a branch: %q", res.Stderr)
	}

	// a branch made with plain git: sync types it by prefix
	testutil.Git(t, repo, "branch", "regular/auto", "develop")
	testutil.Git(t, repo, "branch", "hotfix/9.9.9", "main")
	Run(t, repo, "task", "sync").MustSucceed(t)
	byBranch := map[string]string{}
	for _, v := range tasks(t, repo) {
		byBranch[v.Branch] = v.Type
	}
	for branch, want := range map[string]string{"regular/auto": "regular", "hotfix/9.9.9": "hotfix"} {
		if byBranch[branch] != want {
			t.Errorf("%s typed %q, want %q", branch, byBranch[branch], want)
		}
	}
}
