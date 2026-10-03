package flow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"flowtree/internal/config"
	"flowtree/internal/gitx"
)

// Move is one worktree to move to its place under the base directory.
type Move struct{ Branch, From, To string }

// Skip is a worktree relocate leaves alone, and why.
type Skip struct{ Branch, Reason string }

// RelocatePlan is the outcome of planning: what moves and what is left.
type RelocatePlan struct {
	Moves   []Move
	Skipped []Skip
}

// Probe answers the questions planning asks of the file system and of git.
type Probe struct {
	Locked func(path string) bool
	Exists func(path string) bool
}

// PlanRelocate decides which topic worktrees move to <base dir>/<branch>. It is pure: the probe is
// the only way it looks at the world. It fails when cwd is inside a worktree that would move.
func PlanRelocate(wts []gitx.Worktree, s Settings, cfg config.Config, main, cwd string, p Probe) (RelocatePlan, error) {
	var plan RelocatePlan
	for _, w := range wts {
		if !relocatable(s, w) {
			continue
		}
		to := cfg.WorktreePath(main, w.Branch)
		if filepath.Clean(w.Path) == to {
			continue
		}
		if reason := whySkip(w, to, p); reason != "" {
			plan.Skipped = append(plan.Skipped, Skip{w.Branch, reason})
		} else if inside(cwd, w.Path) {
			return RelocatePlan{}, fmt.Errorf("you are inside the worktree of '%s'; cd out of it (e.g. to %s) and re-run", w.Branch, main)
		} else {
			plan.Moves = append(plan.Moves, Move{w.Branch, w.Path, to})
		}
	}
	return plan, nil
}

// relocatable: the main worktree never moves, and only topic branches live under the base directory.
func relocatable(s Settings, w gitx.Worktree) bool {
	if w.Main || w.Branch == "" {
		return false
	}
	_, topic := s.KindOf(w.Branch)
	return topic
}

func whySkip(w gitx.Worktree, to string, p Probe) string {
	switch {
	case !p.Exists(w.Path):
		return fmt.Sprintf("%s is missing (try 'git worktree prune')", w.Path)
	case p.Locked(w.Path):
		return "worktree is locked"
	case p.Exists(to):
		return to + " already exists"
	}
	return ""
}

// lockedPaths lists the worktrees git reports as locked.
func lockedPaths(g gitx.Git) (map[string]bool, error) {
	out, err := g("worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	locked := map[string]bool{}
	for _, block := range strings.Split(out, "\n\n") {
		path, isLocked := "", false
		for _, line := range strings.Split(block, "\n") {
			if p, ok := strings.CutPrefix(line, "worktree "); ok {
				path = p
			}
			isLocked = isLocked || line == "locked" || strings.HasPrefix(line, "locked ")
		}
		locked[path] = locked[path] || isLocked
	}
	return locked, nil
}

func fileExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// PlanRelocateRepo plans for the current repository, warning about every skipped worktree.
func PlanRelocateRepo(e *Env) (RelocatePlan, error) {
	wts, err := e.Worktrees()
	if err != nil {
		return RelocatePlan{}, err
	}
	locked, err := lockedPaths(e.MainGit())
	if err != nil {
		return RelocatePlan{}, err
	}
	plan, err := PlanRelocate(wts, e.S, e.Config, e.Main, e.Cwd, Probe{
		Locked: func(p string) bool { return locked[p] }, Exists: fileExists,
	})
	for _, s := range plan.Skipped {
		e.Warn("skipping %s: %s", s.Branch, s.Reason)
	}
	return plan, err
}

// RelocateSteps turns a plan into one step per move.
func RelocateSteps(plan RelocatePlan) []Step {
	steps := make([]Step, len(plan.Moves))
	for i, m := range plan.Moves {
		steps[i] = Step{
			Desc: fmt.Sprintf("move %s: %s -> %s", m.Branch, m.From, m.To),
			Do:   func(e *Env) error { return moveWorktree(e, m) },
		}
	}
	return steps
}

func moveWorktree(e *Env, m Move) error {
	if err := os.MkdirAll(filepath.Dir(m.To), 0o755); err != nil {
		return err
	}
	if _, err := e.MainGit()("worktree", "move", m.From, m.To); err != nil {
		return err
	}
	e.Say("moved %s: %s -> %s", m.Branch, m.From, m.To)
	pruneEmpty(filepath.Dir(m.From))
	return nil
}

// pruneEmpty removes dir and then its parents for as long as they are empty.
func pruneEmpty(dir string) {
	if os.Remove(dir) == nil {
		pruneEmpty(filepath.Dir(dir))
	}
}
