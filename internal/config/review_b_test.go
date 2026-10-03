package config_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"flowtree/internal/config"
	"flowtree/internal/gitx"
	"flowtree/internal/testutil"
)

// fakeRunner answers `git config` calls from canned output and records every argument list.
func fakeRunner(replies map[string]string, calls *[][]string) gitx.Runner {
	return func(_ context.Context, _ string, args ...string) (string, error) {
		*calls = append(*calls, args)
		scope := "all"
		for _, a := range args {
			if strings.HasPrefix(a, "--") && slices.Contains([]string{"--system", "--global", "--local"}, a) {
				scope = strings.TrimPrefix(a, "--")
			}
		}
		out, ok := replies[scope]
		if !ok {
			return "", &gitx.Error{Args: args, Err: os.ErrNotExist} // git's "nothing matched" exit status
		}
		return out, nil
	}
}

func TestEntriesNeverUseShowScope(t *testing.T) {
	var calls [][]string
	run := fakeRunner(map[string]string{
		"all":    "flowtree.remote\nup\x00flowtree.basedir\n/x\x00",
		"global": "flowtree.remote\nup\x00",
		"local":  "flowtree.basedir\n/x\x00",
	}, &calls)
	g := gitx.Bind(context.Background(), run, ".")
	entries, err := config.Entries(g)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range calls {
		if slices.Contains(c, "--show-scope") {
			t.Fatalf("git %v: --show-scope needs git 2.26", c)
		}
	}
	got := map[string]string{}
	for _, e := range entries {
		got[e.Key] = e.Scope
	}
	if got["remote"] != "global" || got["baseDir"] != "local" {
		t.Errorf("scopes = %v", got)
	}
}

func TestLoadIsASingleGitCall(t *testing.T) {
	var calls [][]string
	run := fakeRunner(map[string]string{"all": "flowtree.remote\nup\x00"}, &calls)
	g := gitx.Bind(context.Background(), run, ".")
	// the hooks file lookup needs a git call too, so count only `config` calls
	_, _ = config.Load(g, ".", env(nil))
	n := 0
	for _, c := range calls {
		if len(c) > 0 && c[0] == "config" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("Load made %d `git config` calls, want 1 (scopes are only needed for display): %v", n, calls)
	}
}

func TestEntriesMarkUnattributedValuesOther(t *testing.T) {
	var calls [][]string
	run := fakeRunner(map[string]string{"all": "flowtree.remote\nfrom-somewhere\x00"}, &calls)
	entries, err := config.Entries(gitx.Bind(context.Background(), run, "."))
	if err != nil || len(entries) != 1 || entries[0].Scope != "other" {
		t.Errorf("%+v %v", entries, err)
	}
}

func TestEntriesNoMatchIsEmpty(t *testing.T) {
	var calls [][]string
	run := fakeRunner(nil, &calls)
	entries, err := config.Entries(gitx.Bind(context.Background(), run, "."))
	if err != nil || len(entries) != 0 {
		t.Errorf("%+v %v", entries, err)
	}
}

func TestEntriesFollowIncludesInAScope(t *testing.T) {
	g, repo := open(t)
	dir := t.TempDir()
	extra := filepath.Join(dir, "flowtree.inc")
	global := filepath.Join(dir, "gitconfig")
	testutil.Git(t, repo, "config", "--file", extra, "flowtree.remote", "included")
	testutil.Git(t, repo, "config", "--file", global, "include.path", extra)
	t.Setenv("GIT_CONFIG_GLOBAL", global)

	entries, err := config.Entries(g)
	if err != nil || len(entries) != 1 {
		t.Fatalf("%+v %v", entries, err)
	}
	if entries[0].Value != "included" || entries[0].Scope != "global" {
		t.Errorf("a value from a file included by the global config belongs to the global scope: %+v", entries[0])
	}
}
