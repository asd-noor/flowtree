// Package boardui is the terminal kanban of `flowtree board`: the board's columns side by side, a detail
// pane, editing in $EDITOR and live reload. All git work arrives through Source, so the model is pure.
package boardui

import (
	"context"
	"fmt"
	"io"

	tea "charm.land/bubbletea/v2"
)

// Run shows the board until the user quits. in and out must be a terminal.
func Run(ctx context.Context, src Source, in io.Reader, out io.Writer) error {
	mt := modTime(src.Path)
	cards, err := src.Load()
	if err != nil {
		return err
	}
	if _, err := tea.NewProgram(newModel(src, cards, mt), tea.WithInput(in), tea.WithOutput(out), tea.WithContext(ctx)).Run(); err != nil {
		return fmt.Errorf("board: %w", err)
	}
	return nil
}
