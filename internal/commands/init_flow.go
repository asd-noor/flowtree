package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/pflag"

	"flowtree/internal/config"
	"flowtree/internal/flow"
)

// flowOpts are the `init --flow` flags. Empty strings mean "ask, or use the default".
type flowOpts struct {
	enabled                          bool
	master, develop                  string
	workPrefix, relPrefix, hotPrefix string
	versionTag                       string
	remote                           string
}

func bindFlowFlags(f *pflag.FlagSet, o *flowOpts) {
	f.BoolVar(&o.enabled, "flow", false, "also set up git-flow: master/develop branches and flow settings")
	f.StringVar(&o.master, "master", "", "production branch name (default: existing master or main, else master)")
	f.StringVar(&o.develop, "develop", "", "integration branch name (default develop)")
	f.StringVarP(&o.workPrefix, "work-prefix", "p", "", "prefix of work branches (default work/)")
	f.StringVarP(&o.relPrefix, "release-prefix", "r", "", "prefix of release branches (default release/)")
	f.StringVarP(&o.hotPrefix, "hotfix-prefix", "x", "", "prefix of hotfix branches (default hotfix/)")
	f.StringVarP(&o.versionTag, "version-tag", "t", "", "prefix of version tags, e.g. v (default none)")
	f.StringVar(&o.remote, "remote", "", "remote used for fetch and push (default origin)")
}

// flowSettingsIn is the outcome of the flow questions.
type flowSettingsIn = flow.Settings

func chooseFlowSettings(_ context.Context, r *initRun) error {
	if !r.opts.flow.enabled {
		return nil
	}
	if _, ok := flow.Rev(r.git, "HEAD"); !ok {
		return errors.New("the repository has no commits yet; make an initial commit before 'init --flow'")
	}
	s, err := r.flowSettings()
	if err != nil {
		return err
	}
	r.flowS = &s
	r.pairs = append(r.pairs, nonBlank(s.ToConfig().Pairs())...)
	if s.Remote != config.DefaultRemote {
		r.pairs = append(r.pairs, config.Pair{Key: "remote", Value: s.Remote})
	}
	return nil
}

// flowSettings starts from the defaults, applies flags, and asks for what is still unset.
func (r *initRun) flowSettings() (flow.Settings, error) {
	o := r.opts.flow
	s := flow.DefaultSettings()
	s.Master = flow.DetectMaster(r.git)
	s.Remote = r.cfg.Remote
	fields := []struct {
		prompt string
		flag   string
		dst    *string
	}{
		{"Branch name for production releases", o.master, &s.Master},
		{"Branch name for next release development", o.develop, &s.Develop},
		{"Work branches prefix", o.workPrefix, &s.WorkPrefix},
		{"Release branches prefix", o.relPrefix, &s.ReleasePrefix},
		{"Hotfix branches prefix", o.hotPrefix, &s.HotfixPrefix},
		{"Version tag prefix", o.versionTag, &s.VersionTag},
		{"Remote", o.remote, &s.Remote},
	}
	for _, f := range fields {
		*f.dst = r.chooseValue(f.prompt, f.flag, *f.dst)
	}
	if err := s.Validate(); err != nil {
		return flow.Settings{}, fmt.Errorf("invalid flow settings: %w", err)
	}
	return s, nil
}

// chooseValue is the flag when given, else the answer to a prompt (terminal only), else def.
func (r *initRun) chooseValue(prompt, flag, def string) string {
	switch {
	case flag != "":
		return flag
	case r.interactive():
		return ask(r.stdin, r.d.Stderr, prompt, def)
	}
	return def
}

// setupFlowBranches creates master and develop once the settings are stored.
func setupFlowBranches(ctx context.Context, r *initRun) error {
	if !r.opts.flow.enabled {
		return nil
	}
	s, err := flow.FromConfig(r.cfg)
	if err != nil {
		return err
	}
	repo := Repo{Git: r.git, Main: r.main, Cwd: r.main, Config: r.cfg}
	return flow.EnsureBranches(r.d.flowEnv(ctx, repo, s))
}

// nonBlank drops pairs with an empty value: an unset tag prefix needs no key (the others are never empty).
func nonBlank(pairs []config.Pair) []config.Pair {
	var out []config.Pair
	for _, p := range pairs {
		if p.Value != "" {
			out = append(out, p)
		}
	}
	return out
}
