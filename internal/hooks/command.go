package hooks

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"flowtree/internal/config"
)

func runCommand(ctx context.Context, out io.Writer, h config.Hook, repoRoot, worktree string) error {
	if _, err := fmt.Fprintf(out, "  Running: %s\n", h.Command); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", h.Command)
	cmd.Dir = workDir(h, worktree)
	cmd.Env = commandEnv(os.Environ(), h, repoRoot, worktree)
	// One writer for both streams: os/exec serialises writes when Stdout and Stderr are the same value.
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}
	return nil
}

func workDir(h config.Hook, worktree string) string {
	switch {
	case h.WorkDir == "":
		return worktree
	case filepath.IsAbs(h.WorkDir):
		return h.WorkDir
	}
	return filepath.Join(worktree, h.WorkDir)
}

// commandEnv is base, then the hook's own variables, then ours (which win).
func commandEnv(base []string, h config.Hook, repoRoot, worktree string) []string {
	env := append([]string(nil), base...)
	for k, v := range h.Env {
		env = append(env, k+"="+v)
	}
	return append(env, EnvWorktreePath+"="+worktree, EnvRepoRoot+"="+repoRoot)
}
