// Package commands builds the cobra command tree. Commands hold no state of their own:
// everything they touch arrives through Deps, so tests can swap it.
package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"flowtree/internal/config"
	"flowtree/internal/gitx"
	"flowtree/internal/picker"
)

// Deps are the process-level collaborators of every command.
type Deps struct {
	Run    gitx.Runner
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Getenv func(string) string
	Cwd    func() (string, error)
	// IsTTY reports whether interactive UI (picker, prompts) can be used.
	IsTTY func() bool
	// Pick shows an interactive picker over rows, with query prefilled.
	Pick func(ctx context.Context, rows []picker.Row, query string) (picker.Row, error)
}

// RealDeps wires the process environment.
func RealDeps() Deps {
	return Deps{
		Run:    gitx.Exec,
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Getenv: os.Getenv,
		Cwd:    os.Getwd,
		IsTTY:  func() bool { return isTerminal(os.Stdin) && isTerminal(os.Stderr) },
		Pick:   realPick,
	}
}

// realPick runs the TUI, but only when a terminal is attached.
func realPick(ctx context.Context, rows []picker.Row, query string) (picker.Row, error) {
	if !(isTerminal(os.Stdin) && isTerminal(os.Stderr)) {
		return picker.Row{}, errors.New("no terminal: pass a name")
	}
	return picker.Pick(ctx, rows, query, os.Stdin, os.Stderr)
}

// pick is Pick with a clear error when no picker is wired.
func (d Deps) pick(ctx context.Context, rows []picker.Row, query string) (picker.Row, error) {
	if d.Pick == nil {
		return picker.Row{}, errors.New("no terminal: pass a name")
	}
	return d.Pick(ctx, rows, query)
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// Repo is the repository a command runs against, resolved from the current directory.
type Repo struct {
	Git       gitx.Git // bound to the main worktree
	Main      string   // path of the main worktree
	Cwd       string
	Config    config.Config
	Worktrees []gitx.Worktree
}

// OpenRepo locates the repository containing the current directory and loads its config.
func (d Deps) OpenRepo(ctx context.Context) (Repo, error) {
	cwd, err := d.Cwd()
	if err != nil {
		return Repo{}, err
	}
	here := gitx.Bind(ctx, d.Run, cwd)
	wts, err := gitx.Worktrees(here)
	if err != nil || len(wts) == 0 {
		return Repo{}, fmt.Errorf("not in a git repository")
	}
	main := wts[0].Path
	g := gitx.Bind(ctx, d.Run, main)
	cfg, err := config.Load(g, main, d.Getenv)
	if err != nil {
		return Repo{}, err
	}
	return Repo{Git: g, Main: main, Cwd: cwd, Config: cfg, Worktrees: wts}, nil
}

// Printf writes to stdout, ignoring write errors like fmt.Printf does.
func (d Deps) Printf(format string, a ...any) { _, _ = fmt.Fprintf(d.Stdout, format, a...) }

// Eprintf writes to stderr.
func (d Deps) Eprintf(format string, a ...any) { _, _ = fmt.Fprintf(d.Stderr, format, a...) }
