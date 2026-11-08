package flow

import (
	"fmt"
	"strings"

	"flowtree/internal/gitx"
)

// FinishOpts describe `<kind> finish`. Fields for the other kind are ignored.
type FinishOpts struct {
	Kind    Kind
	Name    string
	Fetch   bool
	Cleanup CleanupOpts
	Push    bool

	// release and hotfix
	Squash      bool
	Sign        bool
	SignKey     string
	Message     string
	MessageFile string
	NoTag       bool
	NoBackmerge bool
	FFMaster    bool // release only
	TagName     string
}

// PlanFinish checks the branch and returns the steps that merge it and clean up.
func PlanFinish(e *Env, o FinishOpts) ([]Step, error) {
	branch, err := ResolveTopic(e, o.Kind, o.Name)
	if err != nil {
		return nil, err
	}
	if w, ok, err := e.WorktreeOf(branch); err != nil {
		return nil, err
	} else if ok {
		if err := RequireClean(e, w.Path); err != nil {
			return nil, err
		}
	}
	if err := checkMainCanReturn(e, branch); err != nil {
		return nil, err
	}
	if err := checkNoChildren(e, o.Kind, branch); err != nil {
		return nil, err
	}
	switch o.Kind {
	case Feature:
		return planFinishFeature(e, o, branch)
	case Work:
		return planFinishWork(e, o, branch)
	}
	return planFinishRelease(e, o, branch)
}

func planFinishFeature(e *Env, o FinishOpts, branch string) ([]Step, error) {
	var merged bool
	return Plan(
		If(e.HasRemote(), One("fetch "+e.S.Remote, Fetch)),
		One("check whether "+branch+" is already merged", func(e *Env) error {
			merged = mergedUpstream(e, branch)
			if merged {
				e.Say("'%s' is already merged into '%s'; cleaning up", branch, e.S.Develop)
				return nil
			}
			return CheckSync(e, e.S.Develop)
		}),
		One(fmt.Sprintf("merge %s into %s (--no-ff) unless already merged", branch, e.S.Develop), func(e *Env) error {
			if merged {
				return nil
			}
			if err := MergeInto(e, e.S.Develop, branch, NoFF, ""); err != nil {
				return err
			}
			e.Say("\nSummary of actions:\n- The feature branch '%s' was merged into '%s'", branch, e.S.Develop)
			return nil
		}),
		trackStep(branch),
		One("remove worktree and branch "+branch, func(e *Env) error { return Cleanup(e, branch, o.Cleanup) }),
		If(o.Push, pushStep(e.S.Develop)),
	), nil
}

func planFinishWork(e *Env, o FinishOpts, branch string) ([]Step, error) {
	parent := e.S.ParentOf(branch)
	if !gitx.BranchExists(e.MainGit(), parent) {
		return nil, fmt.Errorf("the parent '%s' of '%s' does not exist", parent, branch)
	}
	if w, ok, err := e.WorktreeOf(parent); err != nil {
		return nil, err
	} else if ok {
		if err := RequireClean(e, w.Path); err != nil {
			return nil, err
		}
	}
	return Plan(
		One("rebase "+branch+" onto "+parent, func(e *Env) error { return rebaseOnto(e, branch, parent) }),
		One("fast-forward "+parent+" to "+branch, func(e *Env) error {
			if err := MergeInto(e, parent, branch, FastForward, ""); err != nil {
				return err
			}
			e.Say("\nSummary of actions:\n- The work branch '%s' was rebased onto '%s' and merged into it", branch, parent)
			return nil
		}),
		trackStep(parent),
		trackStep(branch),
		One("remove worktree and branch "+branch, func(e *Env) error { return Cleanup(e, branch, o.Cleanup) }),
	), nil
}

// rebaseOnto rebases branch onto onto inside its worktree, creating the worktree if needed. A conflict
// aborts the rebase, so nothing changes.
func rebaseOnto(e *Env, branch, onto string) error {
	dir, err := EnsureWorktree(e, branch)
	if err != nil {
		return err
	}
	if err := RequireClean(e, dir); err != nil {
		return err
	}
	if _, err := e.Git(dir)("rebase", onto); err != nil {
		_ = e.Git(dir).Ok("rebase", "--abort")
		return fmt.Errorf("finish was aborted due to conflicts during rebase; rebase manually in '%s' "+
			"(git rebase %s), then re-run: flowtree work finish %s\n%w", dir, onto, e.S.Name(Work, branch), err)
	}
	return nil
}

// mergedUpstream reports whether develop, or its remote copy (a pull request merged on the forge),
// already contains branch. A branch with no commits of its own does not count: it was never merged.
func mergedUpstream(e *Env, branch string) bool {
	g := e.MainGit()
	tip, _ := Rev(g, branch)
	if dv, _ := Rev(g, e.S.Develop); tip == dv {
		return false
	}
	targets := []string{e.S.Develop}
	if e.HasRemote() && gitx.RemoteBranchExists(g, e.S.Remote, e.S.Develop) {
		targets = append(targets, e.S.Remote+"/"+e.S.Develop)
	}
	for _, t := range targets {
		if g.Ok("merge-base", "--is-ancestor", branch, t) {
			return true
		}
	}
	return false
}

