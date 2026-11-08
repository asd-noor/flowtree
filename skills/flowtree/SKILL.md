---
name: flowtree
description: Drives the flowtree CLI, which manages git worktrees and a git-flow variant where feature/*, release/* and hotfix/* branches each get their own worktree. Use when a repo has flowtree settings (`git config --get flowtree.develop` prints a value), or when asked to start, finish, publish or clean up feature/bugfix/release/hotfix work with git-flow semantics in isolated worktrees, tag a release, ship a hotfix, create or remove a worktree, run something in a worktree, set up worktree post-create hooks (copy .env, symlink, run a setup command), or move worktrees to another directory.
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
| `feature` (feature/bugfix/support) | `feature/<name>` | `develop` | `develop` | its own worktree |
| `release` | `release/<version>` | `develop` | `master` + tag, tag merged into `develop` | its own worktree |
| `hotfix` | `hotfix/<version>` | `master` | `master` + tag, tag merged into `develop` | its own worktree |
| `work` (subtask) | `work/<parent>/<name>` | its parent's tip | the parent: rebased, fast-forwarded | its own worktree, local only |

The production branch may be called `main`; the branch names and prefixes are in git config (`flowtree config` lists them).

## Rules for agents

1. **Layout rule.** The main checkout holds only `master` or `develop`; `feature/*`, `release/*` and `hotfix/*`
   live in their own worktrees. Never check out a topic branch in the main checkout yourself
   (`git checkout`/`switch`): start it with `flowtree <kind> start` and work in the worktree. If the rule gets
   broken, `<kind> checkout <name>` moves the branch out again.
2. **Your shell does not keep a `cd`** between tool calls and shell integration is not loaded. Get the worktree
   path from the output and use absolute paths or `git -C <path>` / `cd <path> && ...` in every command:
   ```sh
   dir=$(flowtree feature checkout login) # prints only the path; creates the worktree if missing
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
   must have no uncommitted tracked changes (it refuses otherwise) and its files change. `finish` run from
   inside a topic worktree removes that worktree.
6. Commit your work in the topic worktree before `finish`: it refuses if
   tracked files are modified. Untracked files do not count.
7. Never use `-f`, `--force-branch`, `-D` or `delete -f`, and never remove a worktree the user did not ask
   about, without confirming: they discard uncommitted or unmerged work.

## Workflow

Check the repo is set up: any `flowtree feature list` fails with "flow is not set up in this repository" if not.
Then run `flowtree init --flow -d` (it creates `master`/`develop` if missing and stores the settings in git config); confirm
with the user first if the repo already has branches or a different layout.

**Feature**
```sh
flowtree feature start <name>        # prints "A worktree for it was created at '<path>'"
# edit + commit inside <path>
flowtree feature finish <name>       # run from anywhere; merge commit into develop, never rebases
```
If a pull request already merged the branch, `finish` only cleans up. Options: `-k` (keep branches), `--push`.

**Subtasks** (parallel work on one feature or hotfix; local branches, never pushed)
```sh
flowtree work start -P feature/<name> <sub>   # work/feature/<name>/<sub> off the feature's tip, own worktree
# commit inside its worktree
flowtree work finish <sub>                    # rebase onto the feature, fast-forward it, clean up
```
Finish or delete every subtask before `feature finish` / `hotfix finish`: they refuse while subtasks are open.

**Release**
```sh
flowtree release start 1.4.0      # only one release or hotfix may be open
# version bump, changelog, fixes; commit in the release worktree
flowtree release finish -m "Release 1.4.0" 1.4.0
```

**Hotfix**
```sh
flowtree hotfix start 1.4.1
# fix + commit in the worktree
flowtree hotfix finish -m "Hotfix 1.4.1" 1.4.1
```

`finish` merges into `master` with `--no-ff`, creates an annotated tag (name = version, plus the configured tag
prefix), merges the tag into `develop`, then removes the worktree and branch.

Preview any `start`, `finish` or `delete` with `--dry-run`: it prints the steps and changes nothing.

**Inspect**: `flowtree feature list -v` (also `release`, `hotfix`), `flowtree feature diff <name>`, `flowtree list`.

**Plain worktrees** (not flow branches): `flowtree add <branch>` or `flowtree add -b <new> [<start>]`, then
`flowtree remove [--with-branch] <name>`. Inside a flow repo do **not** create or remove `feature/`, `release/` or
`hotfix/` branches this way: that skips the base branch, the open-release check, tagging and the merge steps.
Use `flowtree <kind> start|finish|delete` for those.

**Board** (the repository's tasks; columns follow git, never move them by hand)
```sh
flowtree task list --json            # pick work from todo; skip tasks with "human": true or in the backlog
flowtree task show <n> --json        # description, subtasks, comments
flowtree task comment <n> "..."      # report progress
flowtree feature start -T <n> <name> # take task #n: its branch, in doing
flowtree feature review <name>       # ready for review (a new commit moves it back to doing)
flowtree task add -d "..." "Title"   # only when the user asks for a new task
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
