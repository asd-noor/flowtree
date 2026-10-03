package flow

import "strings"

// Kind is a type of topic branch.
type Kind int

// The topic branch kinds.
const (
	Work Kind = iota
	Release
	Hotfix
)

// Kinds lists every kind.
var Kinds = []Kind{Work, Release, Hotfix}

func (k Kind) String() string { return [...]string{"work", "release", "hotfix"}[k] }

// Noun is how messages refer to a kind ("The release was tagged").
func (k Kind) Noun() string { return k.String() }

// Prefix is the branch prefix of a kind.
func (s Settings) Prefix(k Kind) string {
	return [...]string{s.WorkPrefix, s.ReleasePrefix, s.HotfixPrefix}[k]
}

// Base is the branch a kind starts from (and, for work, merges into).
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

// LivesInMain reports whether branches of this kind are checked out in the main worktree.
// Releases only touch a changelog and a version, so they do not get a worktree of their own.
func (k Kind) LivesInMain() bool { return k == Release }

// EvacuateTarget is where the main worktree goes back to when a branch of kind k was
// checked out there by hand: hotfixes branch from master, everything else from develop.
func (s Settings) EvacuateTarget(k Kind) string { return s.Base(k) }
