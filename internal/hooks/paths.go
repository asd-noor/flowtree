package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"flowtree/internal/config"
)

// endpoints are the resolved source and destination of a copy or symlink hook.
type endpoints struct {
	src, dst string
	srcInfo  os.FileInfo
}

// resolveUnder makes p absolute against base. Relative paths must stay inside base.
func resolveUnder(base, p string) (string, error) {
	if filepath.IsAbs(p) {
		return filepath.Clean(p), nil
	}
	full := filepath.Join(base, p)
	return full, ensureWithinBase(base, full)
}

func ensureWithinBase(base, target string) error {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return fmt.Errorf("failed to resolve path %s relative to %s: %w", target, base, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("path %s escapes base directory %s", target, base)
	}
	return nil
}

// resolveEndpoints checks both sides of a copy or symlink hook: from is relative to the
// main worktree, to is relative to the new one, and the source must exist.
func resolveEndpoints(h config.Hook, repoRoot, worktree string) (endpoints, error) {
	src, err := resolveUnder(repoRoot, h.From)
	if err != nil {
		return endpoints{}, err
	}
	dst, err := resolveUnder(worktree, h.To)
	if err != nil {
		return endpoints{}, err
	}
	info, err := os.Stat(src)
	if err != nil {
		return endpoints{}, fmt.Errorf("source path does not exist: %s", src)
	}
	if err := ensureDistinct(src, dst, info); err != nil {
		return endpoints{}, err
	}
	return endpoints{src: src, dst: dst, srcInfo: info}, nil
}

func ensureDistinct(src, dst string, srcInfo os.FileInfo) error {
	same := fmt.Errorf("source and destination paths must be different: %s -> %s", src, dst)
	if src == dst {
		return same
	}
	dstInfo, err := os.Stat(dst)
	switch {
	case os.IsNotExist(err):
		return nil
	case err != nil:
		return fmt.Errorf("failed to inspect destination path: %w", err)
	case os.SameFile(srcInfo, dstInfo):
		return same
	}
	return nil
}

func rel(base, p string) string {
	if r, err := filepath.Rel(base, p); err == nil {
		return r
	}
	return p
}
