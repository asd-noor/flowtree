# flowtree

Git worktrees plus an opinionated git-flow. One binary:

- **A worktree manager** in the spirit of [wtp](https://github.com/satococoa/wtp): `add`, `list`, `remove`,
  `print-dir`, `exec`, post-create hooks, shell integration.
- **git-flow where topic branches live in their own worktrees**: `flowtree feature|release|hotfix start|finish|…`.
  Branching and merge behaviour follow `git flow` (AVH edition); the difference is that you `cd` between
  worktrees instead of switching branches.

| flowtree | git-flow | branches from | merges into | lives in |
|----------|----------|---------------|-------------|----------|
| `feature` | feature/bugfix/support | develop | develop (merge commit) | its own worktree |
| `release` | release | develop | master (tagged), the tag back into develop | its own worktree |
| `hotfix`  | hotfix  | master  | master (tagged), the tag back into develop | its own worktree |
| `work`    | (none) | its parent feature or hotfix | the parent (rebased, fast-forward) | its own worktree, local only |

Features and hotfixes are tasks; `work` branches are their subtasks, for parallel work on one task. A work branch
is named `work/<parent>/<name>` (`work/feature/login/api`); `work start -P feature/login api` creates it, or
`work start api` from inside the parent's worktree. A feature or hotfix cannot finish while subtasks are open.

The main checkout rests on `develop` and only ever holds `master` or `develop`. Every topic branch gets its own
worktree.

Documentation in [docs/](docs/): the [branch model](docs/git-flow.md), the
[command reference](docs/commands.md), [configuration](docs/configuration.md),
[shell integration](docs/shell-integration.md), [architecture](docs/architecture.md) and
[development](docs/development.md).

## Install

Clone the repository and install from it:

```sh
git clone <this repository> && cd flowtree
mise run install               # builds bin/flowtree (version from git describe), installs it to ~/.local/bin
# or, without mise:
go install ./cmd/flowtree      # installs into $GOBIN (or $GOPATH/bin); --version then reports "dev"
```

Then, in your shell rc:

```sh
eval "$(flowtree shell-init)"  # cd-ing commands + completion (bash, zsh, fish)
                               # completion only: flowtree completion <shell>, see docs/shell-integration.md
```

(The module is named plain `flowtree`, so `go install ...@latest` from a remote does not work.)

Requires `git` 2.17 or newer (see [Requirements](#requirements)). The shell function makes `start`, `checkout`, `switch` and `add` take you into the
worktree, and moves you out of one that `finish` or `remove` deleted.

## Requirements

`git` 2.17 or newer. Most of it works on any git that has worktrees; these are the features that set the floor:

| git | Needed for |
|-----|------------|
| 2.17 | `git worktree remove` and `move` (`remove`, `relocate`, every `finish`/`delete` cleanup) |
| 2.18 | `rebase --rebase-merges`, only for `feature rebase -p` |
| 2.31 | `relocate` skips locked worktrees; on older git it stops with git's own error at the first locked one |

Settings are read without `git config --show-scope` (2.26), so they work on the same git versions.

## Quick start

```sh
flowtree init --flow               # store settings in git config, create master + develop; prompts (-d for defaults)

flowtree feature start login       # feature/login off develop, in ../worktrees/feature/login
# ...commit...
flowtree feature finish            # merge commit into develop, remove worktree + branch (name inferred from cwd)

flowtree release start 1.2.0       # release/1.2.0 off develop, in ../worktrees/release/1.2.0
flowtree release finish -m "1.2.0" # merge to master, tag, merge the tag into develop, remove worktree + branch

flowtree hotfix start 1.2.1        # hotfix/1.2.1 off master
flowtree hotfix finish -p          # as release finish, then push develop, master and the tag

flowtree switch                    # pick where to go (built-in picker); `flowtree switch login` goes straight there
```

`--dry-run` on `start`, `finish` and `delete` prints the steps without running them.

## Commands

```
flowtree init [--flow] [-w DIR] [-f] [-d]
              [--master B] [--develop B] [-p feature/] [-r release/] [-x hotfix/] [-t TAGPREFIX] [--remote R]
flowtree add [-b NEW] [--exec CMD] [-q] <branch> [<commitish>]
flowtree config [<key> [<value>]] [--global] [--unset] | config hooks
flowtree list | remove [-f] [--with-branch] [--force-branch] <worktree>... | exec <worktree> -- CMD...
flowtree print-dir [<worktree>]    # prints a worktree's path; read-only (`@` = main)
flowtree switch [QUERY]            # changes your worktree; may move the main checkout or create a worktree
flowtree relocate [-n]
flowtree board                     # terminal kanban for people (needs a terminal)
flowtree shell-init [bash|zsh|fish] | completion <shell>
flowtree install-skill <dir>       # install the embedded agent skill into a skills directory

flowtree feature list [-v] | start [-F] <name> [<base>] | finish [flags] [<name>] | checkout|publish [<name>]
                 track <name> | delete [-f] [-r] <name> | diff [<name>] | rebase [-i] [-p] [<name>] | pull <remote> [<name>]
flowtree work    list [-v] | start [-P <parent>] <name> | finish [flags] [<name>] | checkout [<name>] | delete [-f] <name>
flowtree release list [-v] | start [-F] <version> [<base>] | finish [flags] [<version>] | checkout|publish|track|delete
flowtree hotfix  …same as release; branches from master
```

Run `flowtree <command> --help` for every flag. `finish` with no name uses the branch of the worktree you are in
(or, on a terminal, offers a picker). Feature names also accept a unique prefix; work names may be the bare
subtask name.

- `feature finish` merges `--no-ff` into develop and never rebases; if a pull request already merged the branch,
  it only cleans up.
- `release`/`hotfix finish` merges `--no-ff` into master, creates an annotated tag (`-s`/`-u` sign it, `-m`/`-f`
  set the message, `-n` skips it, `-T` renames it), merges the **tag** into develop (`-b` skips that), and with
  `-p` pushes develop, master and the tag.
- `finish` merges inside the worktree that already has the target branch checked out (usually the main
  checkout, on develop; it must have no uncommitted changes), or in a throwaway worktree. That worktree's files
  do change, but the directory you run it from is never switched to another branch. A conflicting merge is
  aborted and nothing changes.
- `finish` is resumable: if the back-merge into develop conflicts after master was merged and tagged, resolve it
  by hand and run `finish` again; it continues from there.
- Finishing deletes the local branch, its worktree and the remote branch; `-k` keeps the branches (the worktree
  is still removed), `-D` removes a worktree with untracked files.
- `release`/`hotfix start` refuses if a release or hotfix is already open (only one of them at a time), or the
  tag exists.

## Board

Each repository has a kanban of its own, stored in the git directory (shared by all worktrees, never pushed):

```sh
flowtree task add -t ui "Login page"   # -> #1, in todo
flowtree task list                     # #1    todo      Login page  [ui]
flowtree task show 1 | edit 1 | comment 1 "..." | backlog 1 | human 1
```

Columns follow git rather than manual moves: a task without a branch is in todo (or the backlog), with a branch it
is doing, and once its branch is merged into its target it is done. `flowtree board` shows the columns side by
side in the terminal (enter for details, `e` to edit, live reload). See [docs/commands.md](docs/commands.md#board).

## Configuration

Settings live in **git config**, section `flowtree`: per repository (`.git/config`, never committed, so worktree
directories can differ between developers) or for every repository (`--global`). Nothing is added to the working
tree. Each clone runs `flowtree init` once.

```sh
flowtree config                          # every setting, with where its value comes from
flowtree config baseDir ~/worktrees/api  # or: git config flowtree.baseDir ~/worktrees/api
flowtree config --global remote upstream
```

Keys: `baseDir`, `remote`, `master`, `develop`, `featurePrefix`, `releasePrefix`, `hotfixPrefix`, `versionTagPrefix`.
An unknown `flowtree.*` key is an error, so typos do not go unnoticed.

- `baseDir`: where worktrees go, `<baseDir>/<branch>`; relative to the main worktree, `~` is expanded. When it is
  not set, the default is `$FLOWTREE_PARENT/<repo name>` if `FLOWTREE_PARENT` is defined, else `../worktrees`.
  `flowtree relocate` moves existing worktrees after a change.
- Hooks (copy a `.env`, symlink a cache, run a setup command) run after every new worktree. They live in
  `.git/flowtree-hooks.json` (inside the git directory, so a clone cannot inject code); `flowtree config hooks`
  shows the path and an example:
  ```json
  { "post_create": [
    { "type": "copy", "from": ".env" },
    { "type": "symlink", "from": ".cache", "to": ".cache" },
    { "type": "command", "command": "npm ci" }
  ] }
  ```
  `copy` and `symlink` read from the main worktree; `command` runs in the new worktree with
  `FLOWTREE_WORKTREE_PATH` and `FLOWTREE_REPO_ROOT` set.

## Scripting and agents

`flowtree install-skill ~/.claude/skills` installs an [Agent Skill](https://agentskills.io) (embedded in the
binary) that teaches an agent the branch model and these rules.

Everything but the picker is non-interactive: `init -d`, a name on every command, `--quiet` on `add`.
`switch`, `print-dir` and `remove` without a name need a terminal. `FLOWTREE_CDFILE` is the file the shell
function reads to learn where to `cd`.

## License

GPL-3.0-or-later; see [LICENSE](LICENSE). Parts are ported from [wtp](https://github.com/satococoa/wtp) (MIT);
see [THIRD_PARTY.md](THIRD_PARTY.md).
