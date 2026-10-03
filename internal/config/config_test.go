package config_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"flowtree/internal/config"
	"flowtree/internal/gitx"
	"flowtree/internal/testutil"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func open(t *testing.T) (gitx.Git, string) {
	t.Helper()
	repo := testutil.NewRepo(t)
	return gitx.In(context.Background(), repo), repo
}

func TestLoadWithNothingSetGivesDefaults(t *testing.T) {
	g, repo := open(t)
	c, err := config.Load(g, repo, env(nil))
	if err != nil || c.BaseDir != "" || c.Remote != "origin" || c.FlowEnabled() || c.HasHooks() {
		t.Fatalf("%+v %v", c, err)
	}
	if got := c.WorktreePath("/code/api", "work/login"); got != "/code/worktrees/work/login" {
		t.Error(got)
	}
}

func TestSetAndLoad(t *testing.T) {
	g, repo := open(t)
	for k, v := range map[string]string{"baseDir": "/abs", "remote": "up", "master": "main", "develop": "dev", "workPrefix": "feat/", "versionTagPrefix": "v"} {
		if err := config.Set(g, config.Local, k, v); err != nil {
			t.Fatal(err)
		}
	}
	c, err := config.Load(g, repo, env(nil))
	if err != nil {
		t.Fatal(err)
	}
	want := config.Flow{Master: "main", Develop: "dev", WorkPrefix: "feat/", VersionTag: "v"}
	if c.BaseDir != "/abs" || c.Remote != "up" || c.Flow != want || !c.FlowEnabled() {
		t.Fatalf("%+v", c)
	}
	if got := c.WorktreePath(repo, "x"); got != "/abs/x" {
		t.Error(got)
	}
}

func TestKeysAreCaseInsensitiveAndPrefixed(t *testing.T) {
	g, repo := open(t)
	if err := config.Set(g, config.Local, "flowtree.BASEDIR", "/x"); err != nil {
		t.Fatal(err)
	}
	if c, _ := config.Load(g, repo, env(nil)); c.BaseDir != "/x" {
		t.Errorf("%+v", c)
	}
}

func TestUnknownKeysAreErrors(t *testing.T) {
	g, repo := open(t)
	if err := config.Set(g, config.Local, "bogus", "1"); err == nil || !strings.Contains(err.Error(), "known:") {
		t.Errorf("Set: %v", err)
	}
	testutil.Git(t, repo, "config", "flowtree.typo", "1") // as a user would, bypassing flowtree
	if _, err := config.Load(g, repo, env(nil)); err == nil || !strings.Contains(err.Error(), "flowtree.typo") {
		t.Errorf("Load: %v", err)
	}
}

func TestSetValidatesValues(t *testing.T) {
	g, _ := open(t)
	if err := config.Set(g, config.Local, "baseDir", " "); err == nil {
		t.Error("empty baseDir must be rejected")
	}
	if err := config.Set(g, config.Local, "versionTagPrefix", ""); err != nil {
		t.Errorf("an empty tag prefix is valid: %v", err)
	}
}

func TestMasterAndDevelopGoTogether(t *testing.T) {
	g, repo := open(t)
	_ = config.Set(g, config.Local, "master", "main")
	if _, err := config.Load(g, repo, env(nil)); err == nil || !strings.Contains(err.Error(), "together") {
		t.Errorf("got %v", err)
	}
}

func TestUnset(t *testing.T) {
	g, repo := open(t)
	_ = config.Set(g, config.Local, "baseDir", "/x")
	if err := config.Unset(g, config.Local, "baseDir"); err != nil {
		t.Fatal(err)
	}
	if err := config.Unset(g, config.Local, "baseDir"); err != nil {
		t.Errorf("unsetting an unset key is fine: %v", err)
	}
	if c, _ := config.Load(g, repo, env(nil)); c.BaseDir != "" {
		t.Errorf("%+v", c)
	}
}

func TestEntriesReportScopeAndLaterWins(t *testing.T) {
	g, repo := open(t)
	global := filepath.Join(t.TempDir(), "gitconfig")
	t.Setenv("GIT_CONFIG_GLOBAL", global)
	testutil.Git(t, repo, "config", "--file", global, "flowtree.remote", "from-global")
	testutil.Git(t, repo, "config", "--file", global, "flowtree.baseDir", "/global")
	_ = config.Set(g, config.Local, "baseDir", "/local")
	entries, err := config.Entries(g)
	if err != nil {
		t.Fatal(err)
	}
	scopes := map[string]string{}
	for _, e := range entries {
		scopes[e.Key] = e.Scope
	}
	if scopes["remote"] != "global" || scopes["baseDir"] != "local" {
		t.Errorf("%+v", entries)
	}
	if c, _ := config.Load(g, repo, env(nil)); c.BaseDir != "/local" || c.Remote != "from-global" {
		t.Errorf("local must win over global: %+v", c)
	}
}

func TestBaseDirDefaults(t *testing.T) {
	c := config.Defaults()
	if got := c.ResolveBaseDir("/code/api"); got != "/code/worktrees" {
		t.Error(got)
	}
	c.Parent = "/wt"
	if got := c.WorktreePath("/code/api", "work/x"); got != "/wt/api/work/x" {
		t.Errorf("FLOWTREE_PARENT/<repo>/<branch>: %s", got)
	}
	c.BaseDir = "/explicit"
	if got := c.WorktreePath("/code/api", "work/x"); got != "/explicit/work/x" {
		t.Errorf("an explicit baseDir wins over FLOWTREE_PARENT: %s", got)
	}
	if got := config.ExpandHome("~/x"); strings.HasPrefix(got, "~") {
		t.Error(got)
	}
}

func TestParentFromEnvironment(t *testing.T) {
	g, repo := open(t)
	c, err := config.Load(g, repo, env(map[string]string{config.ParentEnv: "/srv/trees"}))
	if err != nil || c.Parent != "/srv/trees" {
		t.Fatalf("%+v %v", c, err)
	}
	if got := c.ResolveBaseDir(repo); got != filepath.Join("/srv/trees", filepath.Base(repo)) {
		t.Error(got)
	}
}

func TestHooksFile(t *testing.T) {
	g, repo := open(t)
	if err := config.SaveHooks(g, repo, config.Hooks{PostCreate: []config.Hook{{Type: "copy", From: ".env"}}}); err != nil {
		t.Fatal(err)
	}
	path, _ := config.HooksPath(g, repo)
	if filepath.Dir(path) != filepath.Join(repo, ".git") {
		t.Errorf("hooks file must live in the git directory: %s", path)
	}
	c, err := config.Load(g, repo, env(nil))
	if err != nil || len(c.Hooks.PostCreate) != 1 || c.Hooks.PostCreate[0].To != ".env" {
		t.Fatalf("%+v %v", c.Hooks, err)
	}
}

func TestHooksFileIsStrict(t *testing.T) {
	g, repo := open(t)
	path, _ := config.HooksPath(g, repo)
	for name, body := range map[string]string{
		"unknown field": `{"post_create":[],"nope":1}`,
		"bad hook":      `{"post_create":[{"type":"command"}]}`,
		"bad type":      `{"post_create":[{"type":"x"}]}`,
	} {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := config.Load(g, repo, env(nil)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
