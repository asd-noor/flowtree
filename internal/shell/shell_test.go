package shell

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInit(t *testing.T) {
	for _, name := range Supported {
		s, err := Init(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, want := range []string{"FLOWTREE_CDFILE", "completion " + name} {
			if !strings.Contains(s, want) {
				t.Errorf("%s script lacks %q", name, want)
			}
		}
	}
	if _, err := Init("csh"); err == nil {
		t.Error("want error for unsupported shell")
	}
}

func TestDetect(t *testing.T) {
	if got := Detect("/usr/bin/zsh"); got != "zsh" {
		t.Errorf("Detect = %q", got)
	}
}

// The bash wrapper must cd into the path the program writes to FLOWTREE_CDFILE
// and keep the program's exit code.
func TestBashWrapperFollowsCdFile(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not found")
	}
	dir, target := t.TempDir(), t.TempDir()
	fake := "#!/bin/sh\n" +
		"[ \"$1\" = completion ] && exit 0\n" +
		"printf %s \"$TARGET\" > \"$FLOWTREE_CDFILE\"\nexit 3\n"
	if err := os.WriteFile(filepath.Join(dir, "flowtree"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	script, _ := Init("bash")
	cmd := exec.Command(bash, "-c", script+"\nflowtree go; rc=$?; echo \"$rc $PWD\"")
	cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "TARGET="+target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	want, _ := filepath.EvalSymlinks(target)
	if got := strings.TrimSpace(string(out)); got != "3 "+want && got != "3 "+target {
		t.Errorf("got %q, want exit 3 in %s", got, want)
	}
}

func TestFishWrapperFollowsCdFile(t *testing.T) {
	fish, err := exec.LookPath("fish")
	if err != nil {
		t.Skip("fish not found")
	}
	dir, target := t.TempDir(), t.TempDir()
	fake := "#!/bin/sh\n" +
		"[ \"$1\" = completion ] && exit 0\n" +
		"printf %s \"$TARGET\" > \"$FLOWTREE_CDFILE\"\nexit 3\n"
	if err := os.WriteFile(filepath.Join(dir, "flowtree"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	script, _ := Init("fish")
	cmd := exec.Command(fish, "--no-config", "-c", script+"\nflowtree go; echo $status (pwd)")
	cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "TARGET="+target)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	want, _ := filepath.EvalSymlinks(target)
	if got := strings.TrimSpace(string(out)); got != "3 "+want && got != "3 "+target {
		t.Errorf("got %q, want exit 3 in %s", got, want)
	}
}
