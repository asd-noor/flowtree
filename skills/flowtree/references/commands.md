# flowtree command reference

`<kind>` is `work`, `release` or `hotfix`. `<name>` is the branch name without its prefix; for `release` and
`hotfix` it is the version. With no name, `finish`, `publish`, `diff`, `rebase` and `checkout` use the branch of
the worktree you are in. `work` names also accept a unique prefix. Worktree names (for `print-dir`, `exec`,
`remove`) can be `@` (the main worktree), a branch, a directory name, or an absolute path.

## Setup

| Command | Effect |
|---------|--------|
| `init [--flow] [-d] [-f] [-w <dir>]` | Store settings in the repository's git config (nothing is written to the working tree). `-d` = no prompts (use this). `--flow` also sets up git-flow: creates `master`/`develop` if missing and stores the flow settings. `-w` sets the worktree directory (stored only when it differs from the default). Existing settings are left alone (exit 0) except that `--flow` adds missing flow settings; `-f` resets them first. |
| `config [<key> [<value>]] [--global] [--unset]` | List settings with their origin, print one, set one (this repo, or `--global`) or remove one. Keys: `baseDir`, `remote`, `master`, `develop`, `workPrefix`, `releasePrefix`, `hotfixPrefix`, `versionTagPrefix`. Unknown keys are rejected. |
| `config hooks` | Print the path of the post-create hooks file and its contents (or an example when missing). |
| `init --flow` options | `--master <b>`, `--develop <b>`, `-p <work prefix>`, `-r <release prefix>`, `-x <hotfix prefix>`, `-t <version tag prefix>`, `--remote <name>`. |
| `shell-init [bash\|zsh\|fish]` | Prints the shell function that follows worktrees with `cd`, plus completion. For humans; not useful to agents. |

## Worktrees

| Command | Effect |
|---------|--------|
| `add <branch>` / `add -b <new> [<commitish>]` | Create a worktree at `<baseDir>/<branch>` (existing branch, remote-only branch tracked, or a new one). Hooks run. `-q` prints only the path; `--exec <cmd>` runs a command in it afterwards. |
| `list [-q]` | Worktrees with path, branch and HEAD; `-q` prints names only. |
| `remove [-f] [--with-branch [--force-branch]] <worktree>...` | Remove worktrees. Never the main one, nor the one you are in (unless shell integration is active). `--with-branch` also deletes the branch if merged. |
| `print-dir [<worktree>]` | Print the absolute path of an existing worktree; no name or `@` = main. Read-only. |
| `exec <worktree> -- <command> [args...]` | Run a command in a worktree; exits with its exit code. |
| `switch <query>` | Print the directory of a branch, moving the main checkout onto master/develop/release or creating a missing work/hotfix worktree if needed. A query must match one row exactly or by substring/fuzzy uniquely; otherwise it opens a picker (fails without a terminal). |
| `relocate [-n]` | Move work/hotfix worktrees to `<baseDir>/<branch>` after `baseDir` changed. `-n` = dry run. Skips locked/missing/occupied; refuses if cwd is inside one that would move. |

## Common to all kinds

| Command | Effect |
|---------|--------|
| `<kind> list [-v]` | List branches of this kind; `*` marks the current one; `-v` adds state (no commits yet / based on latest / may be rebased / merged). |
| `<kind> start [-F] [--dry-run] <name> [<base>]` | New branch + worktree (`release`: new branch, main worktree switched onto it, which must be clean). Base defaults to `develop` (`work`, `release`) or `master` (`hotfix`). `-F` fetches first. |
| `<kind> checkout [<name>]` | Print the worktree path, creating the worktree if the branch has none. For `release`: switch the main worktree onto the branch and print its path. |
| `<kind> publish [<name>]` | Push the branch to the remote and set the upstream. |
| `<kind> track <name>` | Check out a branch that exists on the remote (for `release`: switch the main worktree onto it). |
| `<kind> delete [-f] [-r] [--dry-run] <name>` | Remove worktree and local branch. Refuses if unmerged unless `-f`. `-r` also deletes the remote branch. |

## work

| Command | Effect |
|---------|--------|
| `work finish [-F] [-r] [-p] [-k] [--keeplocal] [--keepremote] [-D] [-S] [--no-ff] [--push] [--dry-run] [<name>]` | Merge into `develop`, then clean up. |
| `work diff [<name>]` | Changes on the branch since it left `develop`. |
| `work rebase [-p] [<name>]` | Rebase onto `develop` inside the worktree (`-i` needs a terminal; do not use it). |
| `work pull <remote> [<name>]` | Fast-forward pull of the branch from `<remote>`, creating the branch and worktree if missing. |

Flags: `-F` fetch first; `--no-rebase` skips the rebase onto develop before merging and `-r` forces it (default: `flowtree.finishRebase`, on); `-p` preserve merges while rebasing; `-k` keep
local and remote branch (the worktree is still removed); `--keeplocal`/`--keepremote` keep only that copy;
`-D` force worktree removal even with untracked files; `-S` squash; `--no-ff` always create a merge commit;
`--push` push `develop` afterwards. The default merge is a fast-forward when possible.

## release / hotfix

`<kind> finish [-F] [-s] [-u <key>] [-m <msg> | -f <file>] [-p] [-k] [--keeplocal] [--keepremote] [-D] [-n] [-b] [-S] [-T <tagname>] [--ff-master] [--dry-run] [<version>]`

- `-m`/`-f` tag message (pass one); `-s`/`-u` sign the tag; `-n` no tag; `-T` custom tag name.
- `-b` skip the back-merge into `develop`; `-S` squash into master; `--ff-master` (release only) fast-forward master.
- `-p` pushes `develop`, `master` and the tag to the remote.
- Order: merge into `master` (`--no-ff`) → tag → merge the tag into `develop` → remove worktree/branch → push.
- Re-running `finish` after a failed back-merge resumes (see troubleshooting).
- `release start`/`hotfix start` fail if another branch of that kind is open or the tag already exists.
