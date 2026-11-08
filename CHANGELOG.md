# Changelog

All notable changes to flowtree are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims to follow
[Semantic Versioning](https://semver.org/); until 1.0.0 anything may change between minor versions.

## [v0.4.2] - 2026-11-08

### Changed

- `flowtree board` has a new look: rounded columns with the count in the title, a color per status taken from the
  terminal's 16 ANSI colors, two-line cards (title, then branch and tags), a boxed detail pane and a styled help bar.

## [v0.4.1] - 2026-11-08

### Fixed

- `flowtree board` no longer panics when the selected column is empty, which included every empty board.

## [v0.4.0] - 2026-11-08

### Changed

- **Breaking:** `FLOWTREE_PARENT` is renamed to `FLOWTREE_WORKDIR`; it works the same way (worktrees go to
  `$FLOWTREE_WORKDIR/<repo name>/<branch>` when `baseDir` is not set). Rename it in your shell rc.

## [v0.3.1] - 2026-11-08

### Changed

- `task prompt` no longer tells the worker to commit ("Work there only"); whoever hands out the brief
  sets the commit policy.

## [v0.3.0] - 2026-11-08

### Added

- Subtasks have a description, their brief: `work start -d`, `task edit <n>/<sub>`, shown by `task show`
  and `board`. `flowtree task prompt <n>[/<sub>]` prints a task or subtask as a Markdown brief (descriptions,
  branch, worktree path, how it is finished) to hand to an agent.
- `flowtree board`: `c` writes a comment on the selected task in your editor.
- A per-repository board: `flowtree task add|list|show|edit|comment|todo|backlog|human`, stored in
  `<git-common-dir>/flowtree-board.json` (shared by all worktrees, never pushed). A task's column follows git:
  todo or backlog without a branch, doing while the branch exists, review when asked at the current tip, done once
  merged into its target, abandoned when deleted unmerged. `list` and `show` take `--json`.
- `flowtree board`: a terminal kanban of the board for people, with a detail pane, editing in your editor and live
  reload. Agents keep using `flowtree task`.
- The flow keeps the board current: `feature`/`hotfix start -T <n>` links task #n to the new branch (without `-T`
  a task is added), `work start` adds a subtask, `finish` and `delete` record the last tip. New
  `feature`/`hotfix review [--undo]` marks a task ready for review until the next commit; `feature rebase` keeps
  the mark. A board failure only warns.
- `flowtree task sync [-n]` catches the board up with plain git: tasks for untracked feature/hotfix branches,
  subtasks for unknown work branches, relinking renamed branches by tip, and recording every tip.
- `work` branches: local subtasks of a feature or hotfix for parallel work, named `work/<parent>/<name>`.
  `work start [-P <parent>] <name>` starts one off the parent's tip (without `-P`: the parent whose worktree you
  are in, else a picker); `work finish` rebases it onto the parent and fast-forwards the parent. They cannot be
  published or tracked. `feature`/`hotfix` `finish` and `delete` refuse while subtasks are open. New setting
  `workPrefix` (default `work/`) and `init --work-prefix`.

### Changed

- **Breaking:** the `work` kind is now `feature` (`flowtree feature …`, branches `feature/<name>`, setting
  `featurePrefix`, `init --feature-prefix`). Existing `work/*` branches and `flowtree.workPrefix` are not migrated.
- `feature finish` always merges into develop with a merge commit and never rebases. It fetches first, and when
  develop or its remote copy already contains the branch (a merged pull request) it only cleans up. Its `-F`,
  `-r`/`--no-rebase`, `-p`, `-S` and `--no-ff` flags and the `flowtree.finishRebase` setting are gone.

- Release branches get their own worktree, like work and hotfix branches; the main checkout only ever holds
  master or develop. `release start`, `checkout` and `track` no longer switch the main checkout. A release
  that is still checked out in the main checkout from an older version is moved into a worktree by
  `release checkout` or `switch`, or finished in place as before.
- Only one release or hotfix can be open at a time: `release start` refuses while a hotfix is open, and the
  other way round.
- **Breaking:** the version tag prefix is gone (`flowtree.versionTagPrefix`, `init -t`). A release or hotfix is
  tagged with its version exactly as given: `release start v1.2.0` makes `release/v1.2.0` and tags `v1.2.0`.
  A leftover `flowtree.versionTagPrefix` is now an unknown key; remove it with
  `git config --unset flowtree.versionTagPrefix`.

