// Package e2e runs the real flowtree binary against real git repositories.
//
// Typical use:
//
//	repo := testutil.NewRepo(t)
//	res := e2e.Run(t, repo, "init", "-d")
//	res.MustSucceed(t)
package e2e

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Result is the outcome of one flowtree invocation.
type Result struct {
	Stdout, Stderr string
	Code           int
}

// MustSucceed fails the test unless the command exited 0.
func (r Result) MustSucceed(t testing.TB) Result {
	t.Helper()
	if r.Code != 0 {
		t.Fatalf("flowtree exited %d\nstdout: %s\nstderr: %s", r.Code, r.Stdout, r.Stderr)
	}
	return r
}

// MustFail fails the test unless the command exited non-zero.
func (r Result) MustFail(t testing.TB) Result {
	t.Helper()
	if r.Code == 0 {
		t.Fatalf("flowtree unexpectedly succeeded\nstdout: %s\nstderr: %s", r.Stdout, r.Stderr)
	}
	return r
}

var (
	buildOnce sync.Once
	binPath   string
	errBuild  error
)

// Binary builds ./cmd/flowtree once per test run and returns its path.
func Binary(t testing.TB) string {
	t.Helper()
	buildOnce.Do(func() { binPath, errBuild = build() })
	if errBuild != nil {
		t.Fatalf("build flowtree: %v", errBuild)
	}
	return binPath
}

func build() (string, error) {
	root, err := moduleRoot()
	if err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp("", "flowtree-e2e-")
	if err != nil {
		return "", err
	}
	bin := filepath.Join(dir, "flowtree")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/flowtree")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("%w\n%s", err, out)
	}
	return bin, nil
}

// moduleRoot walks up from the working directory to the directory holding go.mod.
func moduleRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return findUp(wd, "go.mod")
}

func findUp(dir, name string) (string, error) {
	if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
		return dir, nil
	}
	parent := filepath.Dir(dir)
	if parent == dir {
		return "", fmt.Errorf("%s not found above the working directory", name)
	}
	return findUp(parent, name)
}

// Env is the controlled environment: isolated HOME and git config, FLOWTREE_* unset,
// plus extra KEY=VALUE pairs (which win).
func Env(home string, extra ...string) []string {
	kept := dropPrefixed(os.Environ(), "FLOWTREE_", "HOME=", "GIT_CONFIG_GLOBAL=", "GIT_CONFIG_SYSTEM=")
	base := []string{
		"HOME=" + home,
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
	}
	return append(append(kept, base...), extra...)
}

func dropPrefixed(env []string, prefixes ...string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if !hasAnyPrefix(kv, prefixes) {
			out = append(out, kv)
		}
	}
	return out
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// Run runs flowtree in dir with no stdin (so no TTY: prompts are off).
func Run(t testing.TB, dir string, args ...string) Result {
	t.Helper()
	return RunEnv(t, dir, nil, args...)
}

// RunEnv is Run with extra KEY=VALUE environment entries, e.g. "FLOWTREE_PARENT=/x".
func RunEnv(t testing.TB, dir string, extra []string, args ...string) Result {
	t.Helper()
	cmd := exec.Command(Binary(t), args...)
	cmd.Dir = dir
	cmd.Env = Env(t.TempDir(), extra...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit):
	default:
		t.Fatalf("run flowtree %v: %v", args, err)
	}
	return Result{Stdout: stdout.String(), Stderr: stderr.String(), Code: cmd.ProcessState.ExitCode()}
}
