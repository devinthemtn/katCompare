// Package proc builds commands for child processes that must not pop up a
// console window when started from the Windows GUI.
package proc

import "os/exec"

// Command is exec.Command with the child's console window suppressed on
// Windows. Use it for background commands whose output is captured (git
// queries); a GUI app would otherwise flash a console for each one.
func Command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	HideConsole(cmd)
	return cmd
}
