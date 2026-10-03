package commands

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"flowtree/internal/config"
	"flowtree/internal/gitx"
	"flowtree/internal/testutil"
)

func initDeps(t *testing.T, repo, stdin string, tty bool, env map[string]string) (Deps, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, errb bytes.Buffer
	return Deps{
		Run:    gitx.Exec,
		Stdin:  strings.NewReader(stdin),
		Stdout: &out,
		Stderr: &errb,
		Getenv: func(k string) string { return env[k] },
		Cwd:    func() (string, error) { return repo, nil },
		IsTTY:  func() bool { return tty },
	}, &out, &errb
}

func runInitCmd(t *testing.T, d Deps, args ...string) error {
	t.Helper()
	cmd := newInitCmd(d)
	cmd.SetArgs(args)
	cmd.SetOut(d.Stdout)
	cmd.SetErr(d.Stderr)
	return cmd.Execute()
}

func loadCfg(t *testing.T, repo string, env map[string]string) config.Config {
	t.Helper()
	cfg, err := config.Load(gitx.In(context.Background(), repo), repo, func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func localKeys(t *testing.T, repo string) string {
	t.Helper()
	out, _ := gitx.Exec(context.Background(), repo, "config", "--local", "--get-regexp", `^flowtree\.`)
	return out
}

func TestInitDefaultsStoresNothing(t *testing.T) {
	repo := testutil.NewRepo(t)
	d, out, _ := initDeps(t, repo, "", true, nil)
	if err := runInitCmd(t, d, "-d"); err != nil {
		t.Fatal(err)
	}
	if keys := localKeys(t, repo); keys != "" {
		t.Fatalf("defaults must not be stored, git config has %q", keys)
	}
	if !strings.Contains(out.String(), "Nothing to store") {
		t.Fatalf("stdout = %q", out.String())
	}
	if _, err := os.Stat(filepath.Join(repo, ".flowtree.json")); err == nil {
		t.Fatal("no file is written any more")
	}
}

func TestInitFlagBaseDirIsStoredAndExpandsHome(t *testing.T) {
	repo := testutil.NewRepo(t)
	home, _ := os.UserHomeDir()
	d, _, _ := initDeps(t, repo, "", false, nil)
	if err := runInitCmd(t, d, "-w", "~/wt"); err != nil {
		t.Fatal(err)
	}
	if got, want := loadCfg(t, repo, nil).BaseDir, filepath.Join(home, "wt"); got != want {
		t.Fatalf("baseDir = %q, want %q", got, want)
	}
}

func TestInitFollowsParentEnvInsteadOfStoringIt(t *testing.T) {
	repo := testutil.NewRepo(t)
	env := map[string]string{config.ParentEnv: "/srv/trees"}
	d, _, _ := initDeps(t, repo, "", false, env)
	if err := runInitCmd(t, d); err != nil {
		t.Fatal(err)
	}
	if keys := localKeys(t, repo); keys != "" {
		t.Fatalf("nothing should be stored so the default keeps following the environment: %q", keys)
	}
	if got, want := loadCfg(t, repo, env).WorktreePath(repo, "work/x"), "/srv/trees/repo/work/x"; got != want {
		t.Fatalf("worktree path = %q, want %q", got, want)
	}
	if got := loadCfg(t, repo, map[string]string{config.ParentEnv: "/other"}).ResolveBaseDir(repo); got != "/other/repo" {
		t.Fatalf("changing the variable must change the default: %q", got)
	}
}

func TestInitPromptsOnTTY(t *testing.T) {
	repo := testutil.NewRepo(t)
	d, _, errb := initDeps(t, repo, "../mine\n", true, nil)
	if err := runInitCmd(t, d); err != nil {
		t.Fatal(err)
	}
	if got := loadCfg(t, repo, nil).BaseDir; got != "../mine" {
		t.Fatalf("baseDir = %q", got)
	}
	if !strings.Contains(errb.String(), "[../worktrees]") {
		t.Fatalf("prompt should show the default, got %q", errb.String())
	}
}

func TestInitPromptEmptyAnswerKeepsDefault(t *testing.T) {
	repo := testutil.NewRepo(t)
	d, _, errb := initDeps(t, repo, "\n", true, map[string]string{config.ParentEnv: "/srv/trees"})
	if err := runInitCmd(t, d); err != nil {
		t.Fatal(err)
	}
	if keys := localKeys(t, repo); keys != "" {
		t.Fatalf("accepting the default must not store it: %q", keys)
	}
	if !strings.Contains(errb.String(), "[/srv/trees/repo]") {
		t.Fatalf("prompt should show the FLOWTREE_PARENT default, got %q", errb.String())
	}
}

func TestInitExistingSettingsAreLeftAlone(t *testing.T) {
	repo := testutil.NewRepo(t)
	testutil.Git(t, repo, "config", "flowtree.baseDir", "../keep")
	d, out, _ := initDeps(t, repo, "", false, nil)
	if err := runInitCmd(t, d, "-w", "../other"); err != nil {
		t.Fatal(err)
	}
	if got := loadCfg(t, repo, nil).BaseDir; got != "../keep" {
		t.Fatalf("settings were overwritten: %q", got)
	}
	if !strings.Contains(out.String(), "--force") {
		t.Fatalf("note should mention --force, got %q", out.String())
	}
}

func TestInitForceResetsTheSection(t *testing.T) {
	repo := testutil.NewRepo(t)
	testutil.Git(t, repo, "config", "flowtree.baseDir", "../keep")
	testutil.Git(t, repo, "config", "flowtree.remote", "up")
	d, _, _ := initDeps(t, repo, "", false, nil)
	if err := runInitCmd(t, d, "-w", "../other", "--force"); err != nil {
		t.Fatal(err)
	}
	cfg := loadCfg(t, repo, nil)
	if cfg.BaseDir != "../other" || cfg.Remote != "origin" {
		t.Fatalf("--force should reset, then store: %+v", cfg)
	}
}

func TestInitForceRepairsABrokenConfiguration(t *testing.T) {
	repo := testutil.NewRepo(t)
	testutil.Git(t, repo, "config", "flowtree.typo", "1")
	d, _, _ := initDeps(t, repo, "", false, nil)
	if err := runInitCmd(t, d, "-w", "../x"); err == nil {
		t.Fatal("a broken configuration should be reported")
	}
	if err := runInitCmd(t, d, "-w", "../x", "--force"); err != nil {
		t.Fatalf("--force must be able to repair it: %v", err)
	}
}

func TestInitWorksFromLinkedWorktree(t *testing.T) {
	repo := testutil.NewRepo(t)
	wt := filepath.Join(filepath.Dir(repo), "wt")
	testutil.Git(t, repo, "worktree", "add", "-q", "-b", "x", wt)
	d, _, _ := initDeps(t, wt, "", false, nil)
	if err := runInitCmd(t, d, "-w", "../mine"); err != nil {
		t.Fatal(err)
	}
	if got := loadCfg(t, repo, nil).BaseDir; got != "../mine" {
		t.Fatalf("settings must be visible from the main worktree: %q", got)
	}
}

func TestInitFlowExtendsExistingSettings(t *testing.T) {
	repo := testutil.NewRepo(t)
	testutil.Git(t, repo, "config", "flowtree.baseDir", "../keep")
	d, _, _ := initDeps(t, repo, "", false, nil)
	if err := runInitCmd(t, d, "--flow", "-d"); err != nil {
		t.Fatal(err)
	}
	cfg := loadCfg(t, repo, nil)
	if cfg.BaseDir != "../keep" || !cfg.FlowEnabled() || cfg.Flow.Master != "main" || cfg.Flow.Develop != "develop" {
		t.Fatalf("%+v", cfg)
	}
}
