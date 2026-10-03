# Changelog

All notable changes to flowtree are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project aims to follow
[Semantic Versioning](https://semver.org/); until 1.0.0 anything may change between minor versions.

## [Unreleased]

### Changed

- `work finish` now rebases the branch onto develop before merging, so the merge is always clean and, by default,
  linear. Pass `--no-rebase` to merge as is, or set
  `flowtree.finishRebase false` to make that the default (`-r` then forces the rebase for one run). `-S` skips the rebase, and a warning is printed when the rebase
  rewrote commits that were already pushed.

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
