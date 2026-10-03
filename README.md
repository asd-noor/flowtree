# flowtree

Git worktrees plus an opinionated git-flow. One binary:

- **A worktree manager** in the spirit of [wtp](https://github.com/satococoa/wtp): `add`, `list`, `remove`,
  `print-dir`, `exec`, post-create hooks, shell integration.
- **git-flow where topic branches live in their own worktrees**: `flowtree work|release|hotfix start|finish|…`.
  Branching and merge behaviour follow `git flow` (AVH edition); the difference is that you `cd` between
  worktrees instead of switching branches.

| flowtree | git-flow | branches from | merges into | lives in |
|----------|----------|---------------|-------------|----------|
| `work`    | feature/bugfix/support | develop | develop | its own worktree |
| `release` | release | develop | master (tagged), the tag back into develop | the main checkout |
| `hotfix`  | hotfix  | master  | master (tagged), the tag back into develop | its own worktree |

The main checkout rests on `develop` and only ever holds `master`, `develop` or a `release/*` branch. Releases
only touch a changelog and a version, so they switch the main checkout instead of getting a worktree.

Documentation in [docs/](docs/): the [branch model](docs/git-flow.md), the
[command reference](docs/commands.md), [configuration](docs/configuration.md),
[shell integration](docs/shell-integration.md), [architecture](docs/architecture.md) and
[development](docs/development.md).

## Install

```sh
go install flowtree/cmd/flowtree@latest     # or: mise run install  ->  builds bin/flowtree, installs it to ~/.local/bin
eval "$(flowtree shell-init)"                  # in your shell rc: cd-ing commands + completion (bash, zsh, fish)
                                               # completion only: flowtree completion <shell>, see docs/shell-integration.md
```

Requires `git` 2.17 or newer (see [Requirements](#requirements)). The shell function makes `start`, `checkout`, `switch` and `add` take you into the
worktree, and moves you out of one that `finish` or `remove` deleted.

## Requirements

`git` 2.17 or newer. Most of it works on any git that has worktrees; these are the features that set the floor:

| git | Needed for |
|-----|------------|
| 2.17 | `git worktree remove` and `move` (`remove`, `relocate`, every `finish`/`delete` cleanup) |
| 2.18 | `rebase --rebase-merges`, only for `work rebase -p` and `work finish -r -p` |
| 2.31 | `relocate` skips locked worktrees; on older git it stops with git's own error at the first locked one |

Settings are read without `git config --show-scope` (2.26), so they work on the same git versions.

## Quick start

```sh
flowtree init --flow               # store settings in git config, create master + develop; prompts (-d for defaults)

flowtree work start login          # work/login off develop, in ../worktrees/work/login
# ...commit...
flowtree work finish               # merge into develop, remove worktree + branch (name inferred from cwd)

flowtree release start 1.2.0       # release/1.2.0 off develop; the main checkout switches to it
flowtree release finish -m "1.2.0" # merge to master, tag, merge the tag into develop, main back on develop

flowtree hotfix start 1.2.1        # hotfix/1.2.1 off master
flowtree hotfix finish -p          # as release finish, then push develop, master and the tag

flowtree switch                    # pick where to go (built-in picker); `flowtree switch login` goes straight there
```

`--dry-run` on `start`, `finish` and `delete` prints the steps without running them.

## Commands

```
flowtree init [--flow] [-w DIR] [-f] [-d]
              [--master B] [--develop B] [-p work/] [-r release/] [-x hotfix/] [-t TAGPREFIX] [--remote R]
flowtree add [-b NEW] [--exec CMD] [-q] <branch> [<commitish>]
flowtree config [<key> [<value>]] [--global] [--unset] | config hooks
flowtree list | remove [-f] [--with-branch] [--force-branch] <worktree>... | exec <worktree> -- CMD...
flowtree print-dir [<worktree>]    # prints a worktree's path; read-only (`@` = main)
flowtree switch [QUERY]            # changes your worktree; may move the main checkout or create a worktree
flowtree relocate [-n]
flowtree shell-init [bash|zsh|fish] | completion <shell>
flowtree install-skill <dir>       # install the embedded agent skill into a skills directory

flowtree work    list [-v] | start [-F] <name> [<base>] | finish [flags] [<name>] | checkout|publish [<name>]
                 track <name> | delete [-f] [-r] <name> | diff [<name>] | rebase [-i] [-p] [<name>] | pull <remote> [<name>]
flowtree release list [-v] | start [-F] <version> [<base>] | finish [flags] [<version>] | checkout|publish|track|delete
flowtree hotfix  …same as release; branches from master
```

Run `flowtree <command> --help` for every flag. `finish` with no name uses the branch of the worktree you are in
(or, on a terminal, offers a picker). Work names also accept a unique prefix.

- `release`/`hotfix finish` merges `--no-ff` into master, creates an annotated tag (`-s`/`-u` sign it, `-m`/`-f`
  set the message, `-n` skips it, `-T` renames it), merges the **tag** into develop (`-b` skips that), and with
  `-p` pushes develop, master and the tag.
- `finish` merges inside the worktree that already has the target branch checked out (usually the main
  checkout, on develop; it must have no uncommitted changes), or in a throwaway worktree. That worktree's files
  do change, but the directory you run it from is never switched to another branch, except that finishing a
  release returns the main checkout to develop. A conflicting merge is aborted and nothing changes.
- `finish` is resumable: if the back-merge into develop conflicts after master was merged and tagged, resolve it
  by hand and run `finish` again; it continues from there.
- Finishing deletes the local branch, its worktree and the remote branch; `-k` keeps the branches (the worktree
  is still removed), `-D` removes a worktree with untracked files.
- `start` refuses if a release or hotfix is already open, or the tag exists.

## Configuration

Settings live in **git config**, section `flowtree`: per repository (`.git/config`, never committed, so worktree
directories can differ between developers) or for every repository (`--global`). Nothing is added to the working
tree. Each clone runs `flowtree init` once.

```sh
flowtree config                          # every setting, with where its value comes from
flowtree config baseDir ~/worktrees/api  # or: git config flowtree.baseDir ~/worktrees/api
flowtree config --global remote upstream
```

Keys: `baseDir`, `remote`, `master`, `develop`, `workPrefix`, `releasePrefix`, `hotfixPrefix`, `versionTagPrefix`.
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
