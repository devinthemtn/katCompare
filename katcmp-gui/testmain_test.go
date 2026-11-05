package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"katcompare/internal/config"
	"katcompare/internal/git"
)

// TestMain points the config file at an empty folder, so that a setting in the
// real file of whoever runs the tests can't change the results, and runs the
// tests from there.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "katcmp-config")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Setenv("XDG_CONFIG_HOME", dir)
	os.Setenv("AppData", dir)
	os.Setenv("HOME", dir)
	// Run outside any repository. Without this, a test that reaches git mode
	// compares the real repository the tests are in, and a merge saves over its
	// files. A random-key test did exactly that once.
	if err := os.Chdir(dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

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

func TestTestsRunOutsideAnyRepository(t *testing.T) {
	if root, err := git.FindRoot(); err == nil {
		t.Fatalf("the tests are running inside the repository at %s, where a merge could overwrite real files", root)
	}
}
