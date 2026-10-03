package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"flowtree/internal/testutil"
)

func TestAddFailingExecStillReportsPathAndWritesCdFile(t *testing.T) {
	repo := testutil.NewRepo(t)
	cdfile := filepath.Join(t.TempDir(), "cd")
	e := newAddEnv(t, repo, map[string]string{CdFileEnv: cdfile})
	err := e.add(t, "-b", "x", "--quiet", "--exec", "exit 3")
	if err == nil || !strings.Contains(err.Error(), "--exec command failed") || !strings.Contains(err.Error(), "worktree was created at") {
		t.Fatalf("the exec failure must still be returned, got %v", err)
	}
	want := e.wtPath("x")
	if got := e.out.String(); got != want+"\n" {
		t.Errorf("--quiet must print the path on stdout even when --exec fails: %q", got)
	}
	if b, _ := os.ReadFile(cdfile); string(b) != want {
		t.Errorf("cd-file = %q, want %q", b, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Errorf("worktree should remain: %v", err)
	}
}

func TestAddFailingExecWithoutQuietStillReports(t *testing.T) {
	repo := testutil.NewRepo(t)
	cdfile := filepath.Join(t.TempDir(), "cd")
	e := newAddEnv(t, repo, map[string]string{CdFileEnv: cdfile})
	if err := e.add(t, "-b", "y", "--exec", "exit 1"); err == nil {
		t.Fatal("expected the exec failure")
	}
	if !strings.Contains(e.out.String(), "Worktree created successfully") || !strings.Contains(e.out.String(), e.wtPath("y")) {
		t.Errorf("stdout should carry the success report:\n%s", e.out)
	}
	if b, _ := os.ReadFile(cdfile); string(b) != e.wtPath("y") {
		t.Errorf("cd-file = %q", b)
	}
}

func TestAddSuccessfulExecStillRuns(t *testing.T) {
	repo := testutil.NewRepo(t)
	e := newAddEnv(t, repo, nil)
	if err := e.add(t, "-b", "z", "--quiet", "--exec", "pwd > where.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(e.wtPath("z"), "where.txt")); err != nil {
		t.Errorf("--exec should run in the new worktree: %v", err)
	}
	if e.out.String() != e.wtPath("z")+"\n" {
		t.Errorf("stdout = %q", e.out)
	}
}
