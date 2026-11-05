package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// conflictRepo makes a repo whose merge left two conflicts in f.txt, and
// changes into it for the test.
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

	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestConflictEndpoints(t *testing.T) {
	dir := conflictRepo(t)
	a := NewApp(nil)
	if _, err := a.LoadGitComparison("HEAD", ""); err != nil {
		t.Fatal(err)
	}

	blocks, err := a.GetConflicts("f.txt")
	if err != nil || len(blocks) != 2 {
		t.Fatalf("GetConflicts = %d blocks, %v; want 2", len(blocks), err)
	}
	if blocks[0].OursLabel != "HEAD" || blocks[0].TheirsLabel != "other" || len(blocks[0].After) == 0 {
		t.Errorf("block 0 = %+v", blocks[0])
	}

	if _, err := a.SaveConflicts("f.txt", []string{"ours", ""}, true); err == nil {
		t.Fatal("mark resolved with an unresolved conflict should fail")
	}
	msg, err := a.SaveConflicts("f.txt", []string{"ours", ""}, false)
	if err != nil || !strings.Contains(msg, "1 conflict(s) left") {
		t.Fatalf("partial save: %q, %v", msg, err)
	}
	if blocks, _ = a.GetConflicts("f.txt"); len(blocks) != 1 {
		t.Fatalf("after partial save: %d blocks, want 1", len(blocks))
	}

	if _, err := a.SaveConflicts("f.txt", []string{"both"}, true); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "f.txt"))
	if want := "a\nmain1\nb\nc\nd\ne\nmain2\nother2\nf\n"; string(got) != want {
		t.Errorf("file = %q, want %q", got, want)
	}
	out, _ := exec.Command("git", "-C", dir, "diff", "--name-only", "--diff-filter=U").Output()
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("still unmerged after mark resolved: %s", out)
	}
}

func TestSaveMergeRefusesGitRevision(t *testing.T) {
	conflictRepo(t)
	a := NewApp(nil)
	if _, err := a.LoadGitComparison("HEAD", ""); err != nil {
		t.Fatal(err)
	}
	_, err := a.SaveMerge(MergeSaveRequest{RelPath: "f.txt", Target: "left"})
	if err == nil || !strings.Contains(err.Error(), "git revision") {
		t.Fatalf("SaveMerge to a git ref: err = %v, want refusal", err)
	}
	if matches, _ := filepath.Glob("git:*"); len(matches) > 0 {
		t.Errorf("stray files written: %v", matches)
	}
}

// mergeDirs makes two directories holding different versions of f.txt and
// loads them.
func mergeDirs(t *testing.T) (*App, string) {
	t.Helper()
	left, right := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(left, "f.txt"), []byte("a\nleft\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(right, "f.txt"), []byte("a\nright\n"), 0644); err != nil {
		t.Fatal(err)
	}
	a := NewApp(nil)
	if _, err := a.LoadComparison(left, right); err != nil {
		t.Fatal(err)
	}
	return a, left
}

// selectAll returns a save request with every change in relPath checked.
func selectAll(t *testing.T, a *App, relPath, target string) MergeSaveRequest {
	t.Helper()
	diff, err := a.GetDiff(relPath)
	if err != nil {
		t.Fatal(err)
	}
	req := MergeSaveRequest{RelPath: relPath, Target: target}
	for i, l := range diff.Lines {
		switch l.Type {
		case "insert":
			req.SelectedInsertions = append(req.SelectedInsertions, i)
		case "delete":
			req.SelectedDeletions = append(req.SelectedDeletions, i)
		}
	}
	return req
}

func findEntry(result *ComparisonResult, relPath string) *FileEntry {
	for i := range result.Files {
		if result.Files[i].RelPath == relPath {
			return &result.Files[i]
		}
	}
	return nil
}

