package boardui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"flowtree/internal/board"
)

// cardRows is the rows one card takes: its title, its branch line and a gap.
const cardRows = 3

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
	cols := columns(m, max(colH, 4))
	parts := []string{cols}
	if m.detail {
		parts = append(parts, detailBox(m, m.height-2-lipgloss.Height(cols)))
	}
	return strings.Join(append(fit(strings.Split(strings.Join(parts, "\n"), "\n"), m.width), footer(m)), "\n")
}

var helpKeys = [][2]string{{"h/l", "columns"}, {"j/k", "cards"}, {"enter", "detail"}, {"e", "edit"}, {"c", "comment"}, {"q", "quit"}}

func footer(m model) string {
	if m.notice != "" {
		return noticeStyle.Render(ansi.Truncate("! "+m.notice, m.width, "…"))
	}
	items := make([]string, len(helpKeys))
	for i, k := range helpKeys {
		items[i] = bold.Render(k[0]) + " " + faint.Render(k[1])
	}
	return " " + strings.Join(items, faint.Render("  ·  "))
}

// columns draws every column side by side, each a box of exactly height lines.
func columns(m model, height int) string {
	w := m.width / len(board.Columns)
	boxes := make([]string, len(board.Columns))
	for i, s := range board.Columns {
		boxes[i] = column(m, i, s, height, w)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, boxes...)
}

// column is one status as a rounded box of width w: the title in its top edge, then the cards, windowed
// to the box's height.
func column(m model, i int, s board.Status, height, w int) string {
	cs, active := cardsIn(m.cards, s), i == m.col
	a, edge := accentOf(s), lipgloss.NewStyle().Foreground(frameColor)
	if active {
		edge = lipgloss.NewStyle().Foreground(a.bright)
	}
	inner := max(w-2, 0)
	shown, off := window(len(cs), m.row, (height-2+1)/cardRows, active)
	body := make([]string, 0, height-2)
	for j, c := range cs[off : off+shown] {
		body = append(body, cardLines(c, active && off+j == m.row, a, inner)...)
	}
	body = padLines(body, height-2, inner)
	box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder(), false, true, true, true).BorderForeground(edge.GetForeground())
	return columnTop(strings.ToUpper(string(s)), len(cs), active, a, edge, w) + "\n" + box.Render(strings.Join(body, "\n"))
}

// columnTop is a column's top border with its title and count set into it.
func columnTop(name string, n int, active bool, a accent, edge lipgloss.Style, w int) string {
	title := ansi.Truncate(fmt.Sprintf("%s %d", name, n), max(w-4, 0), "…")
	head := lipgloss.NewStyle().Foreground(a.dim)
	if active {
		head = head.Foreground(a.bright).Bold(true)
	}
	if w < 4 {
		return strings.Repeat(" ", max(w, 0))
	}
	fill := max(w-4-ansi.StringWidth(title), 0)
	return edge.Render("╭ ") + head.Render(title) + edge.Render(" "+strings.Repeat("─", fill)+"╮")
}

// cardLines is a card as cardRows lines of width w: a marker and its title, its branch and tags, a gap.
func cardLines(c Card, on bool, a accent, w int) []string {
	badge := ""
	if c.Human {
		badge = " human"
	}
	title := ansi.Truncate(fmt.Sprintf("#%d %s", c.ID, c.Title), max(w-1-len(badge), 0), "…")
	mark, text := " ", lipgloss.NewStyle()
	if on {
		mark, text = lipgloss.NewStyle().Foreground(a.bright).Render("▌"), text.Foreground(a.bright).Bold(true)
	}
	first := mark + text.Render(title) + humanBadge.Render(badge)
	second := "  " + faint.Render(ansi.Truncate(cardSub(c), max(w-2, 0), "…"))
	return padLines([]string{first, second}, cardRows, w)
}

