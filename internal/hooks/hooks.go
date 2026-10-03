// Package hooks runs the post-create hooks configured in .flowtree.json.
package hooks

import (
	"context"
	"fmt"
	"io"

	"flowtree/internal/config"
)

// Env names exported to command hooks.
const (
	EnvWorktreePath = "FLOWTREE_WORKTREE_PATH"
	EnvRepoRoot     = "FLOWTREE_REPO_ROOT"
)

// step runs one hook for a given repository and new worktree.
type step func(ctx context.Context, out io.Writer, h config.Hook, repoRoot, worktree string) error

var steps = map[string]step{
	config.HookCopy:    runCopy,
	config.HookSymlink: runSymlink,
	config.HookCommand: runCommand,
}

// RunPostCreate runs every post-create hook in order, streaming progress to out.
// It stops at the first failing hook.
func RunPostCreate(ctx context.Context, out io.Writer, cfg config.Config, repoRoot, worktreePath string) error {
	hs := cfg.Hooks.PostCreate
	for i, h := range hs {
		if err := runOne(ctx, out, h, repoRoot, worktreePath, i+1, len(hs)); err != nil {
			return fmt.Errorf("failed to execute hook %d: %w", i+1, err)
		}
	}
	return nil
}

func runOne(ctx context.Context, out io.Writer, h config.Hook, repoRoot, worktree string, n, total int) error {
	run, ok := steps[h.Type]
	if !ok {
		return fmt.Errorf("unknown hook type: %s", h.Type)
	}
	if _, err := fmt.Fprintf(out, "\n→ Running hook %d of %d...\n", n, total); err != nil {
		return err
	}
	if err := run(ctx, out, h, repoRoot, worktree); err != nil {
		return err
	}
	_, err := fmt.Fprintf(out, "✓ Hook %d completed\n", n)
	return err
}
