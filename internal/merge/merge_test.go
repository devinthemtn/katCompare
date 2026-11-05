package merge

import (
	"math/rand"
	"os"
	"reflect"
	"strings"
	"testing"

	"katcompare/internal/differ"
)

// ---- helpers ----------------------------------------------------------------

func makeDiff(left, right string) *differ.FileDiff {
	d := differ.New()
	return d.CompareStrings("left", "right", left, right)
}

// ---- NewChangeSelection -----------------------------------------------------

func TestNewChangeSelectionSelectsAll(t *testing.T) {
	diff := makeDiff("a\nb\nc\n", "a\nX\nc\n")
	sel := NewChangeSelection(diff)

	for i, line := range diff.Lines {
		switch line.Type {
		case differ.DiffInsert:
			if !sel.IsInsertionSelected(i) {
				t.Errorf("line %d (insert) should be selected by default", i)
			}
		case differ.DiffDelete:
			if !sel.IsDeletionSelected(i) {
				t.Errorf("line %d (delete) should be selected by default", i)
			}
		}
	}
}

// ---- SelectAll / SelectNone -------------------------------------------------

func TestSelectNoneThenAll(t *testing.T) {
	diff := makeDiff("a\nb\nc\n", "a\nX\nc\n")
	sel := NewChangeSelection(diff)

	sel.SelectNone(diff)
	for i, line := range diff.Lines {
		switch line.Type {
		case differ.DiffInsert:
			if sel.IsInsertionSelected(i) {
				t.Errorf("after SelectNone, line %d (insert) should not be selected", i)
			}
		case differ.DiffDelete:
			if sel.IsDeletionSelected(i) {
				t.Errorf("after SelectNone, line %d (delete) should not be selected", i)
			}
		}
	}

	sel.SelectAll(diff)
	for i, line := range diff.Lines {
		switch line.Type {
		case differ.DiffInsert:
			if !sel.IsInsertionSelected(i) {
				t.Errorf("after SelectAll, line %d (insert) should be selected", i)
			}
		case differ.DiffDelete:
			if !sel.IsDeletionSelected(i) {
				t.Errorf("after SelectAll, line %d (delete) should be selected", i)
			}
		}
	}
}

// ---- Toggle -----------------------------------------------------------------

func TestToggleInsertion(t *testing.T) {
	diff := makeDiff("a\n", "b\n")
	sel := NewChangeSelection(diff)

	for i, line := range diff.Lines {
		if line.Type == differ.DiffInsert {
			before := sel.IsInsertionSelected(i)
			sel.ToggleInsertion(i)
			after := sel.IsInsertionSelected(i)
			if before == after {
				t.Errorf("ToggleInsertion did not change selection at index %d", i)
			}
			// Toggle back
			sel.ToggleInsertion(i)
			if sel.IsInsertionSelected(i) != before {
				t.Errorf("double toggle did not restore original state at index %d", i)
			}
		}
	}
}

// ---- ApplyToLeft ------------------------------------------------------------

func TestApplyToLeftAllSelected(t *testing.T) {
	// left: "a\nb\nc\n"  right: "a\nX\nc\n"
	// Applying all changes to left means left becomes identical to right.
	m := New()
	diff := makeDiff("a\nb\nc\n", "a\nX\nc\n")
	sel := NewChangeSelection(diff) // all selected
	result := m.ApplyToLeft(diff, sel)

	lines := strings.Split(strings.TrimSpace(result.Content), "\n")
	expected := []string{"a", "X", "c"}
	if len(lines) != len(expected) {
		t.Fatalf("ApplyToLeft all-selected: expected %v, got %v", expected, lines)
	}
	for i, want := range expected {
		if lines[i] != want {
			t.Errorf("line %d: want %q got %q", i, want, lines[i])
		}
	}
}

