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
	Squash  bool

	// work
	Rebase   bool
	Preserve bool // keep merges while rebasing
	NoFF     bool

	// release and hotfix
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
	if o.Kind == Work {
		return planFinishWork(e, o, branch)
	}
	return planFinishRelease(e, o, branch)
}

func planFinishWork(e *Env, o FinishOpts, branch string) ([]Step, error) {
	mode := FastForward
	switch {
	case o.Squash:
		mode = Squash
	case o.NoFF:
		mode = NoFF
	}
	return Plan(
		If(o.Fetch, One("fetch "+e.S.Remote, Fetch)),
		One("check "+e.S.Develop+" against "+e.S.Remote, func(e *Env) error { return CheckSync(e, e.S.Develop) }),
		If(o.Rebase, One("rebase "+branch+" onto "+e.S.Develop, func(e *Env) error { return rebaseOnto(e, branch, o.Preserve) })),
		One(fmt.Sprintf("merge %s into %s (%s)", branch, e.S.Develop, modeName(mode)), func(e *Env) error {
			if err := MergeInto(e, e.S.Develop, branch, mode, ""); err != nil {
				return err
			}
			e.Say("\nSummary of actions:\n- The work branch '%s' was merged into '%s'", branch, e.S.Develop)
			if mode == Squash {
				e.Say("  (squashed)")
			}
			return nil
		}),
		One("remove worktree and branch "+branch, func(e *Env) error { return Cleanup(e, branch, o.Cleanup) }),
		If(o.Push, pushStep(e.S.Develop)),
	), nil
}

func modeName(m MergeMode) string {
	return [...]string{"fast-forward if possible", "--no-ff", "squash"}[m]
}

// rebaseOnto rebases branch onto develop inside its worktree, creating the worktree if needed.
func rebaseOnto(e *Env, branch string, preserve bool) error {
	dir, err := EnsureWorktree(e, branch)
	if err != nil {
		return err
	}
	if err := RequireClean(e, dir); err != nil {
		return err
	}
	args := []string{"rebase"}
	if preserve {
		args = append(args, "--rebase-merges")
	}
	if _, err := e.Git(dir)(append(args, e.S.Develop)...); err != nil {
		_ = e.Git(dir).Ok("rebase", "--abort")
		return fmt.Errorf("finish was aborted due to conflicts during rebase; rebase manually in '%s' "+
			"(git rebase %s), then re-run: flowtree work finish %s\n%w", dir, e.S.Develop, e.S.Name(Work, branch), err)
	}
	return nil
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
	tag := firstNonEmpty(o.TagName, e.S.VersionTag+ver)
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
			if untagged {
				e.Say("Resuming: '%s' is already merged into '%s' (nothing left to merge); tagging it now", branch, e.S.Master)
				return nil
			}
			e.Say("Resuming: '%s' is already merged into '%s'%s", branch, e.S.Master, taggedAs(o, tag))
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
		One("remove worktree and branch "+branch, func(e *Env) error { return Cleanup(e, branch, o.Cleanup) }),
		If(o.Push, pushStep(pushed...)),
	), nil
}

// checkMainCanReturn: finishing a branch that is checked out in the main worktree (a release, or a
// work or hotfix branch that was checked out there by hand) sends the main worktree back to develop,
// so develop must not be checked out elsewhere. Checked at plan time, before anything is merged.
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
