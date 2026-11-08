package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"flowtree/internal/board"
	"flowtree/internal/flow"
)

func newTaskCmd(d Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "The repository's kanban: tasks for features and hotfixes, subtasks for their work branches",
		Long: "Tasks live in the git directory (shared by every worktree, never pushed). A task's column\n" +
			"follows from git: no branch (todo, or backlog), branch exists (doing), review asked at the\n" +
			"current tip (review), merged into its target (done), deleted unmerged (abandoned).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	cmd.AddCommand(newTaskAddCmd(d), newTaskListCmd(d), newTaskShowCmd(d), newTaskEditCmd(d),
		newTaskCommentCmd(d), newTaskFlagCmd(d, "todo"), newTaskFlagCmd(d, "backlog"), newTaskHumanCmd(d), newTaskSyncCmd(d))
	return cmd
}

// withBoard opens the flow environment and the board path.
func withBoard(ctx context.Context, d Deps, fn func(e *flow.Env, path string) error) error {
	return withFlow(ctx, d, func(e *flow.Env) error {
		path, err := flow.BoardPath(e)
		if err != nil {
			return err
		}
		return fn(e, path)
	})
}

// updateCard changes one card under the board lock.
func updateCard(path string, id int, fn func(board.Card) (board.Card, error)) error {
	return board.Update(path, func(b board.Board) (board.Board, error) {
		c, ok := b.Card(id)
		if !ok {
			return b, fmt.Errorf("no task #%d", id)
		}
		next, err := fn(c)
		if err != nil {
			return b, err
		}
		return b.With(id, func(board.Card) board.Card { return next })
	})
}

func parseID(s string) (int, error) {
	id, err := strconv.Atoi(strings.TrimPrefix(s, "#"))
	if err != nil || id < 1 {
		return 0, fmt.Errorf("'%s' is not a task number", s)
	}
	return id, nil
}

func splitTags(s string) []string {
	return slices.DeleteFunc(mapSlice(strings.Split(s, ","), strings.TrimSpace), func(t string) bool { return t == "" })
}

// ---- add

func newTaskAddCmd(d Deps) *cobra.Command {
	var desc, tags string
	var backlog, human bool
	cmd := &cobra.Command{
		Use:   "add [-d <description>] [-t <tags>] [--backlog] [--human] <title>...",
		Short: "Add a task to todo (or the backlog)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(strings.Join(args, "")) == "" {
				return errors.New("a task needs a title")
			}
			c := board.Card{Title: strings.Join(args, " "), Description: desc, Tags: splitTags(tags),
				Backlog: backlog, Human: human, Created: time.Now().UTC()}
			return withBoard(cmd.Context(), d, func(_ *flow.Env, path string) error {
				var added board.Card
				err := board.Update(path, func(b board.Board) (board.Board, error) {
					b, added = b.Add(c)
					return b, nil
				})
				if err == nil {
					d.Printf("#%d\n", added.ID)
				}
				return err
			})
		},
	}
	cmd.Flags().StringVarP(&desc, "description", "d", "", "description (Markdown)")
	cmd.Flags().StringVarP(&tags, "tags", "t", "", "comma-separated tags")
	cmd.Flags().BoolVar(&backlog, "backlog", false, "park it in the backlog; agents leave it alone")
	cmd.Flags().BoolVar(&human, "human", false, "a task for a person; agents leave it alone")
	return cmd
}

// ---- list / show

// view is a card with its derived status, as printed by list and show.
type view struct {
	board.Card
	Status   board.Status  `json:"status"`
	Subtasks []subtaskView `json:"subtasks,omitempty"`
}

type subtaskView struct {
	board.Subtask
	Status board.Status `json:"status"`
}

func viewOf(e *flow.Env, c board.Card) view {
	return view{Card: c, Status: flow.CardStatus(e, c), Subtasks: mapSlice(c.Subtasks, func(s board.Subtask) subtaskView {
		return subtaskView{Subtask: s, Status: flow.SubtaskStatus(e, s)}
	})}
}

func newTaskListCmd(d Deps) *cobra.Command {
	var status string
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "list [-s <status>] [--json]",
		Aliases: []string{"ls"},
		Short:   "List tasks with their columns",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if status != "" && !slices.Contains(board.Columns, board.Status(status)) {
				return fmt.Errorf("unknown status '%s'; one of %v", status, board.Columns)
			}
			return withBoard(cmd.Context(), d, func(e *flow.Env, path string) error {
				b, err := board.Load(path)
				if err != nil {
					return err
				}
				views := slices.DeleteFunc(mapSlice(b.Cards, func(c board.Card) view { return viewOf(e, c) }),
					func(v view) bool { return status != "" && v.Status != board.Status(status) })
				if asJSON {
					return printJSON(d, views)
				}
				d.Printf("%s", renderTasks(views))
				return nil
			})
		},
	}
	cmd.Flags().StringVarP(&status, "status", "s", "", "only tasks in this column")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}

