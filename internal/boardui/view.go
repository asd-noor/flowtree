package boardui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"flowtree/internal/board"
)

func (m model) View() tea.View {
	v := tea.NewView(render(m))
	v.AltScreen = true
	return v
}

// render draws the columns, the detail pane when open, and a footer.
func render(m model) string {
	colH := m.height - 2
	if m.detail {
		colH = (m.height - 2) / 2
	}
	lines := columnLines(m, max(colH, 3))
	if m.detail {
		lines = append(append(lines, strings.Repeat("─", m.width)), detailLines(m, m.height-2-len(lines)-1)...)
	}
	return strings.Join(append(fit(lines, m.width), footer(m)), "\n")
}

func footer(m model) string {
	if m.notice != "" {
		return ansi.Truncate("! "+m.notice, m.width, "…")
	}
	return "h/l columns  j/k cards  enter detail  e edit  q quit"
}

// columnLines draws every column side by side: a header, then the cards, windowed to height lines.
func columnLines(m model, height int) []string {
	w := m.width / len(board.Columns)
	cols := make([][]string, len(board.Columns))
	for i, s := range board.Columns {
		cols[i] = columnCells(m, i, s, height, w)
	}
	rows := make([]string, height)
	for r := range rows {
		rows[r] = strings.Join(mapAt(cols, r), "")
	}
	return rows
}

func mapAt(cols [][]string, r int) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c[r]
	}
	return out
}

// columnCells is one column as exactly height cells of width w.
func columnCells(m model, i int, s board.Status, height, w int) []string {
	cs := cardsIn(m.cards, s)
	head := fmt.Sprintf("%s (%d)", strings.ToUpper(string(s)), len(cs))
	if i == m.col {
		head = "[" + head + "]"
	}
	shown, off := window(len(cs), m.row, height-1, i == m.col)
	cells := []string{pad(head, w)}
	for j, c := range cs[off : off+shown] {
		cells = append(cells, pad(cardLine(c, i == m.col && off+j == m.row), w))
	}
	for len(cells) < height {
		cells = append(cells, pad("", w))
	}
	return cells
}

// window picks how many of n cards show and from which, keeping the cursor row in view.
func window(n, cursor, h int, active bool) (shown, off int) {
	shown = min(n, max(h, 0))
	if active && cursor >= shown {
		off = cursor - shown + 1
	}
	return shown, off
}

func cardLine(c Card, on bool) string {
	mark := " "
	if on {
		mark = ">"
	}
	return fmt.Sprintf("%s#%d %s%s", mark, c.ID, c.Title, humanMark(c))
}

func humanMark(c Card) string {
	if c.Human {
		return " [h]"
	}
	return ""
}

// detailLines describes the selected card, cut to height lines.
func detailLines(m model, height int) []string {
	c, ok := m.selected()
	if !ok {
		return []string{"(no card selected)"}
	}
	lines := strings.Split(strings.TrimRight(detailText(c), "\n"), "\n")
	return lines[:max(0, min(len(lines), height))]
}

func detailText(c Card) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#%d %s%s\nstatus: %s\n", c.ID, c.Title, humanMark(c), c.Status)
	if c.Branch != "" {
		fmt.Fprintf(&b, "branch: %s\n", c.Branch)
	}
	if len(c.Tags) > 0 {
		fmt.Fprintf(&b, "tags:   %s\n", strings.Join(c.Tags, ", "))
	}
	if c.Description != "" {
		fmt.Fprintf(&b, "\n%s\n", strings.TrimRight(c.Description, "\n"))
	}
	b.WriteString(subtaskText(c))
	b.WriteString(commentText(c))
	return b.String()
}

func subtaskText(c Card) string {
	if len(c.Subtasks) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\nSubtasks:\n")
	for i, s := range c.Subtasks {
		fmt.Fprintf(&b, "  %-9s %s (%s)\n", subStatus(c, i), s.Name, s.Branch)
	}
	return b.String()
}

func subStatus(c Card, i int) board.Status {
	if i < len(c.SubStatus) {
		return c.SubStatus[i]
	}
	return ""
}

func commentText(c Card) string {
	if len(c.Comments) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\nComments:\n")
	for _, n := range c.Comments {
		fmt.Fprintf(&b, "  %s, %s: %s\n", n.Author, n.At.Format("2006-01-02 15:04"), strings.ReplaceAll(n.Body, "\n", " "))
	}
	return b.String()
}

// pad cuts or pads s to exactly w columns, leaving one column of gutter.
func pad(s string, w int) string {
	s = ansi.Truncate(s, max(w-1, 0), "…")
	return s + strings.Repeat(" ", max(0, w-ansi.StringWidth(s)))
}

func fit(lines []string, width int) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = ansi.Truncate(l, width, "")
	}
	return out
}
