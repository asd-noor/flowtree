# flowtree troubleshooting

Messages are printed as `Error: <message>` on stderr; the first letter is lowercase.

| Message | Meaning | Fix |
|---------|---------|-----|
| `flow is not set up in this repository; run 'flowtree init --flow'` | `flowtree.master` / `flowtree.develop` are not set in git config. | `flowtree init --flow -d` (confirm with the user if the repo already has branches). |
| `could not merge 'X' into 'Y': ...` followed by `Nothing was changed.` | The merge was aborted. Git's own message says why. | Conflicts: in the topic worktree `git merge Y`, resolve, commit, re-run `finish`. "untracked working tree files would be overwritten": move or delete those files in Y's worktree, re-run. |
| `finish was aborted due to conflicts during rebase` | `work finish` rebases first; it stopped and the rebase was aborted. | Rebase by hand in that worktree (`git rebase develop`), resolve, `git rebase --continue`, re-run `finish`. To merge without rebasing, pass `--no-rebase` (or set `flowtree.finishRebase false`). |
| `working tree '<path>' contains unstaged or uncommitted changes. Aborting` | The topic worktree, or the worktree holding the target branch (`develop`/`master`, often the main checkout), has tracked changes. | Commit or stash there, re-run. Untracked files do not count. |
| `there is an existing release branch 'X'; finish that one first` (also hotfix) | Only one of each may be open. | Finish or `delete` the open one (ask the user which). |
| `tag 'X' already exists; pick another name` | Version already tagged. | Choose another version, or `-T <name>` on finish. |
| `branch 'X' is behind 'origin/X'` | A long-lived branch is behind the remote. | Update it first: `git -C <worktree-of-X> pull --ff-only` (or `git fetch origin X:X` if it is not checked out anywhere). |
| `branches 'X' and 'origin/X' have diverged` | Local and remote history differ. | Stop and ask the user; do not force anything. |
| `unknown setting 'flowtree.x' (known: ...)` | A `flowtree.*` key in some git config scope is misspelled or does not exist. | Remove it: `git config --unset flowtree.x` (add `--global` if it is in the global config). |
| `branch 'X' already exists; pick another name` / `... already exists on origin; use 'flowtree <kind> track <name>'` | The name is taken locally or on the remote. | Pick another name, or `track` the remote one. |
| `Warning: could not remove worktree ... Branch 'X' kept` | The worktree has untracked/ignored files. | Inspect them; if disposable, re-run `finish -D`, or `git worktree remove --force <path>` then `git branch -D <branch>`. |
| `Warning: could not delete 'X' from origin` | The remote refused (protected branch, no permission). | Informational; the local finish succeeded. |
| `cannot remove 'X': you are inside it` | cwd is in the worktree to remove. | `cd` to the main worktree first. |
| `you are inside the worktree of 'X'` (relocate) | cwd is in a worktree that would move. | Run from the main checkout. |
| `branch 'X' has not been merged into 'develop'` (delete) | Unmerged work. | Confirm with the user, then `delete -f`. |
| `no terminal: pass a name` | The command wanted the picker. | Pass a name that matches exactly one row (or use `<kind> checkout <name>`). |

## Partial finish

If `release finish` / `hotfix finish` reports a conflict while merging the **tag into develop**, `master` is
already merged and tagged. Merge the tag into `develop` by hand in a worktree that has `develop` checked out
(for example `git worktree add <tmp> develop && git -C <tmp> merge <tag>`), remove that temporary worktree, then
run the same `finish` command again: it prints "Resuming...", skips the merge and the tag, and cleans up.

## Checking state

```sh
flowtree config           # settings and where they come from
flowtree work list -v     # also: release list, hotfix list
flowtree list             # worktrees
git worktree list         # ground truth for where worktrees are
git branch --list 'work/*' 'release/*' 'hotfix/*'
```
flowtree always asks git where a worktree is, so worktrees outside the configured directory still work.
