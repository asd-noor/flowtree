// Package board is the per-repository kanban: cards for features and hotfixes, subtasks for their work
// branches, and comments. It knows branch names but never runs git; status comes from facts the caller
// gathers (see Status).
package board

import (
	"fmt"
	"slices"
	"time"
)

// Board is the whole board of a repository.
type Board struct {
	NextID int    `json:"next_id"`
	Cards  []Card `json:"cards"`
}

// Card is a task: a feature or hotfix, before or after it has a branch.
type Card struct {
	ID          int       `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"` // Markdown
	Tags        []string  `json:"tags,omitempty"`
	Human       bool      `json:"human,omitempty"`   // for a person; agents leave it alone
	Backlog     bool      `json:"backlog,omitempty"` // parked; agents leave it alone until it moves to todo
	Branch      string    `json:"branch,omitempty"`
	Start       string    `json:"start,omitempty"`      // commit the branch started from
	Tip         string    `json:"tip,omitempty"`        // last known tip, so a merged and deleted branch reads as done
	ReviewTip   string    `json:"review_tip,omitempty"` // tip when review was asked for
	Subtasks    []Subtask `json:"subtasks,omitempty"`
	Comments    []Comment `json:"comments,omitempty"`
	Created     time.Time `json:"created"`
}

// Subtask is a work branch of the card's branch.
type Subtask struct {
	Name   string `json:"name"`
	Branch string `json:"branch"`
	Start  string `json:"start,omitempty"`
	Tip    string `json:"tip,omitempty"`
}

// Comment is a note on a card.
type Comment struct {
	Author string    `json:"author"`
	Body   string    `json:"body"`
	At     time.Time `json:"at"`
}

// Empty is a board with no cards.
func Empty() Board { return Board{NextID: 1} }

// Add appends c with the next id and returns the new board and the card as stored.
func (b Board) Add(c Card) (Board, Card) {
	c.ID = max(b.NextID, 1)
	return Board{NextID: c.ID + 1, Cards: append(slices.Clone(b.Cards), c)}, c
}

// Card finds a card by id.
func (b Board) Card(id int) (Card, bool) {
	i := slices.IndexFunc(b.Cards, func(c Card) bool { return c.ID == id })
	if i < 0 {
		return Card{}, false
	}
	return b.Cards[i], true
}

// ByBranch finds the card linked to branch.
func (b Board) ByBranch(branch string) (Card, bool) {
	i := slices.IndexFunc(b.Cards, func(c Card) bool { return c.Branch == branch })
	if i < 0 {
		return Card{}, false
	}
	return b.Cards[i], true
}

// With returns a board where card id is replaced by fn(card).
func (b Board) With(id int, fn func(Card) Card) (Board, error) {
	i := slices.IndexFunc(b.Cards, func(c Card) bool { return c.ID == id })
	if i < 0 {
		return b, fmt.Errorf("no task #%d", id)
	}
	cards := slices.Clone(b.Cards)
	cards[i] = fn(cards[i])
	return Board{NextID: b.NextID, Cards: cards}, nil
}
