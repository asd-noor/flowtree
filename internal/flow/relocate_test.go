package flow_test

import (
	"slices"
	"strings"
	"testing"

	"flowtree/internal/config"
	"flowtree/internal/flow"
	"flowtree/internal/gitx"
)

func relocateFixture() ([]gitx.Worktree, config.Config) {
	cfg := config.Defaults()
	cfg.BaseDir = "/new"
	return []gitx.Worktree{
		{Path: "/code/api", Branch: "develop", Main: true},
		{Path: "/old/work/a", Branch: "work/a"},
		{Path: "/old/hotfix/1", Branch: "hotfix/1"},
		{Path: "/new/work/done", Branch: "work/done"},
		{Path: "/old/detached"},
		{Path: "/old/other", Branch: "topic-without-prefix"},
	}, cfg
}

var sources = []string{"/code/api", "/old/work/a", "/old/hotfix/1", "/new/work/done", "/old/detached", "/old/other"}

// probe says every source worktree exists, plus the extra paths; nothing is locked.
func probe(extra ...string) flow.Probe {
	present := append(append([]string{}, sources...), extra...)
	return flow.Probe{
		Locked: func(string) bool { return false },
		Exists: func(p string) bool { return slices.Contains(present, p) },
	}
}

func TestPlanRelocateMovesOnlyTopicWorktreesOutOfPlace(t *testing.T) {
	wts, cfg := relocateFixture()
	plan, err := flow.PlanRelocate(wts, flow.DefaultSettings(), cfg, "/code/api", "/code/api", probe())
	if err != nil {
		t.Fatal(err)
	}
	want := []flow.Move{{"work/a", "/old/work/a", "/new/work/a"}, {"hotfix/1", "/old/hotfix/1", "/new/hotfix/1"}}
	if len(plan.Moves) != 2 || plan.Moves[0] != want[0] || plan.Moves[1] != want[1] || len(plan.Skipped) != 0 {
		t.Fatalf("%+v", plan)
	}
}

func TestPlanRelocateSkips(t *testing.T) {
	wts, cfg := relocateFixture()
	p := flow.Probe{
		Locked: func(path string) bool { return path == "/old/work/a" },
		Exists: func(path string) bool { return path != "/old/hotfix/1" && !strings.HasPrefix(path, "/new/") },
	}
	plan, err := flow.PlanRelocate(wts, flow.DefaultSettings(), cfg, "/code/api", "/code/api", p)
	if err != nil || len(plan.Moves) != 0 || len(plan.Skipped) != 2 {
		t.Fatalf("%+v %v", plan, err)
	}
	if !strings.Contains(plan.Skipped[0].Reason, "locked") || !strings.Contains(plan.Skipped[1].Reason, "missing") {
		t.Errorf("%+v", plan.Skipped)
	}

	plan, _ = flow.PlanRelocate(wts, flow.DefaultSettings(), cfg, "/code/api", "/code/api", probe("/new/work/a"))
	if len(plan.Skipped) != 1 || !strings.Contains(plan.Skipped[0].Reason, "already exists") || len(plan.Moves) != 1 {
		t.Errorf("%+v", plan)
	}
}

func TestPlanRelocateRefusesWhenInsideAMovingWorktree(t *testing.T) {
	wts, cfg := relocateFixture()
	_, err := flow.PlanRelocate(wts, flow.DefaultSettings(), cfg, "/code/api", "/old/work/a/sub", probe())
	if err == nil || !strings.Contains(err.Error(), "inside") {
		t.Fatalf("got %v", err)
	}
}
