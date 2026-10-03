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

func testDeps(t *testing.T, cwd string, env map[string]string) (Deps, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errb bytes.Buffer
	return Deps{
		Run: gitx.Exec, Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb,
		Getenv: func(k string) string { return env[k] },
		Cwd:    func() (string, error) { return cwd, nil },
		IsTTY:  func() bool { return false },
	}, &out, &errb
}

func run(t *testing.T, d Deps, args ...string) error {
	t.Helper()
	root := NewRoot(d, "test")
	root.SetArgs(args)
	return root.ExecuteContext(context.Background())
}

func TestWriteCdFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "cd")
	d, _, _ := testDeps(t, ".", map[string]string{CdFileEnv: f})
	writeCdFile(d, "/some/dir")
	if b, _ := os.ReadFile(f); string(b) != "/some/dir" {
		t.Errorf("cdfile = %q", b)
	}
	d, _, _ = testDeps(t, ".", nil)
	writeCdFile(d, "/x") // must not panic or write anywhere
}

func TestPrintDirAndExec(t *testing.T) {
	repo := testutil.NewRepo(t)
	wt := filepath.Join(filepath.Dir(repo), "worktrees", "feature", "x")
	testutil.Git(t, repo, "worktree", "add", "-q", "-b", "feature/x", wt)

	d, out, _ := testDeps(t, repo, nil)
	for query, want := range map[string]string{"": repo, "@": repo, "feature/x": wt} {
		out.Reset()
		args := []string{"print-dir"}
		if query != "" {
			args = append(args, query)
		}
		if err := run(t, d, args...); err != nil {
			t.Fatalf("print-dir %q: %v", query, err)
		}
		if got := strings.TrimSpace(out.String()); got != want {
			t.Errorf("print-dir %q = %q, want %q", query, got, want)
		}
	}
	if err := run(t, d, "print-dir", "nope"); err == nil {
		t.Error("want error for unknown worktree")
	}

	out.Reset()
	if err := run(t, d, "exec", "feature/x", "--", "pwd"); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != wt {
		t.Errorf("exec pwd = %q, want %q", got, wt)
	}

	err := run(t, d, "exec", "@", "--", "sh", "-c", "exit 7")
	if code, ok := ExitCode(err); !ok || code != 7 {
		t.Errorf("exit code = %d,%v (err %v)", code, ok, err)
	}
	if err := run(t, d, "exec", "@", "pwd"); err == nil {
		t.Error("want usage error without --")
	}
}

func TestShellInitCmd(t *testing.T) {
	d, out, _ := testDeps(t, ".", map[string]string{"SHELL": "/bin/zsh"})
	if err := run(t, d, "shell-init"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "completion zsh") {
		t.Errorf("default shell not taken from $SHELL:\n%s", out)
	}
	if err := run(t, d, "shell-init", "csh"); err == nil {
		t.Error("want error for unsupported shell")
	}
}
