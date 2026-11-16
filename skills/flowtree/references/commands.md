# flowtree command reference

Regular and hotfix branches are driven by `task` commands, releases by `release`, subtasks by `work`. `<n>` is a
task number (`12` or `#12`); `<version>` is a release's name without its prefix. `release finish` and `checkout`
with no version use the release of the worktree you are in. Worktree names (for `print-dir`, `exec`,
`remove`) can be `@` (the main worktree), a branch, a directory name, or an absolute path.

## Setup

| Command | Effect |
|---------|--------|
| `init [--flow] [-d] [-f] [-w <dir>]` | Store settings in the repository's git config (nothing is written to the working tree). `-d` = no prompts (use this). `--flow` also sets up git-flow: creates `master`/`develop` if missing and stores the flow settings. `-w` sets the worktree directory (stored only when it differs from the default). Existing settings are left alone (exit 0) except that `--flow` adds missing flow settings; `-f` resets them first. |
| `config [<key> [<value>]] [--global] [--unset]` | List settings with their origin, print one, set one (this repo, or `--global`) or remove one. Keys: `baseDir`, `remote`, `master`, `develop`, `regularPrefix`, `releasePrefix`, `hotfixPrefix`, `workPrefix`. Unknown keys are rejected. |
| `config hooks` | Print the path of the post-create hooks file and its contents (or an example when missing). |
| `init --flow` options | `--master <b>`, `--develop <b>`, `-p <regular prefix>`, `-r <release prefix>`, `-x <hotfix prefix>`, `--remote <name>`. |
| `shell-init [bash\|zsh\|fish]` | Prints the shell function that follows worktrees with `cd`, plus completion. For humans; not useful to agents. |

## Worktrees

| Command | Effect |
|---------|--------|
| `add <branch>` / `add -b <new> [<commitish>]` | Create a worktree at `<baseDir>/<branch>` (existing branch, remote-only branch tracked, or a new one). Hooks run. `-q` prints only the path; `--exec <cmd>` runs a command in it afterwards. |
| `list [-q]` | Worktrees with path, branch and HEAD; `-q` prints names only. |
| `remove [-f] [--with-branch [--force-branch]] <worktree>...` | Remove worktrees. Never the main one, nor the one you are in (unless shell integration is active). `--with-branch` also deletes the branch if merged. |
| `print-dir [<worktree>]` | Print the absolute path of an existing worktree; no name or `@` = main. Read-only. |
| `exec <worktree> -- <command> [args...]` | Run a command in a worktree; exits with its exit code. |
| `switch <query>` | Print the directory of a branch, moving the main checkout onto master/develop or creating a missing regular/release/hotfix worktree if needed. A query must match one row exactly or by substring/fuzzy uniquely; otherwise it opens a picker (fails without a terminal). |
| `relocate [-n]` | Move regular/release/hotfix worktrees to `<baseDir>/<branch>` after `baseDir` changed. `-n` = dry run. Skips locked/missing/occupied; refuses if cwd is inside one that would move. |

## release

| Command | Effect |
|---------|--------|
| `release list [-v]` | List release branches; `*` marks the current one; `-v` adds state (no commits yet / based on latest / may be rebased / merged). |
| `release start [-F] [--dry-run] <version> [<base>]` | New branch + worktree, base defaults to `develop`. `-F` fetches first. Creates no task. |
| `release checkout [<version>]` | Print the worktree path, creating the worktree if the branch has none. |
| `release publish [<version>]` | Push the branch to the remote and set the upstream. |
| `release track <version>` | Check out a branch that exists on the remote (in a new worktree). |
| `release delete [-f] [-r] [--dry-run] <version>` | Remove worktree and local branch. Refuses if unmerged unless `-f`. `-r` also deletes the remote branch. |

`release finish [-F] [-s] [-u <key>] [-m <msg> | -f <file>] [-p] [-k] [--keeplocal] [--keepremote] [-D] [-n] [-b] [-S] [-T <tagname>] [--ff-master] [--dry-run] [<version>]`

- `-m`/`-f` tag message (pass one); `-s`/`-u` sign the tag; `-n` no tag; `-T` custom tag name.
- `-b` skip the back-merge into `develop`; `-S` squash into master; `--ff-master` fast-forward master.
- `-p` pushes `develop`, `master` and the tag to the remote.
- `-k` keep local and remote branch (the worktree is still removed); `--keeplocal`/`--keepremote` keep only that
  copy; `-D` force worktree removal even with untracked files.
- Order: merge into `master` (`--no-ff`) → tag → merge the tag into `develop` → remove worktree/branch → push.
- Re-running `finish` after a failed back-merge resumes (see troubleshooting).
- `release start` fails if a release or hotfix is open or the tag already exists.

## work

