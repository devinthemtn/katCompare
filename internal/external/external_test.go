package external

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// ---- helpers ----------------------------------------------------------------

// isolate points git at an empty global config and clears editor env vars,
// so tests don't depend on the machine's setup.
func isolate(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	gitconfig := filepath.Join(home, ".gitconfig")
	if err := os.WriteFile(gitconfig, nil, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", gitconfig)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, env := range editorEnvVars {
		t.Setenv(env, "")
	}
	return gitconfig
}

// fakePath replaces PATH with a dir of stub executables. If keepGit is true
// the real git is linked in too.
func fakePath(t *testing.T, keepGit bool, stubs ...string) {
	t.Helper()
	gitPath, gitErr := exec.LookPath("git")
	bin := t.TempDir()
	for _, s := range stubs {
		if err := os.WriteFile(filepath.Join(bin, s), []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if keepGit {
		if gitErr != nil {
			t.Skip("git not installed")
		}
		if err := os.Symlink(gitPath, filepath.Join(bin, "git")); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
}

func setGitConfig(t *testing.T, gitconfig string, kv ...string) {
	t.Helper()
	for i := 0; i+1 < len(kv); i += 2 {
		if out, err := exec.Command("git", "config", "--file", gitconfig, kv[i], kv[i+1]).CombinedOutput(); err != nil {
			t.Fatalf("git config %s: %v %s", kv[i], err, out)
		}
	}
}

func tempFiles(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	left := filepath.Join(dir, "left.txt")
	right := filepath.Join(dir, "right.txt")
	os.WriteFile(left, []byte("a\n"), 0644)  //nolint:errcheck
	os.WriteFile(right, []byte("b\n"), 0644) //nolint:errcheck
	return left, right
}

// ---- Editor -----------------------------------------------------------------

func TestEditorCommandPrecedence(t *testing.T) {
	isolate(t)
	fakePath(t, false, "zed", "nvim", "xdg-open", "emacsclient")

	t.Setenv("EDITOR", "nvim")
	t.Setenv("VISUAL", "emacsclient -c")
	t.Setenv("FILECMP_EDITOR", "zed")

	cmd, _ := EditorCommand("/x/f.txt", 0, false)
	if got := filepath.Base(cmd.Path); got != "zed" {
		t.Errorf("FILECMP_EDITOR should win, got %s", got)
	}

	t.Setenv("FILECMP_EDITOR", "")
	cmd, _ = EditorCommand("/x/f.txt", 0, false)
	if !reflect.DeepEqual(cmd.Args, []string{"emacsclient", "-c", "/x/f.txt"}) {
		t.Errorf("VISUAL should be used next, got %v", cmd.Args)
	}

	t.Setenv("VISUAL", "")
	cmd, _ = EditorCommand("/x/f.txt", 0, false)
	if !reflect.DeepEqual(cmd.Args, []string{"nvim", "/x/f.txt"}) {
		t.Errorf("EDITOR should be used last, got %v", cmd.Args)
	}
}

func TestEditorCommandSkipsTerminalEditorsInGUI(t *testing.T) {
	isolate(t)
	fakePath(t, false, "nvim", "emacsclient", "xdg-open")

	t.Setenv("VISUAL", "emacsclient -t")
	t.Setenv("EDITOR", "nvim")

	cmd, _ := EditorCommand("/x/f.txt", 0, true)
	if cmd.Args[0] != "xdg-open" && cmd.Args[0] != "open" && cmd.Args[0] != "cmd" {
		t.Errorf("expected OS opener in GUI mode, got %v", cmd.Args)
	}

	cmd, _ = EditorCommand("/x/f.txt", 0, false)
	if cmd.Args[0] != "emacsclient" {
		t.Errorf("terminal editors are fine in the TUI, got %v", cmd.Args)
	}
}

func TestEditorCommandSkipsMissingEditor(t *testing.T) {
	isolate(t)
	fakePath(t, false, "nvim")
	t.Setenv("VISUAL", "not-installed-editor")
	t.Setenv("EDITOR", "nvim")

	cmd, _ := EditorCommand("/x/f.txt", 0, false)
	if cmd.Args[0] != "nvim" {
		t.Errorf("expected fallback to EDITOR, got %v", cmd.Args)
	}
}

func TestEditorFileArgs(t *testing.T) {
	tests := []struct {
		program string
		line    int
		want    []string
	}{
		{"nvim", 12, []string{"+12", "/f"}},
		{"/usr/bin/emacsclient", 3, []string{"+3", "/f"}},
		{"code", 7, []string{"-g", "/f:7"}},
		{"zed", 7, []string{"/f:7"}},
		{"gedit", 7, []string{"/f"}},
		{"nvim", 0, []string{"/f"}},
	}
	for _, tt := range tests {
		if got := editorFileArgs(tt.program, "/f", tt.line); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("editorFileArgs(%q, %d) = %v, want %v", tt.program, tt.line, got, tt.want)
		}
	}
}

func TestNeedsTerminal(t *testing.T) {
	tests := []struct {
		cmd  string
		want bool
	}{
		{"nvim", true},
		{"emacsclient -t", true},
		{"emacsclient -a -c", false},
		{"emacs -nw", true},
		{"emacs", false},
		{"zed", false},
		{"code --wait", false},
	}
	for _, tt := range tests {
		if got := NeedsTerminal(strings.Fields(tt.cmd)); got != tt.want {
			t.Errorf("NeedsTerminal(%q) = %v, want %v", tt.cmd, got, tt.want)
		}
	}
}

// ---- Diff tool: files mode --------------------------------------------------

func TestFilesDiffUsesGitNoIndexWhenConfigured(t *testing.T) {
	gitconfig := isolate(t)
	fakePath(t, true)
	setGitConfig(t, gitconfig, "diff.tool", "meld")
	left, right := tempFiles(t)

	cmd, err := DiffToolCommand(DiffRequest{LeftPath: left, RightPath: right}, true)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"git", "difftool", "--no-index", "-y", "--tool=meld", "--", left, right}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Errorf("args = %v, want %v", cmd.Args, want)
	}
	if cmd.Dir != filepath.Dir(left) {
		t.Errorf("Dir = %q, want the file's directory %q", cmd.Dir, filepath.Dir(left))
	}
}

