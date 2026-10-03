---
name: flowtree
description: Drives the flowtree CLI, which manages git worktrees and a git-flow variant where work/* and hotfix/* branches get their own worktree (release/* switches the main checkout). Use when a repo has flowtree settings (`git config --get flowtree.develop` prints a value), or when asked to start, finish, publish or clean up feature/bugfix/release/hotfix work with git-flow semantics in isolated worktrees, tag a release, ship a hotfix, create or remove a worktree, run something in a worktree, set up worktree post-create hooks (copy .env, symlink, run a setup command), or move worktrees to another directory.
license: GPL-3.0-or-later
compatibility: Requires git (2.17 or newer) and the flowtree binary on PATH. Needs a shell that can run commands.
metadata:
  version: "1.0"
---

# flowtree

One CLI for two jobs: plain **git worktrees** (`add`, `list`, `remove`, `print-dir`, `exec`, hooks) and **git-flow
where topic branches live in their own worktrees**. Branch model:

| Kind | Branch | Starts from | Finishes into | Lives in |
|------|--------|-------------|---------------|----------|
| `work` (feature/bugfix/support) | `work/<name>` | `develop` | `develop` | its own worktree |
| `release` | `release/<version>` | `develop` | `master` + tag, tag merged into `develop` | the main checkout |
| `hotfix` | `hotfix/<version>` | `master` | `master` + tag, tag merged into `develop` | its own worktree |

The production branch may be called `main`; the branch names and prefixes are in git config (`flowtree config` lists them).

## Rules for agents

1. **Layout rule.** The main checkout holds only `master`, `develop` or a `release/*` branch; `work/*` and
   `hotfix/*` live in their own worktrees. Never check out a `work/*` or `hotfix/*` branch in the main
   checkout yourself (`git checkout`/`switch`): start it with `flowtree <kind> start` and work in the worktree.
   `release start` switches the main checkout onto `release/<version>` and you work right there (it must be clean
   first). If the rule gets broken, `work|hotfix checkout <name>` moves the branch out again.
2. **Your shell does not keep a `cd`** between tool calls and shell integration is not loaded. Get the worktree
   path from the output and use absolute paths or `git -C <path>` / `cd <path> && ...` in every command:
   ```sh
   dir=$(flowtree work checkout login)   # prints only the path; creates the worktree if missing
                                         # (release: switches the main checkout and prints its path)
   dir=$(flowtree print-dir login)       # read-only: prints the path of an existing worktree
   dir=$(flowtree add -b spike --quiet)  # plain worktree on a new branch, prints only the path
   git -C "$dir" status
   ```
3. **Stay non-interactive.** There is no terminal, so the picker is unavailable: `switch`, `print-dir` and
   `remove` need a name that matches exactly one worktree, and `finish`/`checkout` need a name unless you run
   them inside the topic worktree. Use `flowtree init --flow -d`. Always pass `-m "<message>"` to
   `release finish` and `hotfix finish`.
4. **Do not push** (`-p`, `--push`, `publish`) or delete remote branches unless the user asked. `finish`
   deletes the remote copy of the topic branch if one exists; use `--keepremote` when that is not wanted.
5. **Finish with flowtree, not by hand.** Do not `git merge`/`git branch -d`/`git worktree remove` a topic
   branch yourself; `finish` merges, tags, back-merges and cleans up in the right order. It merges inside
   the worktree that has the target branch checked out, usually the main checkout on develop: that checkout
   must have no uncommitted tracked changes (it refuses otherwise) and its files change. A release `finish`
   moves the main checkout back to develop, and `finish` run from inside a topic worktree removes that worktree.
6. Commit your work in the topic worktree (the main checkout, for a release) before `finish`: it refuses if
   tracked files are modified. Untracked files do not count.
7. Never use `-f`, `--force-branch`, `-D` or `delete -f`, and never remove a worktree the user did not ask
   about, without confirming: they discard uncommitted or unmerged work.

## Workflow

Check the repo is set up: any `flowtree work list` fails with "flow is not set up in this repository" if not.
Then run `flowtree init --flow -d` (it creates `master`/`develop` if missing and stores the settings in git config); confirm
with the user first if the repo already has branches or a different layout.

**Work item**
```sh
flowtree work start <name>        # prints "A worktree for it was created at '<path>'"
# edit + commit inside <path>
flowtree work finish <name>       # run from anywhere; fast-forwards into develop when possible
```
Options: `--no-ff` (always a merge commit), `-S` (squash), `-r` (rebase onto develop first), `-k` (keep branches).

**Release**
```sh
flowtree release start 1.4.0      # only one release may be open
# the main checkout is now on release/1.4.0: version bump, changelog, fixes; commit there
flowtree release finish -m "Release 1.4.0" 1.4.0
```

**Hotfix**
```sh
flowtree hotfix start 1.4.1
# fix + commit in the worktree
flowtree hotfix finish -m "Hotfix 1.4.1" 1.4.1
```

`finish` merges into `master` with `--no-ff`, creates an annotated tag (name = version, plus the configured tag
prefix), merges the tag into `develop`, then removes the worktree and branch (for a release: switches the main
checkout back to `develop` and deletes the branch).

Preview any `start`, `finish` or `delete` with `--dry-run`: it prints the steps and changes nothing.

**Inspect**: `flowtree work list -v` (also `release`, `hotfix`), `flowtree work diff <name>`, `flowtree list`.

**Plain worktrees** (not flow branches): `flowtree add <branch>` or `flowtree add -b <new> [<start>]`, then
`flowtree remove [--with-branch] <name>`. Inside a flow repo do **not** create or remove `work/`, `release/` or
`hotfix/` branches this way: that skips the base branch, the open-release check, tagging and the merge steps.
Use `flowtree <kind> start|finish|delete` for those.

## Output and errors

Success output of `start`/`finish` ends with a "Summary of actions" block. Errors go to stderr as
`Error: <message>` with a non-zero exit code; warnings start with `Warning:`. Common ones are in
[troubleshooting](references/troubleshooting.md). The key one: a **merge conflict** aborts the merge and changes
nothing. Go to the topic worktree, run `git merge develop` (or `master` for hotfixes), resolve, commit, then
re-run `finish`.

## More

- Every command and flag: [references/commands.md](references/commands.md)
- Failure modes and recovery: [references/troubleshooting.md](references/troubleshooting.md)
- Settings (`flowtree config`), worktree directory and post-create hooks: [references/config.md](references/config.md)
