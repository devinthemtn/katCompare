package git

import (
	"os"
	"runtime"
	"strconv"
)

// wslSafeDirs are the UNC prefixes under which Windows sees WSL distro files.
// Windows git treats repos there as "dubious ownership" (they're owned by the
// Linux user), so a Windows build started from a WSL directory can't use them.
var wslSafeDirs = []string{"//wsl.localhost/*", "//wsl$/*"}

// TrustWSLPaths lets Windows git use repositories inside WSL by adding
// safe.directory entries for the WSL UNC prefixes to this process's
// environment (GIT_CONFIG_COUNT/KEY/VALUE, git's command-line scope, which
// safe.directory honours). Every git child process inherits it. It is a no-op
// outside Windows. The prefix match needs git 2.46 or newer.
func TrustWSLPaths() {
	if runtime.GOOS != "windows" {
		return
	}
	for k, v := range safeDirEnv(os.Getenv("GIT_CONFIG_COUNT"), wslSafeDirs) {
		os.Setenv(k, v)
	}
}

// safeDirEnv returns the environment variables that append a safe.directory
// entry for each dir after any existing GIT_CONFIG_COUNT entries.
func safeDirEnv(count string, dirs []string) map[string]string {
	n, err := strconv.Atoi(count)
	if err != nil || n < 0 {
		n = 0
	}
	env := make(map[string]string, 2*len(dirs)+1)
	for _, dir := range dirs {
		i := strconv.Itoa(n)
		env["GIT_CONFIG_KEY_"+i] = "safe.directory"
		env["GIT_CONFIG_VALUE_"+i] = dir
		n++
	}
	env["GIT_CONFIG_COUNT"] = strconv.Itoa(n)
	return env
}
