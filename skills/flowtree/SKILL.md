---
name: flowtree
description: Drives the flowtree CLI, which manages git worktrees and a git-flow variant where regular/*, release/* and hotfix/* branches each get their own worktree. Use when a repo has flowtree settings (`git config --get flowtree.develop` prints a value), or when asked to start, finish, publish or clean up regular/bugfix/release/hotfix work or board tasks with git-flow semantics in isolated worktrees, tag a release, ship a hotfix, create or remove a worktree, run something in a worktree, set up worktree post-create hooks (copy .env, symlink, run a setup command), or move worktrees to another directory.
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
| `regular` task (feature/bugfix/support) | `regular/<name>` | `develop` | `develop` | its own worktree |
| `hotfix` task | `hotfix/<version>` | `master` | `master` + tag, tag merged into `develop` | its own worktree |
| `release` | `release/<version>` | `develop` | `master` + tag, tag merged into `develop` | its own worktree |
| `work` (subtask) | `work/<parent>/<name>` | its parent's tip | the parent: rebased, fast-forwarded | its own worktree, local only |

Regular and hotfix branches belong to board tasks: a task has a type (`regular` or `hotfix`) and `flowtree task start`
starts the branch of its type. Releases have no task. The production branch may be called `main`; the branch names and
prefixes (`flowtree.regularprefix`, ...) are in git config (`flowtree config` lists them).

## Rules for agents

1. **Layout rule.** The main checkout holds only `master` or `develop`; `regular/*`, `release/*`, `hotfix/*` and `work/*`
   live in their own worktrees. Never check out a topic branch in the main checkout yourself
   (`git checkout`/`switch`): start it with `flowtree task start` and work in the worktree. If the rule gets
   broken, `task checkout <n>` moves the branch out again.
2. **Your shell does not keep a `cd`** between tool calls and shell integration is not loaded. Get the worktree
   path from the output and use absolute paths or `git -C <path>` / `cd <path> && ...` in every command:
   ```sh
   dir=$(flowtree task checkout 3)       # prints only the path; creates the worktree if missing
   dir=$(flowtree print-dir login)       # read-only: prints the path of an existing worktree
   dir=$(flowtree add -b spike --quiet)  # plain worktree on a new branch, prints only the path
   git -C "$dir" status
   ```
3. **Stay non-interactive.** There is no terminal, so the picker is unavailable: `switch`, `print-dir` and
   `remove` need a name that matches exactly one worktree, and `release finish`/`checkout` need a version unless you run
   them inside the release worktree. Use `flowtree init --flow -d`. Always pass `-m "<message>"` to
   `release finish` and to `task finish` of a hotfix task, in the tag message format below.
4. **Do not push** (`-p`, `--push`, `publish`) or delete remote branches unless the user asked. `finish`
   deletes the remote copy of the topic branch if one exists; use `--keepremote` when that is not wanted.
5. **Finish with flowtree, not by hand.** Do not `git merge`/`git branch -d`/`git worktree remove` a topic
   branch yourself; `finish` merges, tags, back-merges and cleans up in the right order. It merges inside
   the worktree that has the target branch checked out, usually the main checkout on develop: that checkout
   must have no uncommitted tracked changes (it refuses otherwise) and its files change. `finish` run from
   inside a topic worktree removes that worktree.
6. Commit your work in the topic worktree before `finish`: it refuses if
   tracked files are modified. Untracked files do not count.
7. Never use `-f`, `--force-branch`, `-D` or `delete -f`, and never remove a worktree the user did not ask
   about, without confirming: they discard uncommitted or unmerged work.

## Workflow

Check the repo is set up: any `flowtree release list` fails with "flow is not set up in this repository" if not.
Then run `flowtree init --flow -d` (it creates `master`/`develop` if missing and stores the settings in git config); confirm
with the user first if the repo already has branches or a different layout.

**Regular task**
```sh
flowtree task add "<title>"          # -> #<n>; only when the user asks for a new task, else take one from the board
flowtree task start <n> <name>       # regular/<name>; prints "A worktree for it was created at '<path>'"
# edit + commit inside <path>
flowtree task finish <n>             # run from anywhere; merge commit into develop, never rebases
```
If a pull request already merged the branch, `finish` only cleans up. Options: `-k` (keep branches), `--push`.
`review`, `diff`, `publish`, `rebase` and `pull` take an optional `<n>` (the task of the worktree you are in);
`checkout`, `finish` and `delete` require it; `task track <n> <remote branch>` checks out a branch from the remote
and links it. `rebase` and `pull` are for regular tasks only.

**Subtasks** (parallel work on one task; local branches, never pushed)
```sh
flowtree work start -P <n> -d "<brief>" <sub>   # work/<task's branch>/<sub>, own worktree
# commit inside its worktree
flowtree work finish <n>/<sub>                  # rebase onto the task's branch, fast-forward it, clean up
```
`work checkout` and `work delete` also take `<n>/<sub>`. Finish or delete every subtask before `task finish`: it
refuses while subtasks are open.

**Release**
```sh
flowtree release start 1.4.0      # only one release or hotfix may be open
# version bump, changelog, fixes; commit in the release worktree
flowtree release finish 1.4.0 -m "1.4.0

- <important point on the release, in short>
- <another one>"
```

**Hotfix**
```sh
flowtree task add --type hotfix "<title>"   # -> #<n>
flowtree task start <n> 1.4.1                 # hotfix/1.4.1 from master
# fix + commit in the worktree
flowtree task finish <n> -m "1.4.1

- <what the hotfix fixes, in short>"
```

`finish` merges into `master` with `--no-ff`, creates an annotated tag named exactly as the version you gave
`start` (`release start v1.4.0` tags `v1.4.0`), merges the tag into `develop`, then removes the worktree and
branch.

**Tag message format.** The first line is the tag name (the version as given, e.g. `v1.4.0`), then a blank
line, then one short bullet per important point of the release:

```
<tag>

- <important point on release in short>
- <important point on release in short>
- ...
```

Preview any `start`, `finish` or `delete` with `--dry-run`: it prints the steps and changes nothing.

**Inspect**: `flowtree task list`, `flowtree task diff [<n>]`, `flowtree release list -v`, `flowtree list`.

**Plain worktrees** (not flow branches): `flowtree add <branch>` or `flowtree add -b <new> [<start>]`, then
`flowtree remove [--with-branch] <name>`. Inside a flow repo do **not** create or remove `regular/`, `release/` or
`hotfix/` branches this way: that skips the base branch, the open-release check, tagging and the merge steps.
Use `flowtree task start|finish|delete` or `flowtree release start|finish|delete` for those.

**Board** (the repository's tasks; columns follow git, never move them by hand)
```sh
flowtree task list --json            # pick work from todo; skip tasks with "human": true or in the backlog
flowtree task prompt <n>[/<sub>]     # your brief: read it first and work from it
flowtree task show <n> --json        # description, subtasks, comments
flowtree task comment <n> "..."      # report progress
flowtree task start <n> <name>       # take task #n: its branch, in doing (a hotfix task takes a version)
flowtree task review [<n>]           # ready for review (a new commit moves it back to doing)
flowtree task add -d "..." "Title"   # only when the user asks for a new task
flowtree task sync                   # after branches were made with plain git (the only way a card is created for one)
```

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
