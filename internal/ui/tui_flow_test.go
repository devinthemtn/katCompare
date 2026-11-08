package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"katcompare/internal/config"
	"katcompare/internal/differ"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// writeTree writes files (relative path -> content) under dir.
func writeTree(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// longFile has its only change far below the first screen.
func longFile(changeAt int, changed string) string {
	var b strings.Builder
	for i := 1; i <= 80; i++ {
		if i == changeAt && changed != "" {
			b.WriteString(changed + "\n")
			continue
		}
		b.WriteString("line\n")
	}
	return b.String()
}

// browseDirs opens a directory comparison the way "katcmp left right" does.
func browseDirs(t *testing.T, width, height int) (m *Model, left, right string) {
	t.Helper()
	dir := t.TempDir()
	left, right = filepath.Join(dir, "left"), filepath.Join(dir, "right")
	writeTree(t, left, map[string]string{
		"long.txt":     longFile(60, ""),
		"onlyleft.txt": "only left\n",
		"same.txt":     "same\n",
		"uni.txt":      "a\nold\nb\n",
	})
	writeTree(t, right, map[string]string{
		"long.txt":      longFile(60, "CHANGED"),
		"onlyright.txt": "only right\n",
		"same.txt":      "same\n",
		"uni.txt":       "a\nnew\nb\n",
	})
	m = New()
	m.windowWidth, m.windowHeight = width, height
	m.SetLeftPath(left)
	m.SetRightPath(right)
	m.OpenDiffIfFiles()
	return m, left, right
}

func TestStartsOnOpenScreen(t *testing.T) {
	m := New()
	if m.viewMode != ViewModeOpen {
		t.Errorf("view = %v, want the open screen", m.viewMode)
	}
}

func TestDirectoriesOpenInFilesPane(t *testing.T) {
	m, _, _ := browseDirs(t, 120, 34)
	if m.viewMode != ViewModeFileSelect {
		t.Fatalf("view = %v, want the files pane", m.viewMode)
	}
	view := m.View()
	for _, want := range []string{"Files", "long.txt", "CHANGED"} {
		if !strings.Contains(view, want) {
			t.Errorf("two-pane view is missing %q:\n%s", want, view)
		}
	}
}

func TestNarrowTerminalShowsOnePane(t *testing.T) {
	m, _, _ := browseDirs(t, 80, 24)
	if view := m.View(); !strings.Contains(view, "long.txt") || strings.Contains(view, "CHANGED") {
		t.Errorf("narrow files pane should show only the list:\n%s", view)
	}
	press(t, m, "enter")
	if view := m.View(); !strings.Contains(view, "CHANGED") || strings.Contains(view, "same.txt") {
		t.Errorf("narrow diff pane should show only the diff:\n%s", view)
	}
	press(t, m, "esc")
	if m.viewMode != ViewModeFileSelect {
		t.Errorf("esc: view = %v, want the files pane", m.viewMode)
	}
}

func TestTextFieldsTakeAllText(t *testing.T) {
	m := New()
	m.windowWidth, m.windowHeight = 120, 34

	_, cmd := m.Update(keyMsg("q"))
	if cmd != nil {
		if _, quit := cmd().(tea.QuitMsg); quit {
			t.Fatal("typing q in a path field quit the app")
		}
	}
	press(t, m, "?")
	if m.viewMode != ViewModeOpen || m.inputLeft != "q?" {
		t.Errorf("view=%v input=%q, want q? typed into the field", m.viewMode, m.inputLeft)
	}

	// A paste arrives as one message, bracketed or not.
	m.inputLeft = ""
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/tmp/pasted path\n"), Paste: true})
	if m.inputLeft != "/tmp/pasted path" {
		t.Errorf("paste: input = %q", m.inputLeft)
	}
	m.inputLeft = ""
	press(t, m, "/tmp/burst")
	if m.inputLeft != "/tmp/burst" {
		t.Errorf("burst: input = %q", m.inputLeft)
	}
	m.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})
	if m.inputLeft != "/tmp/burst " {
		t.Errorf("space: input = %q", m.inputLeft)
	}

	// Backspace removes a whole character, not a byte.
	m.inputLeft = "né"
	press(t, m, "backspace")
	if m.inputLeft != "n" {
		t.Errorf("backspace: input = %q", m.inputLeft)
	}

	press(t, m, "f1")
	if m.viewMode != ViewModeHelp {
		t.Errorf("f1: view = %v, want help", m.viewMode)
	}
	press(t, m, "esc")
	if m.viewMode != ViewModeOpen {
		t.Errorf("leaving help: view = %v, want the open screen back", m.viewMode)
	}
}

func TestOpenScreenCompares(t *testing.T) {
	dir := t.TempDir()
	left, right := filepath.Join(dir, "l"), filepath.Join(dir, "r")
	writeTree(t, left, map[string]string{"f.txt": "a\n"})
	writeTree(t, right, map[string]string{"f.txt": "b\n"})

	m := New()
	m.windowWidth, m.windowHeight = 120, 34
	press(t, m, left, "esc", "enter") // esc: dismiss suggestions; enter: on to the right field
	if m.focus != focusInputRight {
		t.Fatalf("focus = %v, want the right field", m.focus)
	}
	press(t, m, right, "esc", "enter")
	if m.viewMode != ViewModeFileSelect || m.selectedFile != "f.txt" {
		t.Fatalf("view=%v selected=%q err=%q, want the file list", m.viewMode, m.selectedFile, m.errorMsg)
	}

	// A bad path says so and stays on the open screen.
	press(t, m, "o")
	if m.viewMode != ViewModeOpen {
		t.Fatalf("o: view = %v", m.viewMode)
	}
	m.inputRight = filepath.Join(dir, "missing")
	m.clearSuggestions()
	press(t, m, "enter")
	if m.viewMode != ViewModeOpen || m.errorMsg == "" {
		t.Errorf("bad path: view=%v err=%q", m.viewMode, m.errorMsg)
	}

	// A single-candidate Tab completes rather than cycling on the spot.
	m.focus, m.inputLeft = focusInputLeft, left[:len(left)-1]+"l"
	m.updateSuggestions()
	press(t, m, "tab")
	if m.showSuggestions {
		t.Error("Tab with one suggestion should accept it and close the list")
	}
}

