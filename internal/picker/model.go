package picker

import (
	tea "charm.land/bubbletea/v2"
)

// model is the picker's state. Update and View are thin: the logic is in plain functions below.
type model struct {
	rows          []Row
	query         string
	cursor        int
	width, height int
	previews      map[string]string // by row key, filled lazily as rows are highlighted
	chosen        *Row
	done          bool
}

const (
	defaultWidth  = 80
	defaultHeight = 24
)

func newModel(rows []Row, query string) model {
	return model{rows: rows, query: query, width: defaultWidth, height: defaultHeight, previews: map[string]string{}}.cached()
}

// visible is the filtered list for the current query.
func (m model) visible() []Row { return Filter(m.rows, m.query) }

// current is the highlighted row.
func (m model) current() (Row, bool) {
	v := m.visible()
	if len(v) == 0 {
		return Row{}, false
	}
	return v[clamp(m.cursor, len(v))], true
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyPressMsg:
		return m.onKey(msg)
	}
	return m, nil
}

func (m model) onKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch key := msg.String(); key {
	case "enter":
		return m.choose()
	case "esc", "ctrl+c":
		m.done = true
		return m, tea.Quit
	case "up", "ctrl+p":
		return m.moved(-1).cached(), nil
	case "down", "ctrl+n":
		return m.moved(1).cached(), nil
	case "backspace":
		return m.withQuery(dropLastRune(m.query)).cached(), nil
	case "ctrl+u":
		return m.withQuery("").cached(), nil
	default:
		return m.withQuery(m.query + msg.Text).cached(), nil
	}
}

func (m model) choose() (tea.Model, tea.Cmd) {
	row, ok := m.current()
	if !ok {
		return m, nil
	}
	m.chosen, m.done = &row, true
	return m, tea.Quit
}

func (m model) moved(delta int) model {
	m.cursor = clamp(m.cursor+delta, len(m.visible()))
	return m
}

// withQuery sets a new query and puts the cursor back on the best match.
func (m model) withQuery(q string) model {
	m.query, m.cursor = q, 0
	return m
}

// cached makes sure the highlighted row's preview has been computed once.
func (m model) cached() model {
	row, ok := m.current()
	if _, have := m.previews[row.Key]; !ok || have || row.Preview == nil {
		return m
	}
	m.previews[row.Key] = row.Preview()
	return m
}

func (m model) View() tea.View { return tea.NewView(render(m)) }

// clamp keeps a cursor inside a list of n rows (0 when empty).
func clamp(i, n int) int { return max(0, min(i, n-1)) }

func dropLastRune(s string) string {
	r := []rune(s)
	return string(r[:max(0, len(r)-1)])
}

func label(r Row) string {
	if r.Label != "" {
		return r.Label
	}
	return r.Key
}

// window returns the slice of rows to show (at most h) so that the cursor stays visible.
func window(rows []Row, cursor, h int) (shown []Row, offset int) {
	if len(rows) <= h {
		return rows, 0
	}
	offset = max(0, min(cursor-h/2, len(rows)-h))
	return rows[offset : offset+h], offset
}
