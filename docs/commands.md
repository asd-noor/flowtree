# Command reference

`flowtree <command> --help` is authoritative for flags; this page explains what the commands do. Names given
to commands that take a worktree can be `@` (the main worktree), a branch, a directory name, the display name
shown by `list`, or an absolute path.

## Setup

### `init`

```
flowtree init [--flow] [-w DIR] [-f] [-d]
              [--master B] [--develop B] [-p work/] [-r release/] [-x hotfix/] [-t TAGPREFIX] [--remote R]
```

Stores flowtree's settings in the repository's git config (`.git/config`, section `flowtree`; see
[configuration.md](configuration.md)). Nothing is written to the working tree. It works from a linked worktree too.

- Plain `init` only sets the base directory: `-w`, or the answer to the prompt on a terminal. It is stored only
  when it differs from the default (`$FLOWTREE_PARENT/<repo name>` when that variable is defined, else
  `../worktrees`), so an unset base directory keeps following the environment. With nothing to store it says so.
  A leading `~` or `$HOME` is expanded before storing.
- If the repository already has local flowtree settings, `init` prints a note and changes nothing (exit 0).
  `-f` removes the repository's `flowtree` section first, then stores the new settings (it also repairs a broken
  configuration). `--flow` on a repository configured without flow adds the flow settings and keeps the rest.
- `--flow` also sets up git-flow: it asks for (or takes flags for) the branch names, prefixes, version tag
  prefix and remote, stores `master`, `develop`, the three prefixes and `versionTagPrefix` (and `remote` when it is
  not `origin`), creates `master` and `develop` if missing (the production branch defaults to an existing
  `master` or `main`), and puts the main worktree on develop when it was on master and is clean. It needs at
  least one commit and fails before storing anything if the settings are invalid.
- `-d` uses defaults without prompting.

### `config`

```
flowtree config                          # list every setting with its origin
flowtree config <key>                    # print the effective value
flowtree config <key> <value> [--global] # set it (this repository, or every repository)
flowtree config --unset <key> [--global]
flowtree config hooks                    # path and contents of the post-create hooks file
```

Keys: `baseDir`, `remote`, `master`, `develop`, `workPrefix`, `releasePrefix`, `hotfixPrefix`,
`versionTagPrefix` (any case, optional `flowtree.` prefix). The listing marks each value `(local)`, `(global)`,
`(system)` or `(default)`. Values are validated before they are written and unknown keys are rejected. Keys
complete with Tab. `config hooks` prints where `flowtree-hooks.json` lives, and an example when the file does not
exist yet.

### `shell-init [bash|zsh|fish]`, `completion <shell>`

`shell-init` prints the `cd`-following wrapper plus the completions for one shell (`eval "$(flowtree shell-init)"`);
`completion bash|zsh|fish|powershell` prints only the completion script, to install from a file. Both are described,
with setup steps and troubleshooting, in [shell-integration.md](shell-integration.md).

### `install-skill <dir>`

```
flowtree install-skill <dir> [-n] [-f] [--uninstall]
```