func TestApplyToLeftNoneSelected(t *testing.T) {
	// Applying no changes to left means left stays unchanged.
	m := New()
	diff := makeDiff("a\nb\nc\n", "a\nX\nc\n")
	sel := NewChangeSelection(diff)
	sel.SelectNone(diff)
	result := m.ApplyToLeft(diff, sel)

	lines := strings.Split(strings.TrimSpace(result.Content), "\n")
	expected := []string{"a", "b", "c"}
	if len(lines) != len(expected) {
		t.Fatalf("ApplyToLeft none-selected: expected %v, got %v", expected, lines)
	}
	for i, want := range expected {
		if lines[i] != want {
			t.Errorf("line %d: want %q got %q", i, want, lines[i])
		}
	}
}

// ---- ApplyToRight -----------------------------------------------------------

func TestApplyToRightAllSelected(t *testing.T) {
	// Applying all changes to right means right becomes identical to left.
	m := New()
	diff := makeDiff("a\nb\nc\n", "a\nX\nc\n")
	sel := NewChangeSelection(diff)
	result := m.ApplyToRight(diff, sel)

	lines := strings.Split(strings.TrimSpace(result.Content), "\n")
	expected := []string{"a", "b", "c"}
	if len(lines) != len(expected) {
		t.Fatalf("ApplyToRight all-selected: expected %v, got %v", expected, lines)
	}
	for i, want := range expected {
		if lines[i] != want {
			t.Errorf("line %d: want %q got %q", i, want, lines[i])
		}
	}
}

func TestApplyToRightNoneSelected(t *testing.T) {
	// Applying no changes to right means right stays unchanged.
	m := New()
	diff := makeDiff("a\nb\nc\n", "a\nX\nc\n")
	sel := NewChangeSelection(diff)
	sel.SelectNone(diff)
	result := m.ApplyToRight(diff, sel)

	lines := strings.Split(strings.TrimSpace(result.Content), "\n")
	expected := []string{"a", "X", "c"}
	if len(lines) != len(expected) {
		t.Fatalf("ApplyToRight none-selected: expected %v, got %v", expected, lines)
	}
	for i, want := range expected {
		if lines[i] != want {
			t.Errorf("line %d: want %q got %q", i, want, lines[i])
		}
	}
}

// ---- Equal-line passthrough -------------------------------------------------

func TestEqualLinesAlwaysIncluded(t *testing.T) {
	m := New()
	// Only the middle line changes; first and last must survive in all cases.
	diff := makeDiff("first\nchanged\nlast\n", "first\nNEW\nlast\n")
	sel := NewChangeSelection(diff)

	for _, result := range []*MergeResult{
		m.ApplyToLeft(diff, sel),
		m.ApplyToRight(diff, sel),
	} {
		if !strings.Contains(result.Content, "first") {
			t.Error("equal line 'first' missing from result")
		}
		if !strings.Contains(result.Content, "last") {
			t.Error("equal line 'last' missing from result")
		}
	}
}

// ---- GetSelectedStats -------------------------------------------------------

func TestGetSelectedStats(t *testing.T) {
	diff := makeDiff("a\nb\nc\n", "a\nX\nc\n")
	sel := NewChangeSelection(diff)

	selIns, totIns, selDel, totDel := sel.GetSelectedStats(diff)

	if totIns == 0 && totDel == 0 {
		t.Skip("diff produced no changes — test files may be identical")
	}
	if selIns > totIns {
		t.Errorf("selected insertions (%d) > total insertions (%d)", selIns, totIns)
	}
	if selDel > totDel {
		t.Errorf("selected deletions (%d) > total deletions (%d)", selDel, totDel)
	}

	// After SelectNone everything should be 0 selected
	sel.SelectNone(diff)
	selIns, _, selDel, _ = sel.GetSelectedStats(diff)
	if selIns != 0 || selDel != 0 {
		t.Errorf("after SelectNone: selIns=%d selDel=%d, both want 0", selIns, selDel)
	}
}

// ---- Symmetry property ------------------------------------------------------

