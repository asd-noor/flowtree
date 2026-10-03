// Package picker selects one row from a list: a matcher that resolves an unambiguous query without
// any UI, and a bubbletea TUI for everything else.
package picker

import (
	"context"
	"errors"
	"fmt"
	"io"

	tea "charm.land/bubbletea/v2"
)

// Row is one selectable entry.
type Row struct {
	Key     string        // what is returned and matched against, e.g. a branch name or "@"
	Label   string        // what the list shows; defaults to Key
	Alt     string        // an extra string the query may match (e.g. the branch the main worktree is on)
	Preview func() string // text for the preview pane, computed lazily; may be nil
}

// ErrCancelled is returned by Pick when the user quits without choosing.
var ErrCancelled = errors.New("cancelled")

// Pick shows the TUI over rows, with query prefilled, and returns the chosen row.
// It reads keys from in and draws on out, which must be a terminal; callers check that first.
func Pick(ctx context.Context, rows []Row, query string, in io.Reader, out io.Writer) (Row, error) {
	if len(rows) == 0 {
		return Row{}, errors.New("nothing to pick from")
	}
	final, err := tea.NewProgram(newModel(rows, query), tea.WithInput(in), tea.WithOutput(out), tea.WithContext(ctx)).Run()
	if errors.Is(err, tea.ErrInterrupted) {
		return Row{}, ErrCancelled
	}
	if err != nil {
		return Row{}, fmt.Errorf("picker: %w", err)
	}
	return chosenRow(final)
}

func chosenRow(final tea.Model) (Row, error) {
	m, ok := final.(model)
	if !ok || m.chosen == nil {
		return Row{}, ErrCancelled
	}
	return *m.chosen, nil
}
