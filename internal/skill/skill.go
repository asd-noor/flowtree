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

// Install writes every skill of fsys to dest/<name>, all or nothing. Each skill is first written to a
// temporary directory next to its destination; only when every skill is staged are they swapped in,
// and a failed swap is rolled back, so an error never leaves a half-written or missing skill behind.
// An existing directory is replaced when it holds a flowtree skill (its SKILL.md names the skill), and
// left alone with an error otherwise unless Force is set.
func Install(fsys fs.FS, dest string, o Options) ([]Installed, error) {
	names, err := Names(fsys)
	if err != nil {
		return nil, err
	}
	plans, err := planAll(fsys, dest, names, o)
	if err != nil || o.DryRun {
		return installedOf(plans), err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return nil, err
	}
	staged, err := stageAll(fsys, dest, plans)
	if err != nil {
		return nil, err
	}
	if err := swapAll(plans, staged); err != nil {
		return nil, err
	}
	return installedOf(plans), nil
}

// plan is one skill to install: what it is, and whether it replaces something.
type plan struct {
	in    Installed
	files []string // paths in fsys
}

func planAll(fsys fs.FS, dest string, names []string, o Options) ([]plan, error) {
	plans := make([]plan, 0, len(names))
	for _, n := range names {
		p, err := planOne(fsys, dest, n, o)
		if err != nil {
			return nil, err
		}
		plans = append(plans, p)
	}
	return plans, nil
}

func planOne(fsys fs.FS, dest, name string, o Options) (plan, error) {
	dir := filepath.Join(dest, name)
	files, err := Files(fsys, name)
	if err != nil {
		return plan{}, err
	}
	exists, err := pathExists(dir)
	if err != nil {
		return plan{}, err
	}
	if exists && !o.Force && !isInstalled(dir, name) {
		return plan{}, fmt.Errorf("%s exists and does not look like the %s skill; use --force to replace it", dir, name)
	}
	return plan{in: Installed{Name: name, Dir: dir, Files: relTo(files, name), Replaced: exists}, files: files}, nil
}

func installedOf(plans []plan) []Installed {
	out := make([]Installed, len(plans))
	for i, p := range plans {
		out[i] = p.in
	}
	return out
}

// stageAll writes every skill into a fresh temporary directory beside its destination and returns those
// directories, in plan order. On failure everything staged so far is removed.
func stageAll(fsys fs.FS, dest string, plans []plan) ([]string, error) {
	staged := make([]string, 0, len(plans))
	for _, p := range plans {
		tmp, err := os.MkdirTemp(dest, "."+p.in.Name+".tmp-")
		if err == nil {
			staged = append(staged, tmp)
			err = os.Chmod(tmp, 0o755) // MkdirTemp creates 0700
		}
		if err == nil {
			err = writeFiles(fsys, tmp, p.in.Name, p.files)
		}
		if err != nil {
			removeAll(staged)
			return nil, err
		}
	}
	return staged, nil
}

// swapAll moves each staged directory into place, keeping the replaced one aside until every swap has
// worked. Any failure undoes the swaps already made.
func swapAll(plans []plan, staged []string) error {
	var done []swap
	for i, p := range plans {
		s, err := swapIn(p.in.Dir, staged[i])
		if err != nil {
			rollback(done)
			removeAll(staged)
			return err
		}
		done = append(done, s)
	}
	for _, s := range done {
		if s.aside != "" {
			_ = os.RemoveAll(s.aside)
		}
	}
	return nil
}

// swap records one completed replacement so it can be undone.
type swap struct {
	dir   string
	aside string // where the previous content was moved; empty when there was none
}

func swapIn(dir, staged string) (swap, error) {
	s := swap{dir: dir}
	exists, err := pathExists(dir)
	if err != nil {
		return s, err
	}
	if exists {
		s.aside = filepath.Join(filepath.Dir(dir), fmt.Sprintf(".%s.old-%d", filepath.Base(dir), os.Getpid()))
		_ = os.RemoveAll(s.aside)
		if err := os.Rename(dir, s.aside); err != nil {
			return swap{}, err
		}
	}
	if err := os.Rename(staged, dir); err != nil {
		if s.aside != "" {
			_ = os.Rename(s.aside, dir)
		}
		return swap{}, err
	}
	return s, nil
}

// rollback restores the previous content of every swap, newest first.
func rollback(done []swap) {
	for i := len(done) - 1; i >= 0; i-- {
		s := done[i]
		_ = os.RemoveAll(s.dir)
		if s.aside != "" {
			_ = os.Rename(s.aside, s.dir)
		}
	}
}

func removeAll(dirs []string) {
	for _, d := range dirs {
		_ = os.RemoveAll(d)
	}
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