func TestFilterKeepsSelectionInside(t *testing.T) {
	m, _, _ := browseDirs(t, 120, 34)
	press(t, m, "/", "uni")
	if m.selectedFile != "uni.txt" {
		t.Fatalf("selected = %q, want the only match", m.selectedFile)
	}
	press(t, m, "enter", "enter") // finish typing, then open
	if m.viewMode != ViewModeDiff || !strings.Contains(m.currentDiff.LeftFile, "uni.txt") {
		t.Fatalf("view=%v diff=%v", m.viewMode, m.currentDiff)
	}
	press(t, m, "n")
	if m.selectedFile != "uni.txt" {
		t.Errorf("n left the filter: selected = %q", m.selectedFile)
	}
	press(t, m, "esc", "esc") // back to files, clear the filter
	if len(m.getSortedFiles()) != 5 {
		t.Errorf("after clearing: %d files", len(m.getSortedFiles()))
	}

	press(t, m, "/", "nomatch")
	if m.selectedFile != "" || !strings.Contains(m.View(), "No files match") {
		t.Errorf("no match: selected=%q\n%s", m.selectedFile, m.View())
	}
	press(t, m, "esc")
	if m.selectedFile == "" {
		t.Error("clearing the filter should select a file again")
	}
}

func TestOpensOnFirstChange(t *testing.T) {
	m, _, _ := browseDirs(t, 120, 34)
	m.selectedFile = "long.txt"
	press(t, m, "enter")
	if m.viewMode != ViewModeDiff || m.currentDiff.Lines[m.cursor].Type == differ.DiffEqual {
		t.Fatalf("view=%v cursor=%d, want on the first change", m.viewMode, m.cursor)
	}
	if !strings.Contains(m.View(), "CHANGED") {
		t.Errorf("first change is off-screen:\n%s", m.View())
	}
	press(t, m, "m")
	if m.viewMode != ViewModeMerge || m.currentDiff.Lines[m.cursor].Type == differ.DiffEqual {
		t.Fatalf("merge: view=%v cursor=%d, want on the first change", m.viewMode, m.cursor)
	}
}

func TestKeysArrivingTogether(t *testing.T) {
	m, _, _ := browseDirs(t, 120, 34)
	m.selectedFile = "uni.txt"
	m.loadDiff()
	press(t, m, "m")
	if m.viewMode != ViewModeMerge {
		t.Fatalf("view = %v", m.viewMode)
	}
	press(t, m, "n", "a") // ordinary keys still work in merge

	m, _, _ = browseDirs(t, 120, 34)
	press(t, m, "jj") // two presses delivered as one message
	if m.selectedFile != "onlyright.txt" {
		t.Errorf("selected = %q, want two rows down", m.selectedFile)
	}
	press(t, m, "/", "uni") // a text field keeps them together
	if m.filterQuery != "uni" {
		t.Errorf("filter = %q", m.filterQuery)
	}
}

func TestLargeFilesDiffInTheBackground(t *testing.T) {
	big := strings.Repeat("a line of text in a big file\n", 8000)
	dir := t.TempDir()
	left, right := filepath.Join(dir, "l"), filepath.Join(dir, "r")
	writeTree(t, left, map[string]string{"a.txt": "a\n", "big.txt": big + "old\n"})
	writeTree(t, right, map[string]string{"a.txt": "b\n", "big.txt": big + "new\n"})

	m := New()
	m.windowWidth, m.windowHeight = 120, 34
	m.SetLeftPath(left)
	m.SetRightPath(right)
	m.OpenDiffIfFiles()

	// Moving onto it doesn't diff it during the key press.
	_, cmd := m.Update(keyMsg("j"))
	if cmd == nil || m.diffFile == "big.txt" {
		t.Fatalf("cmd=%v diffFile=%q, want the diff left for later", cmd != nil, m.diffFile)
	}
	if !strings.Contains(m.View(), "Loading") {
		t.Errorf("pane should say it's loading:\n%s", m.View())
	}

	// A result computed before a reload is dropped, not shown.
	stale := diffReadyMsg{relPath: "big.txt", gen: m.diffGen - 1, cached: &cachedDiff{}}
	m.Update(stale)
	if m.diffFile == "big.txt" {
		t.Error("a stale result was used")
	}

	settle(m, cmd)
	if m.diffFile != "big.txt" || !strings.Contains(m.View(), "new") {
		t.Errorf("diffFile=%q\n%s", m.diffFile, m.View())
	}

	// Moving away first: the result is kept, but stays out of the pane.
	press(t, m, "k")
	if m.diffFile != "a.txt" {
		t.Errorf("diffFile = %q", m.diffFile)
	}
}

func TestUntickedLinesStayReadable(t *testing.T) {
	for name, style := range map[string]lipgloss.Style{"insert": unselectedInsertStyle, "delete": unselectedDeleteStyle} {
		if style.GetFaint() || style.GetStrikethrough() {
			t.Errorf("unticked %s is faint or struck through", name)
		}
	}
	m, _, _ := browseDirs(t, 120, 34)
	m.selectedFile = "uni.txt"
	press(t, m, "enter", "m", "n")
	if view := m.View(); !strings.Contains(view, "[ ]") || !strings.Contains(view, "old") || !strings.Contains(view, "new") {
		t.Errorf("unticked lines should still show their text:\n%s", view)
	}
}

func TestHelpShowsTheSettingsFile(t *testing.T) {
	path, err := config.Path()
	if err != nil {
		t.Skip(err)
	}
	m := New()
	if !strings.Contains(m.renderHelpBody(), path) {
		t.Errorf("help doesn't show %q", path)
	}
}

func TestTabSwitchesPanes(t *testing.T) {
	m, _, _ := browseDirs(t, 120, 34)
	press(t, m, "tab")
	if m.viewMode != ViewModeDiff {
		t.Fatalf("tab from files: view = %v", m.viewMode)
	}
	press(t, m, "tab")
	if m.viewMode != ViewModeFileSelect {
		t.Fatalf("tab from diff: view = %v", m.viewMode)
	}
}

