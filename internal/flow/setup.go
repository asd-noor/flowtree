package flow

import (
	"fmt"

	"flowtree/internal/gitx"
)

// DetectMaster proposes the production branch: an existing master or main, else "master".
func DetectMaster(g gitx.Git) string {
	for _, b := range []string{"master", "main"} {
		if gitx.BranchExists(g, b) {
			return b
		}
	}
	return DefaultMaster
}

// EnsureBranches creates the master and develop branches when they are missing, and puts the
// main worktree on develop when it was on master and is clean (the resting place of a flow repo).
func EnsureBranches(e *Env) error {
	g := e.MainGit()
	if _, ok := Rev(g, "HEAD"); !ok {
		return fmt.Errorf("the repository has no commits yet; make an initial commit before 'init --flow'")
	}
	steps := Plan(
		createIfMissing(e.S.Master, "HEAD"),
		createIfMissing(e.S.Develop, e.S.Master),
	)
	if err := Run(e, steps); err != nil {
		return err
	}
	return restOnDevelop(e)
}

func createIfMissing(branch, from string) []Step {
	return One("create branch "+branch, func(e *Env) error {
		g := e.MainGit()
		switch {
		case gitx.BranchExists(g, branch):
			return nil
		case e.HasRemote() && gitx.RemoteBranchExists(g, e.S.Remote, branch):
			_, err := g("branch", "--track", branch, e.S.Remote+"/"+branch)
			return err
		}
		if _, err := g("branch", branch, from); err != nil {
			return err
		}
		e.Say("Created branch '%s' from '%s'", branch, from)
		return nil
	})
}

func restOnDevelop(e *Env) error {
	if gitx.CurrentBranch(e.MainGit()) != e.S.Master {
		return nil
	}
	if err := SwitchMain(e, e.S.Develop); err != nil {
		e.Note("Note: the main worktree stays on '%s': %v", e.S.Master, err)
		return nil
	}
	e.Say("The main worktree is now on '%s'", e.S.Develop)
	return nil
}
