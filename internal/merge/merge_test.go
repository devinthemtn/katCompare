package merge

import (
	"strings"
	"testing"

	"golang-fileCmp/internal/differ"
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