func TestSelectionUpdatesDiffPane(t *testing.T) {
	m, _, _ := browseDirs(t, 120, 34)
	for i := 0; i < 10 && m.selectedFile != "uni.txt"; i++ {
		press(t, m, "j")
	}
	if view := m.View(); !strings.Contains(view, "new") {
		t.Errorf("diff pane didn't follow the selection:\n%s", view)
	}
}

func TestMergeStatusAndBackupHidden(t *testing.T) {
	m, left, right := browseDirs(t, 120, 34)
	m.selectedFile = "uni.txt"
	press(t, m, "enter", "m")

	if view := m.View(); !strings.Contains(view, "Unsaved") || !strings.Contains(view, "adds 1 line and removes 1 line in uni.txt") {
		t.Errorf("merge view should say what saving does:\n%s", view)
	}
	press(t, m, "s")
	if got := readFile(t, filepath.Join(left, "uni.txt")); got != readFile(t, filepath.Join(right, "uni.txt")) {
		t.Fatalf("left = %q", got)
	}
	if got := readFile(t, filepath.Join(left, "uni.txt.orig")); got != "a\nold\nb\n" {
		t.Errorf("backup = %q", got)
	}
	if _, listed := m.allFiles["uni.txt.orig"]; listed {
		t.Error("the backup is listed as a file")
	}
	view := m.View()
	for _, want := range []string{"◆", "safe", "Merged safely to uni.txt", "Saved · matches disk"} {
		if !strings.Contains(view, want) {
			t.Errorf("after save, view is missing %q:\n%s", want, view)
		}
	}
	if m.viewMode != ViewModeMerge || m.mergeTarget != "left" {
		t.Errorf("after save: view=%v target=%q", m.viewMode, m.mergeTarget)
	}

	// An outside edit ends the status and shows the backup again on reload.
	writeTree(t, left, map[string]string{"uni.txt": "edited\n"})
	press(t, m, "r")
	if strings.Contains(m.View(), "Merged safely") {
		t.Errorf("status should clear after an outside edit:\n%s", m.View())
	}
	if _, listed := m.allFiles["uni.txt.orig"]; !listed {
		t.Error("the backup should be listed once its save is forgotten")
	}
}

func TestSaveWarnsAboutOutsideEdits(t *testing.T) {
	m, left, _ := browseDirs(t, 120, 34)
	path := filepath.Join(left, "uni.txt")
	m.selectedFile = "uni.txt"
	press(t, m, "enter", "m")

	writeTree(t, left, map[string]string{"uni.txt": "outside edit\n"})
	press(t, m, "s")
	if got := readFile(t, path); got != "outside edit\n" {
		t.Fatalf("first s overwrote the outside edit: %q", got)
	}
	if view := m.View(); !strings.Contains(view, "Changed on disk") || !strings.Contains(m.errorMsg, "again") {
		t.Fatalf("no warning: err=%q\n%s", m.errorMsg, view)
	}

	// Any other key cancels the overwrite.
	press(t, m, "j", "s")
	if got := readFile(t, path); got != "outside edit\n" {
		t.Fatalf("s after another key overwrote the edit: %q", got)
	}

	press(t, m, "s")
	if got := readFile(t, path); got == "outside edit\n" {
		t.Fatal("second s should overwrite")
	}
	if got := readFile(t, path+".orig"); got != "outside edit\n" {
		t.Errorf("backup = %q, want the outside edit", got)
	}

	// With nothing checked there's nothing to save.
	press(t, m, "n", "s")
	if !strings.Contains(m.errorMsg, "No changes") {
		t.Errorf("nothing checked: err = %q", m.errorMsg)
	}
}

func TestSaveRefusedWhenTargetMissing(t *testing.T) {
	m, left, _ := browseDirs(t, 120, 34)
	m.selectedFile = "uni.txt"
	press(t, m, "enter", "m")
	if err := os.Remove(filepath.Join(left, "uni.txt")); err != nil {
		t.Fatal(err)
	}
	press(t, m, "s")
	if !strings.Contains(m.errorMsg, "no longer exists") {
		t.Errorf("err = %q", m.errorMsg)
	}
	if _, err := os.Stat(filepath.Join(left, "uni.txt")); err == nil {
		t.Error("save recreated a deleted file")
	}
}

func TestConflictSaveKeepsBackupAndStatus(t *testing.T) {
	dir := conflictRepo(t)
	orig, _ := os.Getwd()
	defer os.Chdir(orig) //nolint:errcheck
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "f.txt")
	before := readFile(t, path)

	m := New()
	m.windowWidth, m.windowHeight = 120, 40
	if err := m.LoadGitComparison("HEAD", ""); err != nil {
		t.Fatal(err)
	}
	m.ShowFileList()
	m.selectedFile = "f.txt"
	press(t, m, "enter", "m")
	if m.viewMode != ViewModeConflict {
		t.Fatalf("view = %v", m.viewMode)
	}

	press(t, m, "o", "s") // partial save
	if got := readFile(t, path+".orig"); got != before {
		t.Errorf("no backup of the conflicted file: %q", got)
	}
	if !strings.Contains(m.statusMsg+m.errorMsg, "conflict marker") {
		t.Errorf("partial save should warn: status=%q err=%q", m.statusMsg, m.errorMsg)
	}
	if !strings.Contains(m.View(), "Check this merge") {
		t.Errorf("no check banner:\n%s", m.View())
	}

	// U resets every choice.
	press(t, m, "O", "U")
	if view := m.View(); !strings.Contains(view, "0/1 resolved") {
		t.Errorf("U: view:\n%s", view)
	}

	press(t, m, "O", "A")
	if m.viewMode != ViewModeDiff || m.conflicts["f.txt"] {
		t.Fatalf("after A: view=%v conflicts=%v err=%q", m.viewMode, m.conflicts, m.errorMsg)
	}
	if !strings.Contains(m.View(), "Merged safely") {
		t.Errorf("no safe banner:\n%s", m.View())
	}
	if _, listed := m.allFiles["f.txt.orig"]; listed {
		t.Error("the backup is listed as a file")
	}
}

