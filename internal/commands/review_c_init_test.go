package commands

import (
	"path/filepath"
	"strings"
	"testing"

	"flowtree/internal/testutil"
)

// storedBaseDir is the repository's own flowtree.baseDir ("" when unset).
func storedBaseDir(t *testing.T, repo string) string {
	t.Helper()
	return strings.TrimSpace(testutil.Git(t, repo, "config", "--local", "--default", "", "--get", "flowtree.baseDir"))
}

func TestInitForcePromptShowsTheResetDefault(t *testing.T) {
	repo := testutil.NewRepo(t)
	testutil.Git(t, repo, "config", "flowtree.baseDir", "/data/wt")
	d, _, errb := initDeps(t, repo, "\n", true, nil)
	if err := runInitCmd(t, d, "--force"); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(filepath.Dir(repo), "worktrees")
	if !strings.Contains(errb.String(), "["+want+"]") {
		t.Fatalf("after a reset the prompt must show the default, not the old value; got %q", errb.String())
	}
	if got := storedBaseDir(t, repo); got != "" {
		t.Fatalf("Enter accepts the default, so nothing is stored: %q", got)
	}
}

func TestInitForceStoresWhatTheUserTypedEvenIfItEqualsTheOldValue(t *testing.T) {
	for name, tc := range map[string]struct {
		stdin string
		tty   bool
		args  []string
	}{
		"typed at the prompt": {stdin: "/data/wt\n", tty: true},
		"-w":                  {args: []string{"-w", "/data/wt"}},
		"-w with a prompt":    {stdin: "ignored\n", tty: true, args: []string{"-w", "/data/wt"}},
	} {
		t.Run(name, func(t *testing.T) {
			repo := testutil.NewRepo(t)
			testutil.Git(t, repo, "config", "flowtree.baseDir", "/data/wt")
			d, _, _ := initDeps(t, repo, tc.stdin, tc.tty, nil)
			if err := runInitCmd(t, d, append([]string{"--force"}, tc.args...)...); err != nil {
				t.Fatal(err)
			}
			if got := storedBaseDir(t, repo); got != "/data/wt" {
				t.Fatalf("baseDir = %q; the chosen value must survive a reset", got)
			}
		})
	}
}

func TestInitForceNonInteractiveWithoutFlagResetsToDefault(t *testing.T) {
	repo := testutil.NewRepo(t)
	testutil.Git(t, repo, "config", "flowtree.baseDir", "/data/wt")
	d, _, _ := initDeps(t, repo, "", false, nil)
	if err := runInitCmd(t, d, "--force", "-d"); err != nil {
		t.Fatal(err)
	}
	if got := storedBaseDir(t, repo); got != "" {
		t.Fatalf("a reset without a new choice stores nothing, got %q", got)
	}
}

func TestInitForceResetsRemoteAndFlow(t *testing.T) {
	repo := testutil.NewRepo(t)
	for k, v := range map[string]string{"remote": "up", "master": "main", "develop": "dev", "workPrefix": "feat/"} {
		testutil.Git(t, repo, "config", "flowtree."+k, v)
	}
	d, _, _ := initDeps(t, repo, "", false, nil)
	if err := runInitCmd(t, d, "--force", "-w", "../x"); err != nil {
		t.Fatal(err)
	}
	cfg := loadCfg(t, repo, nil)
	if cfg.Remote != "origin" || cfg.FlowEnabled() || cfg.Flow.WorkPrefix != "" || cfg.BaseDir != "../x" {
		t.Fatalf("--force must reset remote and flow too: %+v", cfg)
	}
}

func TestInitForceFlowStartsFromTheResetDefaults(t *testing.T) {
	repo := testutil.NewRepo(t)
	testutil.Git(t, repo, "config", "flowtree.remote", "up")
	testutil.Git(t, repo, "config", "flowtree.workPrefix", "feat/")
	d, _, _ := initDeps(t, repo, "", false, nil)
	if err := runInitCmd(t, d, "--force", "--flow", "-d"); err != nil {
		t.Fatal(err)
	}
	cfg := loadCfg(t, repo, nil)
	if cfg.Remote != "origin" || cfg.Flow.WorkPrefix != "work/" || !cfg.FlowEnabled() {
		t.Fatalf("flow settings must come from the defaults, not the old config: %+v", cfg)
	}
}

func TestInitForceKeepsGlobalSettings(t *testing.T) {
	repo := testutil.NewRepo(t)
	global := filepath.Join(t.TempDir(), "gitconfig")
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	testutil.Git(t, repo, "config", "--file", global, "flowtree.remote", "from-global")
	testutil.Git(t, repo, "config", "flowtree.remote", "local")
	d, _, _ := initDeps(t, repo, "", false, nil)
	if err := runInitCmd(t, d, "--force", "-d"); err != nil {
		t.Fatal(err)
	}
	if got := loadCfg(t, repo, nil).Remote; got != "from-global" {
		t.Fatalf("a reset removes the repository's settings only; remote = %q", got)
	}
}

func TestInitForceRestoresTheOldSettingsWhenInitFails(t *testing.T) {
	repo := testutil.NewRepo(t)
	testutil.Git(t, repo, "config", "flowtree.baseDir", "/data/wt")
	testutil.Git(t, repo, "config", "flowtree.remote", "up")
	d, _, _ := initDeps(t, repo, "", false, nil)
	// same master and develop: rejected after the reset has already happened
	if err := runInitCmd(t, d, "--force", "--flow", "-d", "--master", "x", "--develop", "x"); err == nil {
		t.Fatal("expected invalid flow settings to fail")
	}
	cfg := loadCfg(t, repo, nil)
	if cfg.BaseDir != "/data/wt" || cfg.Remote != "up" || cfg.FlowEnabled() {
		t.Fatalf("a failed init --force must leave the settings as they were: %+v", cfg)
	}
}
