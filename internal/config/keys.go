package config

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
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

func boolValue(name string, set func(*Config, string)) func(*Config, string) error {
	return func(c *Config, v string) error {
		b, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return fmt.Errorf("%s.%s must be true or false, got '%s'", section, name, v)
		}
		set(c, strconv.FormatBool(b))
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
	{"finishRebase", boolValue("finishRebase", func(c *Config, v string) { c.Flow.FinishRebase = v }), func(c Config) string { return c.Flow.FinishRebase }, "true"},
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
	Scope string // local, global, system or other; empty from rawEntries
}

// Scopes in the order git applies them: later ones override earlier ones.
var scopeOrder = []string{"system", "global", "local"}

// Entries lists the flowtree.* keys that are set, in git's order (later entries win), with the scope each
// value comes from: "local", "global", "system", or "other" for sources that have no scope of their own
// here (worktree config, command line). An unknown key is an error, which catches typos.
//
// It deliberately avoids `git config --show-scope` (git 2.26): the effective values come from one plain
// read, and the scopes are worked out from per-scope reads, which any git has.
func Entries(g gitx.Git) ([]Entry, error) {
	entries, err := rawEntries(g)
	if err != nil || len(entries) == 0 {
		return entries, err
	}
	byScope := scopedValues(g)
	for i, e := range entries {
		entries[i].Scope = scopeOf(byScope, e)
	}
	return entries, nil
}

// rawEntries reads the effective flowtree.* keys in one call, without scopes. Load needs only this.
func rawEntries(g gitx.Git) ([]Entry, error) {
	out, err := g("config", "--null", "--get-regexp", `^flowtree\.`)
	if err != nil {
		if isNoMatch(err) {
			return nil, nil
		}
		return nil, err
	}
	return parseEntries(out)
}

// isNoMatch recognises git's exit status 1 with no message: nothing matched.
func isNoMatch(err error) bool {
	var ge *gitx.Error
	return errors.As(err, &ge) && ge.Output == ""
}

// parseEntries parses `--null` output: "key\nvalue\0" repeated. Git lower-cases the variable name.
func parseEntries(out string) ([]Entry, error) {
	var entries []Entry
	for _, field := range strings.Split(strings.TrimSuffix(out, "\x00"), "\x00") {
		if field == "" {
			continue
		}
		key, value, _ := strings.Cut(field, "\n")
		s, err := lookup(key)
		if err != nil {
			return nil, err
		}
		entries = append(entries, Entry{Key: s.name, Value: value})
	}
	return entries, nil
}

// scopedValues reads each scope separately (best effort: a scope that cannot be read has no values) and
// returns, per scope, the values of each key as git spells it (lower case).
func scopedValues(g gitx.Git) map[string]map[string][]string {
	byScope := map[string]map[string][]string{}
	for _, scope := range scopeOrder {
		// --includes: per-scope reads skip include.path by default, the combined read does not
		out, err := g("config", "--"+scope, "--includes", "--null", "--get-regexp", `^flowtree\.`)
		if err != nil {
			continue
		}
		byScope[scope] = groupByKey(out)
	}
	return byScope
}

func groupByKey(out string) map[string][]string {
	values := map[string][]string{}
	for _, field := range strings.Split(strings.TrimSuffix(out, "\x00"), "\x00") {
		if field == "" {
			continue
		}
		key, value, _ := strings.Cut(field, "\n")
		k := strings.ToLower(key)
		values[k] = append(values[k], value)
	}
	return values
}

// scopeOf names the highest-precedence scope that holds this key with this value.
func scopeOf(byScope map[string]map[string][]string, e Entry) string {
	key := strings.ToLower(section + "." + e.Key)
	for i := len(scopeOrder) - 1; i >= 0; i-- {
		if slices.Contains(byScope[scopeOrder[i]][key], e.Value) {
			return scopeOrder[i]
		}
	}
	return "other"
}

// Load reads the configuration of the repository whose main worktree is main.
func Load(g gitx.Git, main string, getenv func(string) string) (Config, error) {
	c := Defaults()
	c.Parent = getenv(ParentEnv)
	entries, err := rawEntries(g)
	if err != nil {
		return Config{}, err
	}
	for _, e := range entries {
		s, _ := lookup(e.Key)
		if err := s.apply(&c, e.Value); err != nil {
			return Config{}, fmt.Errorf("%s (set in git config)", err)
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
