package skill_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"flowtree/internal/skill"
)

// failingFS errors when one file is opened, which is what a read failure in the middle of an install looks like.
type failingFS struct {
	fs.FS
	failOn string
}

func (f failingFS) Open(name string) (fs.File, error) {
	if name == f.failOn {
		return nil, errors.New("injected read failure")
	}
	return f.FS.Open(name)
}

func twoSkills() fstest.MapFS {
	return fstest.MapFS{
		"alpha/SKILL.md":        {Data: []byte("---\nname: alpha\ndescription: a\n---\nNEW\n")},
		"alpha/references/x.md": {Data: []byte("x")},
		"beta/SKILL.md":         {Data: []byte("---\nname: beta\ndescription: b\n---\nNEW\n")},
		"beta/references/y.md":  {Data: []byte("y")},
	}
}

func installedBody(t *testing.T, dest, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dest, name, "SKILL.md"))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return string(b)
}

func entries(t *testing.T, dir string) []string {
	t.Helper()
	list, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(list))
	for i, e := range list {
		names[i] = e.Name()
	}
	return names
}

func oldInstall(t *testing.T, dest string) {
	t.Helper()
	for _, n := range []string{"alpha", "beta"} {
		dir := filepath.Join(dest, n)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body := "---\nname: " + n + "\ndescription: old\n---\nOLD\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "stale.md"), []byte("stale"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFailedInstallKeepsTheOldSkillsAndLeavesNoTempDirs(t *testing.T) {
	dest := t.TempDir()
	oldInstall(t, dest)
	// alpha stages fine, beta fails midway: nothing may change, not even alpha
	broken := failingFS{FS: twoSkills(), failOn: "beta/references/y.md"}
	if _, err := skill.Install(broken, dest, skill.Options{}); err == nil || !strings.Contains(err.Error(), "injected") {
		t.Fatalf("got %v", err)
	}
	for _, n := range []string{"alpha", "beta"} {
		if !strings.Contains(installedBody(t, dest, n), "OLD") {
			t.Errorf("%s was changed by a failed install", n)
		}
		if _, err := os.Stat(filepath.Join(dest, n, "stale.md")); err != nil {
			t.Errorf("%s lost its files", n)
		}
	}
	if got := entries(t, dest); strings.Join(got, ",") != "alpha,beta" {
		t.Errorf("leftovers after a failed install: %v", got)
	}
}

func TestSuccessfulInstallReplacesStaleFilesAndLeavesNoTempDirs(t *testing.T) {
	dest := t.TempDir()
	oldInstall(t, dest)
	got, err := skill.Install(twoSkills(), dest, skill.Options{})
	if err != nil || len(got) != 2 || !got[0].Replaced {
		t.Fatalf("%+v %v", got, err)
	}
	for _, n := range []string{"alpha", "beta"} {
		if !strings.Contains(installedBody(t, dest, n), "NEW") {
			t.Errorf("%s not replaced", n)
		}
		if _, err := os.Stat(filepath.Join(dest, n, "stale.md")); err == nil {
			t.Errorf("%s keeps a stale file", n)
		}
		info, _ := os.Stat(filepath.Join(dest, n))
		if info.Mode().Perm() != 0o755 {
			t.Errorf("%s has mode %v, want 0755", n, info.Mode().Perm())
		}
	}
	if got := entries(t, dest); strings.Join(got, ",") != "alpha,beta" {
		t.Errorf("leftovers after install: %v", got)
	}
}

func TestForceOnForeignDirectoryIsNotDestroyedWhenStagingFails(t *testing.T) {
	dest := t.TempDir()
	foreign := filepath.Join(dest, "alpha")
	if err := os.MkdirAll(foreign, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "mine.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	broken := failingFS{FS: twoSkills(), failOn: "alpha/references/x.md"}
	if _, err := skill.Install(broken, dest, skill.Options{Force: true}); err == nil {
		t.Fatal("expected the injected failure")
	}
	if b, err := os.ReadFile(filepath.Join(foreign, "mine.txt")); err != nil || string(b) != "keep" {
		t.Errorf("a failed forced install destroyed the directory: %q %v", b, err)
	}
	if got := entries(t, dest); strings.Join(got, ",") != "alpha" {
		t.Errorf("leftovers: %v", got)
	}
	// and when staging works, --force does replace it
	if _, err := skill.Install(twoSkills(), dest, skill.Options{Force: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(installedBody(t, dest, "alpha"), "NEW") {
		t.Error("--force should replace the foreign directory")
	}
}

func TestRefusedForeignDirectoryStopsBeforeAnythingIsWritten(t *testing.T) {
	dest := t.TempDir()
	foreign := filepath.Join(dest, "beta")
	if err := os.MkdirAll(foreign, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := skill.Install(twoSkills(), dest, skill.Options{}); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("got %v", err)
	}
	// alpha comes first, but the refusal for beta must stop the whole install
	if got := entries(t, dest); strings.Join(got, ",") != "beta" {
		t.Errorf("a refused install wrote something: %v", got)
	}
}

func TestUnwritableDestinationLeavesTheOldInstallAlone(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permissions are not enforced for root")
	}
	dest := t.TempDir()
	oldInstall(t, dest)
	if err := os.Chmod(dest, 0o555); err != nil { // staging cannot create its temp directory
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(dest, 0o755) }()
	if _, err := skill.Install(twoSkills(), dest, skill.Options{}); err == nil {
		t.Fatal("expected a permission error")
	}
	for _, n := range []string{"alpha", "beta"} {
		if !strings.Contains(installedBody(t, dest, n), "OLD") {
			t.Errorf("%s was changed", n)
		}
	}
}
