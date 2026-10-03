package skill_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"flowtree/internal/skill"
)

func fsys() fstest.MapFS {
	return fstest.MapFS{
		"demo/SKILL.md":              {Data: []byte("---\nname: demo\ndescription: d\n---\n# demo\n")},
		"demo/references/a.md":       {Data: []byte("a")},
		"loose-file.txt":             {Data: []byte("not a skill")},
		"nomanifest/references/b.md": {Data: []byte("b")},
	}
}

func TestNamesAndFiles(t *testing.T) {
	names, err := skill.Names(fsys())
	if err != nil || len(names) != 1 || names[0] != "demo" {
		t.Fatalf("%v %v", names, err)
	}
	files, _ := skill.Files(fsys(), "demo")
	if strings.Join(files, ",") != "demo/SKILL.md,demo/references/a.md" {
		t.Errorf("%v", files)
	}
}

func TestInstallWritesAndReplaces(t *testing.T) {
	dest := t.TempDir()
	got, err := skill.Install(fsys(), dest, skill.Options{})
	if err != nil || len(got) != 1 || got[0].Replaced || len(got[0].Files) != 2 {
		t.Fatalf("%+v %v", got, err)
	}
	if data, _ := os.ReadFile(filepath.Join(dest, "demo", "references", "a.md")); string(data) != "a" {
		t.Errorf("file content %q", data)
	}
	// a stale file from an older install disappears, and the second install reports a replacement
	stale := filepath.Join(dest, "demo", "old.md")
	_ = os.WriteFile(stale, []byte("x"), 0o644)
	got, err = skill.Install(fsys(), dest, skill.Options{})
	if err != nil || !got[0].Replaced {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := os.Stat(stale); err == nil {
		t.Error("stale file should be removed on reinstall")
	}
}

func TestInstallRefusesForeignDirectory(t *testing.T) {
	dest := t.TempDir()
	foreign := filepath.Join(dest, "demo")
	_ = os.MkdirAll(foreign, 0o755)
	_ = os.WriteFile(filepath.Join(foreign, "mine.txt"), []byte("keep"), 0o644)

	if _, err := skill.Install(fsys(), dest, skill.Options{}); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(filepath.Join(foreign, "mine.txt")); err != nil {
		t.Error("a refused install must not touch the directory")
	}
	if _, err := skill.Install(fsys(), dest, skill.Options{Force: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(foreign, "SKILL.md")); err != nil {
		t.Error("--force should install")
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	dest := t.TempDir()
	got, err := skill.Install(fsys(), dest, skill.Options{DryRun: true})
	if err != nil || len(got) != 1 {
		t.Fatalf("%+v %v", got, err)
	}
	if entries, _ := os.ReadDir(dest); len(entries) != 0 {
		t.Error("dry run wrote files")
	}
}

func TestUninstall(t *testing.T) {
	dest := t.TempDir()
	if removed, err := skill.Uninstall(fsys(), dest, skill.Options{}); err != nil || len(removed) != 0 {
		t.Fatalf("nothing installed: %+v %v", removed, err)
	}
	_, _ = skill.Install(fsys(), dest, skill.Options{})
	removed, err := skill.Uninstall(fsys(), dest, skill.Options{})
	if err != nil || len(removed) != 1 {
		t.Fatalf("%+v %v", removed, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "demo")); err == nil {
		t.Error("skill still there")
	}
	foreign := filepath.Join(dest, "demo")
	_ = os.MkdirAll(foreign, 0o755)
	if _, err := skill.Uninstall(fsys(), dest, skill.Options{}); err == nil {
		t.Error("must refuse to remove a directory that is not the skill")
	}
}
