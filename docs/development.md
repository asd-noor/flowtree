# Development

## Tooling

The toolchain is pinned by `mise.toml` (Go 1.27.1, kept in sync with `go.mod`). Tasks:

```sh
mise run build     # bin/flowtree, with the version from git describe
mise run fmt       # fails if a tracked .go file needs gofmt
mise run vet       # go vet ./...
mise run test      # go test ./...
mise run check     # fmt, vet and test: what CI should run
mise run tidy
```

Plain `go build ./cmd/flowtree` and `go test ./...` work too. The only third-party dependencies are cobra and
bubbletea (plus their dependencies); add one only with a reason.

## Tests

- **Unit tests** sit next to the code and use the standard `testing` package, no assertion library.
- **Real repositories**: `internal/testutil` creates temporary git repositories (`NewRepo`, `NewRepoWithRemote`)
  with identity and config isolated from yours. Most tests, including the flow package's, run real git.
- **Commands**: tests build the root with `commands.NewRoot(deps, ...)` and `Deps` pointing at buffers, a fake
  `Pick`, and a controlled `Cwd`/`Getenv`/`IsTTY`.
- **End to end**: `test/e2e` builds the binary once per run and drives it in temporary repositories with an
  isolated environment (`e2e.Run`, `e2e.RunEnv`). Flow lifecycle, dry runs, conflict aborts, resume, switch and
  completion are covered there.
- **Shell wrappers**: `internal/shell` runs the bash and fish wrappers against a fake `flowtree`; each test is
  skipped when its shell is missing. The zsh wrapper is not tested.

Not covered by automated tests: the interactive TUI against a real terminal, `rebase -i`, signed tags, and the
editor path of annotated tags.

## Conventions

The repository follows `CLAUDE.md`: functional style, composition over inheritance, short functions, higher
order functions and chaining, recursion or iterators where they read well, clear package boundaries.
In practice:

- git is only reached through `gitx.Git` values passed in; no package-level state.
- Plans return `[]flow.Step`; effects live in step functions, checks in the plan.
- Commands are constructors taking `Deps`; they contain flag handling and output only.
- Errors are wrapped with `%w` and read as a message to the user, with a hint when one exists.
- Prefer `slices`, `maps` and small helpers over loops with mutation.

## Adding a flow subcommand

1. Domain first: add `PlanXxx(e *Env, o XxxOpts) ([]Step, error)` (or a plain function when it has no steps worth
   describing) under `internal/flow`, with tests against real repositories.
2. Add `newXxxSub(k flow.Kind, d Deps) *cobra.Command` in `internal/commands/topic.go` (or a new file), using
   `runPlan` for plans and `withFlow` otherwise, and register it in `newTopicCmd`.
3. Give it a `ValidArgsFunction` (`completeTopics`, `completeRemoteTopics`, ...).
4. Add an e2e test in `test/e2e`.
5. Document it in `docs/commands.md`.

## Adding a top-level command

Write `newXxxCmd(Deps) *cobra.Command`, add it to `Constructors` in `internal/commands/root.go` (keep the list
sorted), add completion and tests.

## Release

Not set up yet: there is no goreleaser or Homebrew configuration. `mise run build` embeds the version with
`-ldflags "-X main.version=..."`.