| Command | Effect |
|---------|--------|
| `work start [-P <parent>] [-d <brief>] [--dry-run] <name>` | New `work/<parent>/<name>` off the parent's tip, with a worktree; `-d` is its brief on the board. Without `-P`: the regular/hotfix whose worktree you are in; else a picker (fails without a terminal). Names must not contain `/`. |
| `work finish [-k] [--keeplocal] [-D] [--dry-run] <n>/<sub>` | Rebase onto the parent, fast-forward the parent (its worktree must be clean), remove the work worktree and branch. |
| `work checkout <n>/<sub>`, `work delete [-f] [--dry-run] <n>/<sub>` | Print/create the worktree; remove it and the branch without merging. |

`-P` is the number of the task whose branch is the parent. Work branches are local: no `publish`, `track` or `pull`.
`finish`, `checkout` and `delete` need `<n>/<sub>` (e.g. `12/api`). `task finish` and `task delete` refuse while
subtasks are open.

## task (board)

| Command | Effect |
|---------|--------|
| `task add [-d <text>] [--type regular\|hotfix] [--needs <ids>] [--backlog] [--human] <title>...` | New task in todo (or the backlog); prints `#<n>`. `--type` defaults to regular. `--needs 5,6` lists tasks to finish first. |
| `task list [-s <status>] [--ready] [--json]` | Tasks with their columns: backlog, todo, doing, review, done, abandoned; `[blocked]` while a needed task is not done. `--ready`: todo tasks whose needs are all done. JSON has `needs`, `needed_by`, `blocked`. |
| `task show [--json] <n>` | Description, needs and needed by, subtasks and comments of a task. |
| `task edit [--title <t>] [-d <text>] [--type regular\|hotfix] [--needs <ids>] <n>[/<sub>]` | Change fields (`--type` only while the task has no branch; `--needs` replaces the list, `""` clears it; unknown id, self and cycles are refused); with no flags it opens `$EDITOR` (title line, blank line, description). `<n>/<sub>` edits a subtask's description. |
| `task prompt <n>[/<sub>]` | Print the task (or subtask) as a Markdown brief: descriptions, branch, absolute worktree path, how it is finished, and needed tasks (merge the branch of one not done yet with `git merge <branch>`). Hand it to whoever works on it. |
| `task comment [--author <name>] <n> <text>...` | Add a comment (author defaults to git `user.name`). |
| `task todo <n>`, `task backlog <n>` | Move a task that has no branch between todo and the backlog. |
| `task human [--off] <n>` | Mark a task as one for a person; `--off` clears it. |
| `task sync [-n]` | Catch up with plain git: add tasks (typed by the prefix) for regular/hotfix branches without one and subtasks for unknown work branches, relink a task whose branch was renamed (same tip), record every tip. `-n` only prints the changes. |
| `task start [-F] [--dry-run] <n> <name\|version> [<base>]` | Start the branch of an existing task by its type: `regular/<name>` from `develop`, or `hotfix/<version>` from `master` (version required). Worktree, hooks, links the task. Refused when the task has a branch or the branch/tag exists. Creates no task. |
| `task review [--undo] [<n>]` | Mark the task ready for review at the current tip; a new commit moves it back to doing; `--undo` clears it. The branch needs a task (`task sync` adds one). |
| `task diff [<n>]` | Changes on the branch since it left its base. |
| `task publish [<n>]` | Push the branch to the remote and set the upstream. |
| `task rebase [-p] [<n>]` | Regular tasks: rebase onto `develop` inside the worktree (`-i` needs a terminal; do not use it). |
| `task pull [<n>]` | Regular tasks: fast-forward the branch from the remote. |
| `task checkout <n>` | Print the worktree path, creating the worktree if the branch has none. |
| `task finish [flags] <n>` | Regular: fetch, merge into `develop` with `--no-ff` (never rebases), clean up; only cleans up when a pull request already merged it. Hotfix: like `release finish` (flags `-F -s -u -m -f -n -b -S -T/--tagname -p`). Common: `-k`, `--keeplocal`, `--keepremote`, `-D`, `--push` (regular), `--dry-run`. A flag the type does not take is an error. |
| `task delete [-f] [-r] [--dry-run] <n>` | Remove worktree and local branch. Refuses if unmerged unless `-f`. `-r` also deletes the remote branch. |
| `task track <n> <remote branch>` | Check out a remote `regular/...` or `hotfix/...` branch and link it to task `<n>` (no branch yet, same type). |
| `board` | Interactive terminal kanban, for humans only. Agents must not run it (it needs a terminal and blocks); use `task list` / `task show`. |

The flow commands keep the board up to date (a board failure only prints a warning; git is never rolled back):

| Command | Board |
|---------|-------|
| `task start <n> <name\|version>` | links task #n to the new branch (it must exist and have no branch) |
| `work start` | adds a subtask to the parent's task |
| `task review [--undo] [<n>]` | marks the task ready for review at the current tip; a new commit moves it back to doing; `--undo` clears it |
| `task rebase` | moves a review mark that was at the old tip to the new one |
| `task finish`, `task delete`, `release finish` | record the branch's last tip, so the task reads as done (merged) or abandoned (not) once the branch is gone |

Releases have no task, and starting, reviewing and working never create one (`task sync` does). `--dry-run` shows the board steps as `board: ...`.
