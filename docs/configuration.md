# Configuration

## Settings live in git config

flowtree stores its settings in git config, in the `flowtree` section. Nothing is added to the working tree and
there is no config file to commit or ignore. Settings are read from the main worktree, so every worktree of a
repository sees the same ones, and all the usual git scopes apply:

| Scope | File | Use |
|-------|------|-----|
| local | `<repo>/.git/config` | per repository, per machine (what `flowtree init` and `flowtree config <key> <value>` write) |
| global | `~/.gitconfig` | every repository of yours (`flowtree config --global ...`) |
| system | `/etc/gitconfig` | every user of the machine |

**Precedence:** local wins over global, global over system, and any setting that is not set falls back to its
default. Because `.git/config` is never committed, worktree directories and remotes can differ between developers
without anyone stepping on anyone. Each clone runs `flowtree init` (or `init --flow`) once.

| Key | Default | Meaning |
|-----|---------|---------|
| `baseDir` | `$FLOWTREE_PARENT/<repo name>` if the variable is defined, else `../worktrees` | Where worktrees go: `<baseDir>/<branch>`. Relative to the main worktree; `~` and `$HOME` are expanded. |
| `remote` | `origin` | The remote used for fetch, push, track, delete and the sync checks. |
| `master` | none | Production branch. Set by `init --flow`. |
| `develop` | none | Integration branch. Set by `init --flow`. |
| `workPrefix`, `releasePrefix`, `hotfixPrefix` | `work/`, `release/`, `hotfix/` | Branch prefixes; they must differ and must not be empty. |
| `versionTagPrefix` | empty | Prefix of version tags (`v` gives `v1.2.0`). |

Flow is **set up** when both `master` and `develop` are set; setting only one of them is an error. `master` and
`develop` must differ. Key names are case-insensitive and may carry the `flowtree.` prefix; git itself shows them
in lowercase (`flowtree.workprefix`).

**Unknown keys are errors.** A typo such as `flowtree.workPrefx` in any scope makes the commands fail with
`unknown setting 'flowtree.workprefx' (known: ...)`, instead of silently doing nothing. Remove it with
`git config --unset flowtree.workPrefx`.

## `flowtree config`

```sh
flowtree config                        # every setting, with where its value comes from
flowtree config baseDir                # print the effective value (the default when unset)
flowtree config baseDir ~/worktrees/api    # set it for this repository
flowtree config --global remote upstream   # set it for every repository
flowtree config --unset baseDir        # remove it again
flowtree config hooks                  # where the hooks file lives and what is in it
```

The listing marks each value `(local)`, `(global)`, `(system)` or `(default)`. Keys and values are validated
before they are written. You can equally use plain git: `git config flowtree.baseDir ~/wt`.

## Where worktrees live

`<baseDir>/<branch>`, so `work/login` with the default setting is `../worktrees/work/login` relative to the main
worktree. To keep every repository's worktrees under one parent, define `FLOWTREE_PARENT`:

```sh
export FLOWTREE_PARENT=~/worktrees      # ~/code/api gets ~/worktrees/api/<branch>, ~/code/web gets ~/worktrees/web/<branch>
```

This is the **default**, applied whenever `baseDir` is not set, and it is evaluated each time flowtree runs: change
or unset the variable and the default follows. `init` therefore stores `baseDir` only when you pass `-w` or answer the
prompt with something different from the default; accepting the default stores nothing. An explicit `baseDir` always
wins over the environment.

Changing `baseDir` later does not move existing worktrees; `flowtree relocate` does. Old and new locations can
coexist: flowtree always asks git where a worktree really is.

## Hooks

Post-create hooks have structure that git config cannot express, so they live in a JSON file inside the git
directory: `<git-common-dir>/flowtree-hooks.json` (usually `<repo>/.git/flowtree-hooks.json`). Being inside `.git` it
is shared by all worktrees, is never part of a commit, and so cannot be injected by a clone, which matters because
command hooks run code. `flowtree config hooks` prints the exact path and, when it exists, the contents.

```json
{
  "post_create": [
    { "type": "copy", "from": ".env" },
    { "type": "symlink", "from": ".cache", "to": ".cache" },
    { "type": "command", "command": "npm ci", "env": { "NODE_ENV": "development" } }
  ]
}
```

The file is read strictly: unknown fields and invalid hooks are errors. A missing file means no hooks.

`post_create` hooks run, in order, after every worktree flowtree creates (`add`, `work start`, `hotfix start`,
`checkout`, `track`, `switch`, `pull` when they create one). A failing hook prints a warning and the worktree
stays.

| Type | Fields | Effect |
|------|--------|--------|
| `copy` | `from`, `to` (defaults to `from` when `from` is relative) | Copy a file or directory from the **main worktree** into the new one. Works for git-ignored files such as `.env`. |
| `symlink` | `from`, `to` | Link `to` in the new worktree to `from` in the main worktree. Refuses to overwrite an existing path. |
| `command` | `command`, optional `env`, `work_dir` | Run through `sh -c` in the new worktree (or `work_dir`). |

Relative `from` paths must stay inside the main worktree and relative `to` paths inside the new one. Command
hooks get `FLOWTREE_WORKTREE_PATH` (the new worktree) and `FLOWTREE_REPO_ROOT` (the main worktree) in their
environment.

## Environment variables

| Variable | Used by | Meaning |
|----------|---------|---------|
| `FLOWTREE_PARENT` | all commands | Default base directory parent: `$FLOWTREE_PARENT/<repo name>` when `baseDir` is not set. |
| `FLOWTREE_CDFILE` | all commands that move you | A file the command writes the target directory to. The shell wrapper from `shell-init` sets it and does the `cd`. You normally never set it yourself. |
| `FLOWTREE_WORKTREE_PATH`, `FLOWTREE_REPO_ROOT` | command hooks | Set by flowtree for the hook. |
