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
	testutil.Git(t, repo, "branch", "regular/a", "develop")
	card := board.Card{Branch: "regular/a", Start: start}
	if got := flow.CardStatus(e, card); got != board.Doing {
		t.Errorf("fresh branch: %s, want doing (no commits of its own is not merged)", got)
	}

	testutil.Git(t, repo, "checkout", "-q", "regular/a")
	commit(t, repo, "a.txt", "a")
	tip := strings.TrimSpace(testutil.Git(t, repo, "rev-parse", "HEAD"))
	card.ReviewTip = tip
	if got := flow.CardStatus(e, card); got != board.Review {
		t.Errorf("review at the tip: %s", got)
	}

	testutil.Git(t, repo, "checkout", "-q", "develop")
	testutil.Git(t, repo, "merge", "-q", "--no-ff", "--no-edit", "regular/a")
	testutil.Git(t, repo, "branch", "-q", "-d", "regular/a")
	card.Tip = tip
	if got := flow.CardStatus(e, card); got != board.Done {
		t.Errorf("merged and deleted: %s", got)
	}

	testutil.Git(t, repo, "branch", "regular/b", "develop")
	testutil.Git(t, repo, "checkout", "-q", "regular/b")
	commit(t, repo, "b.txt", "b")
	bTip := strings.TrimSpace(testutil.Git(t, repo, "rev-parse", "HEAD"))
	testutil.Git(t, repo, "checkout", "-q", "develop")
	testutil.Git(t, repo, "branch", "-q", "-D", "regular/b")
	if got := flow.CardStatus(e, board.Card{Branch: "regular/b", Tip: bTip}); got != board.Abandoned {
		t.Errorf("deleted unmerged: %s", got)
	}
}

func TestSubtaskTargetIsItsParent(t *testing.T) {
	e, repo := setup(t)
	testutil.Git(t, repo, "branch", "regular/a", "develop")
	testutil.Git(t, repo, "branch", "work/regular/a/x", "regular/a")
	if got := e.S.Target("work/regular/a/x"); got != "regular/a" {
		t.Fatalf("target %q", got)
	}
	testutil.Git(t, repo, "checkout", "-q", "work/regular/a/x")
	commit(t, repo, "x.txt", "x")
	testutil.Git(t, repo, "checkout", "-q", "regular/a")
	testutil.Git(t, repo, "merge", "-q", "--ff-only", "work/regular/a/x")
	tip := strings.TrimSpace(testutil.Git(t, repo, "rev-parse", "work/regular/a/x"))
	testutil.Git(t, repo, "branch", "-q", "-d", "work/regular/a/x")
	start := strings.TrimSpace(testutil.Git(t, repo, "rev-parse", "develop"))
	if got := flow.SubtaskStatus(e, board.Subtask{Branch: "work/regular/a/x", Start: start, Tip: tip}); got != board.Done {
		t.Errorf("subtask merged into its parent: %s", got)
	}
}

// A branch that only took another branch's commits stays doing once those land in develop.
func TestBorrowedCommitsAreNotMerged(t *testing.T) {
	e, repo := setup(t)
	start := strings.TrimSpace(testutil.Git(t, repo, "rev-parse", "develop"))
	testutil.Git(t, repo, "branch", "regular/a", "develop")
	testutil.Git(t, repo, "branch", "regular/b", "develop")
	testutil.Git(t, repo, "checkout", "-q", "regular/a")
	commit(t, repo, "a.txt", "a")
	testutil.Git(t, repo, "checkout", "-q", "regular/b")
	testutil.Git(t, repo, "merge", "-q", "--ff-only", "regular/a")
	testutil.Git(t, repo, "checkout", "-q", "develop")
	testutil.Git(t, repo, "merge", "-q", "--no-ff", "--no-edit", "regular/a")
	testutil.Git(t, repo, "branch", "-q", "-d", "regular/a")
	if got := flow.CardStatus(e, board.Card{Branch: "regular/b", Start: start}); got != board.Doing {
		t.Errorf("borrowed commits only: %s, want doing", got)
	}
}

func TestCarriedReviewMovesOnlyAMarkAtTheOldTip(t *testing.T) {
	b, _ := board.Empty().Add(board.Card{Branch: "regular/a", ReviewTip: "old"})
	if c, _ := flow.CarriedReviewForTest(b, "regular/a", "old", "new").Card(1); c.ReviewTip != "new" || c.Tip != "new" {
		t.Errorf("mark at the old tip should move: %+v", c)
	}
	if c, _ := flow.CarriedReviewForTest(b, "regular/a", "other", "new").Card(1); c.ReviewTip != "old" {
		t.Errorf("a mark at another tip stays: %+v", c)
	}
}

func TestTypeOf(t *testing.T) {
	s := flow.DefaultSettings()
	for name, tc := range map[string]struct {
		card board.Card
		want board.Type
	}{
		"stored wins":    {board.Card{Type: board.Hotfix, Branch: "regular/a"}, board.Hotfix},
		"hotfix prefix":  {board.Card{Branch: "hotfix/1.0.1"}, board.Hotfix},
		"regular prefix": {board.Card{Branch: "regular/a"}, board.Regular},
		"old feature":    {board.Card{Branch: "feature/a"}, board.Regular},
		"no branch":      {board.Card{}, board.Regular},
	} {
		if got := flow.TypeOf(s, tc.card); got != tc.want {
			t.Errorf("%s: %s, want %s", name, got, tc.want)
		}
	}
}