func TestFilesDiffMakesPathsAbsolute(t *testing.T) {
	gitconfig := isolate(t)
	fakePath(t, true)
	setGitConfig(t, gitconfig, "diff.tool", "meld")
	left, _ := tempFiles(t)

	orig, _ := os.Getwd()
	defer os.Chdir(orig)         //nolint:errcheck
	os.Chdir(filepath.Dir(left)) //nolint:errcheck

	cmd, err := DiffToolCommand(DiffRequest{LeftPath: "left.txt", RightPath: "right.txt"}, true)
	if err != nil {
		t.Fatal(err)
	}
	n := len(cmd.Args)
	for _, p := range cmd.Args[n-2:] {
		if !filepath.IsAbs(p) {
			t.Errorf("expected absolute path, got %q", p)
		}
	}
}

func TestFilesDiffOneSidedUsesDevNull(t *testing.T) {
	gitconfig := isolate(t)
	fakePath(t, true)
	setGitConfig(t, gitconfig, "diff.tool", "meld")
	left, _ := tempFiles(t)

	cmd, err := DiffToolCommand(DiffRequest{LeftPath: left}, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := cmd.Args[len(cmd.Args)-1]; got != os.DevNull {
		t.Errorf("missing side should be %s, got %q", os.DevNull, got)
	}
}

func TestFilesDiffDirectFallbackWithoutGit(t *testing.T) {
	isolate(t)
	fakePath(t, false, "meld")
	left, right := tempFiles(t)

	cmd, err := DiffToolCommand(DiffRequest{LeftPath: left, RightPath: right}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cmd.Args, []string{"meld", left, right}) {
		t.Errorf("args = %v", cmd.Args)
	}
}

func TestFilesDiffAutodetectSkipsTerminalToolsInGUI(t *testing.T) {
	isolate(t)
	fakePath(t, false, "nvim")
	left, right := tempFiles(t)

	if _, err := DiffToolCommand(DiffRequest{LeftPath: left, RightPath: right}, true); err == nil {
		t.Error("expected an error: nvim -d needs a terminal")
	}
	cmd, err := DiffToolCommand(DiffRequest{LeftPath: left, RightPath: right}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cmd.Args, []string{"nvim", "-d", left, right}) {
		t.Errorf("args = %v", cmd.Args)
	}
}

func TestFilesDiffNoToolGivesHint(t *testing.T) {
	isolate(t)
	fakePath(t, true)
	left, right := tempFiles(t)

	_, err := DiffToolCommand(DiffRequest{LeftPath: left, RightPath: right}, true)
	if err == nil || !strings.Contains(err.Error(), "diff.tool") {
		t.Errorf("expected setup hint, got %v", err)
	}
}

func TestDirectToolCommandTemplate(t *testing.T) {
	gitconfig := isolate(t)
	fakePath(t, true)
	setGitConfig(t, gitconfig, "difftool.mine.cmd", `mytool "$LOCAL" "$REMOTE"`)

	cmd, err := directToolCommand("", "mine", "/a", "/b")
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Args[len(cmd.Args)-1] != `mytool "$LOCAL" "$REMOTE"` {
		t.Errorf("args = %v", cmd.Args)
	}
	env := strings.Join(cmd.Env, "\n")
	if !strings.Contains(env, "LOCAL=/a") || !strings.Contains(env, "REMOTE=/b") {
		t.Error("LOCAL/REMOTE should be set in the environment")
	}
}

func TestDirectToolCommandKnownTools(t *testing.T) {
	isolate(t)
	fakePath(t, false)
	tests := map[string][]string{
		"meld":     {"meld", "/a", "/b"},
		"vscode":   {"code", "--wait", "--diff", "/a", "/b"},
		"nvimdiff": {"nvim", "-d", "/a", "/b"},
		"bc":       {"bcompare", "/a", "/b"},
		"emerge":   {"emacs", "--eval", `(ediff-files "/a" "/b")`},
	}
	for name, want := range tests {
		cmd, err := directToolCommand("", name, "/a", "/b")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !reflect.DeepEqual(cmd.Args, want) {
			t.Errorf("%s: args = %v, want %v", name, cmd.Args, want)
		}
	}
	if _, err := directToolCommand("", "araxis", "/a", "/b"); err == nil {
		t.Error("expected error for a tool we can't launch without git")
	}
}

// ---- Diff tool: git mode ----------------------------------------------------

func TestGitDiffToolCommand(t *testing.T) {
	gitconfig := isolate(t)
	fakePath(t, true)
	setGitConfig(t, gitconfig, "diff.tool", "meld")
	repo := t.TempDir()

	cmd, err := DiffToolCommand(DiffRequest{GitRoot: repo, LeftRef: "HEAD", RelPath: "a/b.go"}, true)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"git", "-C", repo, "difftool", "-y", "--tool=meld", "HEAD", "--", "a/b.go"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Errorf("args = %v, want %v", cmd.Args, want)
	}

	cmd, _ = DiffToolCommand(DiffRequest{GitRoot: repo, LeftRef: "HEAD~1", RightRef: "main", RelPath: "x"}, true)
	want = []string{"git", "-C", repo, "difftool", "-y", "--tool=meld", "HEAD~1", "main", "--", "x"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Errorf("args = %v, want %v", cmd.Args, want)
	}
}