Installs the agent skill that is embedded in the binary (`skills/flowtree`: `SKILL.md` plus `references/`) as
`<dir>/flowtree`, for example `~/.claude/skills` or `~/.pi/agent/skills`. The skill teaches an agent the
branch model, to work inside worktrees, to stay non-interactive (no picker, `-m` on release and hotfix
finish) and to leave pushing to you. It works for any agent that reads [Agent Skills](https://agentskills.io).

An existing `<dir>/flowtree` that is a flowtree skill is replaced (stale files are removed); any other
directory there is left alone with an error unless `-f` is given. `-n` shows what would be written, and
`--uninstall` removes it again. A leading `~` is expanded.

## Worktrees

### `add`

```
flowtree add <branch> [-q] [--exec CMD]
flowtree add -b <new-branch> [<commitish>] [-q] [--exec CMD]
```

Creates a worktree at `<baseDir>/<branch>`. An existing local branch is checked out; a branch that exists on
exactly one remote is tracked (several remotes: it asks you to create the local branch first). With `-b` a new
branch is created from the commitish (default HEAD). Post-create hooks run (a failing hook is a warning), then
`--exec` runs in the new worktree. `-q` prints only the absolute path on stdout. It checks the common failures
first (branch exists, branch already checked out elsewhere, path exists) and gives hints.

### `list` (alias `ls`), `-q`

Shows PATH, BRANCH and HEAD for every worktree; `@ (main worktree)` is the main one and `*` marks the one you
are in. `-q` prints only the worktree names, one per line.

### `remove` (alias `rm`)

```
flowtree remove [-f] [--with-branch [--force-branch]] <worktree>...
```

Removes worktrees; each name is handled independently and failures are reported together. The main worktree is
never removed. Standing inside the target is refused, unless shell integration is active, in which case you are
moved to the main worktree. `--with-branch` also deletes the branch if it is merged; `--force-branch` deletes it
regardless. On a terminal with no name, a picker offers the removable worktrees and asks for confirmation.

### `print-dir [<worktree>]`

Prints the absolute path of an existing worktree. It is read-only and never changes anything. With no name or
`@` it prints the main worktree; on a terminal with no name it opens a picker over the existing worktrees.
`cd "$(flowtree print-dir feature/auth)"` works without shell integration.

### `exec <worktree> -- <command> [args...]`

Runs a command in the worktree with your terminal attached and exits with the command's exit code.

### `switch [QUERY]`

Takes you where a branch lives; with shell integration your shell `cd`s there. Rows:

- `@`: the main worktree (also matched by the branch it is on, so `switch develop` finds it while it is on develop);
- every `work/*` and `hotfix/*` worktree;
- `master`, `develop` and `release/*` branches the main worktree is not on: selecting one **moves the main
  worktree onto it** (it must be clean);
- `work/*` and `hotfix/*` branches with no worktree: selecting one creates it.

A query is case-insensitive and tiered: exact, then substring, then fuzzy (letters in order). Topic branches
also match without their prefix (`login` for `work/login`). If exactly one row matches it is used at once;
otherwise the picker opens with the query prefilled. Without a terminal an ambiguous query is an error.
Unlike `print-dir`, `switch` can change state, and says so on stderr when it does.

### `relocate [-n]`

Moves work and hotfix worktrees to `<baseDir>/<branch>` after `baseDir` changed (`-n` shows the plan). Locked
worktrees, missing directories and occupied targets are skipped with a reason; it refuses to run while you are
inside a worktree it would move; the main worktree is never moved.

## Flow commands

`<kind>` is `work`, `release` or `hotfix`. `<name>` is the name without its prefix (a version for release and
hotfix). The behaviour behind these is described in [git-flow.md](git-flow.md).

| Command | Effect |
|---------|--------|
| `<kind> list [-v]` (`ls`) | List branches of the kind; `*` marks the current one. `-v` adds the state: no commits yet, based on latest, may be rebased, merged (can be deleted). |
| `<kind> start [-F] [--dry-run] <name> [<base>]` | Create the branch from `<base>` (default develop; master for hotfix) and its worktree (release: switch the main worktree). Runs hooks, prints a summary. |
| `<kind> finish [flags] [<name>]` | Merge and clean up. Flags below. |
| `<kind> checkout [<name>]` | Print the branch's directory, creating its worktree (or switching the main worktree for a release) if needed. |
| `<kind> publish [<name>]` | Push to the remote and set the upstream; refuses if the branch is already there. |
| `<kind> track <name>` | Check out a branch that exists on the remote (a worktree; for release, switch the main worktree). |
| `<kind> delete [-f] [-r] [--dry-run] <name>` | Remove worktree and local branch without merging. Refuses an unmerged branch unless `-f`; `-r` also deletes the remote branch. |

`--dry-run` (on `start`, `finish`, `delete`) prints the planned steps and changes nothing.

### `work finish`

```
flowtree work finish [-F] [-r | --no-rebase] [-p] [-S] [--no-ff] [--push] [-k] [--keeplocal] [--keepremote] [-D] [--dry-run] [<name>]
```

`-F` fetch first; the branch is first rebased onto develop in its worktree (`-p` keeps merges; `--no-rebase` skips it, `-r` forces it; the default is the `finishRebase` setting, on unless configured otherwise); `-S` squash; `--no-ff` always make a merge
commit (default: fast-forward when possible); `--push` push develop afterwards; `-k` keep local and remote
branch; `--keeplocal`/`--keepremote` keep one; `-D` (`--force-delete`) remove the worktree even with untracked files.

### `release finish`, `hotfix finish`

```
flowtree release finish [-F] [-s] [-u KEY] [-m MSG | -f FILE] [-p] [-k] [--keeplocal] [--keepremote]
                        [-D] [-n] [-b] [-S] [-T TAG] [--ff-master] [--dry-run] [<version>]
```

`-m`/`-f` tag message (not both); `-s`/`-u` sign; `-n` no tag; `-b` no back-merge into develop; `-T` custom tag
name; `-S` squash into master; `--ff-master` (release only) fast-forward master; `-p` push develop, master
and the tag. The tag name is `<version tag prefix><version>` unless `-T` is given.

### Work-only commands

| Command | Effect |
|---------|--------|
| `work diff [<name>]` | The changes on the branch since it left develop. |
| `work rebase [-i] [-p] [<name>]` | Rebase onto develop inside the worktree; `-i` is interactive and needs a terminal. |
| `work pull <remote> [<name>]` | Fetch, then fast-forward pull the branch in its worktree; if it does not exist locally, create it tracking `<remote>/<branch>` with a worktree. |
