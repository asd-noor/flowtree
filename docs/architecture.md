# Architecture

flowtree is a Go CLI built on [cobra](https://github.com/spf13/cobra) and, for the picker,
[bubbletea](https://github.com/charmbracelet/bubbletea). Everything else is the standard library. Git is run
as a subprocess; worktrees and merges need real git behaviour, so there is no go-git.

## Layout

```
cmd/flowtree/        main: builds the root command, maps errors to exit codes
skills/              the agent skill (SKILL.md + references), embedded with go:embed
internal/
  gitx/              the git seam: Runner, branch and worktree operations, porcelain parser
  config/            settings in git config (read, set, validation), hooks file, base dir resolution
  worktree/          worktree naming and name resolution ("@", branch, directory, display name)
  hooks/             post-create hooks: copy, symlink, command
  flow/              the git-flow domain; imports no CLI package
  picker/            matcher (exact / substring / fuzzy) and the bubbletea list
  shell/             shell-init scripts
  skill/             installs the embedded agent skills into a skills directory
  commands/          cobra commands; the only package that knows about flags and output
  testutil/          real temporary git repositories for tests
test/e2e/            builds the binary and drives it against temporary repositories
```

Dependencies point one way: `commands` → `flow` → `gitx`/`config`/`hooks`; `worktree` and `picker` are leaves.
`flow` has no cobra, picker or terminal code, so it can be tested with real repositories and nothing else.

## The git seam

```go
type Runner func(ctx context.Context, dir string, args ...string) (string, error)  // trimmed stdout
type Git    func(args ...string) (string, error)                                   // a Runner bound to ctx and dir
```

`gitx.Exec` is the real Runner. Every function takes a `Git` (`gitx.BranchExists(g, name)`,
`gitx.AddWorktree(g, opts)`), and `Git.Ok` and `Git.Lines` cover the common shapes. A failed call returns
`*gitx.Error` carrying git's stderr, and branch lookup failures are `*gitx.BranchError`. Tests inject a fake
Runner or use the real one against a temp repository.

Only interactive commands bypass the seam (`work rebase -i`, `exec`), because they need the terminal.

## Commands receive their world

`commands.Deps` holds the process-level collaborators: the Runner, stdin/stdout/stderr, `Getenv`, `Cwd`,
`IsTTY` and `Pick`. Commands hold no state of their own. `Deps.OpenRepo` finds the main worktree from the current
directory (it also works from a linked worktree), loads the settings (`config.Load`) and lists worktrees; `Deps.OpenFlow`
additionally requires flow settings and returns a `*flow.Env`.

Commands are constructors, `func(Deps) *cobra.Command`, assembled in `commands/root.go`. There is no `init()`
registration and no package-level command. Topic commands are generated per kind: `newTopicCmd(flow.Work)`.

## Plans and steps

Mutating flow commands are split in two. A **plan** function checks what can be checked (names resolve, worktrees
are clean, tags are free) and returns a list of steps; a **runner** executes them.

```go
type Step struct { Desc string; Do func(*Env) error }

flow.Plan(
    flow.If(o.Fetch, flow.One("fetch origin", Fetch)),
    flow.One("check master and develop against origin", ...),
    flow.One("merge release/1.0 into master (--no-ff)", ...),
    flow.If(!o.NoTag, flow.One("tag master as 1.0", ...)),
    ...
)
```

`flow.Run` runs the steps in order and stops at the first error; `flow.Describe` prints them, which is all
`--dry-run` is. Checks that depend on state a previous step changes (fetch, sync, remote branches) are steps
themselves, so they run at the right time.

`flow.Env` carries the context, Runner, settings, config, paths and writers, and `CdTo`, the hook through which a
command asks the shell to change directory.

## Layout rules and safe merging

`flow/layout.go` enforces the main-checkout rule: `EnsureWorktree` creates (or finds) a worktree for a work or
hotfix branch and evacuates the main checkout if the branch was checked out there; `TopicDir` does the same for
releases, which live in the main checkout; `SwitchMain` checks out a branch in the main worktree, requiring it to
be clean.

`flow/merge.go` is why `finish` never disturbs you. `RunOn(branch, fn)` runs `fn` in the worktree that has the
branch checked out (which must be clean), or in a throwaway worktree created in the temp directory and always
removed. `MergeInto(target, source, mode)` builds on it; a failed merge runs `merge --abort` (or `reset --hard` for
squash), so a conflict changes nothing.

`flow/finish.go` makes release and hotfix finish resumable: `alreadyMerged` recognises a finish that was
interrupted after the merge into master (and tag) and lets the plan skip those steps.

## The cd-file protocol

A process cannot change its parent's directory. Commands that should move you call `Env.CdTo(path)`, which writes
the path to the file named by `FLOWTREE_CDFILE` if set. The shell wrapper creates that file, runs the binary and
`cd`s afterwards. One mechanism serves every command, replacing per-command stdout parsing. See
[shell-integration.md](shell-integration.md).

## Configuration

Settings live in git config, section `flowtree`, so they need no file of their own and follow git's scopes
(local over global over system). `config.Load` makes one call, `git config --null --get-regexp '^flowtree\.'` (no
`--show-scope`, which needs git 2.26; `config.Entries`, used by `flowtree config` and `init`, works out each value's
scope from per-scope reads), and applies each key through a table of settings (`config/keys.go`: canonical name, validator,
default). An unknown key is an error, which catches typos. `config.Set` and `Unset` validate before calling
`git config`. Flow settings are the `config.Flow` part of `Config`, converted to `flow.Settings` by
`flow.FromConfig`, which fails with a pointer to `init --flow` unless both `master` and `develop` are set.

`baseDir` has no stored default: `Config.ResolveBaseDir` uses it when set, else `$FLOWTREE_PARENT/<repo name>` when
the variable is defined (captured as `Config.Parent` at load time), else `../worktrees`. That is why `init` stores
the value only when it differs.

Hooks have structure git config cannot express, so they are read from `<git-common-dir>/flowtree-hooks.json`
(strict `encoding/json`). The file sits inside `.git`: shared by all worktrees, never committed, so a clone cannot
inject command hooks.

## The picker

`picker.Resolve(rows, query)` is the no-UI path: case-insensitive tiers (exact, substring, fuzzy), where the first
tier with exactly one matching row wins. `picker.Pick` is the bubbletea program: a filter input, a list, and a
lazily computed preview pane. The model's `Update`/`View` are pure and tested without a terminal. `Deps.Pick` is
the seam commands use, so tests substitute a fake and never start a TUI.

## Errors and exit codes

Commands return errors; `main` prints `Error: <message>` to stderr and exits 1. `exec` returns an error that
carries the child's exit code, which `main` uses without printing anything.

## Origins

The worktree commands and hooks are a Go port of parts of [wtp](https://github.com/satococoa/wtp) (MIT; see
`THIRD_PARTY.md`). The flow semantics come from the author's bash script wtp-flow, which called wtp as a
subprocess. flowtree replaces both with one binary.
