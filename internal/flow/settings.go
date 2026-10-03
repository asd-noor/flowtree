// Package flow implements the git-flow domain: branch kinds, where each branch lives,
// and the steps that start, merge and finish them. It knows nothing about the CLI.
package flow

import (
	"fmt"

	"flowtree/internal/config"
)

// Defaults for the flow settings.
const (
	DefaultMaster     = "master"
	DefaultDevelop    = "develop"
	DefaultWorkPrefix = "work/"
	DefaultRelPrefix  = "release/"
	DefaultHotPrefix  = "hotfix/"
)

// Settings are the resolved flow settings of a repository.
type Settings struct {
	Master, Develop string
	WorkPrefix      string
	ReleasePrefix   string
	HotfixPrefix    string
	VersionTag      string // prefix of version tags, e.g. "v"
	Remote          string
	FinishRebase    bool // `work finish` rebases onto develop first
}

// DefaultSettings returns the settings used by `init --flow` when nothing else is asked for.
func DefaultSettings() Settings {
	return Settings{
		Master: DefaultMaster, Develop: DefaultDevelop,
		WorkPrefix: DefaultWorkPrefix, ReleasePrefix: DefaultRelPrefix, HotfixPrefix: DefaultHotPrefix,
		Remote: config.DefaultRemote, FinishRebase: true,
	}
}

// FromConfig reads the flow section of a config. It fails when `init --flow` has not run.
func FromConfig(c config.Config) (Settings, error) {
	if !c.FlowEnabled() {
		return Settings{}, fmt.Errorf("flow is not set up in this repository; run 'flowtree init --flow'")
	}
	s := DefaultSettings()
	s.Remote = c.Remote
	s = s.override(c.Flow)
	if err := s.Validate(); err != nil {
		return Settings{}, fmt.Errorf("invalid flow settings: %w", err)
	}
	return s, nil
}

func (s Settings) override(f config.Flow) Settings {
	pick := func(dst *string, v string) {
		if v != "" {
			*dst = v
		}
	}
	pick(&s.Master, f.Master)
	pick(&s.Develop, f.Develop)
	pick(&s.WorkPrefix, f.WorkPrefix)
	pick(&s.ReleasePrefix, f.RelPrefix)
	pick(&s.HotfixPrefix, f.HotPrefix)
	s.VersionTag = f.VersionTag
	if f.FinishRebase != "" {
		s.FinishRebase = f.FinishRebase == "true"
	}
	return s
}

// ToConfig is the stored form of the settings.
func (s Settings) ToConfig() config.Flow {
	return config.Flow{
		Master: s.Master, Develop: s.Develop,
		WorkPrefix: s.WorkPrefix, RelPrefix: s.ReleasePrefix, HotPrefix: s.HotfixPrefix,
		VersionTag: s.VersionTag,
	}
}

// Validate checks the settings are usable.
func (s Settings) Validate() error {
	if s.Master == "" || s.Develop == "" {
		return fmt.Errorf("master and develop branch names must not be empty")
	}
	if s.Master == s.Develop {
		return fmt.Errorf("master and develop must be different branches (both are '%s')", s.Master)
	}
	prefixes := []string{s.WorkPrefix, s.ReleasePrefix, s.HotfixPrefix}
	for i, p := range prefixes {
		if p == "" {
			return fmt.Errorf("branch prefixes must not be empty")
		}
		for _, q := range prefixes[i+1:] {
			if p == q {
				return fmt.Errorf("branch prefixes must differ (both are '%s')", p)
			}
		}
	}
	return nil
}