func modeName(m MergeMode) string {
	return [...]string{"fast-forward if possible", "--no-ff", "squash"}[m]
}

func pushStep(branches ...string) []Step {
	return One("push "+strings.Join(branches, ", "), func(e *Env) error {
		if !e.HasRemote() {
			return fmt.Errorf("no remote '%s' to push to", e.S.Remote)
		}
		refs := make([]string, len(branches))
		for i, b := range branches {
			refs[i] = b
			if !gitx.TagExists(e.MainGit(), b) {
				refs[i] = b + ":" + b
			}
		}
		if _, err := e.MainGit()(append([]string{"push", e.S.Remote}, refs...)...); err != nil {
			return err
		}
		e.Say("- %s pushed to %s", strings.Join(branches, ", "), e.S.Remote)
		return nil
	})
}

func label(k Kind) string {
	if k == Release {
		return "Release"
	}
	return "Hotfix"
}

func planFinishRelease(e *Env, o FinishOpts, branch string) ([]Step, error) {
	ver := e.S.Name(o.Kind, branch)
	tag := firstNonEmpty(o.TagName, ver)
	mode := NoFF
	switch {
	case o.Squash:
		mode = Squash
	case o.Kind == Release && o.FFMaster:
		mode = FastForward
	}
	back := tag
	if o.NoTag {
		back = e.S.Master
	}
	resumed := alreadyMerged(e, o, branch, tag)
	untagged := mergedButUntagged(e, o, branch, tag)
	pushed := []string{e.S.Develop, e.S.Master}
	if !o.NoTag {
		pushed = append(pushed, tag)
	}
	return Plan(
		If(o.Fetch, One("fetch "+e.S.Remote, Fetch)),
		One("check "+e.S.Master+" and "+e.S.Develop+" against "+e.S.Remote, func(e *Env) error { return checkReleaseState(e, o.NoTag || resumed, tag) }),
		If(resumed || untagged, One("resume: "+branch+" is already merged into "+e.S.Master, func(e *Env) error {
			e.Say("%s", mergeSkipNote(e, o, branch, tag, untagged))
			return nil
		})),
		If(!resumed && !untagged, One(fmt.Sprintf("merge %s into %s (%s)", branch, e.S.Master, modeName(mode)), func(e *Env) error {
			if err := MergeInto(e, e.S.Master, branch, mode, ""); err != nil {
				return err
			}
			e.Say("\nSummary of actions:\n- %s branch '%s' has been merged into '%s'", label(o.Kind), branch, e.S.Master)
			return nil
		})),
		If(!o.NoTag && !resumed, One("tag "+e.S.Master+" as "+tag, func(e *Env) error {
			if err := makeTag(e, o, tag, ver); err != nil {
				return err
			}
			e.Say("- The %s was tagged '%s'", o.Kind, tag)
			return nil
		})),
		If(!o.NoBackmerge, One("merge "+back+" into "+e.S.Develop, func(e *Env) error {
			return backMerge(e, o, branch, back, tag)
		})),
		trackStep(branch),
		One("remove worktree and branch "+branch, func(e *Env) error { return Cleanup(e, branch, o.Cleanup) }),
		If(o.Push, pushStep(pushed...)),
	), nil
}

// checkMainCanReturn: finishing a topic branch that was checked out in the main worktree by hand
// sends the main worktree back to develop, so develop must not be checked out elsewhere. Checked at
// plan time, before anything is merged.
func checkMainCanReturn(e *Env, branch string) error {
	w, ok, err := e.WorktreeOf(branch)
	if err != nil || !ok || !w.Main {
		return err
	}
	dv, ok, err := e.WorktreeOf(e.S.Develop)
	if err != nil || !ok || dv.Main {
		return err
	}
	return fmt.Errorf("'%s' is checked out in '%s'; the main worktree cannot return to it after finishing '%s'. "+
		"Switch that worktree to another branch first (nothing was changed)", e.S.Develop, dv.Path, branch)
}

func checkReleaseState(e *Env, noTag bool, tag string) error {
	for _, b := range []string{e.S.Master, e.S.Develop} {
		if err := CheckSync(e, b); err != nil {
			return err
		}
	}
	if !noTag && gitx.TagExists(e.MainGit(), tag) {
		return fmt.Errorf("tag '%s' already exists; pick another name", tag)
	}
	return nil
}

