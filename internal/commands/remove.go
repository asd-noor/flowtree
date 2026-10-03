package commands

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"flowtree/internal/gitx"
	"flowtree/internal/worktree"
)

type removeOpts struct {
	force, withBranch, forceBranch bool
}

func newRemoveCmd(d Deps) *cobra.Command {
	var o removeOpts
	cmd := &cobra.Command{
		Use:     "remove [flags] <worktree>...",
		Aliases: []string{"rm"},
		Short:   "Remove worktrees",
		Long: "Removes the named worktrees.\n\n" +
			"Examples:\n" +
			"  flowtree remove feature/old\n" +
			"  flowtree remove --force feature/dirty\n" +
			"  flowtree remove --with-branch feature/done",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.New("worktree name required; usage: flowtree remove <worktree>... (see 'flowtree list')")
			}
			return nil
		},
		ValidArgsFunction: completeWorktrees(d),
		RunE: func(cmd *cobra.Command, args []string) error {
			if o.forceBranch && !o.withBranch {
				return errors.New("--force-branch requires --with-branch")
			}
			repo, err := d.OpenRepo(cmd.Context())
			if err != nil {
				return err
			}
			return errors.Join(mapSlice(args, func(name string) error { return removeOne(d, repo, o, name) })...)
		},
	}
	f := cmd.Flags()
	f.BoolVarP(&o.force, "force", "f", false, "Force removal even if the worktree is dirty")
	f.BoolVar(&o.withBranch, "with-branch", false, "Also delete the branch (only if merged)")
	f.BoolVar(&o.forceBranch, "force-branch", false, "Delete the branch even if unmerged (requires --with-branch)")
	return cmd
}

func removeOne(d Deps, repo Repo, o removeOpts, name string) error {
	target, err := worktree.Resolve(repo.Worktrees, name)
	if err != nil {
		return err
	}
	if target.Main {
		return fmt.Errorf("cannot remove the main worktree (%s)", target.Path)
	}
	inside := isWithin(target.Path, repo.Cwd)
	if inside && d.Getenv(CdFileEnv) == "" {
		return fmt.Errorf("cannot remove '%s': you are inside it (%s); cd out first, or enable shell integration (flowtree shell-init)", name, target.Path)
	}
	if err := gitx.RemoveWorktree(repo.Git, target.Path, o.force); err != nil {
		return fmt.Errorf("remove worktree %s: %w", target.Path, err)
	}
	d.Printf("Removed worktree '%s' at %s\n", name, target.Path)
	if inside {
		writeCdFile(d, repo.Main)
	}
	return removeBranch(d, repo, o, target)
}

func removeBranch(d Deps, repo Repo, o removeOpts, target gitx.Worktree) error {
	if !o.withBranch || target.Branch == "" {
		return nil
	}
	if err := gitx.DeleteBranch(repo.Git, target.Branch, o.forceBranch); err != nil {
		return fmt.Errorf("worktree removed, but branch '%s' was not deleted (use --force-branch if unmerged): %w", target.Branch, err)
	}
	d.Printf("Removed branch '%s'\n", target.Branch)
	return nil
}
