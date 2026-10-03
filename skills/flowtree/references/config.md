# .flowtree.json

Lives in the main worktree. It is per-machine and normally not committed, so do not expect it in a fresh clone:
run `flowtree init --flow -d` once. Create or change it with `init` (`-f` overwrites); editing it by hand is fine.

```json
{
  "version": 1,
  "base_dir": "../worktrees",
  "remote": "origin",
  "flow": {
    "master": "main", "develop": "develop",
    "work_prefix": "work/", "release_prefix": "release/", "hotfix_prefix": "hotfix/",
    "version_tag_prefix": "v"
  },
  "hooks": {
    "post_create": [
      { "type": "copy", "from": ".env" },
      { "type": "symlink", "from": "node_modules", "to": "node_modules" },
      { "type": "command", "command": "npm ci", "env": { "NODE_ENV": "development" }, "work_dir": "." }
    ]
  }
}
```

Unknown fields are errors. `master` and `develop` must differ and the prefixes must differ.

## Worktree directory

`base_dir` (default `../worktrees`, relative to the main worktree; `~` is expanded). A branch `work/login` lives at
`<base_dir>/work/login`. `FLOWTREE_PARENT=~/worktrees` makes `init` default to `<parent>/<repo directory name>`.
Changing `base_dir` does not move existing worktrees: run `flowtree relocate -n`, then `flowtree relocate`.

## Hooks (`hooks.post_create`)

Run in order after every worktree flowtree creates (`add`, `<kind> start`, `checkout`, `track`, `switch`, `pull`).
A failing hook prints a warning; the worktree stays.

| Type | Fields | Effect |
|------|--------|--------|
| `copy` | `from`, `to` (defaults to `from` when `from` is relative) | Copy a file or directory from the **main worktree** into the new one. Works for git-ignored files like `.env`. |
| `symlink` | `from`, `to` | Link `to` in the new worktree to `from` in the main worktree. Refuses to overwrite an existing path. |
| `command` | `command`, optional `env`, `work_dir` | Run via `sh -c` in the new worktree (or `work_dir`) with `FLOWTREE_WORKTREE_PATH` and `FLOWTREE_REPO_ROOT` set. |

Relative `from` paths must stay inside the main worktree and relative `to` paths inside the new one.
Do not add `command` hooks that run untrusted code: they execute on every new worktree.
