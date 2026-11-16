package commands

import (
	"context"
	"os/exec"

	"flowtree/internal/flow"
)

// attachedGit runs git in dir with the user's terminal attached (interactive rebase).
func attachedGit(ctx context.Context, d Deps) flow.Attached {
	return func(dir string, args ...string) error {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = dir
		cmd.Stdin, cmd.Stdout, cmd.Stderr = d.Stdin, d.Stdout, d.Stderr
		return cmd.Run()
	}
}