func TestMergingLastGitFileKeepsItsRow(t *testing.T) {
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
	m.ShowFileList()
	m.selectedFile = "f.txt"
	press(t, m, "enter", "m", "O", "A") // ours == HEAD, so git no longer lists it
	if _, listed := m.allFiles["f.txt"]; !listed {
		t.Fatalf("row gone after resolving the last difference (err=%q)", m.errorMsg)
	}
}

func TestSwapAndReload(t *testing.T) {
	m, left, right := browseDirs(t, 120, 34)
	m.selectedFile = "uni.txt"
	m.loadDiff()
	press(t, m, "x")
	if m.leftPath != right || m.rightPath != left || m.selectedFile != "uni.txt" {
		t.Fatalf("swap: left=%q right=%q selected=%q", m.leftPath, m.rightPath, m.selectedFile)
	}
	if _, ok := m.allFiles["onlyright.txt"]; !ok || m.allFiles["onlyright.txt"].LeftFile == nil {
		t.Error("onlyright.txt should now be on the left")
	}

	writeTree(t, right, map[string]string{"uni.txt": "reloaded\n"}) // right is now the left side
	press(t, m, "r")
	if !strings.Contains(m.statusMsg, "Reloaded") || !strings.Contains(m.View(), "reloaded") {
		t.Errorf("reload: status=%q\n%s", m.statusMsg, m.View())
	}
}

func TestSwapGit(t *testing.T) {
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
	m.ShowFileList()
	press(t, m, "x")
	if !strings.Contains(m.errorMsg, "working tree") || m.leftRef != "HEAD" {
		t.Errorf("swap with the working tree: err=%q left=%q", m.errorMsg, m.leftRef)
	}
	press(t, m, "F")
	if m.errorMsg == "" {
		t.Error("F should explain it needs two directories")
	}
}

func TestCopy(t *testing.T) {
	m, left, right := browseDirs(t, 120, 34)
	press(t, m, "c")
	if m.viewMode != ViewModeCopy {
		t.Fatalf("c: view = %v (input %q)", m.viewMode, m.inputLeft)
	}
	press(t, m, "s")
	if m.viewMode != ViewModeFileSelect || !strings.Contains(m.statusMsg, "1 copied") {
		t.Fatalf("after copy: view=%v status=%q err=%q", m.viewMode, m.statusMsg, m.errorMsg)
	}
	if got := readFile(t, filepath.Join(right, "onlyleft.txt")); got != "only left\n" {
		t.Errorf("copied = %q", got)
	}
	if m.allFiles["onlyleft.txt"].Source != m.allFiles["same.txt"].Source {
		t.Error("the list should show the copy once it's done")
	}
	if strings.Contains(m.View(), "Loading") {
		t.Errorf("diff pane stuck after copying:\n%s", m.View())
	}

	// With only right-only files left, copying defaults to that direction.
	press(t, m, "c")
	if m.copyTarget != "to-left" {
		t.Errorf("copy target = %q, want to-left", m.copyTarget)
	}
	press(t, m, "s")
	if got := readFile(t, filepath.Join(left, "onlyright.txt")); got != "only right\n" {
		t.Errorf("copied = %q", got)
	}

	press(t, m, "c")
	if m.viewMode == ViewModeCopy || m.errorMsg == "" {
		t.Errorf("nothing to copy: view=%v err=%q", m.viewMode, m.errorMsg)
	}
}

func TestCopyRefusedInGitMode(t *testing.T) {
	orig, _ := os.Getwd()
	defer os.Chdir(orig) //nolint:errcheck
	repo := conflictRepo(t)
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	writeTree(t, repo, map[string]string{"added.txt": "new\n"})
	m := New()
	m.windowWidth, m.windowHeight = 120, 40
	if err := m.LoadGitComparison("HEAD", ""); err != nil {
		t.Fatal(err)
	}
	m.ShowFileList()
	press(t, m, "c")
	if m.viewMode == ViewModeCopy || m.errorMsg == "" {
		t.Errorf("view=%v err=%q, want copy refused", m.viewMode, m.errorMsg)
	}
	if _, err := os.Stat(filepath.Join(repo, "git:HEAD")); err == nil {
		t.Error("copy wrote into a folder named git:HEAD")
	}
}

func TestViewKeepsPositionWhenSwitching(t *testing.T) {
	m, _, _ := browseDirs(t, 120, 34)
	m.selectedFile = "long.txt"
	press(t, m, "enter")
	press(t, m, "s")
	if m.sbsRows[m.cursor].Type == 0 && m.sbsRows[m.cursor].RightContent == "line" {
		t.Errorf("s lost the position: cursor=%d", m.cursor)
	}
	if !strings.Contains(m.View(), "CHANGED") {
		t.Errorf("change scrolled away:\n%s", m.View())
	}
}

func TestHostileContentCannotBreakTheScreen(t *testing.T) {
	dir := t.TempDir()
	left, right := filepath.Join(dir, "l.txt"), filepath.Join(dir, "r.txt")
	writeTree(t, dir, map[string]string{
		"l.txt": "a\r\n\x1b[31mred\ttab\x07\r\n",
		"r.txt": "a\r\nb\xffbad\r\n",
	})
	m := New()
	m.windowWidth, m.windowHeight = 120, 34
	m.SetLeftPath(left)
	m.SetRightPath(right)
	m.OpenDiffIfFiles()
	view := m.View()
	for _, bad := range []string{"\r", "\x07", "\t", "\x1b[31mred"} {
		if strings.Contains(view, bad) {
			t.Errorf("view contains %q:\n%q", bad, view)
		}
	}
}