## [0.2.1]

### Fixed

- The interactive picker now uses the terminal's alternate screen, so its output is removed when it closes.

## [0.2.0] - 2026-10-04

### Added

- `flowtree.finishRebase` (default `true`) sets whether `work finish` rebases first; `finish --rebase` (`-r`) and
  `--no-rebase` override it for one run.
- `work finish` warns when its rebase rewrote commits that were already on the remote copy of the branch.

### Changed

- `work finish` now rebases the branch onto develop before merging, so conflicts surface in the feature worktree and
  the merge is clean and, by default, linear. Pass `--no-rebase` to merge as is.
- `work finish -S` skips the rebase, since the squash discards the branch's commits anyway; `-r` still forces it.

## [0.1.0] - 2026-10-03

First release.

### Added

- **Worktree management**, modelled on [wtp](https://github.com/satococoa/wtp): `add` (existing, remote-only or new
  branches; `--exec`; `--quiet`), `list`, `remove` (`--with-branch`, `--force-branch`), `exec`, and `print-dir`
  (a read-only `cd`). Post-create hooks (`copy`, `symlink`, `command`) run for every worktree flowtree creates.
- **git-flow with a worktree per topic branch**: `flowtree work`, `release` and `hotfix` with `start`, `finish`,
  `list`, `checkout`, `publish`, `track` and `delete`, plus `work diff`, `work rebase` and `work pull`.
  - `finish` merges in the worktree that has the target branch (or a throwaway one), aborts conflicting merges
    without changing anything, tags releases and hotfixes, merges the tag back into develop, cleans up, and can
    push. It is resumable after a failed back-merge or tag.
  - Work branches support fast-forward, `--no-ff`, squash and rebase-first merges; releases and hotfixes support
    signed tags, tag messages, `--notag`, `--nobackmerge`, custom tag names and `--ff-master`.
  - `--dry-run` on `start`, `finish` and `delete` prints the planned steps.
  - The main checkout rests on develop and only holds master, develop or a release branch; stray topic
    branches are moved out into their own worktree.
- **`switch`**: go where a branch lives, moving the main checkout onto master, develop or a release, or creating
  a missing work or hotfix worktree. A query that matches exactly one row (exact, substring, then fuzzy) needs no
  picker.
- **A built-in picker** (bubbletea) for `switch`, and for `print-dir`, `remove`, `finish` and `checkout` when no
  name is given on a terminal.
- **`init`** and **`init --flow`**: set up the worktree directory and git-flow (branch names, prefixes, version
  tag prefix, remote), creating `master` and `develop` when missing.
- **`config`**: show settings with their origin, get, set (`--global` too) and unset them, and show where the
  hooks file lives.
- **`relocate`**: move topic worktrees after the base directory changed.
- **Settings in git config** (`flowtree.*`, local overrides global). `FLOWTREE_PARENT` sets the default worktree
  directory to `<parent>/<repo name>`. Hooks live in `<git dir>/flowtree-hooks.json`. Unknown keys are errors.
- **Shell integration**: `shell-init` for bash, zsh and fish (a wrapper that `cd`s you, plus completion) and
  `completion <shell>`; dynamic completion of worktrees, branches, flow names, remotes and config keys.
- **An agent skill**, embedded in the binary and installed with `flowtree install-skill <dir>`.
- Documentation in `docs/` and an end-to-end test suite that drives the real binary against real git repositories.

### Requirements

- git 2.17 or newer (2.18 for `rebase -p`, 2.31 for `relocate` to skip locked worktrees).
- Built without cgo; cross-compiles for linux, macOS (amd64, arm64) and other Go targets.

### Known limitations

- Install from a clone (`mise run install` or `go install ./cmd/flowtree`); the module is named plain `flowtree`,
  so `go install ...@latest` from a remote does not work.
- Not covered by automated tests: the picker on a real terminal, the editor for tag messages on a terminal,
  signed tags, `rebase -i`, and the zsh wrapper.
- Windows compiles but is untested: hooks run through `sh -c`, and the shell wrapper covers bash, zsh and fish only.
- Only one remote is used (`flowtree.remote`, default `origin`).
- `flowtree config` cannot repair a typo'd `flowtree.*` key in git config; remove it with `git config --unset`.