// renderTasks prints one line per task: id, column, branch, title and markers.
func renderTasks(views []view) string {
	var b strings.Builder
	for _, v := range views {
		fmt.Fprintf(&b, "#%-4d %-9s %s%s\n", v.ID, v.Status, v.Title, markers(v))
	}
	return b.String()
}

func markers(v view) string {
	var m []string
	if v.Branch != "" {
		m = append(m, "("+v.Branch+")")
	}
	if v.Human {
		m = append(m, "[human]")
	}
	if len(v.Tags) > 0 {
		m = append(m, "["+strings.Join(v.Tags, ",")+"]")
	}
	if len(m) == 0 {
		return ""
	}
	return "  " + strings.Join(m, " ")
}

func newTaskShowCmd(d Deps) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "show [--json] <n>",
		Short: "Show a task: description, subtasks and comments",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			return withBoard(cmd.Context(), d, func(e *flow.Env, path string) error {
				b, err := board.Load(path)
				if err != nil {
					return err
				}
				c, ok := b.Card(id)
				if !ok {
					return fmt.Errorf("no task #%d", id)
				}
				if asJSON {
					return printJSON(d, viewOf(e, c))
				}
				d.Printf("%s", renderTask(viewOf(e, c)))
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}

func renderTask(v view) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#%d %s\n%s%s\n", v.ID, v.Title, v.Status, markers(v))
	if v.Description != "" {
		fmt.Fprintf(&b, "\n%s\n", strings.TrimRight(v.Description, "\n"))
	}
	if len(v.Subtasks) > 0 {
		b.WriteString("\nSubtasks:\n")
		for _, s := range v.Subtasks {
			fmt.Fprintf(&b, "  %-9s %s (%s)\n", s.Status, s.Name, s.Branch)
		}
	}
	if len(v.Comments) > 0 {
		b.WriteString("\nComments:\n")
		for _, c := range v.Comments {
			fmt.Fprintf(&b, "  %s, %s:\n    %s\n", c.Author, c.At.Format("2006-01-02 15:04"), strings.ReplaceAll(c.Body, "\n", "\n    "))
		}
	}
	return b.String()
}

func printJSON(d Deps, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err == nil {
		d.Printf("%s\n", data)
	}
	return err
}

// ---- edit

func newTaskEditCmd(d Deps) *cobra.Command {
	var title, desc, tags string
	cmd := &cobra.Command{
		Use:   "edit [--title <t>] [-d <description>] [-t <tags>] <n>",
		Short: "Change a task's title, description or tags; with no flags, open $EDITOR",
		Long: "With flags, sets those fields. Without, opens your editor with the title on the first line and\n" +
			"the description below a blank line, like a commit message (needs a terminal).",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			f := cmd.Flags()
			byFlags := f.Changed("title") || f.Changed("description") || f.Changed("tags")
			return withBoard(cmd.Context(), d, func(e *flow.Env, path string) error {
				return updateCard(path, id, func(c board.Card) (board.Card, error) {
					if !byFlags {
						return editInEditor(cmd.Context(), d, e, c)
					}
					if f.Changed("title") {
						if strings.TrimSpace(title) == "" {
							return c, errors.New("empty title; task not changed")
						}
						c.Title = strings.TrimSpace(title)
					}
					if f.Changed("description") {
						c.Description = desc
					}
					if f.Changed("tags") {
						c.Tags = splitTags(tags)
					}
					return c, nil
				})
			})
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "new title")
	cmd.Flags().StringVarP(&desc, "description", "d", "", "new description (Markdown)")
	cmd.Flags().StringVarP(&tags, "tags", "t", "", "new comma-separated tags (empty clears)")
	return cmd
}