// TestEveryViewFitsTheTerminal renders every screen at several sizes with long
// paths, Unicode, wide characters, tabs and long lines.
func TestEveryViewFitsTheTerminal(t *testing.T) {
	long := strings.Repeat("a-very-long-directory-name/", 6)
	wide := strings.Repeat("日本語のテキスト héllo\tワイド ", 30)

	for _, size := range [][2]int{{60, 20}, {80, 24}, {100, 30}, {120, 34}, {200, 50}} {
		w, h := size[0], size[1]

		dir := t.TempDir()
		left, right := filepath.Join(dir, long, "left"), filepath.Join(dir, long, "right")
		name := strings.Repeat("ファイル-long-name-", 8) + ".txt"
		writeTree(t, left, map[string]string{
			name:           "a\n" + wide + "\nb\n",
			"long.txt":     longFile(60, ""),
			"onlyleft.txt": "x\n",
		})
		writeTree(t, right, map[string]string{
			name:            "a\nCHANGED " + wide + "\nb\n",
			"long.txt":      longFile(60, "CHANGED"),
			"onlyright.txt": "y\n",
		})

		m := New()
		m.windowWidth, m.windowHeight = w, h
		check := func(view string) {
			t.Helper()
			out := m.View()
			lines := strings.Split(out, "\n")
			if len(lines) > h {
				t.Errorf("%dx%d %s: %d lines tall:\n%s", w, h, view, len(lines), out)
			}
			for _, line := range lines {
				if got := lipgloss.Width(line); got > w {
					t.Errorf("%dx%d %s: line %d wide: %q", w, h, view, got, line)
				}
			}
		}

		check("empty open")
		m.inputLeft = left
		m.updateSuggestions()
		check("open with suggestions")
		m.errorMsg = strings.Repeat("a long error ", 40)
		check("open with error")
		m.errorMsg = ""
		m.clearSuggestions()

		m.SetLeftPath(left)
		m.SetRightPath(right)
		m.OpenDiffIfFiles()
		check("files")
		press(t, m, "/", "long")
		check("filter")
		press(t, m, "esc")
		press(t, m, "enter")
		check("diff")
		press(t, m, "l", "l", "s")
		check("side by side")
		press(t, m, "s", "n")
		check("next file")
		for i := 0; i < 10 && m.selectedFile != name; i++ {
			press(t, m, "n")
		}
		check("wide diff")
		press(t, m, "s")
		check("wide side by side")
		press(t, m, "s", "m")
		check("merge")
		press(t, m, "s")
		check("after save")
		press(t, m, "esc")
		check("diff with banner")
		press(t, m, "esc", "c")
		check("copy")
		press(t, m, "esc", "?")
		check("help")
		press(t, m, "G")
		check("end of help")
		press(t, m, "esc", "o")
		check("open again")
	}

	// Conflicts and the git picker need a repo.
	repo := conflictRepo(t)
	orig, _ := os.Getwd()
	defer os.Chdir(orig) //nolint:errcheck
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}
	for _, size := range [][2]int{{60, 20}, {80, 24}, {120, 34}} {
		w, h := size[0], size[1]
		m := New()
		m.windowWidth, m.windowHeight = w, h
		if err := m.LoadGitComparison("HEAD", ""); err != nil {
			t.Fatal(err)
		}
		m.selectedFile = "f.txt"
		m.loadDiff()
		m.viewMode = ViewModeFileSelect
		for _, step := range []struct{ name, key string }{
			{"files", ""}, {"diff", "enter"}, {"conflict", "m"}, {"chosen", "o"}, {"back", "esc"}, {"git picker", "ctrl+g"},
		} {
			if step.key != "" {
				press(t, m, step.key)
			}
			out := m.View()
			lines := strings.Split(out, "\n")
			if len(lines) > h {
				t.Errorf("%dx%d %s: %d lines tall:\n%s", w, h, step.name, len(lines), out)
			}
			for _, line := range lines {
				if got := lipgloss.Width(line); got > w {
					t.Errorf("%dx%d %s: line %d wide: %q", w, h, step.name, got, line)
				}
			}
		}
	}
}

// compareDirs compares two folders the way the open screen does.
func compareDirs(t *testing.T, leftFiles, rightFiles map[string]string) (m *Model, left, right string) {
	t.Helper()
	dir := t.TempDir()
	left, right = filepath.Join(dir, "left"), filepath.Join(dir, "right")
	if err := os.MkdirAll(left, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(right, 0755); err != nil {
		t.Fatal(err)
	}
	writeTree(t, left, leftFiles)
	writeTree(t, right, rightFiles)
	m = New()
	m.windowWidth, m.windowHeight = 120, 34
	m.SetLeftPath(left)
	m.SetRightPath(right)
	m.OpenDiffIfFiles()
	return m, left, right
}

func TestConfigIgnoreAndHidden(t *testing.T) {
	files := map[string]string{
		"a.txt": "a\n", "b.log": "b\n", ".env": "x\n", ".git/config": "x\n", "node_modules/m.js": "m\n", "src/node_modules/n.js": "n\n",
	}
	m, _, _ := compareDirs(t, files, files)
	if len(m.allFiles) != 4 { // a, b, and both node_modules files
		t.Errorf("default: %d files", len(m.allFiles))
	}

	writeConfig(t, "[compare]\nignore = [\"node_modules\", \"*.log\"]\n")
	m, _, _ = compareDirs(t, files, files)
	if len(m.allFiles) != 1 || m.allFiles["a.txt"] == nil {
		t.Errorf("ignore: %v", m.getSortedFiles())
	}

	writeConfig(t, "[compare]\nshow_hidden = true\nignore = [\"node_modules\", \"*.log\"]\n")
	m, _, _ = compareDirs(t, files, files)
	if m.allFiles[".env"] == nil || m.allFiles[".git/config"] != nil {
		t.Errorf("show_hidden: %v", m.getSortedFiles())
	}
}

func TestConfigAppliesOnReloadAndOnCompare(t *testing.T) {
	files := map[string]string{"a.txt": "a\n", "b.log": "b\n"}
	m, _, _ := compareDirs(t, files, files)
	if len(m.allFiles) != 2 {
		t.Fatalf("%d files", len(m.allFiles))
	}
	writeConfig(t, "[compare]\nignore = [\"*.log\"]\n")
	press(t, m, "r")
	if len(m.allFiles) != 1 {
		t.Errorf("after reload: %v", m.getSortedFiles())
	}
}

func TestConfigHideIdentical(t *testing.T) {
	writeConfig(t, "[compare]\nhide_identical = true\n")
	m, left, _ := compareDirs(t,
		map[string]string{"same.txt": "s\n", "diff.txt": "a\n", "only.txt": "o\n"},
		map[string]string{"same.txt": "s\n", "diff.txt": "b\n"})
	if m.allFiles["same.txt"] != nil || m.allFiles["diff.txt"] == nil || m.allFiles["only.txt"] == nil {
		t.Fatalf("list: %v", m.getSortedFiles())
	}
	if !strings.Contains(m.View(), "1 identical hidden") {
		t.Errorf("no hidden count:\n%s", m.View())
	}

	// A file that a merge just made identical keeps its row, with its status.
	m.selectedFile = "diff.txt"
	press(t, m, "enter", "m", "s")
	if readFile(t, filepath.Join(left, "diff.txt")) != "b\n" {
		t.Fatalf("merge didn't save: %q", m.errorMsg)
	}
	if m.allFiles["diff.txt"] == nil {
		t.Errorf("the merged file lost its row: %v", m.getSortedFiles())
	}

	// All identical says so.
	m, _, _ = compareDirs(t, map[string]string{"s.txt": "s\n"}, map[string]string{"s.txt": "s\n"})
	if !strings.Contains(m.View(), "All 1 file identical") {
		t.Errorf("all identical:\n%s", m.View())
	}

	// Comparing two single files always shows them.
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"l.txt": "s\n", "r.txt": "s\n"})
	m = New()
	m.windowWidth, m.windowHeight = 120, 34
	m.SetLeftPath(filepath.Join(dir, "l.txt"))
	m.SetRightPath(filepath.Join(dir, "r.txt"))
	m.OpenDiffIfFiles()
	if len(m.allFiles) != 1 || m.viewMode != ViewModeDiff {
		t.Errorf("two files: %d rows, view %v", len(m.allFiles), m.viewMode)
	}
}

