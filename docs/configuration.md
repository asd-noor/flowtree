# Configuration

## `.flowtree.json`

The file lives in the main worktree. It is **per-machine**: worktree directories and local setup differ between
developers, so it is meant not to be committed. `flowtree init` offers to add it to `.git/info/exclude` (default
no, so nothing changes unless you say yes). Each clone runs `flowtree init` once. Without a file, defaults apply
(`base_dir` `../worktrees`, remote `origin`), but the flow commands need `init --flow`.

```json
{
  "version": 1,
  "base_dir": "../worktrees",
  "remote": "origin",
  "flow": {
    "master": "main",
    "develop": "develop",
    "work_prefix": "work/",
    "release_prefix": "release/",
    "hotfix_prefix": "hotfix/",
    "version_tag_prefix": "v"
  },
  "hooks": {
    "post_create": [
      { "type": "copy", "from": ".env" },
      { "type": "symlink", "from": ".cache", "to": ".cache" },
      { "type": "command", "command": "npm ci", "env": { "NODE_ENV": "development" } }
    ]
  }
}
```

Unknown fields are rejected, and the file is validated when it is read (`version` must be `1`).

| Key | Default | Meaning |
|-----|---------|---------|
| `base_dir` | `../worktrees` | Where worktrees go: `<base_dir>/<branch>`. Relative to the main worktree. `~` and `$HOME` are expanded. |
| `remote` | `origin` | The remote used for fetch, push, track, delete and the sync checks. |
| `flow.master` | `master` (an existing `main` is detected by `init --flow`) | Production branch. |
| `flow.develop` | `develop` | Integration branch. |
| `flow.work_prefix`, `release_prefix`, `hotfix_prefix` | `work/`, `release/`, `hotfix/` | Branch prefixes; they must differ. |
| `flow.version_tag_prefix` | none | Prefix of version tags (`v` gives `v1.2.0`). |

`master` and `develop` must differ, and no setting may be empty.

## Where worktrees live

`<base_dir>/<branch>`, so `work/login` with the default setting is `../worktrees/work/login` relative to the
main worktree. To keep every repository's worktrees under one parent, set `FLOWTREE_PARENT`:

```sh
export FLOWTREE_PARENT=~/worktrees      # ~/code/api gets ~/worktrees/api, ~/code/web gets ~/worktrees/web
```

It is only the default `init` proposes (`-w` and the prompt win). Changing `base_dir` later does not move
existing worktrees; `flowtree relocate` does. Old and new locations can coexist: flowtree always asks git
where a worktree really is.

## Hooks

`hooks.post_create` run, in order, after every worktree flowtree creates (`add`, `work start`, `hotfix start`,
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
| `FLOWTREE_PARENT` | `init` | Default `base_dir` parent, see above. |
| `FLOWTREE_CDFILE` | all commands that move you | A file the command writes the target directory to. The shell wrapper from `shell-init` sets it and does the `cd`. You normally never set it yourself. |
| `FLOWTREE_WORKTREE_PATH`, `FLOWTREE_REPO_ROOT` | command hooks | Set by flowtree for the hook. |
