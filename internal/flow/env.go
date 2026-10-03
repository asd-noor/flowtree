package flow

import (
	"context"
	"fmt"
	"io"

	"flowtree/internal/config"
	"flowtree/internal/gitx"
	"flowtree/internal/hooks"
)

// Env is everything flow operations need from the outside world.
type Env struct {
	Ctx    context.Context
	Run    gitx.Runner
	S      Settings
	Config config.Config
	Main   string // main worktree path
	Cwd    string // where the user ran the command, symlinks resolved
	Out    io.Writer
	Err    io.Writer
	// Interactive is true when a terminal is attached; git may then open an editor for tag messages.
	Interactive bool
	// Attach runs git with the user's terminal attached. It is used where git needs one (the editor for
	// a tag message); without it flowtree never leaves git to open an editor.
	Attach Attached
	// CdTo asks the shell wrapper to cd somewhere once the command ends.
	CdTo func(path string)
}

// Git returns git bound to dir.
func (e *Env) Git(dir string) gitx.Git { return gitx.Bind(e.Ctx, e.Run, dir) }

// MainGit returns git bound to the main worktree.
func (e *Env) MainGit() gitx.Git { return e.Git(e.Main) }

// Worktrees lists the current worktrees; it is never cached, since steps change them.
func (e *Env) Worktrees() ([]gitx.Worktree, error) { return gitx.Worktrees(e.MainGit()) }

// WorktreeOf finds the worktree that has branch checked out.
func (e *Env) WorktreeOf(branch string) (gitx.Worktree, bool, error) {
	wts, err := e.Worktrees()
	if err != nil {
		return gitx.Worktree{}, false, err
	}
	w, ok := gitx.WorktreeOf(wts, branch)
	return w, ok, nil
}

// Say prints a line to stdout.
func (e *Env) Say(format string, a ...any) { _, _ = fmt.Fprintf(e.Out, format+"\n", a...) }

// Warn prints a warning to stderr.
func (e *Env) Warn(format string, a ...any) { _, _ = fmt.Fprintf(e.Err, "Warning: "+format+"\n", a...) }

// Note prints an informational line to stderr.
func (e *Env) Note(format string, a ...any) { _, _ = fmt.Fprintf(e.Err, format+"\n", a...) }

func (e *Env) cd(path string) {
	if e.CdTo != nil {
		e.CdTo(path)
	}
}

// runHooks runs the post-create hooks for a new worktree; failures are warnings.
func (e *Env) runHooks(path string) {
	if err := hooks.RunPostCreate(e.Ctx, e.Err, e.Config, e.Main, path); err != nil {
		e.Warn("hook failed: %v", err)
	}
}

// editorAvailable reports whether git may open an editor: there is a terminal and a way to attach it.
func (e *Env) editorAvailable() bool { return e.Interactive && e.Attach != nil }