func TestConfigIgnoreLineEndingsAndWhitespace(t *testing.T) {
	left := map[string]string{"crlf.txt": "a\r\nb\r\n", "ws.txt": "  a b\n", "real.txt": "x\n"}
	right := map[string]string{"crlf.txt": "a\nb\n", "ws.txt": "a b  \n", "real.txt": "y\n"}

	// Off: all three differ.
	m, _, _ := compareDirs(t, left, right)
	if m.differCount != 3 {
		t.Errorf("default: %d differ", m.differCount)
	}
	if strings.Contains(m.View(), "ignoring") {
		t.Errorf("no ignore setting, but the view mentions one:\n%s", m.View())
	}

	writeConfig(t, "[compare]\nignore_line_endings = true\nignore_whitespace = \"all\"\n")
	m, _, _ = compareDirs(t, left, right)
	if m.differCount != 1 {
		t.Errorf("ignoring: %d differ", m.differCount)
	}
	m.selectedFile = "crlf.txt"
	press(t, m, "enter")
	if !strings.Contains(m.View(), "ignoring whitespace, line endings") {
		t.Errorf("a file showing as the same must say why:\n%s", m.View())
	}
	if _, ins, del := m.currentDiff.GetStats(); ins != 0 || del != 0 {
		t.Errorf("diff shows +%d -%d", ins, del)
	}
	press(t, m, "m")
	if m.viewMode == ViewModeMerge || !strings.Contains(m.errorMsg, "when ignoring") {
		t.Errorf("merge on a same-when-ignoring file: view %v, err %q", m.viewMode, m.errorMsg)
	}
}

func TestConfigMergeKeepsBytesThroughTheTUI(t *testing.T) {
	writeConfig(t, "[compare]\nignore_whitespace = \"all\"\n[view]\ndefault_merge_target = \"right\"\n")
	m, _, right := compareDirs(t,
		map[string]string{"f.txt": "keep\nold\n"},
		map[string]string{"f.txt": "  k e e p  \nnew\n"})
	m.selectedFile = "f.txt"
	press(t, m, "enter", "m", "s")
	if got := readFile(t, filepath.Join(right, "f.txt")); got != "  k e e p  \nold\n" {
		t.Errorf("right = %q: the unchanged line lost its spacing", got)
	}
}

// Merge selections are line numbers, so changing the settings while the merge
// view is open must not change what gets saved.
func TestConfigEditDuringMergeDoesNotChangeTheSave(t *testing.T) {
	m, left, _ := compareDirs(t,
		map[string]string{"f.txt": "a\n  b\nold\n"},
		map[string]string{"f.txt": "a\nb\nnew\n"})
	m.selectedFile = "f.txt"
	press(t, m, "enter", "m") // two changes: "  b" and "old"

	writeConfig(t, "[compare]\nignore_whitespace = \"all\"\nignore = [\"*\"]\n")
	press(t, m, "s")
	if got := readFile(t, filepath.Join(left, "f.txt")); got != "a\nb\nnew\n" {
		t.Errorf("saved %q, want what was on screen applied", got)
	}
}

func TestConfigEditorIsUsed(t *testing.T) {
	writeConfig(t, "[tools]\neditor = \"definitely-not-installed-editor\"\n")
	m, _, _ := compareDirs(t, map[string]string{"f.txt": "a\n"}, map[string]string{"f.txt": "b\n"})
	press(t, m, "enter", "e")
	if !strings.Contains(m.errorMsg, "definitely-not-installed-editor") {
		t.Errorf("err = %q, want it to name the missing editor", m.errorMsg)
	}
}

func TestConfigDefaultViewAndTarget(t *testing.T) {
	if m := New(); m.diffViewMode != DiffViewUnified || m.mergeTarget != "left" {
		t.Errorf("defaults: view %v, target %q", m.diffViewMode, m.mergeTarget)
	}
	writeConfig(t, "[view]\ndefault_view = \"sidebyside\"\ndefault_merge_target = \"right\"\n")
	if m := New(); m.diffViewMode != DiffViewSideBySide || m.mergeTarget != "right" {
		t.Errorf("set: view %v, target %q", m.diffViewMode, m.mergeTarget)
	}
}

