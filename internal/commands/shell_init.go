package commands

import (
	"github.com/spf13/cobra"

	"flowtree/internal/shell"
)

func newShellInitCmd(d Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "shell-init [bash|zsh|fish]",
		Short: "Print shell integration: cd-following wrapper and completion",
		Long: "Prints a script that defines a `flowtree` function (so commands like `switch` and `work start` " +
			"can change your directory) and enables tab completion.",
		Example:   "  eval \"$(flowtree shell-init bash)\"      # ~/.bashrc\n  eval \"$(flowtree shell-init zsh)\"       # ~/.zshrc\n  flowtree shell-init fish | source       # config.fish",
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: shell.Supported,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := shell.Detect(d.Getenv("SHELL"))
			if len(args) == 1 {
				name = args[0]
			}
			script, err := shell.Init(name)
			if err != nil {
				return err
			}
			d.Printf("%s", script)
			return nil
		},
	}
}
