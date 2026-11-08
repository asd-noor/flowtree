package e2e

import (
	"encoding/json"
	"strings"
	"testing"
)

type taskView struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
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
	if dry := Run(t, repo, "feature", "start", "--dry-run", "-T", "1", "login").MustSucceed(t).Stdout; !strings.Contains(dry, "board: link task #1") {
		t.Errorf("dry run should show the board step:\n%s", dry)
	}
	Run(t, repo, "feature", "start", "-T", "1", "login").MustSucceed(t)
	if v := tasks(t, repo)[1]; v.Status != "doing" || v.Branch != "feature/login" {
		t.Fatalf("after start: %+v", v)
	}
	fwt := worktreeDir(repo, "feature/login")

	Run(t, fwt, "work", "start", "api").MustSucceed(t)
	api := worktreeDir(repo, "work/feature/login/api")
	commitFile(t, api, "api.txt")
	if v := tasks(t, repo)[1]; len(v.Subtasks) != 1 || v.Subtasks[0].Status != "doing" {
		t.Fatalf("subtask after work start: %+v", v)
	}
	Run(t, api, "work", "finish").MustSucceed(t)
	if v := tasks(t, repo)[1]; v.Subtasks[0].Status != "done" {
		t.Errorf("subtask after work finish: %+v", v)
	}

	Run(t, fwt, "feature", "review").MustSucceed(t)
	if v := tasks(t, repo)[1]; v.Status != "review" {
		t.Errorf("after review: %s", v.Status)
	}
	commitFile(t, fwt, "fix.txt")
	if v := tasks(t, repo)[1]; v.Status != "doing" {
		t.Errorf("a new commit should move it back to doing: %s", v.Status)
	}
	Run(t, fwt, "feature", "review").MustSucceed(t)
	Run(t, fwt, "feature", "review", "--undo").MustSucceed(t)
	if v := tasks(t, repo)[1]; v.Status != "doing" {
		t.Errorf("after review --undo: %s", v.Status)
	}

	Run(t, repo, "feature", "finish", "login").MustSucceed(t)
	if v := tasks(t, repo)[1]; v.Status != "done" {
		t.Errorf("after finish: %s", v.Status)
	}
}

func TestBoardCreatesTasksAndRefusesBadLinks(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "feature", "start", "other").MustSucceed(t)
	if v := tasks(t, repo)[1]; v.Title != "other" || v.Status != "doing" {
		t.Fatalf("start without -T should add a task: %+v", v)
	}
	Run(t, repo, "feature", "delete", "-f", "other").MustSucceed(t)
	if v := tasks(t, repo)[1]; v.Status != "abandoned" {
		t.Errorf("deleted unmerged: %s", v.Status)
	}

	for _, args := range [][]string{{"-T", "9", "x"}, {"-T", "1", "y"}} {
		res := Run(t, repo, append([]string{"feature", "start"}, args...)...).MustFail(t)
		if !strings.Contains(res.Stderr, "task #") || strings.Contains(branches(t, repo), "feature/"+args[2]) {
			t.Errorf("%v: stderr = %q; no branch may be created", args, res.Stderr)
		}
	}
	Run(t, repo, "release", "start", "1.0.0").MustSucceed(t)
	if len(tasks(t, repo)) != 1 {
		t.Error("a release gets no task")
	}
	if res := Run(t, repo, "release", "start", "-T", "1", "2.0.0").MustFail(t); !strings.Contains(res.Stderr, "unknown shorthand") {
		t.Errorf("release start has no -T: %q", res.Stderr)
	}
}

func TestReviewMarkSurvivesARebase(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "feature", "start", "a").MustSucceed(t)
	wt := worktreeDir(repo, "feature/a")
	commitFile(t, wt, "a.txt")
	Run(t, wt, "feature", "review").MustSucceed(t)
	commitFile(t, repo, "on-develop.txt")
	Run(t, wt, "feature", "rebase").MustSucceed(t)
	if v := tasks(t, repo)[1]; v.Status != "review" {
		t.Errorf("a rebase alone must keep the task in review: %s", v.Status)
	}
}
