//go:build !windows

package proc

import "os/exec"

// HideConsole is a no-op outside Windows.
func HideConsole(cmd *exec.Cmd) {}
