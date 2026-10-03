package picker

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

const (
	sideBySideMinWidth = 100
	maxListLines       = 12
	maxPreviewLines    = 12
)

// render draws the whole UI as plain text: query line, list, preview.
func render(m model) string {
	list := listLines(m)
	preview := previewLines(m)
	if m.width >= sideBySideMinWidth {
		return strings.Join(append([]string{queryLine(m)}, sideBySide(list, preview, m.width)...), "\n")
	}
	parts := append([]string{queryLine(m)}, list...)
	if len(preview) > 0 {
		parts = append(append(parts, strings.Repeat("─", min(m.width, 40))), preview...)
	}
	return strings.Join(fit(parts, m.width), "\n")
}

func queryLine(m model) string { return "> " + m.query + "█" }

// listLines renders the visible rows, the highlighted one marked.
func listLines(m model) []string {
	rows := m.visible()
	if len(rows) == 0 {
		return []string{"  (no match)"}
	}
	cur := clamp(m.cursor, len(rows))
	h := min(maxListLines, max(1, m.height-4))
	shown, off := window(rows, cur, h)
	lines := make([]string, len(shown))
	for i, r := range shown {
		lines[i] = marker(off+i == cur) + label(r)
	}
	return lines
}

func marker(on bool) string {
	if on {
		return "▸ "
	}
	return "  "
}

// previewLines is the cached preview of the highlighted row, cut to the pane height.
func previewLines(m model) []string {
	row, ok := m.current()
	text := m.previews[row.Key]
	if !ok || text == "" {
		return nil
	}
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	return lines[:min(len(lines), maxPreviewLines)]
}

// sideBySide puts the preview in a right-hand column next to the list.
func sideBySide(list, preview []string, width int) []string {
	left := width * 2 / 5
	n := max(len(list), len(preview))
	out := make([]string, n)
	for i := range out {
		out[i] = pad(at(list, i), left) + "│ " + ansi.Truncate(at(preview, i), width-left-2, "")
	}
	return out
}

func at(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return ""
}

// pad cuts or pads s to exactly w columns.
func pad(s string, w int) string {
	s = ansi.Truncate(s, w, "")
	return s + strings.Repeat(" ", max(0, w-ansi.StringWidth(s)))
}

// fit cuts every line to width columns.
func fit(lines []string, width int) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = ansi.Truncate(l, width, "")
	}
	return out
}
