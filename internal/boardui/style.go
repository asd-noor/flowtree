package boardui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"flowtree/internal/board"
)

// accent is a status's color: dim for an idle column, bright for the focused one. Only the terminal's 16
// ANSI colors are used, so the board follows the terminal theme.
type accent struct{ dim, bright color.Color }

var accents = map[board.Status]accent{
	board.Backlog:   {lipgloss.BrightBlack, lipgloss.White},
	board.Todo:      {lipgloss.Blue, lipgloss.BrightBlue},
	board.Doing:     {lipgloss.Yellow, lipgloss.BrightYellow},
	board.Review:    {lipgloss.Magenta, lipgloss.BrightMagenta},
	board.Done:      {lipgloss.Green, lipgloss.BrightGreen},
	board.Abandoned: {lipgloss.Red, lipgloss.BrightRed},
}

func accentOf(s board.Status) accent {
	if a, ok := accents[s]; ok {
		return a
	}
	return accent{lipgloss.BrightBlack, lipgloss.White}
}

var (
	frameColor  = lipgloss.BrightBlack
	faint       = lipgloss.NewStyle().Faint(true)
	bold        = lipgloss.NewStyle().Bold(true)
	humanBadge  = lipgloss.NewStyle().Foreground(lipgloss.Cyan).Bold(true)
	noticeStyle = lipgloss.NewStyle().Foreground(lipgloss.BrightRed).Bold(true)
)

// statusText colors s in its status color.
func statusText(st board.Status, s string) string {
	return lipgloss.NewStyle().Foreground(accentOf(st).dim).Render(s)
}