func TestGUIPrefersGuiTool(t *testing.T) {
	gitconfig := isolate(t)
	fakePath(t, true)
	setGitConfig(t, gitconfig, "diff.tool", "nvimdiff", "diff.guitool", "kdiff3")

	if name, _ := resolveTool("", "diff", true); name != "kdiff3" {
		t.Errorf("GUI should use diff.guitool, got %q", name)
	}
	if name, _ := resolveTool("", "diff", false); name != "nvimdiff" {
		t.Errorf("TUI should use diff.tool, got %q", name)
	}
}

// ---- Directory diff ---------------------------------------------------------

func TestDirDiffCommand(t *testing.T) {
	gitconfig := isolate(t)
	fakePath(t, true)
	l, r := t.TempDir(), t.TempDir()

	setGitConfig(t, gitconfig, "diff.tool", "meld")
	cmd, err := DirDiffCommand(l, r, true)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cmd.Args, []string{"meld", l, r}) {
		t.Errorf("args = %v", cmd.Args)
	}

	setGitConfig(t, gitconfig, "diff.tool", "vscode")
	if _, err := DirDiffCommand(l, r, true); err == nil {
		t.Error("vscode can't diff directories; expected an error")
	}
}

// ---- Merge tool -------------------------------------------------------------

func TestMergeToolCommand(t *testing.T) {
	gitconfig := isolate(t)
	fakePath(t, true)
	setGitConfig(t, gitconfig, "merge.tool", "meld")
	repo := t.TempDir()

	cmd, err := MergeToolCommand(repo, "f.txt", true)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"git", "-C", repo, "mergetool", "-y", "--tool=meld", "--", "f.txt"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Errorf("args = %v, want %v", cmd.Args, want)
	}
}

func TestMergeToolNoneConfigured(t *testing.T) {
	isolate(t)
	fakePath(t, true)
	if _, err := MergeToolCommand(t.TempDir(), "f.txt", true); err == nil {
		t.Error("expected setup hint error")
	}
}
