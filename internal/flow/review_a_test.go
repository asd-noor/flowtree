package flow_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"flowtree/internal/flow"
	"flowtree/internal/gitx"
	"flowtree/internal/testutil"
)

// editorScript writes an executable GIT_EDITOR that runs body with the message file as $1.
func editorScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "editor.sh")
	testutil.WriteFile(t, path, "#!/bin/sh\n"+body+"\n")
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// attachedWith is an Attached runner that runs real git with the given editor, like the terminal one
// does (no captured output), and counts its calls.
func attachedWith(editor string, calls *atomic.Int32) flow.Attached {
	return func(dir string, args ...string) error {
		calls.Add(1)
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_EDITOR="+editor)
		return cmd.Run()
	}
}

// openRelease starts release 1.0 with one commit on it, and returns the env ready to finish it.
func openRelease(t *testing.T) (*flow.Env, string) {
	t.Helper()
	e, repo := setup(t)
	steps, err := flow.PlanStart(e, flow.StartOpts{Kind: flow.Release, Name: "1.0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := flow.Run(e, steps); err != nil {
		t.Fatal(err)
	}
	commit(t, repo, "CHANGELOG.md", "1.0\n")
	return e, repo
}

func finishRelease(t *testing.T, e *flow.Env, o flow.FinishOpts) error {
	t.Helper()
	o.Kind, o.Name = flow.Release, "1.0"
	steps, err := flow.PlanFinish(e, o)
	if err != nil {
		return err
	}
	return flow.Run(e, steps)
}

func tagMessage(t *testing.T, repo string) string {
	t.Helper()
	return strings.TrimSpace(testutil.Git(t, repo, "tag", "-l", "--format=%(contents)", "1.0"))
}

func TestTagMessageComesFromTheEditorRunWithTheTerminalAttached(t *testing.T) {
	e, repo := openRelease(t)
	var attached atomic.Int32
	e.Interactive = true
	e.Attach = attachedWith(editorScript(t, `echo "from the editor" > "$1"`), &attached)

	if err := finishRelease(t, e, flow.FinishOpts{}); err != nil {
		t.Fatal(err)
	}
	if attached.Load() != 1 {
		t.Errorf("the editor path must use the attached runner exactly once, used %d times", attached.Load())
	}
	if got := tagMessage(t, repo); got != "from the editor" {
		t.Errorf("tag message = %q", got)
	}
}

func TestMessageFlagNeverNeedsTheTerminal(t *testing.T) {
	e, repo := openRelease(t)
	e.Interactive = true
	e.Attach = func(string, ...string) error { t.Error("an explicit -m must not open an editor"); return nil }
	if err := finishRelease(t, e, flow.FinishOpts{Message: "explicit"}); err != nil {
		t.Fatal(err)
	}
	if got := tagMessage(t, repo); got != "explicit" {
		t.Errorf("tag message = %q", got)
	}
}

func TestNonInteractiveFinishUsesTheDefaultMessage(t *testing.T) {
	e, repo := openRelease(t)
	e.Interactive = false
	e.Attach = func(string, ...string) error {
		t.Error("no terminal: the attached runner must not be used")
		return nil
	}
	if err := finishRelease(t, e, flow.FinishOpts{}); err != nil {
		t.Fatal(err)
	}
	if got := tagMessage(t, repo); got != "Release 1.0" {
		t.Errorf("tag message = %q", got)
	}
}

func TestFailedTagSaysSoAndRerunResumes(t *testing.T) {
	e, repo := openRelease(t)
	var attached atomic.Int32
	e.Interactive = true
	e.Attach = attachedWith(editorScript(t, "exit 1"), &attached) // the editor fails: no message, no tag

	err := finishRelease(t, e, flow.FinishOpts{})
	if err == nil {
		t.Fatal("a failing editor must fail the tag step")
	}
	for _, want := range []string{"already merged into 'main'", "tagging '1.0' failed", "re-run 'flowtree release finish 1.0'"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q:\n%v", want, err)
		}
	}
	if testutil.Git(t, repo, "tag", "-l") != "" {
		t.Fatal("no tag may exist after the failed step")
	}
	if !strings.Contains(testutil.Git(t, repo, "ls-tree", "--name-only", "main"), "CHANGELOG.md") {
		t.Fatal("master should already contain the release")
	}

	// the user fixes the cause (here: gives a message) and runs the same command again
	var out strings.Builder
	e.Out = &out
	if err := finishRelease(t, e, flow.FinishOpts{Message: "second try"}); err != nil {
		t.Fatalf("re-run must complete: %v", err)
	}
	if !strings.Contains(out.String(), "Resuming") {
		t.Errorf("stdout = %q", out.String())
	}
	if got := tagMessage(t, repo); got != "second try" {
		t.Errorf("tag message = %q", got)
	}
	if strings.Contains(testutil.Git(t, repo, "branch", "--format=%(refname:short)"), "release/1.0") {
		t.Error("the release branch should be gone after the resumed finish")
	}
	if got := gitx.CurrentBranch(e.MainGit()); got != "develop" {
		t.Errorf("main worktree on %q, want develop", got)
	}
	if !strings.Contains(testutil.Git(t, repo, "ls-tree", "--name-only", "develop"), "CHANGELOG.md") {
		t.Error("the tag was not merged back into develop")
	}
}

