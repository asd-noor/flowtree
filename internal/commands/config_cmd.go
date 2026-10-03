package commands

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"flowtree/internal/config"
	"flowtree/internal/gitx"
)

func newConfigCmd(d Deps) *cobra.Command {
	var global, unset bool
	cmd := &cobra.Command{
		Use:   "config [<key> [<value>]] | config hooks",
		Short: "Show or change flowtree's settings",
		Long: `Settings are stored in git config (section flowtree). Without arguments this lists every setting with the
place its value comes from. With a key it prints that value; with a key and a value it sets it in this repository
(or for every repository with --global); --unset removes it. Keys: ` + keyList() + `.

'flowtree config hooks' shows where the post-create hooks file lives and what is in it.`,
		Args:              cobra.MaximumNArgs(2),
		ValidArgsFunction: completeConfigKeys,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfig(cmd.Context(), d, args, global, unset)
		},
	}
	cmd.Flags().BoolVar(&global, "global", false, "set or unset in the global git config instead of this repository's")
	cmd.Flags().BoolVar(&unset, "unset", false, "remove the setting")
	return cmd
}

func keyList() string {
	out := ""
	for i, k := range config.Keys() {
		if i > 0 {
			out += ", "
		}
		out += k
	}
	return out
}

func completeConfigKeys(_ *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return withPrefix(append(config.Keys(), "hooks"), prefix), cobra.ShellCompDirectiveNoFileComp
}

// configContext is the git handle and main worktree, found without loading (and so validating) the settings:
// config must work to repair a broken configuration.
type configContext struct {
	g    gitx.Git
	main string
}

func openConfigContext(ctx context.Context, d Deps) (configContext, error) {
	cwd, err := d.Cwd()
	if err != nil {
		return configContext{}, err
	}
	main, err := gitx.MainWorktree(gitx.Bind(ctx, d.Run, cwd))
	if err != nil || main.Path == "" {
		return configContext{}, errors.New("not in a git repository")
	}
	return configContext{g: gitx.Bind(ctx, d.Run, main.Path), main: main.Path}, nil
}

func runConfig(ctx context.Context, d Deps, args []string, global, unset bool) error {
	cc, err := openConfigContext(ctx, d)
	if err != nil {
		return err
	}
	scope := config.Local
	if global {
		scope = config.Global
	}
	switch {
	case len(args) == 1 && args[0] == "hooks":
		return showHooks(d, cc)
	case unset:
		return unsetKey(cc, scope, args)
	case len(args) == 0:
		return listSettings(d, cc)
	case len(args) == 1:
		return showSetting(d, cc, args[0])
	}
	return config.Set(cc.g, scope, args[0], args[1])
}

func unsetKey(cc configContext, scope config.Scope, args []string) error {
	if len(args) != 1 {
		return errors.New("--unset needs exactly one key")
	}
	return config.Unset(cc.g, scope, args[0])
}

// effective is the settings with, for each key, the scope that set it ("" when it is the default).
func effective(cc configContext, d Deps) (config.Config, map[string]string, error) {
	entries, err := config.Entries(cc.g)
	if err != nil {
		return config.Config{}, nil, err
	}
	scopes := map[string]string{}
	for _, e := range entries {
		scopes[e.Key] = e.Scope
	}
	cfg, err := config.Load(cc.g, cc.main, d.Getenv)
	return cfg, scopes, err
}

func listSettings(d Deps, cc configContext) error {
	cfg, scopes, err := effective(cc, d)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(d.Stdout, 0, 4, 2, ' ', 0)
	for _, k := range config.Keys() {
		v, set, _ := cfg.Value(k)
		fmt.Fprintf(tw, "%s\t%s\n", k, describe(cfg, cc.main, k, v, set, scopes[k]))
	}
	return tw.Flush()
}

func showSetting(d Deps, cc configContext, key string) error {
	canon, err := config.Canonical(key)
	if err != nil {
		return err
	}
	cfg, _, err := effective(cc, d)
	if err != nil {
		return err
	}
	v, set, _ := cfg.Value(canon)
	if !set {
		v = defaultFor(cfg, cc.main, canon)
	}
	d.Printf("%s\n", v)
	return nil
}

func defaultFor(cfg config.Config, main, key string) string {
	if key == "baseDir" {
		return cfg.DefaultBaseDirFor(main)
	}
	return config.Default(key)
}

func describe(cfg config.Config, main, key, value string, set bool, scope string) string {
	if set && scope != "" {
		return fmt.Sprintf("%s  (%s)", value, scope)
	}
	def := defaultFor(cfg, main, key)
	if def == "" {
		return "(not set)"
	}
	return fmt.Sprintf("%s  (default)", def)
}

const hooksExample = `{
  "post_create": [
    { "type": "copy", "from": ".env" },
    { "type": "symlink", "from": ".cache", "to": ".cache" },
    { "type": "command", "command": "npm ci" }
  ]
}
`

func showHooks(d Deps, cc configContext) error {
	path, err := config.HooksPath(cc.g, cc.main)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		d.Printf("No hooks file. Create %s, for example:\n\n%s", path, hooksExample)
		return nil
	}
	if err != nil {
		return err
	}
	d.Printf("%s\n\n%s", path, data)
	return nil
}