func TestConfigProblemIsShown(t *testing.T) {
	writeConfig(t, "[compare]\nignore_whitespace = \"some\"\nignore = [\"*.txt\"]\n")
	m, _, _ := compareDirs(t, map[string]string{"f.txt": "a\n"}, map[string]string{"f.txt": "b\n"})
	if !strings.Contains(m.View(), "Using default settings") {
		t.Errorf("no warning:\n%s", m.View())
	}
	if m.allFiles["f.txt"] == nil {
		t.Error("a bad file should give all defaults, ignoring nothing")
	}
	press(t, m, "j") // the warning stays until it's fixed, not just until a key
	if !strings.Contains(m.View(), "Using default settings") {
		t.Errorf("warning went away:\n%s", m.View())
	}
	writeConfig(t, "")
	press(t, m, "r")
	press(t, m, "j")
	if strings.Contains(m.View(), "Using default settings") {
		t.Errorf("warning should clear once fixed:\n%s", m.View())
	}
}

func TestFileEditedAfterLoadingShowsItsNewText(t *testing.T) {
	// Narrow, so that a file isn't read until it's opened.
	m, left, _ := compareDirs(t, map[string]string{"f.txt": "old\n"}, map[string]string{"f.txt": "new\n"})
	m.windowWidth = 80
	m.currentDiff, m.diffFile, m.diffCache, m.diffOrder = nil, "", nil, nil
	writeTree(t, left, map[string]string{"f.txt": "edited after loading\n"})
	press(t, m, "enter")
	if !strings.Contains(m.View(), "edited after loading") {
		t.Errorf("should show the file as it is now:\n%s", m.View())
	}
}

func TestOpenFileKeepsItsTextThroughOutsideEdits(t *testing.T) {
	m, left, _ := compareDirs(t, map[string]string{"f.txt": "old\n"}, map[string]string{"f.txt": "new\n"})
	m.selectedFile = "f.txt"
	press(t, m, "enter", "m")
	writeTree(t, left, map[string]string{"f.txt": "edited while open\n"})
	press(t, m, "j")
	if !strings.Contains(m.View(), "old") {
		t.Errorf("the open view should keep what it showed:\n%s", m.View())
	}
	press(t, m, "s")
	if !strings.Contains(m.View(), "Changed on disk") {
		t.Errorf("no warning about the edit:\n%s", m.View())
	}
	if got := readFile(t, filepath.Join(left, "f.txt")); got != "edited while open\n" {
		t.Errorf("first s overwrote the edit: %q", got)
	}
}

func TestFileDeletedAfterLoadingSaysSo(t *testing.T) {
	m, left, _ := compareDirs(t,
		map[string]string{"a.txt": "a\n", "f.txt": "old\n"},
		map[string]string{"a.txt": "b\n", "f.txt": "new\n"})
	if err := os.Remove(filepath.Join(left, "f.txt")); err != nil {
		t.Fatal(err)
	}
	m.selectedFile = "f.txt"
	press(t, m, "enter")
	if !strings.Contains(m.View(), "Couldn't read this file") {
		t.Errorf("no error shown:\n%s", m.View())
	}
	press(t, m, "m", "s", "j", "esc") // nothing here should crash
	press(t, m, "r")
	if m.allFiles["f.txt"] == nil || m.allFiles["f.txt"].LeftFile != nil {
		t.Errorf("reload should show the file as right-only: %+v", m.allFiles["f.txt"])
	}
}

func TestReloadWhenMergeTargetBecomesUnreadable(t *testing.T) {
	m, left, right := compareDirs(t, map[string]string{"f.txt": "old\n"}, map[string]string{"f.txt": "new\n"})
	m.selectedFile = "f.txt"
	press(t, m, "enter", "m")
	os.Remove(filepath.Join(left, "f.txt"))
	os.Remove(filepath.Join(right, "f.txt"))
	press(t, m, "r") // must not crash
	if m.viewMode == ViewModeMerge {
		t.Errorf("merge view stayed open with no file: %v", m.viewMode)
	}
	_ = m.View()
}

func TestDiffCacheStaysWithinItsLimit(t *testing.T) {
	old := maxCachedBytes
	maxCachedBytes = 1000
	defer func() { maxCachedBytes = old }()

	big := strings.Repeat("x\n", 300) // 600 bytes a side, so a pair is over the limit
	left, right := map[string]string{}, map[string]string{}
	for i := 0; i < 6; i++ {
		name := fmt.Sprintf("f%d.txt", i)
		left[name], right[name] = big+"l\n", big+"r\n"
	}
	m, leftDir, _ := compareDirs(t, left, right)
	for i := 0; i < 6; i++ {
		press(t, m, "j")
		total := 0
		for _, c := range m.diffCache {
			total += len(c.left) + len(c.right)
		}
		if total > maxCachedBytes {
			t.Fatalf("cache holds %d bytes, limit %d", total, maxCachedBytes)
		}
		if m.currentDiff == nil || m.diffFile != m.selectedFile {
			t.Fatalf("the open file has no diff: %q vs %q", m.diffFile, m.selectedFile)
		}
	}

	// The open file was dropped from the cache, but a merge still saves what's shown.
	if len(m.diffCache) != 0 {
		t.Fatalf("expected an empty cache, have %d", len(m.diffCache))
	}
	name := m.selectedFile
	press(t, m, "enter", "m", "s")
	if got := readFile(t, filepath.Join(leftDir, name)); got != big+"r\n" {
		t.Errorf("saved %d bytes, err %q", len(got), m.errorMsg)
	}
}

func TestGoingBackToAnEditedFileShowsItsNewText(t *testing.T) {
	m, left, _ := compareDirs(t,
		map[string]string{"a.txt": "a1\n", "b.txt": "b1\n"},
		map[string]string{"a.txt": "a2\n", "b.txt": "b2\n"})
	press(t, m, "j") // on to b.txt, with a.txt now in the cache
	if m.selectedFile != "b.txt" {
		t.Fatalf("selected %q", m.selectedFile)
	}
	writeTree(t, left, map[string]string{"a.txt": "edited while away, a different size\n"})
	press(t, m, "k")
	if !strings.Contains(m.View(), "edited while away") {
		t.Errorf("should show the file as it is now:\n%s", m.View())
	}
}

