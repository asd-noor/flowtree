package gitx

import (
	"fmt"
	"slices"
	"strings"
)

// BranchError is a branch lookup failure: not found, or ambiguous across remotes.
type BranchError struct {
	Branch  string
	Remotes []string // set when several remotes carry the branch
}

func (e *BranchError) Error() string {
	if len(e.Remotes) > 1 {
		return fmt.Sprintf("branch '%s' exists in multiple remotes: %s; create a local branch for the one you want first",
			e.Branch, strings.Join(e.Remotes, ", "))
	}
	return fmt.Sprintf("branch '%s' not found locally or on any remote", e.Branch)
}

// Ref is a resolved branch: a local branch name, or a remote-tracking ref such as origin/x.
type Ref struct {
	Name   string
	Remote bool
}

// BranchExists reports whether refs/heads/<name> exists.
func BranchExists(g Git, name string) bool {
	return g.Ok("show-ref", "--verify", "--quiet", "refs/heads/"+name)
}

// TagExists reports whether refs/tags/<name> exists.
func TagExists(g Git, name string) bool {
	return g.Ok("show-ref", "--verify", "--quiet", "refs/tags/"+name)
}

// RemoteBranchExists reports whether refs/remotes/<remote>/<name> exists.
func RemoteBranchExists(g Git, remote, name string) bool {
	return g.Ok("show-ref", "--verify", "--quiet", "refs/remotes/"+remote+"/"+name)
}

// LocalBranches lists local branch names, optionally under a prefix.
func LocalBranches(g Git, prefix string) ([]string, error) {
	return g.Lines("for-each-ref", "--format=%(refname:short)", "refs/heads/"+prefix+"*")
}

// RemoteRefs lists remote-tracking refs (origin/x) whose branch part is name.
func RemoteRefs(g Git, name string) ([]string, error) {
	all, err := g.Lines("for-each-ref", "--format=%(refname:short)", "refs/remotes")
	if err != nil {
		return nil, err
	}
	return filter(all, func(ref string) bool {
		_, branch, ok := strings.Cut(ref, "/")
		return ok && branch == name
	}), nil
}

// ResolveBranch finds name locally, else on exactly one remote.
func ResolveBranch(g Git, name string) (Ref, error) {
	if BranchExists(g, name) {
		return Ref{Name: name}, nil
	}
	refs, err := RemoteRefs(g, name)
	if err != nil {
		return Ref{}, err
	}
	switch len(refs) {
	case 0:
		return Ref{}, &BranchError{Branch: name}
	case 1:
		return Ref{Name: refs[0], Remote: true}, nil
	default:
		remotes := make([]string, len(refs))
		for i, r := range refs {
			remotes[i], _, _ = strings.Cut(r, "/")
		}
		slices.Sort(remotes)
		return Ref{}, &BranchError{Branch: name, Remotes: remotes}
	}
}

// CurrentBranch returns the checked-out branch of g's directory ("" when detached).
func CurrentBranch(g Git) string {
	out, err := g("symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return ""
	}
	return out
}

// DeleteBranch deletes a local branch (-D when force, else -d).
func DeleteBranch(g Git, name string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := g("branch", flag, name)
	return err
}
