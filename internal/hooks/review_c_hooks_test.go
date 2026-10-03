package hooks

import (
	"os"
	"path/filepath"
	"testing"

	"flowtree/internal/config"
	"flowtree/internal/testutil"
)

// symlinkTree builds <repo>/tree with a nested directory, an executable, and three kinds of symlinks.
func symlinkTree(t *testing.T, repo string) string {
	t.Helper()
	tree := filepath.Join(repo, "tree")
	testutil.WriteFile(t, filepath.Join(tree, "plain.txt"), "plain\n")
	testutil.WriteFile(t, filepath.Join(tree, "sub", "deep", "x.txt"), "x\n")
	testutil.WriteFile(t, filepath.Join(tree, "bin", "run.sh"), "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(tree, "bin", "run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	for link, target := range map[string]string{
		"dirlink":    "sub",            // to a directory in the tree
		"filelink":   "plain.txt",      // to a file
		"dangling":   "does-not-exist", // to nothing
		"outside":    "../README.md",   // out of the tree
		"abslink":    "/etc",           // absolute, outside
		"sub/nested": "deep/x.txt",     // a link inside a nested directory
	} {
		if err := os.Symlink(target, filepath.Join(tree, link)); err != nil {
			t.Fatal(err)
		}
	}
	return tree
}

func TestCopyHookRecreatesSymlinksInsteadOfFollowingThem(t *testing.T) {
	repo, wt := testutil.NewRepo(t), t.TempDir()
	symlinkTree(t, repo)
	out, err := run(t, repo, wt, config.Hook{Type: config.HookCopy, From: "tree", To: "copied"})
	if err != nil {
		t.Fatalf("a tree with symlinks must copy: %v\n%s", err, out)
	}
	dst := filepath.Join(wt, "copied")
	for link, target := range map[string]string{
		"dirlink": "sub", "filelink": "plain.txt", "dangling": "does-not-exist",
		"outside": "../README.md", "abslink": "/etc", "sub/nested": "deep/x.txt",
	} {
		got, err := os.Readlink(filepath.Join(dst, link))
		if err != nil || got != target {
			t.Errorf("%s: want a symlink to %q, got %q (%v)", link, target, got, err)
		}
	}
}

func TestCopyHookDoesNotDescendIntoDirectorySymlinks(t *testing.T) {
	repo, wt := testutil.NewRepo(t), t.TempDir()
	symlinkTree(t, repo)
	if _, err := run(t, repo, wt, config.Hook{Type: config.HookCopy, From: "tree", To: "copied"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(filepath.Join(wt, "copied", "dirlink"))
	if err != nil || info.Mode()&os.ModeSymlink == 0 || info.IsDir() {
		t.Fatalf("dirlink must stay a symlink, got %v (%v)", info, err)
	}
}

func TestCopyHookKeepsRegularFileModesAndNestedDirs(t *testing.T) {
	repo, wt := testutil.NewRepo(t), t.TempDir()
	symlinkTree(t, repo)
	if _, err := run(t, repo, wt, config.Hook{Type: config.HookCopy, From: "tree", To: "copied"}); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(wt, "copied")
	if b, _ := os.ReadFile(filepath.Join(dst, "sub", "deep", "x.txt")); string(b) != "x\n" {
		t.Errorf("nested file = %q", b)
	}
	info, err := os.Stat(filepath.Join(dst, "bin", "run.sh"))
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Errorf("executable bit lost: %v %v", info, err)
	}
	if info, _ := os.Stat(filepath.Join(dst, "plain.txt")); info == nil || info.Mode().Perm() != 0o644 {
		t.Errorf("plain file mode: %v", info)
	}
}

func TestCopyHookSymlinkToFileIsNotCopiedWorldWritable(t *testing.T) {
	repo, wt := testutil.NewRepo(t), t.TempDir()
	symlinkTree(t, repo)
	if _, err := run(t, repo, wt, config.Hook{Type: config.HookCopy, From: "tree", To: "copied"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(filepath.Join(wt, "copied", "filelink"))
	if err != nil || info.Mode().IsRegular() {
		t.Fatalf("filelink must not become a regular file (it used to get mode 0777): %v %v", info, err)
	}
}

func TestCopyHookFollowsASymlinkNamedDirectlyInTheHook(t *testing.T) {
	repo, wt := testutil.NewRepo(t), t.TempDir()
	testutil.WriteFile(t, filepath.Join(repo, "real.env"), "K=1\n")
	if err := os.Symlink("real.env", filepath.Join(repo, ".env")); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, repo, wt, config.Hook{Type: config.HookCopy, From: ".env"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(filepath.Join(wt, ".env"))
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("a top-level symlink is followed: the worktree gets the real file, got %v %v", info, err)
	}
	if b, _ := os.ReadFile(filepath.Join(wt, ".env")); string(b) != "K=1\n" {
		t.Errorf(".env = %q", b)
	}
}

func TestCopyHookSymlinkToDirectoryNamedDirectlyCopiesTheDirectory(t *testing.T) {
	repo, wt := testutil.NewRepo(t), t.TempDir()
	testutil.WriteFile(t, filepath.Join(repo, "real", "a.txt"), "a\n")
	if err := os.Symlink("real", filepath.Join(repo, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, repo, wt, config.Hook{Type: config.HookCopy, From: "link"}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(wt, "link", "a.txt")); string(b) != "a\n" {
		t.Errorf("a.txt = %q", b)
	}
}

func TestCopyHookReplacesAnExistingSymlinkOnRecopy(t *testing.T) {
	repo, wt := testutil.NewRepo(t), t.TempDir()
	symlinkTree(t, repo)
	hook := config.Hook{Type: config.HookCopy, From: "tree", To: "copied"}
	for range 2 {
		if _, err := run(t, repo, wt, hook); err != nil {
			t.Fatalf("copying twice must work: %v", err)
		}
	}
}

func TestCopyHookSkipsSpecialFiles(t *testing.T) {
	repo, wt := testutil.NewRepo(t), t.TempDir()
	testutil.WriteFile(t, filepath.Join(repo, "tree", "a.txt"), "a\n")
	fifo := filepath.Join(repo, "tree", "pipe")
	if err := mkfifo(fifo); err != nil {
		t.Skip("cannot create a fifo here")
	}
	if _, err := run(t, repo, wt, config.Hook{Type: config.HookCopy, From: "tree", To: "copied"}); err != nil {
		t.Fatalf("a pipe must not block or fail the hook: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(wt, "copied", "pipe")); err == nil {
		t.Error("special files are skipped, not copied")
	}
}
