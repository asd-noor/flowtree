package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"flowtree/internal/board"
	"flowtree/internal/boardui"
	"flowtree/internal/flow"
)

func newBoardCmd(d Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "board",
		Short: "Browse the task board in a terminal (for people; agents use `task`)",
		Long: "A read-mostly kanban of the repository's tasks, one column per status. Move with the arrow keys\n" +
			"or h j k l, enter shows the card's details (J/K, ctrl+d/ctrl+u or pgdn/pgup scroll them),\n" +
			"e edits it in your editor, c adds a comment, q quits. Changes made by other\n" +
			"flowtree commands show up within a second.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !d.IsTTY() {
				return errors.New("no terminal for the board; use `flowtree task list`")
			}
			return withBoard(cmd.Context(), d, func(e *flow.Env, path string) error {
				src := boardui.Source{Load: loadCards(e, path), Edit: editCard(cmd, d, e, path), Comment: commentCard(cmd, e, path)}
				return boardui.Run(cmd.Context(), src, d.Stdin, d.Stderr)
			})
		},
	}
}

// loadCards reads the board and derives every status from git.
func loadCards(e *flow.Env, path string) func() ([]boardui.Card, error) {
	return func() ([]boardui.Card, error) {
		b, err := board.Load(path)
		if err != nil {
			return nil, err
		}
		return mapSlice(b.Cards, func(c board.Card) boardui.Card {
			return boardui.Card{Card: c, Status: flow.CardStatus(e, c),
				SubStatus: mapSlice(c.Subtasks, func(s board.Subtask) board.Status { return flow.SubtaskStatus(e, s) })}
		}), nil
	}
}

// editCard prepares the editor for a card. Once it exits the text is applied to the card as it is on disk
// then, under the board lock, so edits by others in the meantime are not overwritten.
func editCard(cmd *cobra.Command, d Deps, e *flow.Env, path string) func(boardui.Card) (*exec.Cmd, func(error) error, error) {
	return func(c boardui.Card) (*exec.Cmd, func(error) error, error) {
		run, file, err := editorCmd(cmd.Context(), e, fmt.Sprintf("task-%d", c.ID), editText(c.Card))
		if err != nil {
			return nil, nil, err
		}
		done := func(runErr error) error {
			defer func() { _ = os.Remove(file) }()
			if runErr != nil {
				return fmt.Errorf("editor failed: %w", runErr)
			}
			data, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			return updateCard(path, c.ID, func(cur board.Card) (board.Card, error) { return applyEditText(cur, string(data)) })
		}
		return run, done, nil
	}
}

// commentCard prepares the editor for a new comment on a card; an empty text adds nothing.
func commentCard(cmd *cobra.Command, e *flow.Env, path string) func(boardui.Card) (*exec.Cmd, func(error) error, error) {
	return func(c boardui.Card) (*exec.Cmd, func(error) error, error) {
		run, file, err := editorCmd(cmd.Context(), e, fmt.Sprintf("task-%d-comment", c.ID), "")
		if err != nil {
			return nil, nil, err
		}
		done := func(runErr error) error {
			defer func() { _ = os.Remove(file) }()
			if runErr != nil {
				return fmt.Errorf("editor failed: %w", runErr)
			}
			data, err := os.ReadFile(file)
			if err != nil || strings.TrimSpace(string(data)) == "" {
				return err
			}
			who, _ := e.MainGit()("config", "user.name")
			note := board.Comment{Author: orElse(who, "unknown"), Body: strings.TrimSpace(string(data)), At: time.Now().UTC()}
			return updateCard(path, c.ID, func(cur board.Card) (board.Card, error) {
				cur.Comments = append(slices.Clone(cur.Comments), note)
				return cur, nil
			})
		}
		return run, done, nil
	}
}