// tagArgs builds `git tag` arguments: annotated by default, signed with -s/-u. Without a message
// and without a terminal (git could not open an editor) a default message is used.
func tagArgs(e *Env, o FinishOpts, tag, ver string) []string {
	args := []string{"tag", "-a"}
	if o.Sign {
		args = []string{"tag", "-s"}
	}
	if o.SignKey != "" {
		args = []string{"tag", "-u", o.SignKey}
	}
	switch {
	case o.MessageFile != "":
		args = append(args, "-F", o.MessageFile)
	case o.Message != "":
		args = append(args, "-m", o.Message)
	case !e.editorAvailable():
		args = append(args, "-m", label(o.Kind)+" "+ver)
	}
	return append(args, tag, e.S.Master)
}

// needsEditor reports whether git will open an editor for the tag message.
func needsEditor(e *Env, o FinishOpts) bool {
	return o.MessageFile == "" && o.Message == "" && e.editorAvailable()
}

// makeTag creates the tag. When git will open an editor it runs with the terminal attached; otherwise
// through the normal runner. A failure is reported as what it is: master is already merged, and a
// re-run of finish picks up from here.
func makeTag(e *Env, o FinishOpts, tag, ver string) error {
	args := tagArgs(e, o, tag, ver)
	var err error
	if needsEditor(e, o) {
		err = e.Attach(e.Main, args...)
	} else {
		_, err = e.MainGit()(args...)
	}
	if err != nil {
		return fmt.Errorf("the %s '%s' is already merged into '%s' but tagging '%s' failed: %w\n"+
			"Fix the cause (for example the tag message or the signing key), then re-run 'flowtree %s finish %s': it resumes from here",
			o.Kind, ver, e.S.Master, tag, err, o.Kind, ver)
	}
	return nil
}

func backMerge(e *Env, o FinishOpts, branch, src, tag string) error {
	if e.MainGit().Ok("merge-base", "--is-ancestor", src, e.S.Develop) {
		e.Say("- '%s' is already merged into '%s'", src, e.S.Develop)
		return nil
	}
	hint := fmt.Sprintf("'%s' already contains '%s'%s. Merge '%s' into '%s' by hand, then delete '%s'.",
		e.S.Master, branch, taggedAs(o, tag), src, e.S.Develop, branch)
	if err := MergeInto(e, e.S.Develop, src, NoFF, hint); err != nil {
		return err
	}
	what := fmt.Sprintf("tag '%s'", src)
	if o.NoTag {
		what = fmt.Sprintf("'%s'", src)
	}
	e.Say("- %s %s has been back-merged into '%s'", label(o.Kind), what, e.S.Develop)
	return nil
}

func taggedAs(o FinishOpts, tag string) string {
	if o.NoTag {
		return ""
	}
	return " and is tagged '" + tag + "'"
}

// alreadyMerged detects a finish that was interrupted after master was merged (and tagged), for
// example by a conflicting back-merge. Re-running then continues instead of failing on the tag.
// With a tag, master must still be exactly the tagged commit, so nothing unrelated is skipped.
func alreadyMerged(e *Env, o FinishOpts, branch, tag string) bool {
	g := e.MainGit()
	if !g.Ok("merge-base", "--is-ancestor", branch, e.S.Master) {
		return false
	}
	if o.NoTag {
		return true
	}
	tagged, ok := Rev(g, tag)
	master, _ := Rev(g, e.S.Master)
	return ok && tagged == master
}

// mergedButUntagged detects a finish that was interrupted between the merge into master and the tag
// (a failed signature or tag message): the branch is already in master and the tag does not exist yet.
func mergedButUntagged(e *Env, o FinishOpts, branch, tag string) bool {
	g := e.MainGit()
	return !o.NoTag && g.Ok("merge-base", "--is-ancestor", branch, e.S.Master) && !gitx.TagExists(g, tag)
}

// mergeSkipNote says why the merge into master is skipped. A branch whose tip is master's own tip, with no
// tag yet, never had commits of its own (or was fast-forwarded and not tagged): there is no interrupted
// finish to resume, and saying so would be wrong. Anything else is a real resume.
func mergeSkipNote(e *Env, o FinishOpts, branch, tag string, untagged bool) string {
	g := e.MainGit()
	tip, _ := Rev(g, branch)
	master, _ := Rev(g, e.S.Master)
	noTagYet := o.NoTag || !gitx.TagExists(g, tag)
	if tip == master && noTagYet {
		return fmt.Sprintf("'%s' has no commits of its own to merge into '%s'%s", branch, e.S.Master, tagNowSuffix(o, untagged))
	}
	if untagged {
		return fmt.Sprintf("Resuming: '%s' is already merged into '%s' (nothing left to merge); tagging it now", branch, e.S.Master)
	}
	return fmt.Sprintf("Resuming: '%s' is already merged into '%s'%s", branch, e.S.Master, taggedAs(o, tag))
}

func tagNowSuffix(o FinishOpts, untagged bool) string {
	if untagged && !o.NoTag {
		return "; tagging it now"
	}
	return ""
}
