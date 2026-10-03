# flowtree settings

Settings live in git config, section `flowtree`: in the repository's `.git/config` (per repository, never
committed) or, with `--global`, for every repository. A fresh clone has none: run `flowtree init --flow -d` once.
There is no config file in the working tree.

```sh
flowtree config                       # every setting, with its origin: (local) (global) (system) (default)
flowtree config baseDir               # print one (the default when unset)
flowtree config baseDir ~/wt/api      # set it for this repository; --global for every repository
flowtree config --unset baseDir
flowtree config hooks                 # path and contents of the hooks file
```

| Key | Default | Meaning |
|-----|---------|---------|
| `baseDir` | `$FLOWTREE_PARENT/<repo name>` if that variable is defined, else `../worktrees` | Where worktrees go: `<baseDir>/<branch>`. Relative to the main worktree; `~` is expanded. |
| `remote` | `origin` | Remote for fetch, push, track, delete and sync checks. |
| `master`, `develop` | none (set by `init --flow`) | Production and integration branches. Flow is set up when both are set. |
| `workPrefix`, `releasePrefix`, `hotfixPrefix` | `work/`, `release/`, `hotfix/` | Branch prefixes; they must differ. |
| `versionTagPrefix` | empty | Prefix of version tags (`v` gives `v1.2.0`). |

Key names are case-insensitive and may start with `flowtree.`. An unknown `flowtree.*` key is an error. Local
settings win over global ones, global over system, then the default.

## Worktree directory

A branch `work/login` lives at `<baseDir>/work/login`. `FLOWTREE_PARENT=~/worktrees` is the default when `baseDir`
is not set (`~/worktrees/<repo directory name>`); an explicit `baseDir` wins. Changing `baseDir` does not move
existing worktrees: run `flowtree relocate -n`, then `flowtree relocate`.

## Hooks

`post_create` hooks live in `<git-common-dir>/flowtree-hooks.json` (usually `.git/flowtree-hooks.json`), a JSON file
inside the git directory, so it is never committed; `flowtree config hooks` shows the exact path. Unknown fields and
invalid hooks are errors; a missing file means no hooks.

```json
{
  "post_create": [
    { "type": "copy", "from": ".env" },
    { "type": "symlink", "from": "node_modules", "to": "node_modules" },
    { "type": "command", "command": "npm ci", "env": { "NODE_ENV": "development" }, "work_dir": "." }
  ]
}
```

Run in order after every worktree flowtree creates (`add`, `<kind> start`, `checkout`, `track`, `switch`, `pull`).
A failing hook prints a warning; the worktree stays.

| Type | Fields | Effect |
|------|--------|--------|
| `copy` | `from`, `to` (defaults to `from` when `from` is relative) | Copy a file or directory from the **main worktree** into the new one. Works for git-ignored files like `.env`. |
| `symlink` | `from`, `to` | Link `to` in the new worktree to `from` in the main worktree. Refuses to overwrite an existing path. |
| `command` | `command`, optional `env`, `work_dir` | Run via `sh -c` in the new worktree (or `work_dir`) with `FLOWTREE_WORKTREE_PATH` and `FLOWTREE_REPO_ROOT` set. |

Relative `from` paths must stay inside the main worktree and relative `to` paths inside the new one.
Do not add `command` hooks that run untrusted code: they execute on every new worktree.
