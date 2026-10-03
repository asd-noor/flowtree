package commands

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"flowtree/internal/gitx"
	"flowtree/internal/worktree"
)

const headLen = 8

func newListCmd(d Deps) *cobra.Command {
	var quiet bool
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all worktrees",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			repo, err := d.OpenRepo(cmd.Context())
			if err != nil {
				return err
			}
			if quiet {
				d.Printf("%s", renderNames(repo.Worktrees))
				return nil
			}
			d.Printf("%s", renderTable(repo.Worktrees, repo.Cwd, repo.Config.ResolveBaseDir(repo.Main)))
			return nil
		},
	}
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Only print worktree names, one per line")
	return cmd
}

// renderNames lists the display name of every worktree, one per line.
func renderNames(wts []gitx.Worktree) string {
	return joinLines(mapSlice(wts, worktree.Display))
}

// renderTable draws the PATH / BRANCH / HEAD table. The worktree containing cwd gets a `*`.
func renderTable(wts []gitx.Worktree, cwd, baseDir string) string {
	current := currentIndex(wts, cwd)
	rows := make([]string, 0, len(wts)+2)
	rows = append(rows, "PATH\tBRANCH\tHEAD", "----\t------\t----")
	for i, w := range wts {
		rows = append(rows, fmt.Sprintf("%s\t%s\t%s", pathLabel(w, baseDir, i == current), branchLabel(w), shortHead(w)))
	}
	return tabulate(rows)
}

func tabulate(rows []string) string {
	var sb strings.Builder
	tw := tabwriter.NewWriter(&sb, 0, 0, 3, ' ', 0)
	for _, r := range rows {
		_, _ = fmt.Fprintln(tw, r)
	}
	_ = tw.Flush()
	return sb.String()
}

// pathLabel: "@ (main worktree)", else the path relative to baseDir when inside it, else absolute.
func pathLabel(w gitx.Worktree, baseDir string, current bool) string {
	label := w.Path
	if w.Main {
		label = "@ (main worktree)"
	} else if rel, ok := within(baseDir, w.Path); ok {
		label = rel
	}
	if current {
		label += "*"
	}
	return label
}

func branchLabel(w gitx.Worktree) string {
	switch {
	case w.Bare:
		return "(bare)"
	case w.Branch == "":
		return "detached"
	}
	return w.Branch
}

func shortHead(w gitx.Worktree) string {
	if len(w.Head) > headLen {
		return w.Head[:headLen]
	}
	return w.Head
}

// currentIndex is the index of the deepest worktree containing cwd, or -1.
func currentIndex(wts []gitx.Worktree, cwd string) int {
	best := -1
	for i, w := range wts {
		if !isWithin(w.Path, cwd) {
			continue
		}
		if best < 0 || len(w.Path) > len(wts[best].Path) {
			best = i
		}
	}
	return best
}

// within returns target relative to base when target is strictly inside base.
func within(base, target string) (string, bool) {
	rel, err := filepath.Rel(base, target)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

// isWithin reports whether target is base or inside it, resolving symlinks where possible.
func isWithin(base, target string) bool {
	base, target = realPath(base), realPath(target)
	if base == target {
		return true
	}
	_, ok := within(base, target)
	return ok
}

func realPath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}

func mapSlice[T, U any](xs []T, f func(T) U) []U {
	out := make([]U, len(xs))
	for i, x := range xs {
		out[i] = f(x)
	}
	return out
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(slices.Clone(lines), "\n") + "\n"
}
