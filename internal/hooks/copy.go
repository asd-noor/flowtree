package hooks

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"flowtree/internal/config"
)

const dirPerm = 0o755

func runCopy(_ context.Context, out io.Writer, h config.Hook, repoRoot, worktree string) error {
	ep, err := resolveEndpoints(h, repoRoot, worktree)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(ep.dst), dirPerm); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}
	if _, err := fmt.Fprintf(out, "  Copying: %s → %s\n", rel(repoRoot, ep.src), rel(worktree, ep.dst)); err != nil {
		return err
	}
	return copyPath(ep.src, ep.dst, ep.srcInfo)
}

// copyPath copies one filesystem entry; info must describe src without following symlinks, except for
// the hook's own `from`, which resolveEndpoints stats through a symlink: like `cp -H`, a symlink named
// directly in the hook is followed, so the new worktree gets what it points to. Inside a copied
// directory a symlink is recreated as a symlink with the same target and never followed, so a link to a
// directory or to something outside the tree is neither descended into nor turned into a copy.
// Entries that are neither files, directories nor symlinks (sockets, pipes, devices) are skipped:
// opening a pipe would block the hook forever.
func copyPath(src, dst string, info os.FileInfo) error {
	switch mode := info.Mode(); {
	case mode&os.ModeSymlink != 0:
		return copySymlink(src, dst)
	case mode.IsDir():
		return copyDir(src, dst, info)
	case mode.IsRegular():
		return copyFile(src, dst, info)
	}
	return nil
}

// copySymlink recreates the link src at dst, replacing a file or link already there.
func copySymlink(src, dst string) error {
	target, err := os.Readlink(src)
	if err != nil {
		return fmt.Errorf("failed to read symlink %s: %w", src, err)
	}
	if existing, err := os.Lstat(dst); err == nil && !existing.IsDir() {
		if err := os.Remove(dst); err != nil {
			return fmt.Errorf("failed to replace %s: %w", dst, err)
		}
	}
	if err := os.Symlink(target, dst); err != nil {
		return fmt.Errorf("failed to create symlink %s: %w", dst, err)
	}
	return nil
}

func copyFile(src, dst string, info os.FileInfo) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer in.Close()
	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func() {
		if cerr := f.Close(); err == nil && cerr != nil {
			err = fmt.Errorf("failed to write destination file: %w", cerr)
		}
	}()
	if _, err := io.Copy(f, in); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}
	return os.Chmod(dst, info.Mode().Perm())
}

func copyDir(src, dst string, info os.FileInfo) error {
	if err := os.MkdirAll(dst, info.Mode().Perm()|0o700); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}
	return eachEntry(entries, func(e os.DirEntry) error {
		ei, err := e.Info()
		if err != nil {
			return err
		}
		return copyPath(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name()), ei)
	})
}

func eachEntry(entries []os.DirEntry, f func(os.DirEntry) error) error {
	if len(entries) == 0 {
		return nil
	}
	if err := f(entries[0]); err != nil {
		return err
	}
	return eachEntry(entries[1:], f)
}
