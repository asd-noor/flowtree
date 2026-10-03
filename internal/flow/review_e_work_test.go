package flow_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"flowtree/internal/flow"
	"flowtree/internal/gitx"
	"flowtree/internal/testutil"
)

func TestPullAmbiguousPrefixIsReportedNotSwallowed(t *testing.T) {
	e, repo := setup(t)
	testutil.Git(t, repo, "branch", "work/login", "develop")
	testutil.Git(t, repo, "branch", "work/logout", "develop")
	_, err := flow.PlanPull(e, "origin", "log")
	if err == nil || !strings.Contains(err.Error(), "multiple branches match") {
		t.Fatalf("want the ambiguity error, got %v", err)
	}
	if strings.Contains(err.Error(), "not found on") {
		t.Errorf("misleading message: %v", err)
	}
}

func TestPullUnknownLocalNameFallsBackToTheRemoteBranch(t *testing.T) {
	e, _ := setup(t)
	steps, err := flow.PlanPull(e, "origin", "only-on-remote")
	if err != nil || len(steps) != 2 {
		t.Fatalf("a name that matches no local branch must still plan a pull: %v %d", err, len(steps))
	}
	if !strings.Contains(flow.Describe(steps), "work/only-on-remote") {
		t.Errorf("plan should target work/only-on-remote:\n%s", flow.Describe(steps))
	}
}

func TestPullGitFailureIsNotHidden(t *testing.T) {
	e, _ := setup(t)
	boom := errors.New("boom")
	real := e.Run
	e.Run = func(ctx context.Context, dir string, args ...string) (string, error) {
		if len(args) > 0 && args[0] == "for-each-ref" {
			return "", &gitx.Error{Args: args, Output: "boom", Err: boom}
		}
		return real(ctx, dir, args...)
	}
	if _, err := flow.PlanPull(e, "origin", "anything"); err == nil || errors.Is(err, flow.ErrNoSuchBranch) {
		t.Fatalf("a git failure must be reported as such, got %v", err)
	}
}

func TestPullWithoutNameOutsideATopicWorktree(t *testing.T) {
	e, _ := setup(t) // cwd is the main worktree on develop
	if _, err := flow.PlanPull(e, "origin", ""); err == nil || !strings.Contains(err.Error(), "no work branch") {
		t.Fatalf("got %v", err)
	}
}

func TestResolveTopicNotFoundIsDistinguishable(t *testing.T) {
	e, repo := setup(t)
	testutil.Git(t, repo, "branch", "work/login", "develop")
	testutil.Git(t, repo, "branch", "work/logout", "develop")
	if _, err := flow.ResolveTopic(e, flow.Work, "zzz"); !errors.Is(err, flow.ErrNoSuchBranch) {
		t.Errorf("not found should match ErrNoSuchBranch: %v", err)
	}
	if _, err := flow.ResolveTopic(e, flow.Hotfix, "9"); !errors.Is(err, flow.ErrNoSuchBranch) {
		t.Errorf("hotfix not found should match ErrNoSuchBranch: %v", err)
	}
	if _, err := flow.ResolveTopic(e, flow.Work, "log"); errors.Is(err, flow.ErrNoSuchBranch) {
		t.Errorf("ambiguity must not look like not-found: %v", err)
	}
}
