package commands

import "os"

// CdFileEnv names the file the shell wrapper reads to learn where to cd.
const CdFileEnv = "FLOWTREE_CDFILE"

// writeCdFile asks the shell wrapper (see `shell-init`) to cd to path. Best effort.
func writeCdFile(d Deps, path string) {
	if f := d.Getenv(CdFileEnv); f != "" {
		_ = os.WriteFile(f, []byte(path), 0o600)
	}
}