// ---- finding 5: the main worktree must be able to return before anything is merged

// handCheckedOut puts branch (created from base) into the main worktree and develop into another one.
func handCheckedOut(t *testing.T, e *flow.Env, repo, branch, base string) string {
	t.Helper()
	testutil.Git(t, repo, "checkout", "-q", "-b", branch, base)
	commit(t, repo, branch[strings.Index(branch, "/")+1:]+".txt", "x")
	other := filepath.Join(filepath.Dir(repo), "develop-elsewhere")
	testutil.Git(t, repo, "worktree", "add", "-q", other, "develop")
	return other
}

func TestFinishRefusesBeforeMergingWhenMainCannotReturnToDevelop(t *testing.T) {
	for _, tc := range []struct {
		kind         flow.Kind
		branch, base string
	}{
		{flow.Work, "work/x", "develop"},
		{flow.Hotfix, "hotfix/1.0.1", "main"},
	} {
		e, repo := setup(t)
		other := handCheckedOut(t, e, repo, tc.branch, tc.base)
		before := testutil.Git(t, repo, "rev-parse", "develop")

		steps, err := flow.PlanFinish(e, flow.FinishOpts{Kind: tc.kind, Name: e.S.Name(tc.kind, tc.branch)})
		if err == nil {
			err = flow.Run(e, steps)
		}
		if err == nil || !strings.Contains(err.Error(), "cannot return") || !strings.Contains(err.Error(), other) {
			t.Fatalf("%s: got %v", tc.kind, err)
		}
		if testutil.Git(t, repo, "rev-parse", "develop") != before {
			t.Errorf("%s: develop moved although the finish was refused", tc.kind)
		}
		if gitx.CurrentBranch(e.MainGit()) != tc.branch {
			t.Errorf("%s: the main worktree must stay on %s", tc.kind, tc.branch)
		}
		if !strings.Contains(testutil.Git(t, repo, "branch", "--format=%(refname:short)"), tc.branch) {
			t.Errorf("%s: the branch must survive a refused finish", tc.kind)
		}
	}
}

func TestWorkFinishFromMainWorktreeWorksWhenDevelopIsFree(t *testing.T) {
	e, repo := setup(t)
	testutil.Git(t, repo, "checkout", "-q", "-b", "work/x", "develop") // develop is now checked out nowhere
	commit(t, repo, "x.txt", "x")
	steps, err := flow.PlanFinish(e, flow.FinishOpts{Kind: flow.Work, Name: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if err := flow.Run(e, steps); err != nil {
		t.Fatal(err)
	}
	if got := gitx.CurrentBranch(e.MainGit()); got != "develop" {
		t.Errorf("main worktree on %q, want develop", got)
	}
	if !strings.Contains(testutil.Git(t, repo, "ls-tree", "--name-only", "develop"), "x.txt") {
		t.Error("work was not merged")
	}
	if strings.Contains(testutil.Git(t, repo, "branch", "--format=%(refname:short)"), "work/x") {
		t.Error("work branch should be deleted")
	}
}
