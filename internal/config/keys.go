package config

import (
	"errors"
	"fmt"
	"strings"

	"flowtree/internal/gitx"
)

const section = "flowtree"

// setting is one flowtree.* key: how to store its value in a Config and what it shows by default.
type setting struct {
	name    string // canonical camelCase name
	apply   func(*Config, string) error
	current func(Config) string
	def     string // what an unset key means, for display
}

func nonEmpty(name string, set func(*Config, string)) func(*Config, string) error {
	return func(c *Config, v string) error {
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("%s.%s must not be empty", section, name)
		}
		set(c, v)
		return nil
	}
}

func anyValue(set func(*Config, string)) func(*Config, string) error {
	return func(c *Config, v string) error { set(c, v); return nil }
}

var settings = []setting{
	{"baseDir", nonEmpty("baseDir", func(c *Config, v string) { c.BaseDir = v }), func(c Config) string { return c.BaseDir }, "../worktrees, or $FLOWTREE_PARENT/<repo name>"},
	{"remote", nonEmpty("remote", func(c *Config, v string) { c.Remote = v }), func(c Config) string { return c.Remote }, DefaultRemote},
	{"master", nonEmpty("master", func(c *Config, v string) { c.Flow.Master = v }), func(c Config) string { return c.Flow.Master }, ""},
	{"develop", nonEmpty("develop", func(c *Config, v string) { c.Flow.Develop = v }), func(c Config) string { return c.Flow.Develop }, ""},
	{"workPrefix", nonEmpty("workPrefix", func(c *Config, v string) { c.Flow.WorkPrefix = v }), func(c Config) string { return c.Flow.WorkPrefix }, "work/"},
	{"releasePrefix", nonEmpty("releasePrefix", func(c *Config, v string) { c.Flow.RelPrefix = v }), func(c Config) string { return c.Flow.RelPrefix }, "release/"},
	{"hotfixPrefix", nonEmpty("hotfixPrefix", func(c *Config, v string) { c.Flow.HotPrefix = v }), func(c Config) string { return c.Flow.HotPrefix }, "hotfix/"},
	{"versionTagPrefix", anyValue(func(c *Config, v string) { c.Flow.VersionTag = v }), func(c Config) string { return c.Flow.VersionTag }, ""},
}

// Keys lists the setting names, for completion and help.
func Keys() []string {
	names := make([]string, len(settings))
	for i, s := range settings {
		names[i] = s.name
	}
	return names
}

// ErrUnknownKey is returned for a setting flowtree does not have.
var ErrUnknownKey = errors.New("unknown setting")

// lookup finds a setting by name, ignoring case and an optional "flowtree." prefix.
func lookup(name string) (setting, error) {
	short := strings.TrimPrefix(strings.ToLower(name), section+".")
	for _, s := range settings {
		if strings.ToLower(s.name) == short {
			return s, nil
		}
	}
	return setting{}, fmt.Errorf("%w '%s.%s' (known: %s)", ErrUnknownKey, section, short, strings.Join(Keys(), ", "))
}

// Canonical returns the canonical name of a setting.
func Canonical(name string) (string, error) {
	s, err := lookup(name)
	return s.name, err
}

// Entry is one flowtree.* key as git reports it.
type Entry struct {
	Key   string // canonical name, without the section
	Value string
	Scope string // local, global, system, ...
}

// Entries lists the flowtree.* keys that are set, in git's order (later entries win).
// An unknown key is an error, which catches typos.
func Entries(g gitx.Git) ([]Entry, error) {
	out, err := g("config", "--show-scope", "--null", "--get-regexp", `^flowtree\.`)
	if err != nil {
		var ge *gitx.Error
		if errors.As(err, &ge) && ge.Output == "" { // exit status 1: no matching keys
			return nil, nil
		}
		return nil, err
	}
	return parseEntries(out)
}

// parseEntries parses `--show-scope --null` output: "scope\0key\nvalue\0" repeated.
func parseEntries(out string) ([]Entry, error) {
	fields := strings.Split(strings.TrimSuffix(out, "\x00"), "\x00")
	var entries []Entry
	for i := 0; i+1 < len(fields); i += 2 {
		key, value, _ := strings.Cut(fields[i+1], "\n")
		s, err := lookup(key)
		if err != nil {
			return nil, err
		}
		entries = append(entries, Entry{Key: s.name, Value: value, Scope: fields[i]})
	}
	return entries, nil
}

// Load reads the configuration of the repository whose main worktree is main.
func Load(g gitx.Git, main string, getenv func(string) string) (Config, error) {
	c := Defaults()
	c.Parent = getenv(ParentEnv)
	entries, err := Entries(g)
	if err != nil {
		return Config{}, err
	}
	for _, e := range entries {
		s, _ := lookup(e.Key)
		if err := s.apply(&c, e.Value); err != nil {
			return Config{}, fmt.Errorf("%s (set in %s config)", err, e.Scope)
		}
	}
	hooks, err := LoadHooks(g, main)
	if err != nil {
		return Config{}, err
	}
	c.Hooks = hooks
	return c, c.Validate()
}

// Scope selects which git config file Set and Unset change.
type Scope string

// The scopes flowtree writes to.
const (
	Local  Scope = "--local"
	Global Scope = "--global"
)

// Set stores a setting after checking the key and the value.
func Set(g gitx.Git, scope Scope, key, value string) error {
	s, err := lookup(key)
	if err != nil {
		return err
	}
	if err := s.apply(&Config{}, value); err != nil {
		return err
	}
	_, err = g("config", string(scope), section+"."+s.name, value)
	return err
}

// Unset removes a setting; removing one that is not set is not an error.
func Unset(g gitx.Git, scope Scope, key string) error {
	s, err := lookup(key)
	if err != nil {
		return err
	}
	if _, err := g("config", string(scope), "--unset-all", section+"."+s.name); err != nil {
		var ge *gitx.Error
		if errors.As(err, &ge) && ge.Output == "" {
			return nil
		}
		return err
	}
	return nil
}

// Pair is a setting name and value to store.
type Pair struct{ Key, Value string }

// SetAll stores several settings, stopping at the first failure.
func SetAll(g gitx.Git, scope Scope, pairs []Pair) error {
	if len(pairs) == 0 {
		return nil
	}
	if err := Set(g, scope, pairs[0].Key, pairs[0].Value); err != nil {
		return err
	}
	return SetAll(g, scope, pairs[1:])
}

// Pairs returns the flow settings as pairs to store.
func (f Flow) Pairs() []Pair {
	return []Pair{
		{"master", f.Master}, {"develop", f.Develop},
		{"workPrefix", f.WorkPrefix}, {"releasePrefix", f.RelPrefix}, {"hotfixPrefix", f.HotPrefix},
		{"versionTagPrefix", f.VersionTag},
	}
}

// Value returns the effective value of one setting and whether it is set (otherwise it is the default).
func (c Config) Value(key string) (value string, set bool, err error) {
	s, err := lookup(key)
	if err != nil {
		return "", false, err
	}
	v := s.current(c)
	return v, v != "", nil
}

// Default describes what an unset setting means.
func Default(key string) string {
	s, err := lookup(key)
	if err != nil {
		return ""
	}
	return s.def
}
