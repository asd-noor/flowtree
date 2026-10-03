package commands

import (
	"strings"
	"testing"

	"flowtree/internal/testutil"
)

// --force must be able to put back a configuration it cannot parse (an unknown key from a typo) when init
// fails after the reset.
func TestInitForceRestoresABrokenConfigurationWhenInitFails(t *testing.T) {
	repo := testutil.NewRepo(t)
	testutil.Git(t, repo, "config", "flowtree.baseDir", "/keep")
	testutil.Git(t, repo, "config", "flowtree.typo", "1")
	d, _, _ := initDeps(t, repo, "", false, nil)

	// --flow with master == develop fails after the reset
	err := runInitCmd(t, d, "--force", "--flow", "-d", "--master", "x", "--develop", "x")
	if err == nil {
		t.Fatal("expected init to fail")
	}
	got := localKeys(t, repo)
	if !strings.Contains(got, "flowtree.basedir /keep") || !strings.Contains(got, "flowtree.typo 1") {
		t.Fatalf("the old settings, typo included, must be back; git config has:\n%s", got)
	}
}

func TestInitForceRestoreKeepsEveryValue(t *testing.T) {
	repo := testutil.NewRepo(t)
	testutil.Git(t, repo, "config", "flowtree.remote", "up")
	testutil.Git(t, repo, "config", "flowtree.versionTagPrefix", "") // an empty value must survive too
	d, _, _ := initDeps(t, repo, "", false, nil)
	if err := runInitCmd(t, d, "--force", "--flow", "-d", "--master", "x", "--develop", "x"); err == nil {
		t.Fatal("expected init to fail")
	}
	got := localKeys(t, repo)
	if !strings.Contains(got, "flowtree.remote up") || !strings.Contains(got, "flowtree.versiontagprefix") {
		t.Fatalf("%s", got)
	}
}
