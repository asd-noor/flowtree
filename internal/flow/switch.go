package flow

import (
	"fmt"
	"strings"

	"flowtree/internal/gitx"
)

// SwitchAction is what selecting a SwitchTarget does.
type SwitchAction int

// The switch actions.
const (
	GoDir    SwitchAction = iota // the directory exists: just go there
	MoveMain                     // check the branch out in the main worktree
	Ensure                       // give a work or hotfix branch its own worktree (create or repair)
)

// MainKey is the key of the main worktree row.
const MainKey = "@"

// SwitchTarget is one place `switch` can go.
type SwitchTarget struct {
	Key    string // what the user types and what is returned
	Label  string // what a list shows
	Alt    string // an extra string a query may match
	Dir    string // set for GoDir
	Branch string // the branch the target is about; empty for a detached main worktree
	Action SwitchAction
}

// SwitchTargets lists where switch can go, from the current state.
func (e *Env) SwitchTargets() ([]SwitchTarget, error) {
	wts, err := e.Worktrees()
	if err != nil {
		return nil, err
	}
	branches, err := gitx.LocalBranches(e.MainGit(), "")
	if err != nil {
		return nil, err
	}
	return BuildSwitchTargets(e.S, e.Main, wts, branches), nil
}

// BuildSwitchTargets is the pure part of SwitchTargets. Rows, in order: the main worktree, a topic
// branch checked out there by hand (selecting it repairs the layout), the other worktrees, then
// branches without a worktree that switch can reach.
func BuildSwitchTargets(s Settings, main string, wts []gitx.Worktree, branches []string) []SwitchTarget {
	mainBranch := ""
	if len(wts) > 0 {
		mainBranch = wts[0].Branch
	}
	rows := joinTargets(
		[]SwitchTarget{mainRow(s, main, mainBranch)},
		strayRow(s, mainBranch),
		worktreeRows(s, wts),
	)
	seen := keySet(rows, mainBranch)
	return append(rows, branchRows(s, branches, seen)...)
}

// joinTargets concatenates target groups.
func joinTargets(groups ...[]SwitchTarget) []SwitchTarget {
	var out []SwitchTarget
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

func mainRow(s Settings, main, branch string) SwitchTarget {
	shown := branch
	if shown == "" {
		shown = "detached"
	}
	t := SwitchTarget{Key: MainKey, Label: fmt.Sprintf("@  [%s]", shown), Dir: main, Branch: branch, Action: GoDir}
	if s.longLived(branch) {
		t.Alt = branch // so `switch develop` finds the main worktree while it is on develop
	}
	return t
}

func strayRow(s Settings, mainBranch string) []SwitchTarget {
	if _, ok := s.topicKind(mainBranch); !ok {
		return nil
	}
	return []SwitchTarget{{
		Key: mainBranch, Label: mainBranch + "  (checked out in the main worktree; will be moved out)",
		Branch: mainBranch, Alt: s.shortName(mainBranch), Action: Ensure,
	}}
}

func worktreeRows(s Settings, wts []gitx.Worktree) []SwitchTarget {
	var out []SwitchTarget
	for _, w := range wts {
		if !w.Main && w.Branch != "" {
			out = append(out, SwitchTarget{Key: w.Branch, Label: w.Branch, Dir: w.Path, Branch: w.Branch, Alt: s.shortName(w.Branch), Action: GoDir})
		}
	}
	return out
}

func keySet(rows []SwitchTarget, mainBranch string) map[string]bool {
	seen := map[string]bool{mainBranch: true}
	for _, r := range rows {
		seen[r.Key] = true
	}
	return seen
}

func branchRows(s Settings, branches []string, seen map[string]bool) []SwitchTarget {
	var out []SwitchTarget
	for _, b := range branches {
		if seen[b] {
			continue
		}
		switch {
		case s.longLived(b):
			out = append(out, SwitchTarget{Key: b, Label: b + "  (moves the main worktree here)", Branch: b, Alt: s.shortName(b), Action: MoveMain})
		case s.isTopic(b):
			out = append(out, SwitchTarget{Key: b, Label: b + "  (no worktree yet; one is created)", Branch: b, Alt: s.shortName(b), Action: Ensure})
		}
	}
	return out
}

// longLived: the branches the main worktree may rest on.
func (s Settings) longLived(b string) bool {
	return b != "" && (b == s.Master || b == s.Develop || strings.HasPrefix(b, s.ReleasePrefix))
}

// topicKind reports the kind of a branch that gets its own worktree (work and hotfix).
func (s Settings) topicKind(b string) (Kind, bool) {
	for _, k := range []Kind{Work, Hotfix} {
		if b != "" && strings.HasPrefix(b, s.Prefix(k)) {
			return k, true
		}
	}
	return 0, false
}

func (s Settings) isTopic(b string) bool { _, ok := s.topicKind(b); return ok }

// Switch performs the action of t, asks the shell to cd, and returns the directory.
func Switch(e *Env, t SwitchTarget) (string, error) {
	dir, err := switchDir(e, t)
	if err != nil {
		return "", err
	}
	e.cd(dir)
	return dir, nil
}

func switchDir(e *Env, t SwitchTarget) (string, error) {
	switch t.Action {
	case MoveMain:
		if err := SwitchMain(e, t.Branch); err != nil {
			return "", err
		}
		e.Note("Note: the main worktree is now on '%s'.", t.Branch)
		return e.Main, nil
	case Ensure:
		return EnsureWorktree(e, t.Branch)
	}
	return t.Dir, nil
}

// shortName is a topic branch without its prefix ("work/login" -> "login"), so `switch login` is an
// exact match. Other branches have none.
func (s Settings) shortName(b string) string {
	for _, k := range Kinds {
		if strings.HasPrefix(b, s.Prefix(k)) {
			return s.Name(k, b)
		}
	}
	return ""
}
