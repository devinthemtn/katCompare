package ui

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TestRandomKeysNeverBreakTheScreen presses random keys in every view, with
// files changing and disappearing underneath, at several sizes. Whatever
// happens, the app mustn't crash, and the screen must fit the terminal.
func TestRandomKeysNeverBreakTheScreen(t *testing.T) {
	keys := []string{
		"j", "k", "g", "G", "h", "l", "n", "p", "s", "m", "c", "r", "x", "o", "/", "?", "t", "a",
		"A", "o", "t", "b", "B", "u", "U", "O", "T", " ", "]", "[",
		"enter", "esc", "tab", "shift+tab", "up", "down", "pgup", "pgdown", "backspace", "f1", "f2", "ctrl+d", "ctrl+g",
		"text", "typed/path", "uni",
	}
	check := func(t *testing.T, m *Model, seed int64, step int, history []string) {
		t.Helper()
		out := m.View()
		lines := strings.Split(out, "\n")
		fail := func(why string) {
			t.Fatalf("seed %d step %d view %v %dx%d: %s\nlast keys %q\n%s", seed, step, m.viewMode, m.windowWidth, m.windowHeight, why, history, out)
		}
		if len(lines) > m.windowHeight {
			fail(fmt.Sprintf("%d lines tall", len(lines)))
		}
		for _, line := range lines {
			if lipgloss.Width(line) > m.windowWidth {
				fail(fmt.Sprintf("line %d wide", lipgloss.Width(line)))
			}
		}
	}
	mutate := func(r *rand.Rand, dirs []string) {
		dir := dirs[r.Intn(len(dirs))]
		name := fmt.Sprintf("f%d.txt", r.Intn(6))
		path := filepath.Join(dir, name)
		switch r.Intn(4) {
		case 0:
			os.Remove(path)
		case 1:
			os.WriteFile(path, []byte(fmt.Sprintf("changed %d\nnew line\n", r.Intn(100))), 0644) //nolint:errcheck
		case 2:
			os.WriteFile(path, []byte("a\r\nb\r\n"), 0644) //nolint:errcheck
		case 3:
			os.WriteFile(path, []byte{0, 1, 2}, 0644) //nolint:errcheck
		}
	}
	run := func(t *testing.T, seed int64, m *Model, dirs []string) {
		r := rand.New(rand.NewSource(seed))
		var history []string
		for step := 0; step < 400; step++ {
			key := keys[r.Intn(len(keys))]
			history = append(history, key)
			if len(history) > 25 {
				history = history[1:]
			}
			// e, D, R and F would start external programs, so they aren't pressed.
			_, cmd := m.Update(keyMsg(key))
			if settleUntilQuit(m, cmd) {
				return
			}
			if r.Intn(25) == 0 && len(dirs) > 0 {
				mutate(r, dirs)
			}
			if r.Intn(40) == 0 {
				sizes := [][2]int{{60, 20}, {80, 24}, {99, 30}, {100, 30}, {120, 34}, {200, 50}, {40, 10}}
				size := sizes[r.Intn(len(sizes))]
				m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			}
			if r.Intn(40) == 0 {
				m.Update(tea.FocusMsg{})
			}
			check(t, m, seed, step, history)
		}
	}

	for seed := int64(0); seed < 40; seed++ {
		seed := seed
		t.Run(fmt.Sprintf("folders/%d", seed), func(t *testing.T) {
			if seed%2 == 1 {
				writeConfig(t, "[compare]\nhide_identical = true\nignore_line_endings = true\nignore_whitespace = \"all\"\nshow_hidden = true\n")
			}
			left, right := map[string]string{}, map[string]string{}
			for i := 0; i < 6; i++ {
				name := fmt.Sprintf("f%d.txt", i)
				left[name] = strings.Repeat(fmt.Sprintf("line %d\n", i), 30)
				right[name] = strings.Repeat(fmt.Sprintf("line %d\n", i), 20) + "changed\n"
			}
			left["same.txt"], right["same.txt"] = "same\n", "same\n"
			left["only-left.txt"] = "l\n"
			right["only-right.txt"] = "r\n"
			m, l, r := compareDirs(t, left, right)
			run(t, seed, m, []string{l, r})
		})
	}
	for seed := int64(100); seed < 120; seed++ {
		seed := seed
		t.Run(fmt.Sprintf("git/%d", seed), func(t *testing.T) {
			dir := conflictRepo(t)
			orig, _ := os.Getwd()
			defer os.Chdir(orig) //nolint:errcheck
			if err := os.Chdir(dir); err != nil {
				t.Fatal(err)
			}
			writeTree(t, dir, map[string]string{"added.txt": "new\n"})
			m := New()
			m.windowWidth, m.windowHeight = 120, 34
			if err := m.LoadGitComparison("HEAD", ""); err != nil {
				t.Fatal(err)
			}
			m.ShowFileList()
			run(t, seed, m, nil)
		})
	}
	for seed := int64(200); seed < 210; seed++ {
		seed := seed
		t.Run(fmt.Sprintf("open-screen/%d", seed), func(t *testing.T) {
			m := New()
			m.windowWidth, m.windowHeight = 80, 24
			run(t, seed, m, nil)
		})
	}
}

// settleUntilQuit runs a command and what follows from it, and reports whether
// the app quit.
func settleUntilQuit(m *Model, cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	switch msg := cmd().(type) {
	case nil:
		return false
	case tea.QuitMsg:
		return true
	case tea.BatchMsg:
		for _, c := range msg {
			if settleUntilQuit(m, c) {
				return true
			}
		}
		return false
	default:
		_, next := m.Update(msg)
		return settleUntilQuit(m, next)
	}
}
