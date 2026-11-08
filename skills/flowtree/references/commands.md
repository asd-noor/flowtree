# flowtree command reference

`<kind>` is `feature`, `release` or `hotfix`. `<name>` is the branch name without its prefix; for `release` and
`hotfix` it is the version. With no name, `finish`, `publish`, `diff`, `rebase` and `checkout` use the branch of
the worktree you are in. `feature` names also accept a unique prefix. Worktree names (for `print-dir`, `exec`,
`remove`) can be `@` (the main worktree), a branch, a directory name, or an absolute path.

## Setup

| Command | Effect |
|---------|--------|
| `init [--flow] [-d] [-f] [-w <dir>]` | Store settings in the repository's git config (nothing is written to the working tree). `-d` = no prompts (use this). `--flow` also sets up git-flow: creates `master`/`develop` if missing and stores the flow settings. `-w` sets the worktree directory (stored only when it differs from the default). Existing settings are left alone (exit 0) except that `--flow` adds missing flow settings; `-f` resets them first. |
| `config [<key> [<value>]] [--global] [--unset]` | List settings with their origin, print one, set one (this repo, or `--global`) or remove one. Keys: `baseDir`, `remote`, `master`, `develop`, `featurePrefix`, `releasePrefix`, `hotfixPrefix`, `workPrefix`, `versionTagPrefix`. Unknown keys are rejected. |
| `config hooks` | Print the path of the post-create hooks file and its contents (or an example when missing). |
| `init --flow` options | `--master <b>`, `--develop <b>`, `-p <feature prefix>`, `-r <release prefix>`, `-x <hotfix prefix>`, `-t <version tag prefix>`, `--remote <name>`. |
| `shell-init [bash\|zsh\|fish]` | Prints the shell function that follows worktrees with `cd`, plus completion. For humans; not useful to agents. |

## Worktrees

| Command | Effect |
|---------|--------|
| `add <branch>` / `add -b <new> [<commitish>]` | Create a worktree at `<baseDir>/<branch>` (existing branch, remote-only branch tracked, or a new one). Hooks run. `-q` prints only the path; `--exec <cmd>` runs a command in it afterwards. |
| `list [-q]` | Worktrees with path, branch and HEAD; `-q` prints names only. |
| `remove [-f] [--with-branch [--force-branch]] <worktree>...` | Remove worktrees. Never the main one, nor the one you are in (unless shell integration is active). `--with-branch` also deletes the branch if merged. |
| `print-dir [<worktree>]` | Print the absolute path of an existing worktree; no name or `@` = main. Read-only. |
| `exec <worktree> -- <command> [args...]` | Run a command in a worktree; exits with its exit code. |
| `switch <query>` | Print the directory of a branch, moving the main checkout onto master/develop or creating a missing feature/release/hotfix worktree if needed. A query must match one row exactly or by substring/fuzzy uniquely; otherwise it opens a picker (fails without a terminal). |
| `relocate [-n]` | Move feature/release/hotfix worktrees to `<baseDir>/<branch>` after `baseDir` changed. `-n` = dry run. Skips locked/missing/occupied; refuses if cwd is inside one that would move. |

## Common to all kinds

| Command | Effect |
|---------|--------|
| `<kind> list [-v]` | List branches of this kind; `*` marks the current one; `-v` adds state (no commits yet / based on latest / may be rebased / merged). |
| `<kind> start [-F] [--dry-run] <name> [<base>]` | New branch + worktree. Base defaults to `develop` (`feature`, `release`) or `master` (`hotfix`). `-F` fetches first. |
| `<kind> checkout [<name>]` | Print the worktree path, creating the worktree if the branch has none. |
| `<kind> publish [<name>]` | Push the branch to the remote and set the upstream. |
| `<kind> track <name>` | Check out a branch that exists on the remote (in a new worktree). |
| `<kind> delete [-f] [-r] [--dry-run] <name>` | Remove worktree and local branch. Refuses if unmerged unless `-f`. `-r` also deletes the remote branch. |

## feature

