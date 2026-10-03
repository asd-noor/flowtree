// Package config holds flowtree's settings. They live in git config (section "flowtree", read from the
// main worktree so local, global and system scopes all apply); post-create hooks, which have structure
// git config cannot express, live in a JSON file inside the git directory.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Defaults and names.
const (
	DefaultBaseDir = "../worktrees"
	DefaultRemote  = "origin"

	// ParentEnv names a directory under which every repository gets <parent>/<repo name>, unless
	// flowtree.baseDir is set.
	ParentEnv = "FLOWTREE_PARENT"
)

// Config is the effective configuration of a repository.
type Config struct {
	BaseDir string // flowtree.baseDir; empty means "use the default"
	Remote  string
	Parent  string // $FLOWTREE_PARENT at load time
	Flow    Flow
	Hooks   Hooks
}

// Flow holds the git-flow settings; see package flow. Empty fields mean "not set".
type Flow struct {
	Master       string
	Develop      string
	WorkPrefix   string
	RelPrefix    string
	HotPrefix    string
	VersionTag   string
	FinishRebase string // flowtree.finishRebase: "true" or "false"; empty means true
}

// Hooks groups hooks by the event they run on.
type Hooks struct {
	PostCreate []Hook `json:"post_create,omitempty"`
}

// Defaults returns the configuration of a repository with nothing set.
func Defaults() Config { return Config{Remote: DefaultRemote} }

// FlowEnabled reports whether `init --flow` has run: both branch names are set.
func (c Config) FlowEnabled() bool { return c.Flow.Master != "" && c.Flow.Develop != "" }

// HasHooks reports whether any post-create hook is configured.
func (c Config) HasHooks() bool { return len(c.Hooks.PostCreate) > 0 }

// Validate checks the configuration.
func (c Config) Validate() error {
	if (c.Flow.Master == "") != (c.Flow.Develop == "") {
		return fmt.Errorf("flowtree.master and flowtree.develop must be set together (run 'flowtree init --flow')")
	}
	for i, h := range c.Hooks.PostCreate {
		if err := h.Validate(); err != nil {
			return fmt.Errorf("invalid hook %d: %w", i+1, err)
		}
	}
	return nil
}

// ExpandHome expands a leading ~ or $HOME, which would otherwise be taken as a relative path.
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

// DefaultBaseDirFor is the base directory used when flowtree.baseDir is not set:
// $FLOWTREE_PARENT/<repo directory name> when the variable is defined, else ../worktrees.
func (c Config) DefaultBaseDirFor(mainWorktree string) string {
	if c.Parent == "" {
		return DefaultBaseDir
	}
	return filepath.Join(ExpandHome(c.Parent), filepath.Base(mainWorktree))
}

// ResolveBaseDir returns the absolute worktree base directory for a main worktree.
func (c Config) ResolveBaseDir(mainWorktree string) string {
	base := ExpandHome(firstNonEmpty(c.BaseDir, c.DefaultBaseDirFor(mainWorktree)))
	if !filepath.IsAbs(base) {
		base = filepath.Join(mainWorktree, base)
	}
	return filepath.Clean(base)
}

// WorktreePath is where the worktree for a branch lives: <base dir>/<branch>.
func (c Config) WorktreePath(mainWorktree, branch string) string {
	return filepath.Join(c.ResolveBaseDir(mainWorktree), filepath.FromSlash(branch))
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
