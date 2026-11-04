package merge

import (
	"os"
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
