package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

// fakeTools puts stub programs on an otherwise empty PATH and leaves out git,
// so tools are launched directly. Each stub writes its arguments, one per
// line, to <name>.args in the returned directory.
func fakeTools(t *testing.T, names ...string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("stub tools are shell scripts")
	}
	bin := t.TempDir()
	for _, n := range names {
		script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"" + filepath.Join(bin, n+".args") + "\"\n"
		if err := os.WriteFile(filepath.Join(bin, n), []byte(script), 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	for _, env := range []string{"KATCMP_EDITOR", "FILECMP_EDITOR", "VISUAL", "EDITOR"} {
		t.Setenv(env, "")
	}
	return bin
}

// launchedArgs waits for a stub started by fakeTools to record its arguments.
func launchedArgs(t *testing.T, bin, name string) []string {
	t.Helper()
	path := filepath.Join(bin, name+".args")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
			return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s was not launched", name)
	return nil
}

func TestStartDetached(t *testing.T) {
	bin := fakeTools(t, "tool")
	if err := startDetached(exec.Command(filepath.Join(bin, "tool"), "x")); err != nil {
		t.Fatal(err)
	}
	if got := launchedArgs(t, bin, "tool"); !reflect.DeepEqual(got, []string{"x"}) {
		t.Errorf("args = %v", got)
	}
	err := startDetached(exec.Command(filepath.Join(bin, "missing")))
	if err == nil || !strings.Contains(err.Error(), "failed to launch") {
		t.Errorf("missing program: %v", err)
	}
}

func TestGetToolInfo(t *testing.T) {
	fakeTools(t)
	if got := NewApp(nil).GetToolInfo(); got != (ToolInfo{}) {
		t.Errorf("with no tools installed: %+v", got)
	}
	fakeTools(t, "meld")
	if got := NewApp(nil).GetToolInfo(); !got.DiffToolConfigured || !got.CanDirDiff {
		t.Errorf("with meld installed: %+v", got)
	}
}

func TestOpenInEditor(t *testing.T) {
	a, left := mergeDirs(t)
	bin := fakeTools(t, "myeditor")
	writeConfig(t, "[tools]\neditor = \""+filepath.Join(bin, "myeditor")+"\"\n")

	if err := a.OpenInEditor("missing.txt", "left", 0); err == nil {
		t.Error("unknown file should fail")
	}
	if err := a.OpenInEditor("f.txt", "left", 2); err != nil {
		t.Fatal(err)
	}
	args := launchedArgs(t, bin, "myeditor")
	if len(args) == 0 || args[len(args)-1] != filepath.Join(left, "f.txt") {
		t.Errorf("editor args = %v, want the left file", args)
	}

	writeConfig(t, "[tools]\neditor = \"no-such-editor\"\n")
	if err := a.OpenInEditor("f.txt", "left", 0); err == nil {
		t.Error("a missing configured editor should fail")
	}
}

func TestOpenInEditorRefusesGitRevision(t *testing.T) {
	conflictRepo(t)
	a := NewApp(nil)
	if _, err := a.LoadGitComparison("HEAD", ""); err != nil {
		t.Fatal(err)
	}
	if err := a.OpenInEditor("f.txt", "left", 0); err == nil || !strings.Contains(err.Error(), "isn't a file on disk") {
		t.Errorf("opening HEAD's version: %v", err)
	}
}

func TestOpenInDiffTool(t *testing.T) {
	a, left := mergeDirs(t)
	bin := fakeTools(t, "meld")

	if err := a.OpenInDiffTool("missing.txt"); err == nil {
		t.Error("unknown file should fail")
	}
	if err := a.OpenInDiffTool("f.txt"); err != nil {
		t.Fatal(err)
	}
	args := launchedArgs(t, bin, "meld")
	if len(args) != 2 || args[0] != filepath.Join(left, "f.txt") || filepath.Base(args[1]) != "f.txt" {
		t.Errorf("meld args = %v", args)
	}

	fakeTools(t)
	if err := a.OpenInDiffTool("f.txt"); err == nil {
		t.Error("no diff tool installed should fail")
	}
}

func TestOpenDirsInDiffTool(t *testing.T) {
	a, left := mergeDirs(t)
	bin := fakeTools(t, "meld")
	if err := a.OpenDirsInDiffTool(); err != nil {
		t.Fatal(err)
	}
	args := launchedArgs(t, bin, "meld")
	if len(args) != 2 || args[0] != left {
		t.Errorf("meld args = %v, want the two folders", args)
	}

	lf := filepath.Join(left, "f.txt")
	if _, err := a.LoadComparison(lf, lf); err != nil {
		t.Fatal(err)
	}
	if err := a.OpenDirsInDiffTool(); err == nil {
		t.Error("comparing two files should refuse a folder diff")
	}
}
