package config

import (
	"fmt"
	"path/filepath"
)

// Hook types.
const (
	HookCopy    = "copy"
	HookSymlink = "symlink"
	HookCommand = "command"
)

// Hook is one post-create step.
type Hook struct {
	Type    string            `json:"type"`
	From    string            `json:"from,omitempty"`
	To      string            `json:"to,omitempty"`
	Command string            `json:"command,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	WorkDir string            `json:"work_dir,omitempty"`
}

// WithDefaults makes a copy hook's `to` default to its relative `from`.
func (h Hook) WithDefaults() Hook {
	if h.Type == HookCopy && h.To == "" && h.From != "" && !filepath.IsAbs(h.From) {
		h.To = h.From
	}
	return h
}

// Validate checks that the fields fit the hook type.
func (h Hook) Validate() error {
	switch h.Type {
	case HookCopy:
		return firstErr(
			require(h.From != "", "copy hook requires 'from'"),
			require(h.To != "" || !filepath.IsAbs(h.From), "copy hook with absolute 'from' requires 'to'"),
			require(h.Command == "", "copy hook must not have 'command'"),
		)
	case HookSymlink:
		return firstErr(
			require(h.From != "" && h.To != "", "symlink hook requires 'from' and 'to'"),
			require(h.Command == "", "symlink hook must not have 'command'"),
		)
	case HookCommand:
		return firstErr(
			require(h.Command != "", "command hook requires 'command'"),
			require(h.From == "" && h.To == "", "command hook must not have 'from' or 'to'"),
		)
	}
	return fmt.Errorf("invalid hook type %q: must be copy, symlink or command", h.Type)
}

func require(ok bool, msg string) error {
	if ok {
		return nil
	}
	return fmt.Errorf("%s", msg)
}

func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