// cardSub is the dim line under a card's title: its branch and tags.
func cardSub(c Card) string {
	parts := []string{}
	if c.Branch != "" {
		parts = append(parts, c.Branch)
	}
	if len(c.Tags) > 0 {
		parts = append(parts, "#"+strings.Join(c.Tags, " #"))
	}
	if len(parts) == 0 {
		return "no branch"
	}
	return strings.Join(parts, "  ")
}

// window picks how many of n cards show and from which, keeping the cursor row in view.
func window(n, cursor, h int, active bool) (shown, off int) {
	shown = min(n, max(h, 0))
	if active {
		off = max(0, min(n-shown, cursor-shown+1))
	}
	return shown, off
}

// detailBox is the selected card in a rounded box of exactly height lines.
func detailBox(m model, height int) string {
	if height < 3 {
		return ""
	}
	inner := max(m.width-4, 0)
	lines := padLines(detailLines(m, height-2), height-2, inner)
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(frameColor).Padding(0, 1).
		Render(strings.Join(lines, "\n"))
}

// detailLines describes the selected card, cut to height lines.
func detailLines(m model, height int) []string {
	c, ok := m.selected()
	if !ok {
		return []string{faint.Render("(no card selected)")}
	}
	lines := strings.Split(strings.TrimRight(detailText(c), "\n"), "\n")
	return lines[:max(0, min(len(lines), height))]
}

func detailText(c Card) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s%s\n", bold.Render(fmt.Sprintf("#%d %s", c.ID, c.Title)), humanBadge.Render(humanMark(c)))
	fmt.Fprintf(&b, "%s %s\n", label("status"), statusText(c.Status, string(c.Status)))
	if c.Branch != "" {
		fmt.Fprintf(&b, "%s %s\n", label("branch"), c.Branch)
	}
	if len(c.Tags) > 0 {
		fmt.Fprintf(&b, "%s %s\n", label("tags"), strings.Join(c.Tags, ", "))
	}
	if c.Description != "" {
		fmt.Fprintf(&b, "\n%s\n", strings.TrimRight(c.Description, "\n"))
	}
	b.WriteString(subtaskText(c))
	b.WriteString(commentText(c))
	return b.String()
}

func humanMark(c Card) string {
	if c.Human {
		return " human"
	}
	return ""
}

// label is a dim, aligned field name of the detail pane.
func label(s string) string { return faint.Render(fmt.Sprintf("%-7s", s+":")) }

func subtaskText(c Card) string {
	if len(c.Subtasks) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n" + bold.Render("Subtasks") + "\n")
	for i, s := range c.Subtasks {
		st := subStatus(c, i)
		fmt.Fprintf(&b, "  %s %s %s\n", statusText(st, fmt.Sprintf("%-9s", st)), s.Name, faint.Render("("+s.Branch+")"))
		if s.Description != "" {
			fmt.Fprintf(&b, "            %s\n", strings.ReplaceAll(strings.TrimSpace(s.Description), "\n", "\n            "))
		}
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
	b.WriteString("\n" + bold.Render("Comments") + "\n")
	for _, n := range c.Comments {
		fmt.Fprintf(&b, "  %s %s\n", faint.Render(n.Author+", "+n.At.Format("2006-01-02 15:04")+":"), strings.ReplaceAll(n.Body, "\n", " "))
	}
	return b.String()
}

// padLines cuts or pads lines to exactly n lines, each exactly w columns wide.
func padLines(lines []string, n, w int) []string {
	out := make([]string, max(n, 0))
	for i := range out {
		if i < len(lines) {
			out[i] = pad(lines[i], w)
		} else {
			out[i] = pad("", w)
		}
	}
	return out
}

// pad cuts or pads s, which may hold ANSI styles, to exactly w columns.
func pad(s string, w int) string {
	s = ansi.Truncate(s, max(w, 0), "…")
	return s + strings.Repeat(" ", max(0, w-ansi.StringWidth(s)))
}

func fit(lines []string, width int) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = ansi.Truncate(l, width, "")
	}
	return out
}
