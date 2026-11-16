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
	"unicode"

	"github.com/spf13/cobra"

	"flowtree/internal/board"
	"flowtree/internal/flow"
)

func newTaskCmd(d Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "The repository's kanban: tasks for regular and hotfix branches, subtasks for their work branches",
		Long: "Tasks live in the git directory (shared by every worktree, never pushed). A task's column\n" +
			"follows from git: no branch (todo, or backlog), branch exists (doing), review asked at the\n" +
			"current tip (review), merged into its target (done), deleted unmerged (abandoned).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	cmd.AddCommand(newTaskAddCmd(d), newTaskListCmd(d), newTaskShowCmd(d), newTaskEditCmd(d),
		newTaskCommentCmd(d), newTaskFlagCmd(d, "todo"), newTaskFlagCmd(d, "backlog"), newTaskHumanCmd(d), newTaskSyncCmd(d), newTaskPromptCmd(d),
		newTaskStartCmd(d))
	cmd.AddCommand(taskBranchCmds(d)...)
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
	return board.Update(path, func(b board.Board) (board.Board, error) { return cardEdit(b, id, fn) })
}

// cardEdit returns the board with card id replaced by what fn makes of it.
func cardEdit(b board.Board, id int, fn func(board.Card) (board.Card, error)) (board.Board, error) {
	c, ok := b.Card(id)
	if !ok {
		return b, fmt.Errorf("no task #%d", id)
	}
	next, err := fn(c)
	if err != nil {
		return b, err
	}
	return b.With(id, func(board.Card) board.Card { return next })
}

// parseRef reads "<n>" or "<n>/<subtask>".
func parseRef(s string) (int, string, error) {
	n, sub, _ := strings.Cut(s, "/")
	id, err := parseID(n)
	return id, sub, err
}

func parseID(s string) (int, error) {
	id, err := strconv.Atoi(strings.TrimPrefix(s, "#"))
	if err != nil || id < 1 {
		return 0, fmt.Errorf("'%s' is not a task number", s)
	}
	return id, nil
}

func parseType(s string) (board.Type, error) {
	if t := board.Type(s); t == board.Regular || t == board.Hotfix {
		return t, nil
	}
	return "", fmt.Errorf("unknown type '%s'; one of regular, hotfix", s)
}

