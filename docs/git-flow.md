# The branch model

flowtree implements git-flow (AVH edition semantics) with one change: topic branches live in their own git
worktrees, so you `cd` between them instead of switching branches in one checkout.

## Branches

| Kind | Prefix | Starts from | `finish` merges into | Lives in |
|------|--------|-------------|----------------------|----------|
| production | `master` (or `main`) | | | the main checkout |
| integration | `develop` | | | the main checkout |
| `work` | `work/` | develop | develop | its own worktree |
| `release` | `release/` | develop | master (tagged), then the tag into develop | its own worktree |
| `hotfix` | `hotfix/` | master | master (tagged), then the tag into develop | its own worktree |

Names and prefixes are settings (`flowtree init --flow`, see [configuration.md](configuration.md)). `work`
replaces git-flow's `feature`, `bugfix` and `support`; there is no `feature` alias.

## The main-checkout rule

The main checkout (the directory you cloned into) rests on `develop`. It only ever holds `master` or
`develop`. `work/*`, `release/*` and `hotfix/*` branches each get a worktree at `<baseDir>/<branch>`.

If you check a topic branch out in the main checkout by hand, the commands that need its worktree
(`checkout`, `switch`, `rebase`, `pull`) move the main checkout back (master for hotfix, else develop; it
must be clean), give the branch a worktree and say so on stderr.

## Lifecycle

```
flowtree init --flow                 # once per clone: settings, master + develop

flowtree work start login            # work/login from develop, in <baseDir>/work/login
flowtree work finish                 # merge into develop, remove worktree and branch

flowtree release start 1.2.0         # release/1.2.0 from develop, in its own worktree
flowtree release finish -m "1.2.0"   # merge to master, tag, merge the tag into develop

flowtree hotfix start 1.2.1          # hotfix/1.2.1 from master, in its own worktree
flowtree hotfix finish -p            # like release finish, then push develop, master and the tag
```

## `finish`

**Work**

1. optional `-F` fetch, then refuse if develop is behind or has diverged from the remote;
2. rebase of the branch onto develop (skipped with `-S`, which discards the commits anyway; `-r` forces it) (`-p` keeps merges; `--no-rebase` skips it; set `flowtree.finishRebase false` to make that the default, and `-r` forces it per run), inside its worktree. A conflict aborts the rebase and finish stops before merging;
3. merge into develop: fast-forward when possible, `--no-ff` for a merge commit, `-S` to squash;
4. clean up (below);
5. optional `--push` of develop.

**Release and hotfix**

1. optional fetch; refuse if master or develop is behind or has diverged; refuse if the tag already exists;
2. merge into master with `--no-ff` (`--ff-master` for releases fast-forwards, `-S` squashes);
3. tag master: annotated, `-s`/`-u` sign it, `-m`/`-f` set the message, `-n` skips the tag, `-T` renames it.
   With no message and a terminal, git opens your editor; without a terminal a default message is used;
4. merge the **tag** into develop (`-b` skips this; with `-n` it merges master);
5. clean up; optional `-p` push of develop, master and the tag.

**Clean up** removes the worktree, the local branch and the branch on the remote. `-k` keeps both branches
(the worktree is still removed and `checkout` recreates it); `--keeplocal` and `--keepremote` keep one;
`-D` removes a worktree that has untracked files. A topic branch checked out by hand in the main checkout is not
removed: the main checkout goes back to develop.

### Safety properties

- `finish` merges inside the worktree that already has the target branch checked out, or in a throwaway
  worktree that is always removed afterwards. That worktree is often the main checkout (it rests on develop): it
  must have no uncommitted changes to tracked files, and the merge does update its files. What `finish` does not
  do is switch the branch of the directory you ran it from: the exceptions are that finishing a topic branch
  that was checked out by hand in the main checkout returns the main checkout to develop, and that
  finishing from inside a topic worktree removes that worktree (your shell is sent to the main checkout).
- A conflicting merge is aborted and nothing changes, instead of being left half done. Resolve it on the topic
  branch (`git merge develop`) and run `finish` again.
- `finish` is **resumable**. If the back-merge into develop conflicts after master was merged and tagged, merge
  the tag into develop by hand, then run `finish` again: it sees that the branch is already merged and tagged
  (master is exactly the tagged commit), prints "Resuming...", skips the merge, the tag and an already-done
  back-merge, and finishes the cleanup.
- `start` refuses if a release or hotfix is already open (only one of the two at a time: a hotfix fixes a shipped
  release, so they never overlap), if the tag exists, if the branch exists locally, or if
  it exists on the remote (use `track`).
- Finishing and starting refuse when master or develop is behind or has diverged from the remote.

## Names

`finish`, `publish`, `checkout`, `rebase` and the other per-branch commands take the name without its prefix
(`login`, not `work/login`; the prefix is accepted too). Work names also accept a unique prefix of the name.
With no name they use the branch of the worktree you are in. Outside such a worktree, on a terminal, `finish`
and `checkout` open a picker over the open branches of that kind.

## Differences from git-flow

- `start`, `track` and `checkout` create a worktree instead of switching your checkout.
- A release and a hotfix cannot be open at the same time.
- `bugfix` and `support` are folded into `work`, so support-style branches start from develop, not master.
- A conflicting merge is aborted rather than left in progress.
- While a release is open, `work finish` and `hotfix finish` still merge into develop, through a throwaway
  worktree. `hotfix finish` back-merges into develop only, as AVH does.
- Settings live in the `flowtree` section of git config (`flowtree config`), hooks in a file inside `.git/`.

## Known limits

- Signed tags (`-s`, `-u`) are untested.
- With no `-m` on a terminal, git opens your editor for the tag message; that path is untested.
- Only one remote is used (the `remote` setting, default `origin`).