| Command | Effect |
|---------|--------|
| `feature finish [-k] [--keeplocal] [--keepremote] [-D] [--push] [--dry-run] [<name>]` | Fetch, merge into `develop` with `--no-ff` (never rebases), clean up. Only cleans up when a pull request already merged it. |
| `feature diff [<name>]` | Changes on the branch since it left `develop`. |
| `feature rebase [-p] [<name>]` | Rebase onto `develop` inside the worktree (`-i` needs a terminal; do not use it). |
| `feature pull <remote> [<name>]` | Fast-forward pull of the branch from `<remote>`, creating the branch and worktree if missing. |

Flags: `-k` keep local and remote branch (the worktree is still removed); `--keeplocal`/`--keepremote` keep only
that copy; `-D` force worktree removal even with untracked files; `--push` push `develop` afterwards.

## work

| Command | Effect |
|---------|--------|
| `work start [-P <parent>] [--dry-run] <name>` | New `work/<parent>/<name>` off the parent's tip, with a worktree. Without `-P`: the feature/hotfix whose worktree you are in; else a picker (fails without a terminal). Names must not contain `/`. |
| `work finish [-k] [--keeplocal] [-D] [--dry-run] [<name>]` | Rebase onto the parent, fast-forward the parent (its worktree must be clean), remove the work worktree and branch. |

Work branches are local: no `publish`, `track` or `pull`. A bare name means the subtask of the parent you are in,
else the only one of that name; otherwise give `feature/login/api`. `feature`/`hotfix` `finish` and `delete`
refuse while subtasks are open.

## release / hotfix

`<kind> finish [-F] [-s] [-u <key>] [-m <msg> | -f <file>] [-p] [-k] [--keeplocal] [--keepremote] [-D] [-n] [-b] [-S] [-T <tagname>] [--ff-master] [--dry-run] [<version>]`

- `-m`/`-f` tag message (pass one); `-s`/`-u` sign the tag; `-n` no tag; `-T` custom tag name.
- `-b` skip the back-merge into `develop`; `-S` squash into master; `--ff-master` (release only) fast-forward master.
- `-p` pushes `develop`, `master` and the tag to the remote.
- Order: merge into `master` (`--no-ff`) → tag → merge the tag into `develop` → remove worktree/branch → push.
- Re-running `finish` after a failed back-merge resumes (see troubleshooting).
- `release start`/`hotfix start` fail if another branch of that kind is open or the tag already exists.

## task (board)

| Command | Effect |
|---------|--------|
| `task add [-d <text>] [-t <tags>] [--backlog] [--human] <title>...` | New task in todo (or the backlog); prints `#<n>`. |
| `task list [-s <status>] [--json]` | Tasks with their columns: backlog, todo, doing, review, done, abandoned. |
| `task show [--json] <n>` | Description, subtasks and comments of a task. |
| `task edit [--title <t>] [-d <text>] [-t <tags>] <n>` | Change fields; with no flags it opens `$EDITOR` (title line, blank line, description). |
| `task comment [--author <name>] <n> <text>...` | Add a comment (author defaults to git `user.name`). |
| `task todo <n>`, `task backlog <n>` | Move a task that has no branch between todo and the backlog. |
| `task human [--off] <n>` | Mark a task as one for a person; `--off` clears it. |
| `board` | Interactive terminal kanban, for humans only. Agents must not run it (it needs a terminal and blocks); use `task list` / `task show`. |

The flow commands keep the board up to date (a board failure only prints a warning; git is never rolled back):

| Command | Board |
|---------|-------|
| `feature`/`hotfix start -T <n> <name>` | links task #n to the new branch (it must exist and have no branch) |
| `feature`/`hotfix start <name>` | adds a task named after the branch |
| `work start` | adds a subtask to the parent's task |
| `feature`/`hotfix review [--undo] [<name>]` | marks the task ready for review at the current tip; a new commit moves it back to doing; `--undo` clears it |
| `feature rebase` | moves a review mark that was at the old tip to the new one |
| `finish`, `delete` | record the branch's last tip, so the task reads as done (merged) or abandoned (not) once the branch is gone |

Releases have no task. `--dry-run` shows the board steps as `board: ...`.