// parseNeeds reads "5,6", "#5, #6", "5 6" or "" (no needs).
func parseNeeds(s string) ([]int, error) {
	var ids []int
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || unicode.IsSpace(r) }) {
		id, err := parseID(f)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func splitTags(s string) []string {
	return slices.DeleteFunc(mapSlice(strings.Split(s, ","), strings.TrimSpace), func(t string) bool { return t == "" })
}

// ---- add

func newTaskAddCmd(d Deps) *cobra.Command {
	var desc, tags, typ, needs string
	var backlog, human bool
	cmd := &cobra.Command{
		Use:   "add [-d <description>] [-t <tags>] [--type regular|hotfix] [--needs <ids>] [--backlog] [--human] <title>...",
		Short: "Add a task to todo (or the backlog)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(strings.Join(args, "")) == "" {
				return errors.New("a task needs a title")
			}
			t, err := parseType(typ)
			if err != nil {
				return err
			}
			deps, err := parseNeeds(needs)
			if err != nil {
				return err
			}
			c := board.Card{Title: strings.Join(args, " "), Type: t, Description: desc, Tags: splitTags(tags),
				Backlog: backlog, Human: human, Created: time.Now().UTC()}
			return withBoard(cmd.Context(), d, func(_ *flow.Env, path string) error {
				var added board.Card
				err := board.Update(path, func(b board.Board) (board.Board, error) {
					b, added = b.Add(c)
					return b.SetNeeds(added.ID, deps)
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
	cmd.Flags().StringVar(&typ, "type", string(board.Regular), "task type: regular or hotfix")
	cmd.Flags().StringVar(&needs, "needs", "", "comma-separated ids of tasks that should be done first")
	cmd.Flags().BoolVar(&backlog, "backlog", false, "park it in the backlog; agents leave it alone")
	cmd.Flags().BoolVar(&human, "human", false, "a task for a person; agents leave it alone")
	return cmd
}

// ---- list / show

// view is a card with its derived status, as printed by list and show.
type view struct {
	board.Card
	Type     board.Type    `json:"type"` // the effective type; shadows the stored one, which may be empty
	Status   board.Status  `json:"status"`
	NeededBy []int         `json:"needed_by,omitempty"`
	Blocked  bool          `json:"blocked"`
	Subtasks []subtaskView `json:"subtasks,omitempty"`

	needs, neededBy []depRef // for the text output
}

// depRef is a task another one needs or is needed by, with its column and branch.
type depRef struct {
	ID     int
	Branch string
	Status board.Status
}

type subtaskView struct {
	board.Subtask
	Status board.Status `json:"status"`
}

// statuses is the column of every card; one pass of git queries that dependency checks then look up.
func statuses(e *flow.Env, b board.Board) map[int]board.Status {
	out := make(map[int]board.Status, len(b.Cards))
	for _, c := range b.Cards {
		out[c.ID] = flow.CardStatus(e, c)
	}
	return out
}

func lookup(st map[int]board.Status) func(int) board.Status {
	return func(id int) board.Status { return st[id] }
}

// depRefs describes the cards ids, in order.
func depRefs(b board.Board, st map[int]board.Status, ids []int) []depRef {
	return mapSlice(ids, func(id int) depRef {
		c, _ := b.Card(id)
		return depRef{ID: id, Branch: c.Branch, Status: st[id]}
	})
}

func viewOf(e *flow.Env, b board.Board, st map[int]board.Status, c board.Card) view {
	by := b.NeededBy(c.ID)
	return view{Card: c, Type: flow.TypeOf(e.S, c), Status: st[c.ID], NeededBy: by, Blocked: board.Blocked(c, lookup(st)),
		needs: depRefs(b, st, c.Needs), neededBy: depRefs(b, st, by), Subtasks: mapSlice(c.Subtasks, func(s board.Subtask) subtaskView {
			return subtaskView{Subtask: s, Status: flow.SubtaskStatus(e, s)}
		})}
}

func newTaskListCmd(d Deps) *cobra.Command {
	var status string
	var asJSON, ready bool
	cmd := &cobra.Command{
		Use:     "list [-s <status>] [--ready] [--json]",
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
				st := statuses(e, b)
				views := slices.DeleteFunc(mapSlice(b.Cards, func(c board.Card) view { return viewOf(e, b, st, c) }),
					func(v view) bool {
						return status != "" && v.Status != board.Status(status) || ready && !board.Ready(v.Card, v.Status, lookup(st))
					})
				if asJSON {
					return printJSON(d, views)
				}
				d.Printf("%s", renderTasks(views))
				return nil
			})
		},
	}
	cmd.Flags().StringVarP(&status, "status", "s", "", "only tasks in this column")
	cmd.Flags().BoolVar(&ready, "ready", false, "only todo tasks whose needs are all done")
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
	m := []string{"[" + string(v.Type) + "]"}
	if v.Branch != "" {
		m = append(m, "("+v.Branch+")")
	}
	if v.Human {
		m = append(m, "[human]")
	}
	if v.Blocked {
		m = append(m, "[blocked]")
	}
	if len(v.Tags) > 0 {
		m = append(m, "["+strings.Join(v.Tags, ",")+"]")
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
				v := viewOf(e, b, statuses(e, b), c)
				if asJSON {
					return printJSON(d, v)
				}
				d.Printf("%s", renderTask(v))
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
	b.WriteString(depsText("Needs", v.needs) + depsText("Needed by", v.neededBy))
	if len(v.Subtasks) > 0 {
		b.WriteString("\nSubtasks:\n")
		for _, s := range v.Subtasks {
			fmt.Fprintf(&b, "  %-9s %s (%s)\n", s.Status, s.Name, s.Branch)
			if s.Description != "" {
				fmt.Fprintf(&b, "            %s\n", strings.ReplaceAll(strings.TrimSpace(s.Description), "\n", "\n            "))
			}
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

// warnUnfinishedNeeds warns, never refuses, when task id needs tasks that are not done.
func warnUnfinishedNeeds(e *flow.Env, id int) {
	path, err := flow.BoardPath(e)
	if err != nil {
		return
	}
	b, err := board.Load(path)
	c, ok := b.Card(id)
	if err != nil || !ok || len(c.Needs) == 0 {
		return
	}
	st := statuses(e, b)
	if un := board.Unfinished(c, lookup(st)); len(un) > 0 {
		e.Warn("task #%d has unfinished needs: %s", id, strings.Join(mapSlice(un, func(n int) string { return fmt.Sprintf("#%d (%s)", n, st[n]) }), ", "))
	}
}

func depsText(title string, refs []depRef) string {
	if len(refs) == 0 {
		return ""
	}
	lines := mapSlice(refs, func(r depRef) string {
		return fmt.Sprintf("  #%-4d %-9s %s\n", r.ID, r.Status, orElse(r.Branch, "(no branch)"))
	})
	return "\n" + title + ":\n" + strings.Join(lines, "")
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
	var title, desc, tags, typ, needs string
	cmd := &cobra.Command{
		Use:   "edit [--title <t>] [-d <description>] [-t <tags>] [--type regular|hotfix] [--needs <ids>] <n>[/<subtask>]",
		Short: "Change a task's title, description, tags, type or needs, or a subtask's description; with no flags, open $EDITOR",
		Long: "With flags, sets those fields. Without, opens your editor with the title on the first line and\n" +
			"the description below a blank line, like a commit message (needs a terminal). <n>/<subtask>\n" +
			"edits a subtask's description (-d, or the editor). --type works only while the task has no branch.\n" +
			"--needs replaces the ids of the tasks that should be done first; an empty value clears them.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, sub, err := parseRef(args[0])
			if err != nil {
				return err
			}
			f := cmd.Flags()
			byFlags := f.Changed("title") || f.Changed("description") || f.Changed("tags") || f.Changed("type") || f.Changed("needs")
			if sub != "" {
				if f.Changed("title") || f.Changed("tags") || f.Changed("type") || f.Changed("needs") {
					return errors.New("a subtask has only a description (-d); its name is its work branch")
				}
				return withBoard(cmd.Context(), d, func(e *flow.Env, path string) error {
					return editSubtask(cmd.Context(), d, e, path, id, sub, desc, byFlags)
				})
			}
			deps, err := parseNeeds(needs)
			if err != nil {
				return err
			}
			return withBoard(cmd.Context(), d, func(e *flow.Env, path string) error {
				return board.Update(path, func(b board.Board) (board.Board, error) {
					b, err := cardEdit(b, id, func(c board.Card) (board.Card, error) {
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
						if f.Changed("type") {
							return withType(c, typ)
						}
						return c, nil
					})
					if err != nil || !f.Changed("needs") {
						return b, err
					}
					return b.SetNeeds(id, deps)
				})
			})
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "new title")
	cmd.Flags().StringVarP(&desc, "description", "d", "", "new description (Markdown)")
	cmd.Flags().StringVarP(&tags, "tags", "t", "", "new comma-separated tags (empty clears)")
	cmd.Flags().StringVar(&needs, "needs", "", "ids of the tasks that should be done first (empty clears)")
	cmd.Flags().StringVar(&typ, "type", "", "new type: regular or hotfix (only before the task has a branch)")
	return cmd
}

// withType sets the type of a task that has no branch yet; with one, the type follows the branch.
func withType(c board.Card, typ string) (board.Card, error) {
	t, err := parseType(typ)
	if err != nil {
		return c, err
	}
	if c.Branch != "" {
		return c, fmt.Errorf("task #%d already has a branch; its type follows the branch", c.ID)
	}
	c.Type = t
	return c, nil
}

// editSubtask sets a subtask's description from -d, or from the editor when no flag was given. The editor
// runs before the board is locked, so a slow edit never blocks other commands.
func editSubtask(ctx context.Context, d Deps, e *flow.Env, path string, id int, sub, desc string, byFlag bool) error {
	if !byFlag {
		cur, err := subtaskOf(path, id, sub)
		if err != nil {
			return err
		}
		text, err := runEditor(ctx, d, e, fmt.Sprintf("task-%d-%s", id, sub), cur.Description)
		if err != nil {
			return err
		}
		desc = strings.TrimSpace(text)
	}
	return board.Update(path, func(b board.Board) (board.Board, error) {
		return b.WithSubtask(id, sub, func(s board.Subtask) board.Subtask { s.Description = desc; return s })
	})
}

func subtaskOf(path string, id int, sub string) (board.Subtask, error) {
	b, err := board.Load(path)
	if err != nil {
		return board.Subtask{}, err
	}
	c, ok := b.Card(id)
	if !ok {
		return board.Subtask{}, fmt.Errorf("no task #%d", id)
	}
	i := slices.IndexFunc(c.Subtasks, func(s board.Subtask) bool { return s.Name == sub })
	if i < 0 {
		return board.Subtask{}, fmt.Errorf("task #%d has no subtask '%s'", id, sub)
	}
	return c.Subtasks[i], nil
}

// editInEditor runs the user's git editor on the card's title and description.
// ponytail: the board stays locked while the editor is open; a parallel write waits up to the lock timeout.
func editInEditor(ctx context.Context, d Deps, e *flow.Env, c board.Card) (board.Card, error) {
	text, err := runEditor(ctx, d, e, fmt.Sprintf("task-%d", c.ID), editText(c))
	if err != nil {
		return c, err
	}
	return applyEditText(c, text)
}

// runEditor lets the user edit text in their git editor and returns the result.
func runEditor(ctx context.Context, d Deps, e *flow.Env, name, text string) (string, error) {
	if !d.IsTTY() {
		return "", errors.New("no terminal for an editor; pass the fields as flags")
	}
	run, file, err := editorCmd(ctx, e, name, text)
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(file) }()
	run.Stdin, run.Stdout, run.Stderr = d.Stdin, d.Stdout, d.Stderr
	if err := run.Run(); err != nil {
		return "", fmt.Errorf("editor failed: %w", err)
	}
	data, err := os.ReadFile(file)
	return string(data), err
}

// editorCmd writes text to a temp file and returns the editor command for it, not yet started, with the
// file for the caller to read back and remove.
func editorCmd(ctx context.Context, e *flow.Env, name, text string) (*exec.Cmd, string, error) {
	editor, err := e.MainGit()("var", "GIT_EDITOR")
	if err != nil {
		return nil, "", err
	}
	file := filepath.Join(os.TempDir(), "flowtree-"+name+".md")
	if err := os.WriteFile(file, []byte(text), 0o600); err != nil {
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
		Long: "Adds tasks for regular and hotfix branches that have none, subtasks for unknown work branches,\n" +
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

// ---- prompt

func newTaskPromptCmd(d Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "prompt <n>[/<subtask>]",
		Short: "Print a task (or subtask) as a brief for whoever works on it, e.g. an agent",
		Long: "Markdown with the task's title and description, the subtask's description, the branch and its\n" +
			"worktree, and how the work is finished.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, sub, err := parseRef(args[0])
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
				p, err := promptOf(e, c, sub)
				if err != nil {
					return err
				}
				p.Needs = depRefs(b, statuses(e, b), c.Needs)
				d.Printf("%s", renderPrompt(p))
				return nil
			})
		},
	}
}

// prompt is what a brief says; renderPrompt turns it into Markdown.
type prompt struct {
	ID          int
	Title, Desc string
	Sub         *board.Subtask
	Branch, Dir string // Dir is empty when the branch has no worktree
	Kind        flow.Kind
	Name        string // the branch's name for flowtree commands
	Arg         string // what `task start` takes: <name> or, for a hotfix, <version>
	Needs       []depRef
}

func promptOf(e *flow.Env, c board.Card, sub string) (prompt, error) {
	p := prompt{ID: c.ID, Title: c.Title, Desc: c.Description, Branch: c.Branch, Arg: startArg(flow.TypeOf(e.S, c))}
	if sub != "" {
		i := slices.IndexFunc(c.Subtasks, func(s board.Subtask) bool { return s.Name == sub })
		if i < 0 {
			return p, fmt.Errorf("task #%d has no subtask '%s'", c.ID, sub)
		}
		p.Sub, p.Branch = &c.Subtasks[i], c.Subtasks[i].Branch
	}
	if p.Branch == "" {
		return p, nil
	}
	k, _ := e.S.KindOf(p.Branch)
	p.Kind, p.Name = k, e.S.Name(k, p.Branch)
	if p.Sub != nil {
		p.Name = p.Sub.Name
	}
	if w, ok, err := e.WorktreeOf(p.Branch); err != nil {
		return p, err
	} else if ok {
		p.Dir = w.Path
	}
	return p, nil
}

func renderPrompt(p prompt) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Task #%d: %s\n\n", p.ID, p.Title)
	if p.Desc != "" {
		fmt.Fprintf(&b, "%s\n\n", strings.TrimSpace(p.Desc))
	}
	if p.Sub != nil {
		fmt.Fprintf(&b, "## Subtask: %s\n\nYou work on one subtask of task #%d; others may work on its other subtasks in parallel.\n\n", p.Sub.Name, p.ID)
		if p.Sub.Description != "" {
			fmt.Fprintf(&b, "%s\n\n", strings.TrimSpace(p.Sub.Description))
		}
	}
	b.WriteString(needsText(p.Needs))
	b.WriteString("## Where and how\n\n")
	b.WriteString(whereText(p))
	return b.String()
}

// needsText lists the tasks this one needs; for one that is not done it says how to get its work in.
func needsText(needs []depRef) string {
	if len(needs) == 0 {
		return ""
	}
	return "## Needs\n\nThese tasks should be done first:\n\n" + strings.Join(mapSlice(needs, needLine), "") + "\n"
}

func needLine(r depRef) string {
	switch {
	case r.Status == board.Done:
		return fmt.Sprintf("- #%d (done)\n", r.ID)
	case r.Status == board.Abandoned:
		return fmt.Sprintf("- #%d (abandoned): its work will not come; ask whether this task still needs it.\n", r.ID)
	case r.Branch == "":
		return fmt.Sprintf("- #%d (%s): not started, no branch yet.\n", r.ID, r.Status)
	}
	return fmt.Sprintf("- #%d (%s): not in develop yet; merge its branch first with `git merge %s`.\n", r.ID, r.Status, r.Branch)
}

func whereText(p prompt) string {
	if p.Branch == "" {
		return fmt.Sprintf("- No branch yet. Start one with `flowtree task start %d %s`.\n", p.ID, p.Arg)
	}
	ref := p.Name
	if p.Kind == flow.Work {
		ref = fmt.Sprintf("%d/%s", p.ID, p.Name)
	}
	dir := fmt.Sprintf("`%s`", p.Dir)
	if p.Dir == "" {
		dir = fmt.Sprintf("none yet; create it with `flowtree %s`", checkoutCmd(p, ref))
	}
	lines := []string{
		fmt.Sprintf("- Branch: `%s`", p.Branch),
		"- Worktree: " + dir + ". Work there only.",
		fmt.Sprintf("- Report progress with `flowtree task comment %d \"...\"`.", p.ID),
	}
	if p.Kind == flow.Work {
		lines = append(lines, "- Do not merge, rebase or push. When it is done, it is finished into its parent with "+
			fmt.Sprintf("`flowtree work finish %s`.", ref))
	} else {
		lines = append(lines, fmt.Sprintf("- When it is ready, mark it with `flowtree task review %d`; it is finished with `flowtree task finish %d`.", p.ID, p.ID))
	}
	return strings.Join(lines, "\n") + "\n"
}

// checkoutCmd is the command that creates the worktree: tasks by number, work branches by "<n>/<sub>".
func checkoutCmd(p prompt, ref string) string {
	if p.Kind == flow.Work {
		return "work checkout " + ref
	}
	return fmt.Sprintf("task checkout %d", p.ID)
}
