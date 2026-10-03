package commands

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"context"

	"flowtree/internal/picker"
	"flowtree/internal/testutil"
)

// pickerFake answers Pick with the row whose key is next in keys, recording what it was shown.
type pickerFake struct {
	answer string
	err    error
	rows   []picker.Row
	query  string
	calls  int
}

func (p *pickerFake) pick(_ context.Context, rows []picker.Row, query string) (picker.Row, error) {
	p.calls++
	p.rows, p.query = rows, query
	if p.err != nil {
		return picker.Row{}, p.err
	}
	for _, r := range rows {
		if r.Key == p.answer {
			return r, nil
		}
	}
	return picker.Row{}, errors.New("fake: no row " + p.answer)
}

func (p *pickerFake) keys() []string {
	return mapSlice(p.rows, func(r picker.Row) string { return r.Key })
}

// tty makes d look like an interactive session with the given stdin and picker.
func tty(d Deps, stdin string, p *pickerFake) Deps {
	d.IsTTY = func() bool { return true }
	d.Stdin = strings.NewReader(stdin)
	d.Pick = p.pick
	return d
}

// flowRepo returns a repository set up with `init --flow -d`; its main worktree rests on develop.
func flowRepo(t *testing.T) string {
	t.Helper()
	repo := testutil.NewRepo(t)
	d, _, _ := testDeps(t, repo, nil)
	if err := run(t, d, "init", "--flow", "-d"); err != nil {
		t.Fatal(err)
	}
	return repo
}

func wtPath(repo, branch string) string {
	return filepath.Join(filepath.Dir(repo), "worktrees", filepath.FromSlash(branch))
}

func branchList(t *testing.T, repo string) string {
	return testutil.Git(t, repo, "branch", "--format=%(refname:short)")
}

func TestSwitchUniqueQueryNeedsNoPicker(t *testing.T) {
	repo := flowRepo(t)
	testutil.Git(t, repo, "branch", "work/login", "develop")
	cdfile := filepath.Join(t.TempDir(), "cd")
	d, out, _ := testDeps(t, repo, map[string]string{CdFileEnv: cdfile})
	p := &pickerFake{answer: "never"}
	d = tty(d, "", p)

	if err := run(t, d, "switch", "logi"); err != nil {
		t.Fatal(err)
	}
	want := wtPath(repo, "work/login")
	if got := strings.TrimSpace(out.String()); got != want {
		t.Errorf("printed %q, want %q", got, want)
	}
	if b, _ := os.ReadFile(cdfile); string(b) != want {
		t.Errorf("cd-file = %q", b)
	}
	if p.calls != 0 {
		t.Error("an unambiguous query must not open the picker")
	}
	if _, err := os.Stat(want); err != nil {
		t.Error("the missing worktree should have been created")
	}
}