// Terminals that pass Ctrl+- along send it as Ctrl+_. Font size is the terminal's
// business, so it mustn't do anything here, including while typing a path.
func TestCtrlMinusDoesNothing(t *testing.T) {
	m, _, _ := compareDirs(t, map[string]string{"f.txt": "a\n"}, map[string]string{"f.txt": "b\n"})
	key := tea.KeyMsg{Type: tea.KeyCtrlUnderscore}
	for _, view := range []string{"o", "esc", "enter", "m"} {
		before := m.viewMode
		_, cmd := m.Update(key)
		if cmd != nil || m.viewMode != before || m.errorMsg != "" {
			t.Errorf("view %v: cmd=%v view=%v err=%q", before, cmd != nil, m.viewMode, m.errorMsg)
		}
		press(t, m, view)
	}
	m = New()
	m.inputLeft = "typed"
	m.Update(key)
	if m.inputLeft != "typed" {
		t.Errorf("input = %q", m.inputLeft)
	}
}

// A terminal's zoom reaches the app as a change of size, in the middle of anything.
func TestZoomingTheTerminalKeepsThePosition(t *testing.T) {
	m, _, _ := browseDirs(t, 120, 34)
	m.selectedFile = "long.txt"
	press(t, m, "enter", "m")
	cursor := m.cursor
	for _, size := range [][2]int{{150, 42}, {90, 28}, {60, 18}, {200, 60}, {120, 34}} {
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		out := m.View()
		if m.viewMode != ViewModeMerge || m.cursor != cursor {
			t.Fatalf("%v: view %v cursor %d", size, m.viewMode, m.cursor)
		}
		if !strings.Contains(out, "▶") {
			t.Errorf("%v: the cursor scrolled away:\n%s", size, out)
		}
		lines := strings.Split(out, "\n")
		if len(lines) > size[1] {
			t.Errorf("%v: %d lines tall", size, len(lines))
		}
		for _, line := range lines {
			if lipgloss.Width(line) > size[0] {
				t.Errorf("%v: line %d wide", size, lipgloss.Width(line))
			}
		}
	}
}

// Zellij, for one, keeps Ctrl+G for itself, so git comparison needs other ways in.
func TestGitCompareWithoutCtrlG(t *testing.T) {
	repo := conflictRepo(t)
	orig, _ := os.Getwd()
	defer os.Chdir(orig) //nolint:errcheck
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}

	// The Git button: Tab from the left field past the right field and Compare.
	m := New()
	m.windowWidth, m.windowHeight = 120, 34
	if !strings.Contains(m.View(), "Git") {
		t.Fatalf("no Git button:\n%s", m.View())
	}
	press(t, m, "tab", "tab", "tab")
	if m.focus != focusGitBtn {
		t.Fatalf("focus = %v, want the Git button", m.focus)
	}
	press(t, m, "enter")
	if m.viewMode != ViewModeGitSelect {
		t.Fatalf("Enter on Git: view = %v, err = %q", m.viewMode, m.errorMsg)
	}
	press(t, m, "esc")
	if m.viewMode != ViewModeOpen {
		t.Errorf("leaving the picker: view = %v", m.viewMode)
	}

	// Space presses it too, and Shift+Tab reaches it from the other way.
	m = New()
	m.windowWidth, m.windowHeight = 120, 34
	press(t, m, "shift+tab", "shift+tab")
	if m.focus != focusGitBtn {
		t.Fatalf("focus = %v, want the Git button", m.focus)
	}
	m.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")})
	if m.viewMode != ViewModeGitSelect {
		t.Errorf("Space on Git: view = %v", m.viewMode)
	}

	// Quit stays last, and Tab comes back round to the left field.
	m = New()
	press(t, m, "tab", "tab", "tab", "tab")
	if m.focus != focusQuitBtn {
		t.Errorf("focus = %v, want Quit", m.focus)
	}
	press(t, m, "tab")
	if m.focus != focusInputLeft {
		t.Errorf("focus = %v, want the left field", m.focus)
	}

	// F2 works while typing a path, without typing anything.
	m = New()
	m.windowWidth, m.windowHeight = 120, 34
	press(t, m, "typed", "f2")
	if m.viewMode != ViewModeGitSelect || m.inputLeft != "typed" {
		t.Errorf("F2 on the open screen: view = %v, input = %q", m.viewMode, m.inputLeft)
	}

	// And from the file list and the diff.
	m = New()
	m.windowWidth, m.windowHeight = 120, 34
	if err := m.LoadGitComparison("HEAD", ""); err != nil {
		t.Fatal(err)
	}
	m.ShowFileList()
	press(t, m, "f2")
	if m.viewMode != ViewModeGitSelect {
		t.Errorf("F2 from the file list: view = %v", m.viewMode)
	}
	press(t, m, "esc", "enter", "f2")
	if m.viewMode != ViewModeGitSelect {
		t.Errorf("F2 from the diff: view = %v", m.viewMode)
	}
}

func TestGitButtonOutsideARepositorySaysSo(t *testing.T) {
	m := New()
	m.windowWidth, m.windowHeight = 120, 34
	press(t, m, "tab", "tab", "tab", "enter")
	if m.viewMode != ViewModeOpen || m.errorMsg == "" {
		t.Errorf("view = %v, err = %q", m.viewMode, m.errorMsg)
	}
}

func TestGitModeLeavesOutBinaryFiles(t *testing.T) {
	dir := conflictRepo(t)
	orig, _ := os.Getwd()
	defer os.Chdir(orig) //nolint:errcheck
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	writeTree(t, dir, map[string]string{"image.png": "\x89PNG\x00", "notes.txt": "new\n"})
	if out, err := exec.Command("git", "-C", dir, "add", "image.png", "notes.txt").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	m := New()
	m.windowWidth, m.windowHeight = 120, 40
	if err := m.LoadGitComparison("HEAD", ""); err != nil {
		t.Fatal(err)
	}
	if _, listed := m.allFiles["image.png"]; listed {
		t.Error("binary image.png is listed")
	}
	if _, listed := m.allFiles["notes.txt"]; !listed {
		t.Error("text notes.txt is missing")
	}
}
