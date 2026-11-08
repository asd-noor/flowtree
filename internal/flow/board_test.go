package flow_test

import (
	"strings"
	"testing"

	"flowtree/internal/board"
	"flowtree/internal/flow"
	"flowtree/internal/testutil"
)

func TestCardStatusFollowsGit(t *testing.T) {
	e, repo := setup(t)
	start := strings.TrimSpace(testutil.Git(t, repo, "rev-parse", "develop"))
	testutil.Git(t, repo, "branch", "feature/a", "develop")
	card := board.Card{Branch: "feature/a", Start: start}
	if got := flow.CardStatus(e, card); got != board.Doing {
		t.Errorf("fresh branch: %s, want doing (no commits of its own is not merged)", got)
	}

	testutil.Git(t, repo, "checkout", "-q", "feature/a")
	commit(t, repo, "a.txt", "a")
	tip := strings.TrimSpace(testutil.Git(t, repo, "rev-parse", "HEAD"))
	card.ReviewTip = tip
	if got := flow.CardStatus(e, card); got != board.Review {
		t.Errorf("review at the tip: %s", got)
	}

	testutil.Git(t, repo, "checkout", "-q", "develop")
	testutil.Git(t, repo, "merge", "-q", "--no-ff", "--no-edit", "feature/a")
	testutil.Git(t, repo, "branch", "-q", "-d", "feature/a")
	card.Tip = tip
	if got := flow.CardStatus(e, card); got != board.Done {
		t.Errorf("merged and deleted: %s", got)
	}

	testutil.Git(t, repo, "branch", "feature/b", "develop")
	testutil.Git(t, repo, "checkout", "-q", "feature/b")
	commit(t, repo, "b.txt", "b")
	bTip := strings.TrimSpace(testutil.Git(t, repo, "rev-parse", "HEAD"))
	testutil.Git(t, repo, "checkout", "-q", "develop")
	testutil.Git(t, repo, "branch", "-q", "-D", "feature/b")
	if got := flow.CardStatus(e, board.Card{Branch: "feature/b", Tip: bTip}); got != board.Abandoned {
		t.Errorf("deleted unmerged: %s", got)
	}
}

func TestSubtaskTargetIsItsParent(t *testing.T) {
	e, repo := setup(t)
	testutil.Git(t, repo, "branch", "feature/a", "develop")
	testutil.Git(t, repo, "branch", "work/feature/a/x", "feature/a")
	if got := e.S.Target("work/feature/a/x"); got != "feature/a" {
		t.Fatalf("target %q", got)
	}
	testutil.Git(t, repo, "checkout", "-q", "work/feature/a/x")
	commit(t, repo, "x.txt", "x")
	testutil.Git(t, repo, "checkout", "-q", "feature/a")
	testutil.Git(t, repo, "merge", "-q", "--ff-only", "work/feature/a/x")
	start := strings.TrimSpace(testutil.Git(t, repo, "rev-parse", "develop"))
	if got := flow.SubtaskStatus(e, board.Subtask{Branch: "work/feature/a/x", Start: start}); got != board.Done {
		t.Errorf("subtask merged into its parent: %s", got)
	}
}

func TestCarriedReviewMovesOnlyAMarkAtTheOldTip(t *testing.T) {
	b, _ := board.Empty().Add(board.Card{Branch: "feature/a", ReviewTip: "old"})
	if c, _ := flow.CarriedReviewForTest(b, "feature/a", "old", "new").Card(1); c.ReviewTip != "new" || c.Tip != "new" {
		t.Errorf("mark at the old tip should move: %+v", c)
	}
	if c, _ := flow.CarriedReviewForTest(b, "feature/a", "other", "new").Card(1); c.ReviewTip != "old" {
		t.Errorf("a mark at another tip stays: %+v", c)
	}
}
