package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"flowtree/internal/testutil"
)

func TestAddFailingExecKeepsPathAndCdTarget(t *testing.T) {
	repo := testutil.NewRepo(t)
	cdfile := filepath.Join(t.TempDir(), "cd")
	res := RunEnv(t, repo, []string{"FLOWTREE_CDFILE=" + cdfile}, "add", "-b", "feat", "--quiet", "--exec", "exit 7").MustFail(t)
	wt := filepath.Join(filepath.Dir(repo), "worktrees", "feat")
	if strings.TrimSpace(res.Stdout) != wt {
		t.Errorf("stdout = %q, want the path %s", res.Stdout, wt)
	}
	if b, _ := os.ReadFile(cdfile); string(b) != wt {
		t.Errorf("cd-file = %q", b)
	}
	if !strings.Contains(res.Stderr, "--exec command failed") {
		t.Errorf("stderr = %q", res.Stderr)
	}
}
