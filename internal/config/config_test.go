package config_test

import (
	"path/filepath"
	"strings"
	"testing"

	"flowtree/internal/config"
)

func TestLoadMissingGivesDefaults(t *testing.T) {
	c, err := config.Load(t.TempDir())
	if err != nil || c.BaseDir != config.DefaultBaseDir || c.Remote != "origin" {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	in := config.Config{BaseDir: "/x", Hooks: config.Hooks{PostCreate: []config.Hook{{Type: "copy", From: ".env"}}}}
	if err := config.Save(dir, in); err != nil {
		t.Fatal(err)
	}
	out, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if out.BaseDir != "/x" || out.Hooks.PostCreate[0].To != ".env" {
		t.Fatalf("%+v", out)
	}
}

func TestParseRejects(t *testing.T) {
	for name, in := range map[string]string{
		"unknown field": `{"version":1,"nope":1}`,
		"bad hook":      `{"version":1,"hooks":{"post_create":[{"type":"command"}]}}`,
		"bad version":   `{"version":9}`,
		"bad type":      `{"version":1,"hooks":{"post_create":[{"type":"x"}]}}`,
	} {
		if _, err := config.Parse([]byte(in)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestWorktreePath(t *testing.T) {
	c := config.Defaults()
	if got := c.WorktreePath("/code/api", "work/login"); got != "/code/worktrees/work/login" {
		t.Error(got)
	}
	c.BaseDir = "/abs"
	if got := c.WorktreePath("/code/api", "x"); got != "/abs/x" {
		t.Error(got)
	}
}

func TestDefaultBaseDirFor(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	if got := config.DefaultBaseDirFor("/code/api", env(nil)); got != config.DefaultBaseDir {
		t.Error(got)
	}
	got := config.DefaultBaseDirFor("/code/api", env(map[string]string{config.ParentEnv: "/wt"}))
	if got != filepath.Join("/wt", "api") {
		t.Error(got)
	}
	if got := config.ExpandHome("~/x"); strings.HasPrefix(got, "~") {
		t.Error(got)
	}
}
