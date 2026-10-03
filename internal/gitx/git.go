// Package gitx runs git and wraps the few operations flowtree needs.
//
// Everything is built on Runner, so tests can swap in a fake.
package gitx

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
)

// Runner runs git with args in dir and returns trimmed stdout.
type Runner func(ctx context.Context, dir string, args ...string) (string, error)

// Git is a Runner bound to a context and a directory.
type Git func(args ...string) (string, error)

// Error is a failed git invocation. Output is what git printed (stderr, else stdout).
type Error struct {
	Args   []string
	Output string
	Err    error
}

func (e *Error) Error() string {
	cmd := "git " + strings.Join(e.Args, " ")
	if e.Output == "" {
		return cmd + ": " + e.Err.Error()
	}
	return cmd + ": " + e.Output
}

func (e *Error) Unwrap() error { return e.Err }

// Exec is the real Runner.
func Exec(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		out := strings.TrimSpace(stderr.String())
		if out == "" {
			out = strings.TrimSpace(stdout.String())
		}
		return "", &Error{Args: args, Output: out, Err: err}
	}
	return strings.TrimRight(stdout.String(), "\r\n"), nil
}

// Bind fixes the context and directory of a Runner.
func Bind(ctx context.Context, run Runner, dir string) Git {
	return func(args ...string) (string, error) { return run(ctx, dir, args...) }
}

// In returns a Git for dir backed by the real git.
func In(ctx context.Context, dir string) Git { return Bind(ctx, Exec, dir) }

// Ok reports whether git succeeded.
func (g Git) Ok(args ...string) bool {
	_, err := g(args...)
	return err == nil
}

// Lines runs git and splits its output into non-empty lines.
func (g Git) Lines(args ...string) ([]string, error) {
	out, err := g(args...)
	if err != nil {
		return nil, err
	}
	return nonEmptyLines(out), nil
}

func nonEmptyLines(s string) []string {
	return filter(strings.Split(s, "\n"), func(l string) bool { return strings.TrimSpace(l) != "" })
}

func filter[T any](xs []T, keep func(T) bool) []T {
	out := make([]T, 0, len(xs))
	for _, x := range xs {
		if keep(x) {
			out = append(out, x)
		}
	}
	return out
}
