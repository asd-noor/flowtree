package commands

import (
	"errors"
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"

	"flowtree/internal/worktree"
)

// exitError makes the process exit with a child's exit code without printing anything.
type exitError struct{ code int }

func (e *exitError) Error() string { return fmt.Sprintf("exit status %d", e.code) }

// ExitCode extracts the exit code an error asks for, if any.
func ExitCode(err error) (int, bool) {
	var ee *exitError
	if errors.As(err, &ee) {
		return ee.code, true
	}
	return 0, false
}

func newExecCmd(d Deps) *cobra.Command {
	return &cobra.Command{
		Use:     "exec <worktree> -- <command> [args...]",
		Short:   "Run a command in a worktree",
		Example: "  flowtree exec feature/auth -- go test ./...\n  flowtree exec @ -- pwd",
		Args: func(cmd *cobra.Command, args []string) error {
			if cmd.ArgsLenAtDash() != 1 || len(args) < 2 {
				return errors.New("usage: flowtree exec <worktree> -- <command> [args...]")
			}
			return nil
		},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveDefault
			}
			return completeWorktrees(d)(cmd, args, prefix)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := d.OpenRepo(cmd.Context())
			if err != nil {
				return err
			}
			w, err := worktree.Resolve(repo.Worktrees, args[0])
			if err != nil {
				return err
			}
			return runIn(d, cmd, w.Path, args[1], args[2:])
		},
	}
}

func runIn(d Deps, cmd *cobra.Command, dir, name string, args []string) error {
	c := exec.CommandContext(cmd.Context(), name, args...)
	c.Dir = dir
	c.Stdin, c.Stdout, c.Stderr = d.Stdin, d.Stdout, d.Stderr
	err := c.Run()
	var ee *exec.ExitError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &ee):
		return &exitError{code: exitStatus(ee)}
	default:
		return fmt.Errorf("failed to run %q in %s: %w", name, dir, err)
	}
}

// codeOrFailure turns the -1 that ExitCode reports for "no exit code" into a plain failure status.
func codeOrFailure(code int) int {
	if code < 0 {
		return 1
	}
	return code
}
