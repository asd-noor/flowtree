# Shell integration

A program cannot change its parent shell's directory, so flowtree uses a small wrapper function.

```sh
# bash or zsh, in ~/.bashrc / ~/.zshrc
eval "$(flowtree shell-init)"

# fish, in ~/.config/fish/config.fish
flowtree shell-init fish | source
```

`shell-init` takes the shell as an argument (`bash`, `zsh`, `fish`) and otherwise reads `$SHELL`. It prints:

1. a `flowtree` function that wraps the binary, and
2. cobra's generated completion for that shell.

`flowtree` must be on your `PATH` when the line runs.

## How `cd` works: the cd-file

The wrapper creates a temporary file, runs `command flowtree "$@"` with `FLOWTREE_CDFILE` pointing at it, and
afterwards `cd`s to the directory written in it, if any (and if it still exists). The exit status of flowtree is
preserved. Commands write the file when you should end up somewhere else:

| Command | You end up in |
|---------|---------------|
| `add` | the new worktree |
| `work\|hotfix start`, `track`, `checkout` | the branch's worktree |
| `release start`, `track`, `checkout` | the main worktree (it is now on the release) |
| `switch` | where you chose |
| `finish`, `delete`, `remove` of the worktree you are standing in | the main worktree |
| `work pull` that creates a worktree | that worktree |

Without the wrapper everything still works; commands that print a path (`checkout`, `switch`, `print-dir`,
`add -q`) can be used as `cd "$(flowtree checkout login)"`. One difference: `remove` refuses to delete the
worktree you are standing in unless the wrapper is active, because otherwise your shell would be left in a
deleted directory.

## Completion

Completion is dynamic and reads the repository you are in:

- worktree names for `print-dir`, `exec`, `remove`;
- branch names (local, plus remote-only) for `add`;
- open `work`/`release`/`hotfix` names for `finish`, `checkout`, `publish`, `delete`, `rebase`, `diff`;
- branches that exist only on the remote for `track`;
- remotes for `work pull`;
- row names for `switch`.

Completion never prints errors into your prompt; outside a repository it simply suggests nothing. Only the
bash and fish wrappers are covered by automated tests; zsh uses the same wrapper text and cobra's zsh completion.

### Completion without the wrapper

`flowtree completion <shell>` prints only the completion script, for people who want completions but not the
`cd`-following wrapper, or who prefer to load completions from a file:

```sh
# bash (needs the bash-completion package); loaded on demand from this directory
flowtree completion bash > ~/.local/share/bash-completion/completions/flowtree

# zsh: write it to a directory on your fpath, and make sure compinit runs after fpath is set
flowtree completion zsh > "${fpath[1]}/_flowtree"

# fish
flowtree completion fish > ~/.config/fish/completions/flowtree.fish

# PowerShell
flowtree completion powershell | Out-String | Invoke-Expression
```

Open a new shell afterwards. The script asks the installed `flowtree` for suggestions each time you press Tab,
so it keeps working after upgrades and never needs regenerating.

### If completion does nothing

- `flowtree` must be on your `PATH` (`which flowtree`); the script calls it by name.
- bash: the `bash-completion` package must be installed and loaded, and `eval "$(flowtree shell-init)"` must
  come after it is sourced in your rc file.
- zsh: `compinit` must have run. `shell-init` runs it for you when it has not; with the file-based setup, call
  it yourself after the `fpath` is set up.
- Inside a repository, `flowtree __complete work finish ""` prints what would be suggested; if that is empty,
  the problem is the repository (for example flow not set up: `flowtree init --flow`), not the shell.

## The picker

`switch`, `print-dir` and `remove` without a name, and `finish`/`checkout` without a name outside a topic
worktree, open a built-in picker when a terminal is attached:

| Key | Action |
|-----|--------|
| type | filter (exact, substring and fuzzy matches, best first) |
| up / down, ctrl-p / ctrl-n | move |
| enter | choose |
| esc, ctrl-c | cancel (exit status 1) |
| backspace, ctrl-u | edit the query |

The right-hand pane (below the list on narrow terminals) previews the highlighted row: its path, `git status`
and recent commits. The picker draws inline on stderr, so command substitution such as
`cd "$(flowtree print-dir)"` still works.

It needs a terminal on stdin and stderr. In scripts and agents pass a name that matches exactly one row, or the
command fails with "no terminal: pass a name".
