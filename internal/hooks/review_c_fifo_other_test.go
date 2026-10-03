//go:build !unix

package hooks

import "errors"

func mkfifo(string) error { return errors.New("fifos are not supported here") }
