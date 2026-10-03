package commands

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"flowtree/internal/config"
	"flowtree/internal/skill"
	"flowtree/skills"
)

func newInstallSkillCmd(d Deps) *cobra.Command {
	var o skill.Options
	var uninstall bool
	cmd := &cobra.Command{
		Use:   "install-skill <dir>",
		Short: "Install the flowtree agent skill into a skills directory",
		Long: `Writes the agent skill that is embedded in this binary to <dir>/flowtree (SKILL.md and references/),
for example ~/.claude/skills or ~/.pi/agent/skills. It teaches an agent the branch model, to work inside
worktrees, to stay non-interactive and to leave pushing to you.

An existing <dir>/flowtree is replaced when it is a flowtree skill; anything else there is left alone unless
--force is given. --uninstall removes it again. A leading ~ in <dir> is expanded.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dest := absDir(config.ExpandHome(args[0]))
			if uninstall {
				return runUninstallSkill(d, dest, o)
			}
			return runInstallSkill(d, dest, o)
		},
	}
	cmd.Flags().BoolVarP(&o.Force, "force", "f", false, "replace or remove a directory that is not a flowtree skill")
	cmd.Flags().BoolVarP(&o.DryRun, "dry-run", "n", false, "show what would be written without writing it")
	cmd.Flags().BoolVar(&uninstall, "uninstall", false, "remove the installed skill instead")
	return cmd
}

func absDir(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

func runInstallSkill(d Deps, dest string, o skill.Options) error {
	done, err := skill.Install(skills.FS, dest, o)
	for _, in := range done {
		verb := "Installed"
		switch {
		case o.DryRun:
			verb = "Would install"
		case in.Replaced:
			verb = "Replaced"
		}
		d.Printf("%s skill '%s' in %s (%d files)\n", verb, in.Name, in.Dir, len(in.Files))
	}
	if err != nil {
		return err
	}
	if len(done) == 0 {
		return fmt.Errorf("no skills embedded in this binary")
	}
	return nil
}

func runUninstallSkill(d Deps, dest string, o skill.Options) error {
	removed, err := skill.Uninstall(skills.FS, dest, o)
	for _, in := range removed {
		verb := "Removed"
		if o.DryRun {
			verb = "Would remove"
		}
		d.Printf("%s skill '%s' from %s\n", verb, in.Name, in.Dir)
	}
	if err == nil && len(removed) == 0 {
		d.Printf("Nothing to remove in %s\n", dest)
	}
	return err
}
