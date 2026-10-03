// Package commands builds the cobra command tree. Commands hold no state of their own:
// everything they touch arrives through Deps, so tests can swap it.
package commands

import (
	"context"
	"fmt"
	"io"
	"os"

	"flowtree/internal/config"
	"flowtree/internal/gitx"
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
	}
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
	cfg, err := config.Load(main)
	if err != nil {
		return Repo{}, err
	}
	return Repo{Git: gitx.Bind(ctx, d.Run, main), Main: main, Cwd: cwd, Config: cfg, Worktrees: wts}, nil
}

// Printf writes to stdout, ignoring write errors like fmt.Printf does.
func (d Deps) Printf(format string, a ...any) { _, _ = fmt.Fprintf(d.Stdout, format, a...) }

// Eprintf writes to stderr.
func (d Deps) Eprintf(format string, a ...any) { _, _ = fmt.Fprintf(d.Stderr, format, a...) }
