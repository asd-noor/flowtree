package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"flowtree/internal/gitx"
)

// HooksFileName is the hooks file inside the git directory. Being inside .git it is shared by all
// worktrees, never part of a commit, and so cannot be injected by a clone (command hooks run code).
const HooksFileName = "flowtree-hooks.json"

// HooksPath is where the hooks file lives for a repository: <git common dir>/flowtree-hooks.json.
func HooksPath(g gitx.Git, main string) (string, error) {
	common, err := g("rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(main, common)
	}
	return filepath.Join(common, HooksFileName), nil
}

// LoadHooks reads the hooks file; a missing file means no hooks. Unknown fields are errors, and
// hooks get their defaults applied and are validated.
func LoadHooks(g gitx.Git, main string) (Hooks, error) {
	path, err := HooksPath(g, main)
	if err != nil {
		return Hooks{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Hooks{}, nil
	}
	if err != nil {
		return Hooks{}, fmt.Errorf("read %s: %w", path, err)
	}
	return ParseHooks(data, path)
}

// ParseHooks decodes the hooks JSON; name is only used in error messages.
func ParseHooks(data []byte, name string) (Hooks, error) {
	var h Hooks
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&h); err != nil {
		return Hooks{}, fmt.Errorf("parse %s: %w", name, err)
	}
	for i := range h.PostCreate {
		h.PostCreate[i] = h.PostCreate[i].WithDefaults()
	}
	return h, nil
}

// SaveHooks writes the hooks file.
func SaveHooks(g gitx.Git, main string, h Hooks) error {
	path, err := HooksPath(g, main)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
