//go:build unix

package commands

import (
	"os/exec"
	"syscall"
)

// exitStatus is the status a shell would report for the child: its exit code, or 128+signal when a
// signal killed it (ExitCode() alone says -1 for that).
func exitStatus(ee *exec.ExitError) int {
	if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return 128 + int(ws.Signal())
	}
	return codeOrFailure(ee.ExitCode())
}