func TestMergeSymmetry(t *testing.T) {
	// ApplyToLeft(allSelected) should equal ApplyToRight(noneSelected)
	// because both produce the right-file content.
	m := New()
	diff := makeDiff("alpha\nbeta\ngamma\n", "alpha\nBETA\ngamma\n")
	selAll := NewChangeSelection(diff)
	selNone := NewChangeSelection(diff)
	selNone.SelectNone(diff)

	leftResult := m.ApplyToLeft(diff, selAll)
	rightResult := m.ApplyToRight(diff, selNone)

	if leftResult.Content != rightResult.Content {
		t.Errorf("symmetry fail:\nApplyToLeft(all)=%q\nApplyToRight(none)=%q",
			leftResult.Content, rightResult.Content)
	}
}

// ---- ToggleBlock / SaveWithBackup -------------------------------------------

func TestToggleBlock(t *testing.T) {
	// One block (delete b, insert X, Y) between equal lines.
	diff := makeDiff("a\nb\nc\n", "a\nX\nY\nc\n")
	sel := NewChangeSelection(diff)
	var changed []int
	for i, l := range diff.Lines {
		if l.Type != differ.DiffEqual {
			changed = append(changed, i)
		}
	}
	isOn := func(i int) bool { return sel.IsInsertionSelected(i) || sel.IsDeletionSelected(i) }

	sel.ToggleBlock(diff, changed[1]) // all on -> all off
	for _, i := range changed {
		if isOn(i) {
			t.Fatalf("line %d still selected after toggling a fully selected block", i)
		}
	}
	sel.ToggleInsertion(changed[len(changed)-1])
	sel.ToggleBlock(diff, changed[0]) // partly on -> all on
	for _, i := range changed {
		if !isOn(i) {
			t.Fatalf("line %d not selected after toggling a partly selected block", i)
		}
	}
	sel.ToggleBlock(diff, 0) // equal line: no-op
	if !isOn(changed[0]) {
		t.Fatal("toggling an equal line changed the selection")
	}
}

