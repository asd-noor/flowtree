package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"flowtree/internal/testutil"
)

func TestConfigListShowsOrigins(t *testing.T) {
	repo := flowRepo(t)
	out := Run(t, repo, "config").MustSucceed(t).Stdout
	for _, want := range []string{"master", "main  (local)", "remote", "origin  (default)", "baseDir", "../worktrees  (default)"} {
		if !strings.Contains(out, want) {
			t.Errorf("config output lacks %q:\n%s", want, out)
		}
	}
}

func TestConfigGetSetUnset(t *testing.T) {
	repo := testutil.NewRepo(t)
	Run(t, repo, "config", "baseDir", "/x").MustSucceed(t)
	if got := strings.TrimSpace(Run(t, repo, "config", "baseDir").MustSucceed(t).Stdout); got != "/x" {
		t.Errorf("get = %q", got)
	}
	if got := strings.TrimSpace(testutil.Git(t, repo, "config", "--local", "flowtree.baseDir")); got != "/x" {
		t.Errorf("git config = %q", got)
	}
	Run(t, repo, "config", "--unset", "baseDir").MustSucceed(t)
	if got := strings.TrimSpace(Run(t, repo, "config", "baseDir").MustSucceed(t).Stdout); got != "../worktrees" {
		t.Errorf("after unset = %q", got)
	}
}

func TestConfigRejectsUnknownKeysAndBadValues(t *testing.T) {
	repo := testutil.NewRepo(t)
	if res := Run(t, repo, "config", "bogus", "1").MustFail(t); !strings.Contains(res.Stderr, "unknown setting") {
		t.Errorf("stderr = %q", res.Stderr)
	}
	if res := Run(t, repo, "config", "baseDir", "").MustFail(t); !strings.Contains(res.Stderr, "must not be empty") {
		t.Errorf("stderr = %q", res.Stderr)
	}
}

func TestTypoedKeyInGitConfigIsReported(t *testing.T) {
	repo := flowRepo(t)
	testutil.Git(t, repo, "config", "flowtree.workPrefx", "feat/")
	res := Run(t, repo, "work", "list").MustFail(t)
	if !strings.Contains(res.Stderr, "workprefx") {
		t.Errorf("stderr = %q", res.Stderr)
	}
	// the message names the key, so it can be removed with git
	testutil.Git(t, repo, "config", "--unset", "flowtree.workPrefx")
	Run(t, repo, "work", "list").MustSucceed(t)
}

func TestGlobalSettingsApplyToEveryRepository(t *testing.T) {
	home := t.TempDir()
	global := filepath.Join(home, "gitconfig")
	repo := testutil.NewRepo(t)
	env := []string{"GIT_CONFIG_GLOBAL=" + global}
	RunEnv(t, repo, env, "config", "--global", "remote", "upstream").MustSucceed(t)
	if got := strings.TrimSpace(RunEnv(t, repo, env, "config", "remote").MustSucceed(t).Stdout); got != "upstream" {
		t.Errorf("remote = %q", got)
	}
	if out := RunEnv(t, repo, env, "config").MustSucceed(t).Stdout; !strings.Contains(out, "upstream  (global)") {
		t.Errorf("list should show the global origin:\n%s", out)
	}
	if data, _ := os.ReadFile(global); !strings.Contains(string(data), "upstream") {
		t.Errorf("global config file: %s", data)
	}
}

func TestHooksFileInGitDirIsUsed(t *testing.T) {
	repo := flowRepo(t)
	testutil.WriteFile(t, filepath.Join(repo, ".env"), "K=1\n")
	testutil.WriteFile(t, filepath.Join(repo, ".git", "flowtree-hooks.json"),
		`{"post_create":[{"type":"copy","from":".env"}]}`)
	Run(t, repo, "work", "start", "a").MustSucceed(t)
	if got, _ := os.ReadFile(filepath.Join(worktreeDir(repo, "work/a"), ".env")); string(got) != "K=1\n" {
		t.Errorf("hook did not copy .env: %q", got)
	}
	out := Run(t, repo, "config", "hooks").MustSucceed(t).Stdout
	if !strings.Contains(out, "flowtree-hooks.json") || !strings.Contains(out, "copy") {
		t.Errorf("config hooks:\n%s", out)
	}
}

func TestConfigHooksWithoutFileExplainsHowToCreateOne(t *testing.T) {
	repo := testutil.NewRepo(t)
	out := Run(t, repo, "config", "hooks").MustSucceed(t).Stdout
	if !strings.Contains(out, "No hooks file") || !strings.Contains(out, "post_create") {
		t.Errorf("config hooks:\n%s", out)
	}
}
