// Package skill installs the embedded agent skills (directories with a SKILL.md) into a skills folder.
package skill

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

const manifest = "SKILL.md"

// Options control an install.
type Options struct {
	Force  bool // replace a directory that is not a flowtree-installed skill
	DryRun bool // report what would be written, write nothing
}

// Names lists the skills in fsys: its top-level directories that contain a SKILL.md.
func Names(fsys fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if _, err := fs.Stat(fsys, path.Join(e.Name(), manifest)); e.IsDir() && err == nil {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// Files lists the files of one skill, as paths relative to fsys, sorted.
func Files(fsys fs.FS, name string) ([]string, error) {
	var files []string
	err := fs.WalkDir(fsys, name, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			files = append(files, p)
		}
		return err
	})
	slices.Sort(files)
	return files, err
}

// Installed describes what Install did (or would do) for one skill.
type Installed struct {
	Name     string
	Dir      string   // <dest>/<name>
	Files    []string // paths inside Dir, slash separated
	Replaced bool     // an earlier install was replaced
}

// Install writes every skill of fsys to dest/<name>. An existing directory is replaced when it holds a
// flowtree skill (its SKILL.md names the skill), and left alone with an error otherwise unless Force is set.
func Install(fsys fs.FS, dest string, o Options) ([]Installed, error) {
	names, err := Names(fsys)
	if err != nil {
		return nil, err
	}
	out := make([]Installed, 0, len(names))
	for _, n := range names {
		in, err := installOne(fsys, dest, n, o)
		if err != nil {
			return out, err
		}
		out = append(out, in)
	}
	return out, nil
}

func installOne(fsys fs.FS, dest, name string, o Options) (Installed, error) {
	dir := filepath.Join(dest, name)
	files, err := Files(fsys, name)
	if err != nil {
		return Installed{}, err
	}
	in := Installed{Name: name, Dir: dir, Files: relTo(files, name)}
	exists, err := pathExists(dir)
	if err != nil {
		return in, err
	}
	if exists && !o.Force && !isInstalled(dir, name) {
		return in, fmt.Errorf("%s exists and does not look like the %s skill; use --force to replace it", dir, name)
	}
	in.Replaced = exists
	if o.DryRun {
		return in, nil
	}
	if exists {
		if err := os.RemoveAll(dir); err != nil {
			return in, err
		}
	}
	return in, writeFiles(fsys, dir, name, files)
}

// Uninstall removes dest/<name> for every skill of fsys that is installed there, and reports which.
// A directory that does not look like the skill is an error unless Force is set.
func Uninstall(fsys fs.FS, dest string, o Options) ([]Installed, error) {
	names, err := Names(fsys)
	if err != nil {
		return nil, err
	}
	var removed []Installed
	for _, n := range names {
		dir := filepath.Join(dest, n)
		exists, err := pathExists(dir)
		if err != nil || !exists {
			if err != nil {
				return removed, err
			}
			continue
		}
		if !o.Force && !isInstalled(dir, n) {
			return removed, fmt.Errorf("%s does not look like the %s skill; use --force to remove it", dir, n)
		}
		if !o.DryRun {
			if err := os.RemoveAll(dir); err != nil {
				return removed, err
			}
		}
		removed = append(removed, Installed{Name: n, Dir: dir, Replaced: true})
	}
	return removed, nil
}

func writeFiles(fsys fs.FS, dir, name string, files []string) error {
	for _, f := range files {
		data, err := fs.ReadFile(fsys, f)
		if err != nil {
			return err
		}
		target := filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(f, name+"/")))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func relTo(files []string, name string) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = strings.TrimPrefix(f, name+"/")
	}
	return out
}

func pathExists(p string) (bool, error) {
	_, err := os.Lstat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// isInstalled reports whether dir holds a skill called name: its SKILL.md has a "name: <name>" line in
// the front matter.
func isInstalled(dir, name string) bool {
	data, err := os.ReadFile(filepath.Join(dir, manifest))
	if err != nil {
		return false
	}
	return frontMatterName(string(data)) == name
}

func frontMatterName(doc string) string {
	lines := strings.Split(doc, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}
	for _, l := range lines[1:] {
		if strings.TrimSpace(l) == "---" {
			return ""
		}
		if v, ok := strings.CutPrefix(l, "name:"); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