func TestSaveWithBackup(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/f.txt"
	if err := os.WriteFile(path, []byte("old\n"), 0640); err != nil {
		t.Fatal(err)
	}
	backup, err := SaveWithBackup(path, "new")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "new\n" {
		t.Errorf("saved %q, want %q (trailing newline kept)", got, "new\n")
	}
	orig, _ := os.ReadFile(backup)
	if backup != path+".orig" || string(orig) != "old\n" {
		t.Errorf("backup %s = %q", backup, orig)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0640 {
		t.Errorf("mode = %v, want 0640", info.Mode().Perm())
	}
	if _, err := SaveWithBackup(dir+"/missing", "x"); err == nil {
		t.Error("expected error for a missing file")
	}
}

func TestSaveVerified(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/f.txt"
	if err := os.WriteFile(path, []byte("old\r\n"), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := SaveVerified(path, "new")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Issues) != 0 {
		t.Errorf("clean save: issues = %v", report.Issues)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "new\r\n" {
		t.Errorf("saved %q, want the old newline style kept", got)
	}
	if report.Hash != HashContent(got) {
		t.Error("hash doesn't match the saved file")
	}
	if orig, _ := os.ReadFile(report.Backup); string(orig) != "old\r\n" {
		t.Errorf("backup = %q", orig)
	}

	report, err = SaveVerified(path, "a\n<<<<<<< HEAD\nb\n=======\nc\n>>>>>>> x\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Issues) != 1 || !strings.Contains(report.Issues[0], "2 conflict marker line(s) remain, first at line 2") {
		t.Errorf("leftover markers: issues = %v", report.Issues)
	}

	if _, err := SaveVerified(dir+"/missing", "x"); err == nil {
		t.Error("expected error for a missing file")
	}
}

// Lines reach the save with any \r still on them, so a CRLF file's last line
// must not gain a second \r. Changed lines keep the ending they came with; a
// last line without one takes the target's, and a target with no trailing
// newline stays that way.
func TestSaveKeepsLineEndings(t *testing.T) {
	cases := []struct {
		name, target, other, want string
	}{
		{"LF", "alpha\nbeta\n", "alpha\nBETA\n", "alpha\nBETA\n"},
		{"CRLF, last line differs", "alpha\r\nbeta\r\n", "alpha\r\nBETA\r\n", "alpha\r\nBETA\r\n"},
		{"CRLF, middle line differs", "a\r\nb\r\nc\r\n", "a\r\nB\r\nc\r\n", "a\r\nB\r\nc\r\n"},
		{"CRLF, blank last line", "a\r\n\r\n", "A\r\n\r\n", "A\r\n\r\n"},
		{"CRLF, no trailing newline", "a\r\nb", "A\r\nb", "A\r\nb"},
		{"last line taken from CRLF into LF", "a\nb\n", "a\nB\r\n", "a\nB\r\n"},
		{"last line taken from LF into CRLF", "a\r\nb\r\n", "a\r\nB\n", "a\r\nB\r\n"},
		{"last line taken from CRLF, target has no newline", "a\r\nb", "a\r\nB\r\n", "a\r\nB"},
		{"target ends in a bare CR", "a\nb\r", "A\nb\r", "A\nb\r"},
	}
	for _, c := range cases {
		for _, side := range []string{"left", "right"} {
			t.Run(c.name+"/"+side, func(t *testing.T) {
				path := t.TempDir() + "/f.txt"
				if err := os.WriteFile(path, []byte(c.target), 0644); err != nil {
					t.Fatal(err)
				}

				var result *MergeResult
				if side == "left" {
					diff := differ.New().CompareStrings("l", "r", c.target, c.other)
					result = New().ApplyToLeft(diff, NewChangeSelection(diff))
				} else {
					diff := differ.New().CompareStrings("l", "r", c.other, c.target)
					result = New().ApplyToRight(diff, NewChangeSelection(diff))
				}

				report, err := SaveMerged(path, result)
				if err != nil {
					t.Fatal(err)
				}
				got, _ := os.ReadFile(path)
				if string(got) != c.want {
					t.Errorf("saved %q, want %q", got, c.want)
				}
				if len(report.Issues) != 0 {
					t.Errorf("issues = %v", report.Issues)
				}
				if orig, _ := os.ReadFile(report.Backup); string(orig) != c.target {
					t.Errorf("backup = %q, want %q", orig, c.target)
				}
			})
		}
	}
}

func TestTracker(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/f.txt"
	if err := os.WriteFile(path, []byte("old\n"), 0644); err != nil {
		t.Fatal(err)
	}
	tracker := NewTracker()
	if tracker.StatusOf(path) != nil {
		t.Error("status before any save")
	}

	report, err := SaveVerified(path, "new")
	if err != nil {
		t.Fatal(err)
	}
	tracker.Record(path, &Record{Report: report, Kind: KindMerge, Applied: 1})

	rec := tracker.StatusOf("", path)
	if rec == nil || rec.State() != StateSafe || rec.Target != "f.txt" {
		t.Fatalf("status after save = %+v", rec)
	}
	backups := tracker.Backups()
	if !AllBackups([]string{report.Backup}, backups) {
		t.Errorf("backups = %v", backups)
	}
	if AllBackups([]string{report.Backup, ""}, backups) || AllBackups(nil, backups) {
		t.Error("a side that isn't a file, or no sides, shouldn't count as a backup")
	}
	if got := tracker.SavedUnder(dir); len(got) != 1 || got[0] != "f.txt" {
		t.Errorf("SavedUnder = %v", got)
	}
	if got := tracker.SavedUnder(t.TempDir()); len(got) != 0 {
		t.Errorf("SavedUnder another dir = %v", got)
	}

	// A check beats a safe save when both sides have one.
	other := dir + "/g.txt"
	if err := os.WriteFile(other, []byte("old\n"), 0644); err != nil {
		t.Fatal(err)
	}
	report, err = SaveVerified(other, "<<<<<<< HEAD\n")
	if err != nil {
		t.Fatal(err)
	}
	tracker.Record(other, &Record{Report: report, Kind: KindConflicts})
	if rec := tracker.StatusOf(path, other); rec == nil || rec.State() != StateCheck {
		t.Errorf("status of both = %+v, want check", rec)
	}

	// An outside edit forgets the save.
	if err := os.WriteFile(path, []byte("edited\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if tracker.StatusOf(path) != nil {
		t.Error("status should clear when the file changes")
	}
	if tracker.Backups()[path+".orig"] {
		t.Error("backup should stop being hidden once its save is forgotten")
	}
}

func TestDiskState(t *testing.T) {
	path := t.TempDir() + "/f.txt"
	if err := os.WriteFile(path, []byte("a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ loaded, want string }{{"a\n", DiskUnchanged}, {"b\n", DiskChanged}} {
		if got, err := DiskState(path, c.loaded); err != nil || got != c.want {
			t.Errorf("DiskState(%q) = %q, %v; want %q", c.loaded, got, err, c.want)
		}
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if got, err := DiskState(path, "a\n"); err != nil || got != DiskMissing {
		t.Errorf("DiskState after removal = %q, %v", got, err)
	}
}

// mergeWith compares target and other under opts, applies every change to the
// target, saves it, and returns the bytes on disk.
func mergeWith(t *testing.T, opts differ.Options, side, target, other string) string {
	t.Helper()
	path := t.TempDir() + "/f.txt"
	if err := os.WriteFile(path, []byte(target), 0644); err != nil {
		t.Fatal(err)
	}
	d := differ.NewWithOptions(opts)
	var result *MergeResult
	if side == "left" {
		diff := d.CompareStrings("l", "r", target, other)
		result = New().ApplyToLeft(diff, NewChangeSelection(diff))
	} else {
		diff := d.CompareStrings("l", "r", other, target)
		result = New().ApplyToRight(diff, NewChangeSelection(diff))
	}
	if _, err := SaveMerged(path, result); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	return string(got)
}

// Ignoring whitespace must not rewrite the target's unchanged lines with the
// other side's whitespace: that would change the file behind the user's back.
func TestMergeKeepsTargetsBytesWhenIgnoring(t *testing.T) {
	all := differ.Options{IgnoreWhitespace: differ.WhitespaceAll}
	target := "    keep  my   spacing\nold line\n\ttabs here \n"
	other := "keep my spacing\nnew line\ntabs here\n"
	want := "    keep  my   spacing\nnew line\n\ttabs here \n"

	for _, side := range []string{"left", "right"} {
		if got := mergeWith(t, all, side, target, other); got != want {
			t.Errorf("%s:\n got %q\nwant %q", side, got, want)
		}
	}
}

func TestMergeKeepsTargetsLineEndingsWhenIgnoring(t *testing.T) {
	endings := differ.Options{IgnoreLineEndings: true}
	cases := []struct {
		name, target, other, want string
	}{
		{"LF lines into a CRLF file", "a\r\nold\r\nb\r\n", "a\nnew\nb\n", "a\r\nnew\r\nb\r\n"},
		{"CRLF lines into an LF file", "a\nold\nb\n", "a\r\nnew\r\nb\r\n", "a\nnew\nb\n"},
		{"new last line", "a\r\nb\r\n", "a\nb\nc\n", "a\r\nb\r\nc\r\n"},
		{"only endings differ: nothing to change", "a\r\nb\r\n", "a\nb\n", "a\r\nb\r\n"},
		{"mixed target: keeps its lines, new line takes the most common", "a\r\nb\r\nc\nold\r\n", "a\nb\nc\nnew\n", "a\r\nb\r\nc\nnew\r\n"},
	}
	for _, c := range cases {
		for _, side := range []string{"left", "right"} {
			if got := mergeWith(t, endings, side, c.target, c.other); got != c.want {
				t.Errorf("%s/%s:\n got %q\nwant %q", c.name, side, got, c.want)
			}
		}
	}
}

// With nothing ignored, a merge behaves exactly as it did before options.
func TestMergeWithoutOptionsIsUnchanged(t *testing.T) {
	cases := []struct {
		name, target, other, want string
	}{
		{"LF", "a\nold\nb\n", "a\nnew\nb\n", "a\nnew\nb\n"},
		{"CRLF", "a\r\nold\r\nb\r\n", "a\r\nnew\r\nb\r\n", "a\r\nnew\r\nb\r\n"},
		{"a CRLF line into an LF file stays CRLF", "a\nold\n", "a\nnew\r\n", "a\nnew\r\n"},
		{"an LF line into a CRLF file keeps the target's end", "a\r\nold\r\n", "a\r\nnew\n", "a\r\nnew\r\n"},
	}
	for _, c := range cases {
		for _, side := range []string{"left", "right"} {
			if got := mergeWith(t, differ.Options{}, side, c.target, c.other); got != c.want {
				t.Errorf("%s/%s:\n got %q\nwant %q", c.name, side, got, c.want)
			}
		}
	}
}

// Whatever the files and picks, a merge must equal what's worked out here from
// the diff lines alone: the target's own text for lines it keeps, the other
// side's for lines it takes.
func TestRandomMergesMatchAnIndependentAnswer(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	vocabulary := []string{"a", "b", "c", "  a", "a  ", "\tb", "x y", "xy", "", "é", "日本語", "a\r", "b\r", "\r"}
	line := func() string { return vocabulary[r.Intn(len(vocabulary))] }
	build := func() string {
		var b strings.Builder
		for i, n := 0, r.Intn(25); i < n; i++ {
			b.WriteString(line() + "\n")
		}
		return b.String()
	}
	optionSets := []differ.Options{
		{},
		{IgnoreWhitespace: differ.WhitespaceTrailing},
		{IgnoreWhitespace: differ.WhitespaceAll},
		{IgnoreLineEndings: true},
		{IgnoreLineEndings: true, IgnoreWhitespace: differ.WhitespaceAll},
	}
	for trial := 0; trial < 4000; trial++ {
		left, right := build(), build()
		opts := optionSets[r.Intn(len(optionSets))]
		diff := differ.NewWithOptions(opts).CompareStrings("l", "r", left, right)

		sel := &ChangeSelection{ApplyInsertions: map[int]bool{}, ApplyDeletions: map[int]bool{}}
		for i, l := range diff.Lines {
			if r.Intn(2) == 0 {
				continue
			}
			if l.Type == differ.DiffInsert {
				sel.ApplyInsertions[i] = true
			} else if l.Type == differ.DiffDelete {
				sel.ApplyDeletions[i] = true
			}
		}

		var wantLeft, wantRight []string
		for i, l := range diff.Lines {
			switch l.Type {
			case differ.DiffEqual:
				wantLeft = append(wantLeft, l.Content)
				wantRight = append(wantRight, l.RightContent)
			case differ.DiffDelete:
				if !sel.ApplyDeletions[i] {
					wantLeft = append(wantLeft, l.Content)
				} else {
					wantRight = append(wantRight, l.Content)
				}
			case differ.DiffInsert:
				if sel.ApplyInsertions[i] {
					wantLeft = append(wantLeft, l.Content)
				} else {
					wantRight = append(wantRight, l.Content)
				}
			}
		}
		strip := func(lines []string) []string {
			out := make([]string, len(lines))
			for i, l := range lines {
				out[i] = l
				if opts.IgnoreLineEndings {
					out[i] = strings.TrimSuffix(l, "\r")
				}
			}
			return out
		}
		gotL := New().ApplyToLeft(diff, sel).Content
		gotR := New().ApplyToRight(diff, sel).Content
		if a, b := strings.Split(gotL, "\n"), wantLeft; len(b) > 0 && !reflect.DeepEqual(strip(a), strip(b)) {
			t.Fatalf("trial %d %+v left:\n got %q\nwant %q\nfrom %q / %q", trial, opts, a, b, left, right)
		}
		if a, b := strings.Split(gotR, "\n"), wantRight; len(b) > 0 && !reflect.DeepEqual(strip(a), strip(b)) {
			t.Fatalf("trial %d %+v right:\n got %q\nwant %q\nfrom %q / %q", trial, opts, a, b, left, right)
		}

		// With everything picked and nothing ignored, the target becomes the other side.
		if opts == (differ.Options{}) {
			all := NewChangeSelection(diff)
			if got := New().ApplyToLeft(diff, all).Content; right != "" && got+"\n" != right && got != right {
				t.Fatalf("trial %d: applying all to left gave %q, want %q", trial, got, right)
			}
			if got := New().ApplyToRight(diff, all).Content; left != "" && got+"\n" != left && got != left {
				t.Fatalf("trial %d: applying all to right gave %q, want %q", trial, got, left)
			}
			none := &ChangeSelection{ApplyInsertions: map[int]bool{}, ApplyDeletions: map[int]bool{}}
			if got := New().ApplyToLeft(diff, none).Content; left != "" && got+"\n" != left && got != left {
				t.Fatalf("trial %d: applying none to left gave %q, want %q", trial, got, left)
			}
		}
	}
}

// The end of a file is easy to get wrong: a merge result has lost its final
// newline, but can still end in blank lines.
func TestMergeKeepsTheEndOfTheFile(t *testing.T) {
	cases := []struct {
		name, target, other, want string
	}{
		{"blank lines at the end", "\n\n\n", "\n\n", "\n\n"},
		{"gaining a blank line", "a\n", "a\n\n", "a\n\n"},
		{"losing a blank line", "a\n\n", "a\n", "a\n"},
		{"keeping blank lines through a change", "old\n\n\n", "new\n\n\n", "new\n\n\n"},
		{"CRLF blank lines", "old\r\n\r\n", "new\r\n\r\n", "new\r\n\r\n"},
		{"no final newline stays that way", "a\nb", "a\nc", "a\nc"},
		{"everything removed", "a\nb\n", "", ""},
		{"filling an empty file", "", "a\nb\n", "a\nb\n"},
		{"just one blank line", "a\n", "\n", "\n"},
	}
	for _, c := range cases {
		for _, side := range []string{"left", "right"} {
			if got := mergeWith(t, differ.Options{}, side, c.target, c.other); got != c.want {
				t.Errorf("%s/%s: got %q, want %q", c.name, side, got, c.want)
			}
		}
	}
}

// Taking every change makes the target the other side; taking none leaves it.
func TestRandomFullMergesSaveExactBytes(t *testing.T) {
	r := rand.New(rand.NewSource(11))
	vocabulary := []string{"a", "b", "  c", "", "", "é", "d e"}
	build := func() string {
		var b strings.Builder
		for i, n := 0, 1+r.Intn(12); i < n; i++ {
			b.WriteString(vocabulary[r.Intn(len(vocabulary))] + "\n")
		}
		return b.String()
	}
	for trial := 0; trial < 3000; trial++ {
		target, other := build(), build()
		for _, side := range []string{"left", "right"} {
			if got := mergeWith(t, differ.Options{}, side, target, other); got != other {
				t.Fatalf("trial %d %s: all picks gave %q, want %q (target %q)", trial, side, got, other, target)
			}
			path := t.TempDir() + "/f.txt"
			if err := os.WriteFile(path, []byte(target), 0644); err != nil {
				t.Fatal(err)
			}
			none := &ChangeSelection{ApplyInsertions: map[int]bool{}, ApplyDeletions: map[int]bool{}}
			var result *MergeResult
			if side == "left" {
				result = New().ApplyToLeft(differ.New().CompareStrings("l", "r", target, other), none)
			} else {
				result = New().ApplyToRight(differ.New().CompareStrings("l", "r", other, target), none)
			}
			if _, err := SaveMerged(path, result); err != nil {
				t.Fatal(err)
			}
			if got, _ := os.ReadFile(path); string(got) != target {
				t.Fatalf("trial %d %s: no picks gave %q, want %q", trial, side, got, target)
			}
		}
	}
}
