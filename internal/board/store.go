package board

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// FileName is the board's file inside the git common directory, shared by every worktree of a clone.
const FileName = "flowtree-board.json"

// Path is where the board of a repository lives.
func Path(gitCommonDir string) string { return filepath.Join(gitCommonDir, FileName) }

// Load reads the board; a missing file is an empty board.
func Load(path string) (Board, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Empty(), nil
	}
	if err != nil {
		return Board{}, err
	}
	var b Board
	if err := json.Unmarshal(data, &b); err != nil {
		return Board{}, fmt.Errorf("board file '%s' is damaged: %w", path, err)
	}
	return b, nil
}

// Update applies fn to the board under a lock and writes the result atomically, so parallel commands in
// other worktrees neither lose writes nor see a half-written file.
func Update(path string, fn func(Board) (Board, error)) error {
	unlock, err := lock(path + ".lock")
	if err != nil {
		return err
	}
	defer unlock()
	b, err := Load(path)
	if err != nil {
		return err
	}
	next, err := fn(b)
	if err != nil {
		return err
	}
	return write(path, next)
}

func write(path string, b Board) error {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), FileName+".tmp-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

const (
	lockWait  = 5 * time.Second
	lockStale = time.Minute // a lock this old was left by a crashed process
)

// lock takes an exclusive lock file. ponytail: polling a lock file instead of flock keeps it portable;
// a lock older than lockStale is taken over, so a crash never blocks the board for long.
func lock(path string) (func(), error) {
	deadline := time.Now().Add(lockWait)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_ = f.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
		if st, err := os.Stat(path); err == nil && time.Since(st.ModTime()) > lockStale {
			_ = os.Remove(path)
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("the board is locked by another flowtree command (%s); try again", path)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
