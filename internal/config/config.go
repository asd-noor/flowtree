// Package config reads and writes .flowtree.json, the per-machine configuration.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	FileName       = ".flowtree.json"
	CurrentVersion = 1
	DefaultBaseDir = "../worktrees"
	DefaultRemote  = "origin"

	// ParentEnv names a directory under which every repository gets <parent>/<repo name>.
	ParentEnv = "FLOWTREE_PARENT"
)

// Config is the content of .flowtree.json.
type Config struct {
	Version int    `json:"version"`
	BaseDir string `json:"base_dir,omitempty"`
	Remote  string `json:"remote,omitempty"`
	Hooks   Hooks  `json:"hooks,omitzero"`
	Flow    *Flow  `json:"flow,omitempty"`
}

// Hooks groups hooks by the event they run on.
type Hooks struct {
	PostCreate []Hook `json:"post_create,omitempty"`
}

// Flow holds the git-flow settings; see package flow.
type Flow struct {
	Master     string `json:"master,omitempty"`
	Develop    string `json:"develop,omitempty"`
	WorkPrefix string `json:"work_prefix,omitempty"`
	RelPrefix  string `json:"release_prefix,omitempty"`
	HotPrefix  string `json:"hotfix_prefix,omitempty"`
	VersionTag string `json:"version_tag_prefix,omitempty"`
}

// Defaults returns the configuration used when no file exists.
func Defaults() Config {
	return Config{Version: CurrentVersion, BaseDir: DefaultBaseDir, Remote: DefaultRemote}
}

// Path is where the file lives for a given main worktree.
func Path(mainWorktree string) string { return filepath.Join(mainWorktree, FileName) }

// Load reads the file in mainWorktree. A missing file yields Defaults.
func Load(mainWorktree string) (Config, error) {
	data, err := os.ReadFile(Path(mainWorktree))
	if errors.Is(err, fs.ErrNotExist) {
		return Defaults(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read %s: %w", FileName, err)
	}
	return Parse(data)
}

// Parse decodes, defaults and validates a configuration. Unknown fields are errors.
func Parse(data []byte) (Config, error) {
	var c Config
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", FileName, err)
	}
	c = c.WithDefaults()
	return c, c.Validate()
}

// Save writes the file atomically-enough for a config: validate, then write.
func Save(mainWorktree string, c Config) error {
	c = c.WithDefaults()
	if err := c.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(Path(mainWorktree), append(data, '\n'), 0o644)
}

// WithDefaults fills in empty settings.
func (c Config) WithDefaults() Config {
	if c.Version == 0 {
		c.Version = CurrentVersion
	}
	if c.BaseDir == "" {
		c.BaseDir = DefaultBaseDir
	}
	if c.Remote == "" {
		c.Remote = DefaultRemote
	}
	c.Hooks.PostCreate = mapHooks(c.Hooks.PostCreate, Hook.WithDefaults)
	return c
}

// Validate checks the configuration.
func (c Config) Validate() error {
	if c.Version != CurrentVersion {
		return fmt.Errorf("%s: unsupported version %d (want %d)", FileName, c.Version, CurrentVersion)
	}
	for i, h := range c.Hooks.PostCreate {
		if err := h.Validate(); err != nil {
			return fmt.Errorf("invalid hook %d: %w", i+1, err)
		}
	}
	return nil
}

// HasHooks reports whether any post-create hook is configured.
func (c Config) HasHooks() bool { return len(c.Hooks.PostCreate) > 0 }

// ExpandHome expands a leading ~ or $HOME, which JSON consumers would otherwise treat as relative.
func ExpandHome(p string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	switch {
	case p == "~" || p == "$HOME":
		return home
	case strings.HasPrefix(p, "~/"):
		return filepath.Join(home, p[2:])
	case strings.HasPrefix(p, "$HOME/"):
		return filepath.Join(home, p[len("$HOME/"):])
	}
	return p
}

// ResolveBaseDir returns the absolute worktree base directory for a main worktree.
func (c Config) ResolveBaseDir(mainWorktree string) string {
	base := ExpandHome(c.BaseDir)
	if !filepath.IsAbs(base) {
		base = filepath.Join(mainWorktree, base)
	}
	return filepath.Clean(base)
}

// WorktreePath is where the worktree for a branch lives: <base_dir>/<branch>.
func (c Config) WorktreePath(mainWorktree, branch string) string {
	return filepath.Join(c.ResolveBaseDir(mainWorktree), filepath.FromSlash(branch))
}

// DefaultBaseDirFor is the base_dir `init` proposes: $FLOWTREE_PARENT/<repo name> when set.
func DefaultBaseDirFor(mainWorktree string, getenv func(string) string) string {
	parent := getenv(ParentEnv)
	if parent == "" {
		return DefaultBaseDir
	}
	return filepath.Join(ExpandHome(parent), filepath.Base(mainWorktree))
}

func mapHooks(hs []Hook, f func(Hook) Hook) []Hook {
	if hs == nil {
		return nil
	}
	out := make([]Hook, len(hs))
	for i, h := range hs {
		out[i] = f(h)
	}
	return out
}
