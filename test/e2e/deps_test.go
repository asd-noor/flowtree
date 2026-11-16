package e2e

import (
	"encoding/json"
	"strings"
	"testing"
)

type depView struct {
	ID       int    `json:"id"`
	Status   string `json:"status"`
	Needs    []int  `json:"needs"`
	NeededBy []int  `json:"needed_by"`
	Blocked  bool   `json:"blocked"`
}

func depTasks(t *testing.T, repo string, extra ...string) map[int]depView {
	t.Helper()
	var vs []depView
	args := append([]string{"task", "list", "--json"}, extra...)
	if err := json.Unmarshal([]byte(Run(t, repo, args...).MustSucceed(t).Stdout), &vs); err != nil {
		t.Fatal(err)
	}
	out := map[int]depView{}
	for _, v := range vs {
		out[v.ID] = v
	}
	return out
}

// finishTask starts task id as branch name, commits and finishes it, so it is done.
func finishTask(t *testing.T, repo string, id, name string) {
	t.Helper()
	Run(t, repo, "task", "start", id, name).MustSucceed(t)
	commitFile(t, worktreeDir(repo, "regular/"+name), name+".txt")
	Run(t, repo, "task", "finish", id).MustSucceed(t)
}

func TestNeedsAddEditAndRefusals(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "task", "add", "A").MustSucceed(t)
	Run(t, repo, "task", "add", "B").MustSucceed(t)
	Run(t, repo, "task", "add", "--needs", "#2, 1,1", "C").MustSucceed(t)
	if v := depTasks(t, repo)[3]; len(v.Needs) != 2 || v.Needs[0] != 1 || v.Needs[1] != 2 || !v.Blocked {
		t.Errorf("add --needs: %+v", v)
	}
	if v := depTasks(t, repo)[1]; len(v.NeededBy) != 1 || v.NeededBy[0] != 3 {
		t.Errorf("needed_by: %+v", v)
	}

	Run(t, repo, "task", "edit", "--needs", "2", "3").MustSucceed(t)
	if v := depTasks(t, repo)[3]; len(v.Needs) != 1 || v.Needs[0] != 2 {
		t.Errorf("edit --needs replaces: %+v", v)
	}
	Run(t, repo, "task", "edit", "--needs", "", "3").MustSucceed(t)
	if v := depTasks(t, repo)[3]; len(v.Needs) != 0 || v.Blocked {
		t.Errorf("edit --needs \"\" clears: %+v", v)
	}

	Run(t, repo, "task", "edit", "--needs", "3", "1").MustSucceed(t)
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"edit", "--needs", "9", "1"}, "no task #9"},
		{[]string{"edit", "--needs", "1", "1"}, "cannot need itself"},
		{[]string{"edit", "--needs", "1", "3"}, "#3 -> #1 -> #3"},
		{[]string{"add", "--needs", "9", "D"}, "no task #9"},
		{[]string{"edit", "--needs", "x", "1"}, "not a task number"},
	} {
		res := Run(t, repo, append([]string{"task"}, c.args...)...).MustFail(t)
		if !strings.Contains(res.Stderr, c.want) {
			t.Errorf("%v: %q, want %q", c.args, res.Stderr, c.want)
		}
	}
	if len(depTasks(t, repo)) != 3 {
		t.Error("a refused add created a card")
	}
}

func TestNeedsListShowReady(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "task", "add", "A").MustSucceed(t)
	Run(t, repo, "task", "add", "--needs", "1", "B").MustSucceed(t)
	Run(t, repo, "task", "add", "C").MustSucceed(t)

	list := Run(t, repo, "task", "list").MustSucceed(t).Stdout
	if lines := strings.Split(list, "\n"); strings.Contains(lines[0], "blocked") || !strings.Contains(lines[1], "[blocked]") {
		t.Errorf("list markers:\n%s", list)
	}
	ready := Run(t, repo, "task", "list", "--ready").MustSucceed(t).Stdout
	if !strings.Contains(ready, "A") || strings.Contains(ready, "B") || !strings.Contains(ready, "C") {
		t.Errorf("--ready:\n%s", ready)
	}
	show := Run(t, repo, "task", "show", "2").MustSucceed(t).Stdout
	if !strings.Contains(show, "Needs:\n  #1") || !strings.Contains(show, "(no branch)") {
		t.Errorf("show B:\n%s", show)
	}
	if show := Run(t, repo, "task", "show", "1").MustSucceed(t).Stdout; !strings.Contains(show, "Needed by:\n  #2") {
		t.Errorf("show A:\n%s", show)
	}

	finishTask(t, repo, "1", "a")
	if v := depTasks(t, repo)[2]; v.Blocked {
		t.Errorf("B still blocked after A is done: %+v", v)
	}
	if v := depTasks(t, repo, "--ready"); len(v) != 2 || v[2].Status != "todo" {
		t.Errorf("--ready --json: %+v", v)
	}
}

func TestNeedsPromptMergeHint(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "task", "add", "A").MustSucceed(t)
	Run(t, repo, "task", "add", "B").MustSucceed(t)
	Run(t, repo, "task", "add", "--needs", "1,2", "C").MustSucceed(t)
	Run(t, repo, "task", "start", "1", "a").MustSucceed(t)

	p := Run(t, repo, "task", "prompt", "3").MustSucceed(t).Stdout
	for _, want := range []string{"## Needs", "#1 (doing)", "git merge regular/a", "#2 (todo): not started"} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt lacks %q:\n%s", want, p)
		}
	}
	if p := Run(t, repo, "task", "prompt", "1").MustSucceed(t).Stdout; strings.Contains(p, "## Needs") {
		t.Errorf("task without needs got a section:\n%s", p)
	}
}

func TestNeedsWarnOnStartAndFinish(t *testing.T) {
	repo := flowRepo(t)
	Run(t, repo, "task", "add", "A").MustSucceed(t)
	Run(t, repo, "task", "add", "--needs", "1", "B").MustSucceed(t)

	res := Run(t, repo, "task", "start", "2", "b").MustSucceed(t)
	if !strings.Contains(res.Stderr, "Warning: task #2 has unfinished needs: #1 (todo)") {
		t.Errorf("start stderr: %q", res.Stderr)
	}
	commitFile(t, worktreeDir(repo, "regular/b"), "b.txt")
	res = Run(t, repo, "task", "finish", "2").MustSucceed(t)
	if !strings.Contains(res.Stderr, "unfinished needs: #1") {
		t.Errorf("finish stderr: %q", res.Stderr)
	}
	if v := depTasks(t, repo)[2]; v.Status != "done" {
		t.Errorf("finish should still go ahead: %+v", v)
	}
	Run(t, repo, "task", "add", "C").MustSucceed(t)
	if res := Run(t, repo, "task", "start", "3", "c").MustSucceed(t); strings.Contains(res.Stderr, "Warning") {
		t.Errorf("warned without needs: %q", res.Stderr)
	}
}
