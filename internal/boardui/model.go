package boardui

import (
	"os/exec"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"flowtree/internal/board"
)

// Card is a board card with the statuses git gave it, so the model never needs git.
type Card struct {
	board.Card
	Status    board.Status
	SubStatus []board.Status // parallel to Card.Subtasks
}

// Source is everything the board reaches outside itself for.
type Source struct {
	// Load reads the board and computes every status.
	Load func() ([]Card, error)
	// Edit prepares an external editor for a card: the process to run, and what to do once it exits.
	Edit func(Card) (*exec.Cmd, func(runErr error) error, error)
}

const (
	defaultWidth  = 80
	defaultHeight = 24
	// reloadEvery is how often the board is reloaded: statuses follow git, so a plain commit can move a
	// card without the board file changing.
	reloadEvery = time.Second
)

type model struct {
	src           Source
	cards         []Card
	col, row      int
	detail        bool
	width, height int
	notice        string // last error, shown until the next key
}

type (
	tickMsg   time.Time
	loadedMsg struct {
		cards []Card
		err   error
	}
	editedMsg struct{ err error }
)

func newModel(src Source, cards []Card) model {
	return model{src: src, cards: cards, width: defaultWidth, height: defaultHeight}.clamped()
}

func (m model) Init() tea.Cmd { return tick() }

func tick() tea.Cmd { return tea.Tick(reloadEvery, func(t time.Time) tea.Msg { return tickMsg(t) }) }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyPressMsg:
		return m.onKey(msg)
	case tickMsg:
		return m, tea.Batch(reload(m.src), tick())
	case loadedMsg:
		return m.loaded(msg), nil
	case editedMsg:
		return m.edited(msg)
	}
	return m, nil
}

func (m model) onKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	m.notice = ""
	switch msg.String() {
	case "q", "esc", "ctrl+c":
		return m, tea.Quit
	case "left", "h":
		return m.moveCol(-1), nil
	case "right", "l":
		return m.moveCol(1), nil
	case "up", "k":
		return m.moveRow(-1), nil
	case "down", "j":
		return m.moveRow(1), nil
	case "enter", "tab":
		m.detail = !m.detail
	case "e":
		return m.edit()
	}
	return m, nil
}

// cardsIn is the cards of one column, in board order.
func cardsIn(cards []Card, s board.Status) []Card {
	return slices.DeleteFunc(slices.Clone(cards), func(c Card) bool { return c.Status != s })
}

func (m model) column() []Card { return cardsIn(m.cards, board.Columns[m.col]) }

// selected is the card under the cursor.
func (m model) selected() (Card, bool) {
	cs := m.column()
	if m.row < 0 || m.row >= len(cs) {
		return Card{}, false
	}
	return cs[m.row], true
}

func (m model) moveCol(d int) model {
	m.col = max(0, min(len(board.Columns)-1, m.col+d))
	return m.clamped()
}

func (m model) moveRow(d int) model {
	m.row += d
	return m.clamped()
}

// clamped keeps the cursor on a card of its column.
func (m model) clamped() model {
	m.row = max(0, min(len(m.column())-1, m.row))
	return m
}

// focus moves the cursor to card id, when it is on the board.
func (m model) focus(id int) model {
	i := slices.IndexFunc(m.cards, func(c Card) bool { return c.ID == id })
	if i < 0 {
		return m.clamped()
	}
	m.col = slices.Index(board.Columns, m.cards[i].Status)
	m.row = slices.IndexFunc(m.column(), func(c Card) bool { return c.ID == id })
	return m.clamped()
}

// loaded swaps in a reloaded board and keeps the cursor on the same card when it still exists.
func (m model) loaded(msg loadedMsg) model {
	if msg.err != nil {
		m.notice = msg.err.Error()
		return m
	}
	cur, had := m.selected()
	m.cards = msg.cards
	if had {
		return m.focus(cur.ID)
	}
	return m.clamped()
}

func (m model) edit() (tea.Model, tea.Cmd) {
	c, ok := m.selected()
	if !ok || m.src.Edit == nil {
		return m, nil
	}
	cmd, done, err := m.src.Edit(c)
	if err != nil {
		m.notice = err.Error()
		return m, nil
	}
	return m, tea.ExecProcess(cmd, func(runErr error) tea.Msg { return editedMsg{err: done(runErr)} })
}

func (m model) edited(msg editedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.notice = msg.err.Error()
	}
	return m, reload(m.src)
}

func reload(src Source) tea.Cmd {
	return func() tea.Msg {
		cards, err := src.Load()
		return loadedMsg{cards: cards, err: err}
	}
}