// editInEditor runs the user's git editor on the card's title and description.
// ponytail: the board stays locked while the editor is open; a parallel write waits up to the lock timeout.
func editInEditor(ctx context.Context, d Deps, e *flow.Env, c board.Card) (board.Card, error) {
	if !d.IsTTY() {
		return c, errors.New("no terminal for an editor; pass --title, -d or -t")
	}
	run, file, err := editorCmd(ctx, e, c)
	if err != nil {
		return c, err
	}
	defer func() { _ = os.Remove(file) }()
	run.Stdin, run.Stdout, run.Stderr = d.Stdin, d.Stdout, d.Stderr
	if err := run.Run(); err != nil {
		return c, fmt.Errorf("editor failed: %w", err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return c, err
	}
	return applyEditText(c, string(data))
}

// editorCmd writes the card's text to a temp file and returns the editor command for it, not yet started,
// with the file for the caller to read back and remove.
func editorCmd(ctx context.Context, e *flow.Env, c board.Card) (*exec.Cmd, string, error) {
	editor, err := e.MainGit()("var", "GIT_EDITOR")
	if err != nil {
		return nil, "", err
	}
	file := filepath.Join(os.TempDir(), fmt.Sprintf("flowtree-task-%d.md", c.ID))
	if err := os.WriteFile(file, []byte(editText(c)), 0o600); err != nil {
		return nil, "", err
	}
	return exec.CommandContext(ctx, "sh", "-c", editor+` "$@"`, editor, file), file, nil
}

func editText(c board.Card) string { return c.Title + "\n\n" + c.Description }

// applyEditText reads the edited text back: the first line is the title, the rest the description.
func applyEditText(c board.Card, text string) (board.Card, error) {
	title, desc, _ := strings.Cut(text, "\n")
	if strings.TrimSpace(title) == "" {
		return c, errors.New("empty title; task not changed")
	}
	c.Title, c.Description = strings.TrimSpace(title), strings.TrimSpace(desc)
	return c, nil
}

// ---- comment / flags

func newTaskCommentCmd(d Deps) *cobra.Command {
	var author string
	cmd := &cobra.Command{
		Use:   "comment [--author <name>] <n> <text>...",
		Short: "Add a comment to a task",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			return withBoard(cmd.Context(), d, func(e *flow.Env, path string) error {
				who := author
				if who == "" {
					who, _ = e.MainGit()("config", "user.name")
				}
				note := board.Comment{Author: orElse(who, "unknown"), Body: strings.Join(args[1:], " "), At: time.Now().UTC()}
				return updateCard(path, id, func(c board.Card) (board.Card, error) {
					c.Comments = append(slices.Clone(c.Comments), note)
					return c, nil
				})
			})
		},
	}
	cmd.Flags().StringVar(&author, "author", "", "comment author (default: git user.name)")
	return cmd
}

func orElse(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// newTaskFlagCmd builds `task todo` and `task backlog`, which move a task without a branch.
func newTaskFlagCmd(d Deps, to string) *cobra.Command {
	return &cobra.Command{
		Use:   to + " <n>",
		Short: fmt.Sprintf("Move a task that has no branch yet to %s", to),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			return withBoard(cmd.Context(), d, func(_ *flow.Env, path string) error {
				return updateCard(path, id, func(c board.Card) (board.Card, error) {
					if c.Branch != "" {
						return c, fmt.Errorf("task #%d has a branch (%s); its column follows git", id, c.Branch)
					}
					c.Backlog = to == "backlog"
					return c, nil
				})
			})
		},
	}
}

func newTaskHumanCmd(d Deps) *cobra.Command {
	var off bool
	cmd := &cobra.Command{
		Use:   "human [--off] <n>",
		Short: "Mark a task as one for a person (agents leave it alone); --off clears it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			return withBoard(cmd.Context(), d, func(_ *flow.Env, path string) error {
				return updateCard(path, id, func(c board.Card) (board.Card, error) { c.Human = !off; return c, nil })
			})
		},
	}
	cmd.Flags().BoolVar(&off, "off", false, "clear the mark")
	return cmd
}

func newTaskSyncCmd(d Deps) *cobra.Command {
	var dry bool
	cmd := &cobra.Command{
		Use:   "sync [-n]",
		Short: "Bring the board in line with branches made or renamed with plain git",
		Long: "Adds tasks for feature and hotfix branches that have none, subtasks for unknown work branches,\n" +
			"relinks a task whose branch was renamed (same tip), and records every branch's tip so a later\n" +
			"merge-and-delete still reads as done. -n prints the changes without writing them.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withFlow(cmd.Context(), d, func(e *flow.Env) error {
				changes, err := flow.Sync(e, dry)
				if err != nil {
					return err
				}
				switch {
				case len(changes) == 0:
					d.Printf("The board is in line with the branches.\n")
				case dry:
					d.Printf("Dry run; would have:\n- %s\n", strings.Join(changes, "\n- "))
				default:
					d.Printf("- %s\n", strings.Join(changes, "\n- "))
				}
				return nil
			})
		},
	}
	cmd.Flags().BoolVarP(&dry, "dry-run", "n", false, "print the changes without writing them")
	return cmd
}
