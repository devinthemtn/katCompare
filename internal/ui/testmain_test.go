package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"katcompare/internal/config"
)

// TestMain points the config file at an empty folder, so that a setting in the
// real file of whoever runs the tests can't change the results.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "katcmp-config")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Setenv("XDG_CONFIG_HOME", dir)
	os.Setenv("AppData", dir)
	os.Setenv("HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// writeConfig sets the config file's content for one test.
func writeConfig(t *testing.T, content string) {
	t.Helper()
	path, err := config.Path()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(path) })
}
