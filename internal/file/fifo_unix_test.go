//go:build !windows

package file

import "syscall"

func syscallMkfifo(path string) error {
	return syscall.Mkfifo(path, 0644)
}
