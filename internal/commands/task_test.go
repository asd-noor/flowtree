package commands

import (
	"strings"
	"testing"

	"flowtree/internal/board"
)

func TestTaskCommands(t *testing.T) {
	repo := flowRepo(t)
	d, out, _ := testDeps(t, repo, nil)
	must := func(args ...string) string {
		t.Helper()
		out.Reset()
		if err := run(t, d, args...); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		return out.String()
	}
	if got := must("task", "add", "-t", "ui, auth", "Login", "page"); got != "#1\n" {
		t.Fatalf("add printed %q", got)
	}
	must("task", "add", "--backlog", "--human", "Later")
	must("task", "edit", "1", "-d", "Body")
	must("task", "comment", "--author", "me", "#1", "first", "note")

	if got := must("task", "list"); !strings.Contains(got, "#1    todo      Login page  [ui,auth]") || !strings.Contains(got, "#2    backlog   Later  [human]") {
		t.Errorf("list:\n%s", got)
	}
	if got := must("task", "list", "-s", "backlog"); strings.Contains(got, "#1") {
		t.Errorf("filtered list:\n%s", got)
	}
	if got := must("task", "show", "1"); !strings.Contains(got, "Body") || !strings.Contains(got, "me, ") || !strings.Contains(got, "first note") {
		t.Errorf("show:\n%s", got)
	}
	must("task", "todo", "2")
	must("task", "human", "--off", "2")
	if got := must("task", "show", "--json", "2"); !strings.Contains(got, `"status": "todo"`) || strings.Contains(got, `"human"`) {
		t.Errorf("json:\n%s", got)
	}

	for _, args := range [][]string{
		{"task", "show", "9"}, {"task", "show", "x"}, {"task", "list", "-s", "nope"},
		{"task", "edit", "1"}, // no flags and no terminal
		{"task", "edit", "1", "--title", ""},
	} {
		if err := run(t, d, args...); err == nil {
			t.Errorf("%v should fail", args)
		}
	}
}

func TestApplyEditText(t *testing.T) {
	c, err := applyEditText(board.Card{Title: "old"}, "  New title \n\nline 1\nline 2\n")
	if err != nil || c.Title != "New title" || c.Description != "line 1\nline 2" {
		t.Fatalf("%+v %v", c, err)
	}
	if _, err := applyEditText(board.Card{}, "\n\nbody"); err == nil {
		t.Error("an empty title must be refused")
	}
	if editText(board.Card{Title: "T", Description: "D"}) != "T\n\nD" {
		t.Error("edit text layout")
	}
}

func TestBoardNeedsTerminal(t *testing.T) {
	d, _, _ := testDeps(t, flowRepo(t), nil)
	err := run(t, d, "board")
	if err == nil || !strings.Contains(err.Error(), "flowtree task list") {
		t.Fatalf("err = %v", err)
	}
}
