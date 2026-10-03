package commands

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"flowtree/skills"
)

func TestInstallSkillCommand(t *testing.T) {
	dest := t.TempDir()
	d, out, _ := testDeps(t, ".", nil)
	if err := run(t, d, "install-skill", dest); err != nil || !strings.Contains(out.String(), "Installed skill 'flowtree'") {
		t.Fatalf("%q %v", out, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "flowtree", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := run(t, d, "install-skill", dest, "--uninstall"); err != nil || !strings.Contains(out.String(), "Removed") {
		t.Fatalf("%q %v", out, err)
	}
}

// The embedded skill must describe commands that exist: every `flowtree <command>` inside a code span or
// code block names a real command, and `flowtree work|release|hotfix <sub>` a real subcommand.
func TestSkillMentionsOnlyRealCommands(t *testing.T) {
	root := NewRoot(Deps{}, "test")
	top := map[string]bool{}
	for _, c := range root.Commands() {
		top[c.Name()] = true
	}
	mention := regexp.MustCompile(`flowtree\s+([a-z][a-z-]*)(?:\s+([a-z][a-z-]*))?`)
	err := fs.WalkDir(skills.FS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".md") {
			return err
		}
		data, _ := fs.ReadFile(skills.FS, p)
		for _, span := range codeSpans(string(data)) {
			for _, m := range mention.FindAllStringSubmatch(span, -1) {
				checkMention(t, p, root, top, m[1], m[2])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func checkMention(t *testing.T, file string, root *cobra.Command, top map[string]bool, first, second string) {
	t.Helper()
	if !top[first] {
		t.Errorf("%s: 'flowtree %s' is not a command", file, first)
		return
	}
	if first != "work" && first != "release" && first != "hotfix" || second == "" {
		return
	}
	for _, c := range root.Commands() {
		if c.Name() != first {
			continue
		}
		for _, s := range c.Commands() {
			if s.Name() == second {
				return
			}
		}
		t.Errorf("%s: 'flowtree %s %s' is not a subcommand", file, first, second)
	}
}

// codeSpans returns the text inside backticks and fenced blocks.
func codeSpans(doc string) []string {
	var spans []string
	inFence := false
	for _, line := range strings.Split(doc, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			spans = append(spans, line)
			continue
		}
		parts := strings.Split(line, "`")
		for i := 1; i < len(parts); i += 2 {
			spans = append(spans, parts[i])
		}
	}
	return spans
}

func TestSkillFrontMatter(t *testing.T) {
	names, _ := fs.ReadDir(skills.FS, ".")
	for _, e := range names {
		data, err := fs.ReadFile(skills.FS, e.Name()+"/SKILL.md")
		if err != nil {
			t.Errorf("%s: %v", e.Name(), err)
			continue
		}
		doc := string(data)
		if !strings.HasPrefix(doc, "---\nname: "+e.Name()+"\ndescription: ") {
			t.Errorf("%s: front matter must start with name and description", e.Name())
		}
		desc := regexp.MustCompile(`(?m)^description: (.*)$`).FindStringSubmatch(doc)
		if len(desc) < 2 || len(desc[1]) > 1024 {
			t.Errorf("%s: description missing or longer than 1024 characters", e.Name())
		}
	}
}