func TestSaveMergeStatus(t *testing.T) {
	a, left := mergeDirs(t)
	if _, err := a.SaveMerge(selectAll(t, a, "f.txt", "left")); err != nil {
		t.Fatal(err)
	}

	result, err := a.LoadComparison(a.leftPath, a.rightPath)
	if err != nil {
		t.Fatal(err)
	}
	entry := findEntry(result, "f.txt")
	if entry == nil || entry.MergeStatus == nil || entry.MergeStatus.State != "safe" {
		t.Fatalf("entry after merge = %+v, want safe", entry)
	}
	if s := entry.MergeStatus; s.Target != "f.txt" || s.Backup != "f.txt.orig" || s.Kind != "merge" {
		t.Errorf("status = %+v", s)
	}
	if findEntry(result, "f.txt.orig") != nil {
		t.Error("the backup is listed as a file")
	}
	if diff, err := a.GetDiff("f.txt"); err != nil || diff.MergeStatus == nil {
		t.Errorf("GetDiff status = %+v, %v", diff, err)
	}

	// An edit after the save clears the status and shows the backup again.
	if err := os.WriteFile(filepath.Join(left, "f.txt"), []byte("edited\n"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err = a.LoadComparison(a.leftPath, a.rightPath)
	if err != nil {
		t.Fatal(err)
	}
	if entry := findEntry(result, "f.txt"); entry == nil || entry.MergeStatus != nil {
		t.Errorf("entry after an outside edit = %+v, want no status", entry)
	}
	if findEntry(result, "f.txt.orig") == nil {
		t.Error("the backup should be listed once its save is forgotten")
	}
}

func TestSaveMergeReportsLeftoverMarkers(t *testing.T) {
	a, left := mergeDirs(t)
	right := a.rightPath
	if err := os.WriteFile(filepath.Join(right, "f.txt"), []byte("a\n<<<<<<< HEAD\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.LoadComparison(left, right); err != nil {
		t.Fatal(err)
	}
	msg, err := a.SaveMerge(selectAll(t, a, "f.txt", "left"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "Check: 1 conflict marker line(s) remain, first at line 2") {
		t.Errorf("message = %q", msg)
	}
	result, err := a.LoadComparison(left, right)
	if err != nil {
		t.Fatal(err)
	}
	if s := findEntry(result, "f.txt").MergeStatus; s == nil || s.State != "check" || len(s.Issues) != 1 {
		t.Errorf("status = %+v, want check", s)
	}
}

func TestMergingLastGitFileKeepsItsRow(t *testing.T) {
	dir := conflictRepo(t)
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("merge", "--abort")
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("working copy\n"), 0644); err != nil {
		t.Fatal(err)
	}

	a := NewApp(nil)
	if _, err := a.LoadGitComparison("HEAD", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SaveMerge(selectAll(t, a, "f.txt", "right")); err != nil {
		t.Fatal(err)
	}

	result, err := a.LoadGitComparison("HEAD", "")
	if err != nil {
		t.Fatalf("reload after merging the last file: %v", err)
	}
	entry := findEntry(result, "f.txt")
	if entry == nil || !entry.Identical || entry.MergeStatus == nil || entry.MergeStatus.State != "safe" {
		t.Fatalf("entry = %+v, want identical and safe", entry)
	}

	// A later edit forgets the save, so once the file matches HEAD again its
	// row goes as usual.
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("again\n"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err = a.LoadGitComparison("HEAD", "")
	if err != nil {
		t.Fatal(err)
	}
	if entry := findEntry(result, "f.txt"); entry == nil || entry.MergeStatus != nil {
		t.Fatalf("entry after an edit = %+v, want no status", entry)
	}
	run("checkout", "--", "f.txt")
	if _, err := a.LoadGitComparison("HEAD", ""); err == nil {
		t.Error("expected no changes once the save is forgotten")
	}
}

func TestSaveConflictsStatusAndBackup(t *testing.T) {
	dir := conflictRepo(t)
	path := filepath.Join(dir, "f.txt")
	before, _ := os.ReadFile(path)
	a := NewApp(nil)
	if _, err := a.LoadGitComparison("HEAD", ""); err != nil {
		t.Fatal(err)
	}

	msg, err := a.SaveConflicts("f.txt", []string{"ours", ""}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "Check: 2 conflict marker line(s) remain") {
		t.Errorf("partial save message = %q", msg)
	}
	if orig, _ := os.ReadFile(path + ".orig"); string(orig) != string(before) {
		t.Errorf("backup = %q, want the previous version", orig)
	}
	result, err := a.LoadGitComparison("HEAD", "")
	if err != nil {
		t.Fatal(err)
	}
	if s := findEntry(result, "f.txt").MergeStatus; s == nil || s.State != "check" || s.Kind != "conflicts" {
		t.Errorf("partial save status = %+v, want check", s)
	}

	if _, err := a.SaveConflicts("f.txt", []string{"both"}, true); err != nil {
		t.Fatal(err)
	}
	result, err = a.LoadGitComparison("HEAD", "")
	if err != nil {
		t.Fatal(err)
	}
	if s := findEntry(result, "f.txt").MergeStatus; s == nil || s.State != "safe" {
		t.Errorf("full save status = %+v, want safe", s)
	}
}

func TestCheckTargetOnDisk(t *testing.T) {
	a, left := mergeDirs(t)
	path := filepath.Join(left, "f.txt")
	check := func(want string) {
		t.Helper()
		got, err := a.CheckTargetOnDisk("f.txt", "left")
		if err != nil || got != want {
			t.Errorf("CheckTargetOnDisk = %q, %v; want %q", got, err, want)
		}
	}

	if _, err := a.CheckTargetOnDisk("f.txt", "left"); err == nil {
		t.Error("a file that isn't open has nothing to compare with")
	}
	if _, err := a.GetDiff("f.txt"); err != nil {
		t.Fatal(err)
	}
	check("unchanged")

	if _, err := a.SaveMerge(selectAll(t, a, "f.txt", "left")); err != nil {
		t.Fatal(err)
	}
	if _, err := a.LoadComparison(a.leftPath, a.rightPath); err != nil {
		t.Fatal(err)
	}
	if _, err := a.GetDiff("f.txt"); err != nil {
		t.Fatal(err)
	}
	check("unchanged")

	data, _ := os.ReadFile(path)
	crlf := strings.ReplaceAll(string(data), "\n", "\r\n")
	if err := os.WriteFile(path, []byte(crlf), 0644); err != nil {
		t.Fatal(err)
	}
	check("changed")

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	check("missing")
}

// Merge selections are line numbers, so the diff a save recomputes must match
// the one on screen even if the settings changed in between.
func TestConfigEditBetweenViewAndSave(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	path := filepath.Join(left, "f.txt")
	if err := os.WriteFile(path, []byte("a\n  b\nold\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(right, "f.txt"), []byte("a\nb\nnew\n"), 0644); err != nil {
		t.Fatal(err)
	}
	a := NewApp(nil)
	if _, err := a.LoadComparison(left, right); err != nil {
		t.Fatal(err)
	}
	req := selectAll(t, a, "f.txt", "left")

	writeConfig(t, "[compare]\nignore_whitespace = \"all\"\n")
	if _, err := a.SaveMerge(req); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "a\nb\nnew\n" {
		t.Errorf("saved %q, want what was on screen applied", got)
	}
}

func TestConfigSettingsInTheApp(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	write := func(dir, name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(left, "crlf.txt", "a\r\nb\r\n")
	write(right, "crlf.txt", "a\nb\n")
	write(left, "same.txt", "s\n")
	write(right, "same.txt", "s\n")
	write(left, "real.txt", "x\n")
	write(right, "real.txt", "y\n")
	write(left, "skip.log", "x\n")

	a := NewApp(nil)
	result, err := a.LoadComparison(left, right)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 4 || result.HiddenIdentical != 0 {
		t.Errorf("default: %d files, %d hidden", len(result.Files), result.HiddenIdentical)
	}
	if e := findEntry(result, "crlf.txt"); e.Identical {
		t.Error("default: CRLF vs LF should differ")
	}
	if d, _ := a.GetDiff("crlf.txt"); d.Ignoring != "" {
		t.Errorf("default: Ignoring = %q", d.Ignoring)
	}

	writeConfig(t, "[compare]\nignore = [\"*.log\"]\nignore_line_endings = true\nhide_identical = true\n[view]\ndefault_view = \"sidebyside\"\ndefault_merge_target = \"right\"\n[gui]\ndiff_font_size = 16\n")
	result, err = a.LoadComparison(left, right)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 1 || findEntry(result, "real.txt") == nil || result.HiddenIdentical != 2 {
		t.Errorf("set: %+v, %d hidden", result.Files, result.HiddenIdentical)
	}
	if d, err := a.GetDiff("crlf.txt"); err != nil || d.Ignoring != "line endings" || d.Inserted+d.Deleted != 0 {
		t.Errorf("hidden file's diff: %+v, %v", d, err)
	}
	s := a.GetSettings()
	if s.DefaultView != "sidebyside" || s.DefaultMergeTarget != "right" || s.DiffFontSize != 16 || s.ConfigError != "" {
		t.Errorf("settings = %+v", s)
	}

	writeConfig(t, "[gui]\ndiff_font_size = 1\n")
	s = a.GetSettings()
	if s.ConfigError == "" || s.DiffFontSize != 12.5 {
		t.Errorf("bad value: %+v", s)
	}
}

func TestOnDemandLoading(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	write := func(dir, name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(left, "f.txt", "old\n")
	write(right, "f.txt", "new\n")
	write(left, "g.txt", "g1\n")
	write(right, "g.txt", "g2\n")

	a := NewApp(nil)
	if _, err := a.LoadComparison(left, right); err != nil {
		t.Fatal(err)
	}

	// Edited after loading, before opening: shows the new text.
	write(left, "f.txt", "edited\n")
	diff, err := a.GetDiff("f.txt")
	if err != nil || diff.Lines[0].Content != "edited" {
		t.Fatalf("diff = %+v, %v", diff, err)
	}

	// Edited while open: the disk check notices.
	write(left, "f.txt", "edited again\n")
	if got, _ := a.CheckTargetOnDisk("f.txt", "left"); got != "changed" {
		t.Errorf("disk check = %q", got)
	}

	// A save uses the diff on screen, whatever's on disk now.
	req := MergeSaveRequest{RelPath: "f.txt", Target: "left", SelectedInsertions: []int{1}, SelectedDeletions: []int{0}, Token: diff.Token}
	if _, err := a.SaveMerge(req); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(left, "f.txt")); string(got) != "new\n" {
		t.Errorf("saved %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(left, "f.txt.orig")); string(got) != "edited again\n" {
		t.Errorf("backup = %q", got)
	}

	// A save for a file that isn't open is refused, rather than guessing at lines.
	if _, err := a.LoadComparison(left, right); err != nil {
		t.Fatal(err)
	}
	if _, err := a.GetDiff("g.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SaveMerge(req); err == nil || !strings.Contains(err.Error(), "nothing was saved") {
		t.Errorf("save of a file that isn't open: %v", err)
	}

	// Going back to a file reads it afresh. A save from the earlier reading is
	// refused, because its line numbers may no longer match.
	first, err := a.GetDiff("f.txt")
	if err != nil {
		t.Fatal(err)
	}
	write(left, "f.txt", "different\nagain\n")
	if second, err := a.GetDiff("f.txt"); err != nil || second.Lines[0].Content != "different" {
		t.Fatalf("going back: %+v, %v", second, err)
	}
	stale := MergeSaveRequest{RelPath: "f.txt", Target: "left", SelectedInsertions: []int{1}, SelectedDeletions: []int{0}, Token: first.Token}
	if _, err := a.SaveMerge(stale); err == nil || !strings.Contains(err.Error(), "nothing was saved") {
		t.Errorf("save from an older reading: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(left, "f.txt")); string(got) != "different\nagain\n" {
		t.Errorf("the refused save changed the file: %q", got)
	}

	// Side-by-side rows come separately, for the open file only.
	if rows, err := a.GetSideBySide("g.txt"); err != nil || len(rows) != 1 || rows[0].LeftContent != "g1" || rows[0].RightContent != "g2" {
		t.Errorf("rows = %+v, %v", rows, err)
	}
	if _, err := a.GetSideBySide("never-opened.txt"); err == nil {
		t.Error("rows for a file that isn't open should be an error")
	}

	// Deleted after loading: says so.
	if _, err := a.LoadComparison(left, right); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(left, "g.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := a.GetDiff("g.txt"); err == nil || !strings.Contains(err.Error(), "may have changed") {
		t.Errorf("deleted file: %v", err)
	}
}

// The page checks the disk in the background, so a check for the previous file
// can arrive after another was opened. It mustn't take over as the open file, or
// saving the file on screen would be refused.
func TestLateDiskCheckDoesNotChangeTheOpenFile(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	for name, sides := range map[string][2]string{"a.txt": {"a1\n", "a2\n"}, "b.txt": {"b1\n", "b2\n"}} {
		if err := os.WriteFile(filepath.Join(left, name), []byte(sides[0]), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(right, name), []byte(sides[1]), 0644); err != nil {
			t.Fatal(err)
		}
	}
	a := NewApp(nil)
	if _, err := a.LoadComparison(left, right); err != nil {
		t.Fatal(err)
	}
	if _, err := a.GetDiff("a.txt"); err != nil {
		t.Fatal(err)
	}
	req := selectAll(t, a, "b.txt", "left") // b.txt is now open
	a.CheckTargetOnDisk("a.txt", "left")    // the late check for a.txt
	if _, err := a.SaveMerge(req); err != nil {
		t.Errorf("saving the file on screen was refused: %v", err)
	}
	if got, _ := os.ReadFile(filepath.Join(left, "b.txt")); string(got) != "b2\n" {
		t.Errorf("b.txt = %q", got)
	}
}
