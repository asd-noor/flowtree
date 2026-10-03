package commands

import (
	"bytes"
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

func excludeContent(t *testing.T, repo string) string {
	t.Helper()
	b, _ := os.ReadFile(filepath.Join(repo, ".git", "info", "exclude"))
	return string(b)
}

func TestWithLine(t *testing.T) {
	cases := []struct {
		name, in, want string
		changed        bool
	}{
		{"empty", "", "/x\n", true},
		{"no trailing newline", "a", "a\n/x\n", true},
		{"trailing newline", "a\n", "a\n/x\n", true},
		{"present", "a\n/x\n", "a\n/x\n", false},
		{"present no newline", "/x", "/x", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, changed := withLine(c.in, "/x")
			if got != c.want || changed != c.changed {
				t.Fatalf("withLine(%q) = %q, %v; want %q, %v", c.in, got, changed, c.want, c.changed)
			}
		})
	}
}

func TestInitDefaultsWritesConfigOnly(t *testing.T) {
	repo := testutil.NewRepo(t)
	d, _, _ := initDeps(t, repo, "", true, nil)
	if err := runInitCmd(t, d, "-d"); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(repo)
	if err != nil || cfg.BaseDir != config.DefaultBaseDir {
		t.Fatalf("cfg = %+v, err = %v", cfg, err)
	}
	if strings.Contains(excludeContent(t, repo), config.FileName) {
		t.Fatal("-d must never exclude")
	}
}

func TestInitHonoursParentEnv(t *testing.T) {
	repo := testutil.NewRepo(t)
	d, _, _ := initDeps(t, repo, "", false, map[string]string{config.ParentEnv: "/srv/trees"})
	if err := runInitCmd(t, d); err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.Load(repo)
	if want := "/srv/trees/repo"; cfg.BaseDir != want {
		t.Fatalf("base_dir = %q, want %q", cfg.BaseDir, want)
	}
}

func TestInitFlagBaseDirExpandsHome(t *testing.T) {
	repo := testutil.NewRepo(t)
	home, _ := os.UserHomeDir()
	d, _, _ := initDeps(t, repo, "", false, nil)
	if err := runInitCmd(t, d, "-w", "~/wt"); err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.Load(repo)
	if want := filepath.Join(home, "wt"); cfg.BaseDir != want {
		t.Fatalf("base_dir = %q, want %q", cfg.BaseDir, want)
	}
}

func TestInitPromptsOnTTY(t *testing.T) {
	repo := testutil.NewRepo(t)
	d, _, errb := initDeps(t, repo, "../mine\ny\n", true, nil)
	if err := runInitCmd(t, d); err != nil {
		t.Fatal(err)
	}
	cfg, _ := config.Load(repo)
	if cfg.BaseDir != "../mine" {
		t.Fatalf("base_dir = %q", cfg.BaseDir)
	}
	if !strings.Contains(errb.String(), "[../worktrees]") {
		t.Fatalf("prompt should show the default, got %q", errb.String())
	}
	if !strings.Contains(excludeContent(t, repo), "/"+config.FileName) {
		t.Fatal("answering y should exclude")
	}
}

func TestInitExcludeDefaultsToNo(t *testing.T) {
	for _, stdin := range []string{"\n\n", "\n"} { // empty answer, and EOF on the second prompt
		repo := testutil.NewRepo(t)
		d, _, _ := initDeps(t, repo, stdin, true, nil)
		if err := runInitCmd(t, d); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(excludeContent(t, repo), config.FileName) {
			t.Fatalf("stdin %q: default must be no", stdin)
		}
	}
}

func TestInitExcludeFlagIsIdempotent(t *testing.T) {
	repo := testutil.NewRepo(t)
	for range 2 {
		d, _, _ := initDeps(t, repo, "", false, nil)
		if err := runInitCmd(t, d, "--exclude", "--force"); err != nil {
			t.Fatal(err)
		}
	}
	if n := strings.Count(excludeContent(t, repo), "/"+config.FileName); n != 1 {
		t.Fatalf("exclude line appears %d times", n)
	}
	if out := testutil.Git(t, repo, "status", "--porcelain"); strings.Contains(out, config.FileName) {
		t.Fatalf("file should be ignored, status: %q", out)
	}
}

func TestInitExistingFileIsLeftAlone(t *testing.T) {
	repo := testutil.NewRepo(t)
	testutil.WriteFile(t, config.Path(repo), `{"version":1,"base_dir":"../keep"}`)
	d, out, _ := initDeps(t, repo, "", false, nil)
	if err := runInitCmd(t, d, "-w", "../other"); err != nil {
		t.Fatal(err)
	}
	if cfg, _ := config.Load(repo); cfg.BaseDir != "../keep" {
		t.Fatalf("file was overwritten: %+v", cfg)
	}
	if !strings.Contains(out.String(), "--force") {
		t.Fatalf("note should mention --force, got %q", out.String())
	}
	if err := runInitCmd(t, d, "-w", "../other", "--force"); err != nil {
		t.Fatal(err)
	}
	if cfg, _ := config.Load(repo); cfg.BaseDir != "../other" {
		t.Fatalf("--force did not overwrite: %+v", cfg)
	}
}

func TestInitSkipsTrackedFile(t *testing.T) {
	repo := testutil.NewRepo(t)
	testutil.WriteFile(t, config.Path(repo), `{"version":1}`)
	testutil.Git(t, repo, "add", config.FileName)
	testutil.Git(t, repo, "commit", "-q", "-m", "track")
	d, out, _ := initDeps(t, repo, "", false, nil)
	if err := runInitCmd(t, d, "--force", "--exclude"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(excludeContent(t, repo), config.FileName) || !strings.Contains(out.String(), "tracked") {
		t.Fatalf("tracked file must not be excluded; out %q", out.String())
	}
}

func TestInitWorksFromLinkedWorktree(t *testing.T) {
	repo := testutil.NewRepo(t)
	wt := filepath.Join(filepath.Dir(repo), "wt")
	testutil.Git(t, repo, "worktree", "add", "-q", "-b", "x", wt)
	d, _, _ := initDeps(t, wt, "", false, nil)
	if err := runInitCmd(t, d, "--exclude"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(config.Path(repo)); err != nil {
		t.Fatalf("config must land in the main worktree: %v", err)
	}
	if !strings.Contains(excludeContent(t, repo), "/"+config.FileName) {
		t.Fatal("exclude must go to the common git dir")
	}
}

func TestInitOutsideRepoFails(t *testing.T) {
	d, _, _ := initDeps(t, t.TempDir(), "", false, nil)
	if err := runInitCmd(t, d); err == nil {
		t.Fatal("expected an error outside a repository")
	}
}