func TestSwitchQueryMatchesMainBranch(t *testing.T) {
	repo := flowRepo(t)
	d, out, _ := testDeps(t, repo, nil)
	if err := run(t, d, "switch", "develop"); err != nil { // main worktree is on develop
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != repo {
		t.Errorf("printed %q, want main worktree %q", got, repo)
	}
}

func TestSwitchMovesMainWorktree(t *testing.T) {
	repo := flowRepo(t)
	d, out, errb := testDeps(t, repo, nil)
	if err := run(t, d, "switch", "main"); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != repo || !strings.Contains(errb.String(), "now on 'main'") {
		t.Errorf("stdout %q stderr %q", out.String(), errb.String())
	}
	if got := strings.TrimSpace(testutil.Git(t, repo, "branch", "--show-current")); got != "main" {
		t.Errorf("main worktree on %q", got)
	}
}

func TestSwitchAmbiguousQueryUsesPicker(t *testing.T) {
	repo := flowRepo(t)
	testutil.Git(t, repo, "branch", "work/login", "develop")
	testutil.Git(t, repo, "branch", "work/logout", "develop")
	p := &pickerFake{answer: "work/logout"}
	d, out, _ := testDeps(t, repo, nil)
	d = tty(d, "", p)
	if err := run(t, d, "switch", "log"); err != nil {
		t.Fatal(err)
	}
	if p.calls != 1 || p.query != "log" {
		t.Errorf("picker calls %d, query %q", p.calls, p.query)
	}
	if got, want := strings.Join(p.keys(), " "), "@ main work/login work/logout"; got != want {
		t.Errorf("rows %q, want %q", got, want)
	}
	if got := strings.TrimSpace(out.String()); got != wtPath(repo, "work/logout") {
		t.Errorf("printed %q", got)
	}
	if p.rows[1].Preview == nil || !strings.Contains(p.rows[1].Preview(), "moved onto 'main'") {
		t.Error("rows need previews")
	}
}

func TestSwitchWithoutTerminalFailsClearly(t *testing.T) {
	repo := flowRepo(t)
	testutil.Git(t, repo, "branch", "work/a", "develop")
	testutil.Git(t, repo, "branch", "work/b", "develop")
	d, _, _ := testDeps(t, repo, nil)
	for _, q := range [][]string{{"switch"}, {"switch", "work"}} {
		err := run(t, d, q...)
		if err == nil || !strings.Contains(err.Error(), "no terminal") {
			t.Errorf("%v: err = %v", q, err)
		}
	}
	d.Pick = (&pickerFake{err: picker.ErrCancelled}).pick
	if err := run(t, d, "switch"); !errors.Is(err, picker.ErrCancelled) {
		t.Errorf("a cancelled picker should surface ErrCancelled, got %v", err)
	}
}

func TestSwitchNeedsFlow(t *testing.T) {
	repo := testutil.NewRepo(t)
	d, _, _ := testDeps(t, repo, nil)
	if err := run(t, d, "switch", "x"); err == nil || !strings.Contains(err.Error(), "init --flow") {
		t.Errorf("err = %v", err)
	}
}

func TestSwitchCompletion(t *testing.T) {
	repo := flowRepo(t)
	testutil.Git(t, repo, "branch", "work/login", "develop")
	d, out, _ := testDeps(t, repo, nil)
	if err := run(t, d, "__complete", "switch", "w"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "work/login") || strings.Contains(out.String(), "main") {
		t.Errorf("completion: %q", out.String())
	}
}

func TestPrintDirPicksOnTerminal(t *testing.T) {
	repo := flowRepo(t)
	wt := wtPath(repo, "work/a")
	testutil.Git(t, repo, "worktree", "add", "-q", "-b", "work/a", wt, "develop")
	p := &pickerFake{answer: "work/a"}
	d, out, _ := testDeps(t, repo, nil)
	if err := run(t, tty(d, "", p), "print-dir"); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != wt {
		t.Errorf("printed %q, want %q", got, wt)
	}
	if got := strings.Join(p.keys(), " "); got != "@ work/a" {
		t.Errorf("rows %q: only existing worktrees, main first", got)
	}
	// a name never opens the picker, and without a terminal no name means the main worktree
	p.calls = 0
	out.Reset()
	if err := run(t, tty(d, "", p), "print-dir", "@"); err != nil || p.calls != 0 {
		t.Errorf("named: %v, picker calls %d", err, p.calls)
	}
	out.Reset()
	if err := run(t, d, "print-dir"); err != nil || strings.TrimSpace(out.String()) != repo {
		t.Errorf("no terminal: %v %q", err, out.String())
	}
}

func TestRemovePicksAndConfirms(t *testing.T) {
	repo := flowRepo(t)
	wt := wtPath(repo, "work/a")
	testutil.Git(t, repo, "worktree", "add", "-q", "-b", "work/a", wt, "develop")
	base, _, _ := testDeps(t, repo, nil)

	p := &pickerFake{answer: "work/a"}
	if err := run(t, tty(base, "n\n", p), "remove"); err == nil {
		t.Error("declining must cancel")
	}
	if _, err := os.Stat(wt); err != nil {
		t.Fatal("declined removal removed the worktree")
	}
	if got := strings.Join(p.keys(), " "); got != "work/a" {
		t.Errorf("rows %q: the main worktree must not be offered", got)
	}

	if err := run(t, tty(base, "y\n", p), "remove"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(wt); err == nil {
		t.Error("confirmed removal left the worktree")
	}
	if err := run(t, tty(base, "y\n", p), "remove"); err == nil || !strings.Contains(err.Error(), "no worktrees") {
		t.Errorf("nothing left to pick: %v", err)
	}
	if err := run(t, base, "remove"); err == nil || !strings.Contains(err.Error(), "worktree name required") {
		t.Errorf("without a terminal the usage error stays: %v", err)
	}
}

func TestCheckoutAndFinishPickOpenBranch(t *testing.T) {
	repo := flowRepo(t)
	d, out, _ := testDeps(t, repo, nil)
	for _, n := range []string{"a", "b"} {
		if err := run(t, d, "work", "start", n); err != nil {
			t.Fatal(err)
		}
	}
	out.Reset()
	p := &pickerFake{answer: "b"}
	if err := run(t, tty(d, "", p), "work", "checkout"); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != wtPath(repo, "work/b") {
		t.Errorf("checkout printed %q", got)
	}
	if got := strings.Join(p.keys(), " "); got != "a b" {
		t.Errorf("rows %q", got)
	}

	testutil.WriteFile(t, filepath.Join(wtPath(repo, "work/b"), "b.txt"), "b\n")
	testutil.Git(t, wtPath(repo, "work/b"), "add", "-A")
	testutil.Git(t, wtPath(repo, "work/b"), "commit", "-q", "-m", "b")
	if err := run(t, tty(d, "", p), "work", "finish"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(branchList(t, repo), "work/b") || !strings.Contains(branchList(t, repo), "work/a") {
		t.Errorf("finish should have merged and removed only work/b:\n%s", branchList(t, repo))
	}
}

func TestFinishInsideWorktreeDoesNotPick(t *testing.T) {
	repo := flowRepo(t)
	d, _, _ := testDeps(t, repo, nil)
	if err := run(t, d, "work", "start", "a"); err != nil {
		t.Fatal(err)
	}
	wt := wtPath(repo, "work/a")
	testutil.WriteFile(t, filepath.Join(wt, "a.txt"), "a\n")
	testutil.Git(t, wt, "add", "-A")
	testutil.Git(t, wt, "commit", "-q", "-m", "a")
	p := &pickerFake{answer: "never"}
	inside, _, _ := testDeps(t, wt, nil)
	if err := run(t, tty(inside, "", p), "work", "finish"); err != nil {
		t.Fatal(err)
	}
	if p.calls != 0 {
		t.Error("the branch is known from the working directory; no picker")
	}
}

func TestFinishNoNameWithoutTerminalKeepsError(t *testing.T) {
	repo := flowRepo(t)
	d, _, _ := testDeps(t, repo, nil)
	err := run(t, d, "work", "finish")
	if err == nil || !strings.Contains(err.Error(), "specify a <name>") {
		t.Errorf("err = %v", err)
	}
}
