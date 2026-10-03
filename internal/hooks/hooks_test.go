package hooks

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"flowtree/internal/config"
)

func setup(t *testing.T) (repo, wt string) {
	t.Helper()
	root := t.TempDir()
	repo, wt = filepath.Join(root, "repo"), filepath.Join(root, "wt")
	for _, d := range []string{repo, wt} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return repo, wt
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func run(t *testing.T, repo, wt string, hs ...config.Hook) (string, error) {
	t.Helper()
	cfg := config.Config{Hooks: config.Hooks{PostCreate: hs}}.WithDefaults()
	var out bytes.Buffer
	err := RunPostCreate(context.Background(), &out, cfg, repo, wt)
	return out.String(), err
}

func TestCopyFileAndDir(t *testing.T) {
	repo, wt := setup(t)
	write(t, filepath.Join(repo, ".env"), "SECRET=1")
	write(t, filepath.Join(repo, "cfg", "a", "b.txt"), "deep")
	out, err := run(t, repo, wt,
		config.Hook{Type: config.HookCopy, From: ".env"},
		config.Hook{Type: config.HookCopy, From: "cfg", To: "conf"})
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if got := read(t, filepath.Join(wt, ".env")); got != "SECRET=1" {
		t.Errorf(".env = %q", got)
	}
	if got := read(t, filepath.Join(wt, "conf", "a", "b.txt")); got != "deep" {
		t.Errorf("dir copy = %q", got)
	}
	for _, want := range []string{"→ Running hook 1 of 2...", "✓ Hook 2 completed", "Copying: .env → .env"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestCopyPreservesMode(t *testing.T) {
	repo, wt := setup(t)
	write(t, filepath.Join(repo, "run.sh"), "#!/bin/sh")
	if err := os.Chmod(filepath.Join(repo, "run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, repo, wt, config.Hook{Type: config.HookCopy, From: "run.sh"}); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(wt, "run.sh"))
	if err != nil || fi.Mode().Perm() != 0o755 {
		t.Errorf("mode = %v, err = %v", fi.Mode(), err)
	}
}

func TestCopyRejectsEscapeAndMissing(t *testing.T) {
	repo, wt := setup(t)
	for name, h := range map[string]config.Hook{
		"source escapes repo": {Type: config.HookCopy, From: "../secret", To: "x"},
		"dest escapes wt":     {Type: config.HookCopy, From: "a", To: "../x"},
		"missing source":      {Type: config.HookCopy, From: "nope", To: "x"},
	} {
		write(t, filepath.Join(repo, "a"), "a")
		if _, err := run(t, repo, wt, h); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestSymlink(t *testing.T) {
	repo, wt := setup(t)
	write(t, filepath.Join(repo, ".bin", "tool"), "x")
	if _, err := run(t, repo, wt, config.Hook{Type: config.HookSymlink, From: ".bin", To: ".bin"}); err != nil {
		t.Fatal(err)
	}
	target, err := os.Readlink(filepath.Join(wt, ".bin"))
	if err != nil || target != filepath.Join(repo, ".bin") {
		t.Errorf("link = %q, err = %v", target, err)
	}
	if _, err := run(t, repo, wt, config.Hook{Type: config.HookSymlink, From: ".bin", To: ".bin"}); err == nil {
		t.Error("expected an error when destination exists")
	}
}

func TestCommandEnvAndWorkDir(t *testing.T) {
	repo, wt := setup(t)
	if err := os.Mkdir(filepath.Join(wt, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := run(t, repo, wt,
		config.Hook{Type: config.HookCommand, Command: `echo "$FLOWTREE_WORKTREE_PATH|$FLOWTREE_REPO_ROOT|$MINE" > env.txt; pwd > pwd.txt`,
			Env: map[string]string{"MINE": "yes"}, WorkDir: "sub"})
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if got, want := strings.TrimSpace(read(t, filepath.Join(wt, "sub", "env.txt"))), wt+"|"+repo+"|yes"; got != want {
		t.Errorf("env = %q, want %q", got, want)
	}
}

func TestCommandStreamsBothStreamsAndFails(t *testing.T) {
	repo, wt := setup(t)
	out, err := run(t, repo, wt, config.Hook{Type: config.HookCommand, Command: "echo out; echo err >&2; exit 3"})
	if err == nil || !strings.Contains(err.Error(), "failed to execute hook 1") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(out, "out") || !strings.Contains(out, "err") {
		t.Errorf("output = %q", out)
	}
}

func TestStopsAtFirstFailure(t *testing.T) {
	repo, wt := setup(t)
	_, err := run(t, repo, wt,
		config.Hook{Type: config.HookCommand, Command: "false"},
		config.Hook{Type: config.HookCommand, Command: "touch ran"})
	if err == nil {
		t.Fatal("expected an error")
	}
	if _, statErr := os.Stat(filepath.Join(wt, "ran")); statErr == nil {
		t.Error("second hook ran after the first failed")
	}
}

func TestNoHooks(t *testing.T) {
	repo, wt := setup(t)
	if out, err := run(t, repo, wt); err != nil || out != "" {
		t.Errorf("out = %q, err = %v", out, err)
	}
}
