package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func press(t *testing.T, m *Model, keys ...string) {
	t.Helper()
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		_, cmd := m.Update(msg)
		if cmd != nil {
			if out := cmd(); out != nil {
				m.Update(out)
			}
		}
	}
}

// conflictRepo makes a repo whose merge left two conflicts in f.txt.
func conflictRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		cmd.Run() //nolint:errcheck // the merge is expected to fail
	}
	write := func(s string) {
		if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte(s), 0644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q", "-b", "main")
	write("a\nbase1\nb\nc\nd\ne\nbase2\nf\n")
	run("add", ".")
	run("commit", "-qm", "base")
	run("checkout", "-qb", "other")
	write("a\nother1\nb\nc\nd\ne\nother2\nf\n")
	run("commit", "-qam", "other")
	run("checkout", "-q", "main")
	write("a\nmain1\nb\nc\nd\ne\nmain2\nf\n")
	run("commit", "-qam", "main")
	run("merge", "-q", "other")
	return dir
}

func TestResolveConflictsInTUI(t *testing.T) {
	dir := conflictRepo(t)
	orig, _ := os.Getwd()
	defer os.Chdir(orig) //nolint:errcheck
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	m := New()
	m.windowWidth, m.windowHeight = 120, 40
	if err := m.LoadGitComparison("HEAD", ""); err != nil {
		t.Fatal(err)
	}
	m.selectedFile = "f.txt"
	m.loadDiff()
	m.viewMode = ViewModeDiff

	press(t, m, "m")
	if m.viewMode != ViewModeConflict || len(m.conflictFile.Blocks) != 2 {
		t.Fatalf("view=%v blocks=%d, want conflict view with 2 blocks", m.viewMode, len(m.conflictFile.Blocks))
	}
	if !strings.Contains(m.View(), "0/2 resolved") {
		t.Errorf("view missing progress:\n%s", m.View())
	}

	press(t, m, "A") // refused: nothing resolved yet
	if m.viewMode != ViewModeConflict || m.errorMsg == "" {
		t.Fatalf("A with unresolved conflicts should be refused (err=%q)", m.errorMsg)
	}

	press(t, m, "o", "t", "A") // first ours, then (auto-advanced) second theirs
	if m.viewMode != ViewModeDiff || m.errorMsg != "" {
		t.Fatalf("after A: view=%v err=%q", m.viewMode, m.errorMsg)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "f.txt"))
	if want := "a\nmain1\nb\nc\nd\ne\nother2\nf\n"; string(got) != want {
		t.Errorf("resolved file = %q, want %q", got, want)
	}
	if m.conflicts["f.txt"] {
		t.Error("file still marked conflicted after A")
	}
}

func TestMergeSaveInTUI(t *testing.T) {
	dir := t.TempDir()
	left, right := filepath.Join(dir, "l.txt"), filepath.Join(dir, "r.txt")
	os.WriteFile(left, []byte("a\nold1\nold2\nb\n"), 0644) //nolint:errcheck
	os.WriteFile(right, []byte("a\nnew1\nb\n"), 0644)      //nolint:errcheck

	m := New()
	m.windowWidth, m.windowHeight = 120, 40
	m.SetLeftPath(left)
	m.SetRightPath(right)
	m.OpenDiffIfFiles()
	m.viewMode = ViewModeDiff

	press(t, m, "m", "s") // all changes selected, target left
	if m.errorMsg != "" || !strings.Contains(m.statusMsg, "Saved") {
		t.Fatalf("save: err=%q status=%q", m.errorMsg, m.statusMsg)
	}
	got, _ := os.ReadFile(left)
	if string(got) != "a\nnew1\nb\n" {
		t.Errorf("left = %q, want right's content with trailing newline", got)
	}
	if bak, _ := os.ReadFile(left + ".orig"); string(bak) != "a\nold1\nold2\nb\n" {
		t.Errorf("backup = %q", bak)
	}
}
