package flow

import "strings"

// Kind is a type of topic branch.
type Kind int

// The topic branch kinds.
const (
	Feature Kind = iota
	Release
	Hotfix
	Work // a local subtask of a feature or hotfix: work/<parent branch>/<name>
)

// Kinds lists every kind.
var Kinds = []Kind{Feature, Release, Hotfix, Work}

func (k Kind) String() string { return [...]string{"feature", "release", "hotfix", "work"}[k] }

// Noun is how messages refer to a kind ("The release was tagged").
func (k Kind) Noun() string { return k.String() }

// Prefix is the branch prefix of a kind.
func (s Settings) Prefix(k Kind) string {
	return [...]string{s.FeaturePrefix, s.ReleasePrefix, s.HotfixPrefix, s.WorkPrefix}[k]
}

// Base is the branch a kind starts from (and, for features, merges into). Work branches have none of
// their own: see BaseOf.
func (s Settings) Base(k Kind) string {
	if k == Hotfix {
		return s.Master
	}
	return s.Develop
}

// Branch is the full branch name for a topic name; a name that already has the prefix is kept.
func (s Settings) Branch(k Kind, name string) string {
	return s.Prefix(k) + strings.TrimPrefix(name, s.Prefix(k))
}

// Name strips the prefix from a branch of kind k.
func (s Settings) Name(k Kind, branch string) string { return strings.TrimPrefix(branch, s.Prefix(k)) }

// KindOf reports which kind a branch belongs to.
func (s Settings) KindOf(branch string) (Kind, bool) {
	for _, k := range Kinds {
		if strings.HasPrefix(branch, s.Prefix(k)) {
			return k, true
		}
	}
	return 0, false
}

// EvacuateTarget is where the main worktree goes back to when a branch of kind k was
// checked out there by hand: hotfixes branch from master, everything else from develop.
func (s Settings) EvacuateTarget(k Kind) string { return s.Base(k) }

// BaseOf is the branch that branch of kind k starts from and merges into: a work branch's parent,
// else the kind's base.
func (s Settings) BaseOf(k Kind, branch string) string {
	if k == Work {
		return s.ParentOf(branch)
	}
	return s.Base(k)
}

// WorkBranch is the work branch name for a subtask of parent: work/<parent>/<name>.
func (s Settings) WorkBranch(parent, name string) string { return s.WorkPrefix + parent + "/" + name }

// ParentOf is the parent of a work branch: work/feature/login/api -> feature/login.
func (s Settings) ParentOf(branch string) string {
	rest := strings.TrimPrefix(branch, s.WorkPrefix)
	if i := strings.LastIndex(rest, "/"); i >= 0 {
		return rest[:i]
	}
	return ""
}

// IsParent reports whether a branch can have work branches: a feature or a hotfix.
func (s Settings) IsParent(branch string) bool {
	k, ok := s.KindOf(branch)
	return ok && (k == Feature || k == Hotfix)
}
