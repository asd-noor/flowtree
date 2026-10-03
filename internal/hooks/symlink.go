package hooks

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"flowtree/internal/config"
)

func runSymlink(_ context.Context, out io.Writer, h config.Hook, repoRoot, worktree string) error {
	ep, err := resolveEndpoints(h, repoRoot, worktree)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(ep.dst), dirPerm); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}
	if _, err := os.Lstat(ep.dst); err == nil {
		return fmt.Errorf("destination path already exists: %s", ep.dst)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to inspect destination path: %w", err)
	}
	if _, err := fmt.Fprintf(out, "  Symlinking: %s → %s\n", rel(repoRoot, ep.src), rel(worktree, ep.dst)); err != nil {
		return err
	}
	if err := os.Symlink(ep.src, ep.dst); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}
	return nil
}
