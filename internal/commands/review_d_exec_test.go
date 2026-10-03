package commands

import (
	"testing"

	"flowtree/internal/testutil"
)

func execStatus(t *testing.T, script string) int {
	t.Helper()
	repo := testutil.NewRepo(t)
	d, _, _ := testDeps(t, repo, nil)
	err := run(t, d, "exec", "@", "--", "sh", "-c", script)
	if err == nil {
		return 0
	}
	code, ok := ExitCode(err)
	if !ok {
		t.Fatalf("expected an exit-code error, got %v", err)
	}
	return code
}

func TestExecExitStatus(t *testing.T) {
	for name, tc := range map[string]struct {
		script string
		want   int
	}{
		"success":   {"exit 0", 0},
		"exit code": {"exit 3", 3},
		"SIGTERM":   {"kill -TERM $$", 143},
		"SIGKILL":   {"kill -KILL $$", 137},
	} {
		if got := execStatus(t, tc.script); got != tc.want {
			t.Errorf("%s: status %d, want %d", name, got, tc.want)
		}
	}
}
