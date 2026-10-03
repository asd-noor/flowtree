//go:build !unix

package commands

import "os/exec"

// exitStatus is the child's exit code; platforms without signals have nothing else to report.
func exitStatus(ee *exec.ExitError) int { return codeOrFailure(ee.ExitCode()) }
