package differ

import (
	"strings"
	"testing"
)

// ---- helpers ----------------------------------------------------------------

func mustCompare(t *testing.T, d *Differ, left, right string) *FileDiff {
	t.Helper()
	diff := d.CompareStrings("left", "right", left, right)
	if diff == nil {
		t.Fatal("CompareStrings returned nil")
	}
	return diff
}

func lineTypes(diff *FileDiff) []DiffType {
	types := make([]DiffType, len(diff.Lines))
	for i, l := range diff.Lines {
		types[i] = l.Type
	}
	return types
}

// ---- identical files --------------------------------------------------------

func TestIdenticalFiles(t *testing.T) {
	d := New()
	diff := mustCompare(t, d, "hello\nworld\n", "hello\nworld\n")

	for _, line := range diff.Lines {
		if line.Type != DiffEqual {
			t.Errorf("expected all lines Equal, got %v for %q", line.Type, line.Content)
		}
	}

	eq, ins, del := diff.GetStats()
	if ins != 0 || del != 0 {
		t.Errorf("identical files: want 0 insertions 0 deletions, got ins=%d del=%d", ins, del)
	}
	if eq == 0 {
		t.Errorf("identical files: expected some equal lines, got 0")
	}
}

// ---- empty files ------------------------------------------------------------

func TestBothEmpty(t *testing.T) {
	d := New()
	diff := mustCompare(t, d, "", "")
	if len(diff.Lines) != 0 {
		t.Errorf("expected 0 lines for empty vs empty, got %d", len(diff.Lines))
	}
}

func TestLeftEmpty(t *testing.T) {
	d := New()
	diff := mustCompare(t, d, "", "line1\nline2\n")

	for _, line := range diff.Lines {
		if line.Type != DiffInsert {
			t.Errorf("left-empty: expected all Insert, got %v", line.Type)
		}
	}
}

func TestRightEmpty(t *testing.T) {
	d := New()
	diff := mustCompare(t, d, "line1\nline2\n", "")

	for _, line := range diff.Lines {
		if line.Type != DiffDelete {
			t.Errorf("right-empty: expected all Delete, got %v", line.Type)
		}
	}
}

// ---- line number tracking ---------------------------------------------------

func TestLineNumbers(t *testing.T) {
	d := New()
	// left:  a, b, c
	// right: a, X, c   (b replaced by X)
	diff := mustCompare(t, d, "a\nb\nc\n", "a\nX\nc\n")

	for _, line := range diff.Lines {
		switch line.Type {
		case DiffEqual:
			if line.LeftLineNum <= 0 {
				t.Errorf("equal line %q: LeftLineNum should be >0, got %d", line.Content, line.LeftLineNum)
			}
			if line.RightLineNum <= 0 {
				t.Errorf("equal line %q: RightLineNum should be >0, got %d", line.Content, line.RightLineNum)
			}
		case DiffDelete:
			if line.LeftLineNum <= 0 {
				t.Errorf("delete line %q: LeftLineNum should be >0, got %d", line.Content, line.LeftLineNum)
			}
			if line.RightLineNum != -1 {
				t.Errorf("delete line %q: RightLineNum should be -1, got %d", line.Content, line.RightLineNum)
			}
		case DiffInsert:
			if line.LeftLineNum != -1 {
				t.Errorf("insert line %q: LeftLineNum should be -1, got %d", line.Content, line.LeftLineNum)
			}
			if line.RightLineNum <= 0 {
				t.Errorf("insert line %q: RightLineNum should be >0, got %d", line.Content, line.RightLineNum)
			}
		}

		// LineNum (unified compat field) must match the primary side
		switch line.Type {
		case DiffEqual, DiffDelete:
			if line.LineNum != line.LeftLineNum {
				t.Errorf("equal/delete: LineNum (%d) != LeftLineNum (%d)", line.LineNum, line.LeftLineNum)
			}
		case DiffInsert:
			if line.LineNum != line.RightLineNum {
				t.Errorf("insert: LineNum (%d) != RightLineNum (%d)", line.LineNum, line.RightLineNum)
			}
		}
	}
}

func TestLineNumbersMonotonicallyIncrease(t *testing.T) {
	d := New()
	left := "a\nb\nc\nd\ne\n"
	right := "a\nX\nc\nY\ne\n"
	diff := mustCompare(t, d, left, right)

	prevLeft, prevRight := 0, 0
	for _, line := range diff.Lines {
		if line.LeftLineNum > 0 {
			if line.LeftLineNum <= prevLeft {
				t.Errorf("LeftLineNum went backwards: prev=%d cur=%d (content=%q)",
					prevLeft, line.LeftLineNum, line.Content)
			}
			prevLeft = line.LeftLineNum
		}
		if line.RightLineNum > 0 {
			if line.RightLineNum <= prevRight {
				t.Errorf("RightLineNum went backwards: prev=%d cur=%d (content=%q)",
					prevRight, line.RightLineNum, line.Content)
			}
			prevRight = line.RightLineNum
		}
	}
}

// ---- GetStats ---------------------------------------------------------------

func TestGetStats(t *testing.T) {
	d := New()
	// 2 equal, 1 deleted, 1 inserted
	diff := mustCompare(t, d, "keep1\nremoved\nkeep2\n", "keep1\nadded\nkeep2\n")

	eq, ins, del := diff.GetStats()
	if ins == 0 {
		t.Error("expected at least 1 insertion")
	}
	if del == 0 {
		t.Error("expected at least 1 deletion")
	}
	if eq == 0 {
		t.Error("expected at least 1 equal line")
	}
	total := eq + ins + del
	if total != len(diff.Lines) {
		t.Errorf("stats sum %d != len(Lines) %d", total, len(diff.Lines))
	}
}

// ---- BuildSideBySideRows ----------------------------------------------------

func TestSBSAllEqual(t *testing.T) {
	d := New()
	diff := mustCompare(t, d, "a\nb\nc\n", "a\nb\nc\n")
	rows := BuildSideBySideRows(diff.Lines)

	for _, row := range rows {
		if row.Type != SBSEqual {
			t.Errorf("all-equal diff: expected SBSEqual row, got %v (L=%q R=%q)",
				row.Type, row.LeftContent, row.RightContent)
		}
		if row.LeftContent != row.RightContent {
			t.Errorf("equal row has different content: L=%q R=%q", row.LeftContent, row.RightContent)
		}
	}
}

func TestSBSModifiedPairing(t *testing.T) {
	// A single-line replacement should yield exactly one SBSModified row,
	// not two separate Delete+Insert rows.
	d := New()
	diff := mustCompare(t, d, "old\n", "new\n")
	rows := BuildSideBySideRows(diff.Lines)

	modCount := 0
	for _, row := range rows {
		if row.Type == SBSModified {
			modCount++
			if row.LeftContent == "" {
				t.Error("Modified row has empty LeftContent")
			}
			if row.RightContent == "" {
				t.Error("Modified row has empty RightContent")
			}
		}
		// Should never get a bare Insert immediately after a Delete
		if row.Type == SBSInsert || row.Type == SBSDelete {
			// These are fine if they appear alone, but we shouldn't get paired ones
		}
	}
	if modCount == 0 {
		t.Error("expected at least one SBSModified row for a single-line replacement")
	}
}

func TestSBSDeleteOnly(t *testing.T) {
	d := New()
	diff := mustCompare(t, d, "a\nb\nc\n", "a\nc\n") // b removed
	rows := BuildSideBySideRows(diff.Lines)

	deleteCount := 0
	for _, row := range rows {
		if row.Type == SBSDelete {
			deleteCount++
			if row.LeftContent == "" {
				t.Error("Delete row has empty LeftContent")
			}
			if row.RightContent != "" {
				t.Errorf("Delete row should have empty RightContent, got %q", row.RightContent)
			}
			if row.RightLineNum != -1 {
				t.Errorf("Delete row RightLineNum should be -1, got %d", row.RightLineNum)
			}
		}
	}
	if deleteCount == 0 {
		t.Error("expected at least one SBSDelete row")
	}
}

func TestSBSInsertOnly(t *testing.T) {
	d := New()
	diff := mustCompare(t, d, "a\nc\n", "a\nb\nc\n") // b added
	rows := BuildSideBySideRows(diff.Lines)

	insertCount := 0
	for _, row := range rows {
		if row.Type == SBSInsert {
			insertCount++
			if row.RightContent == "" {
				t.Error("Insert row has empty RightContent")
			}
			if row.LeftContent != "" {
				t.Errorf("Insert row should have empty LeftContent, got %q", row.LeftContent)
			}
			if row.LeftLineNum != -1 {
				t.Errorf("Insert row LeftLineNum should be -1, got %d", row.LeftLineNum)
			}
		}
	}
	if insertCount == 0 {
		t.Error("expected at least one SBSInsert row")
	}
}

func TestSBSRowCountNeverExceedsDiffLines(t *testing.T) {
	// Pairing collapses Delete+Insert into one row, so SBS rows <= diff lines
	d := New()
	cases := []struct{ left, right string }{
		{"a\nb\nc\n", "a\nX\nc\n"},
		{"line1\nline2\nline3\n", "line1\nline3\n"},
		{"", "new\n"},
		{"old\n", ""},
		{"a\nb\nc\nd\n", "a\nB\nC\nd\n"},
	}
	for _, tc := range cases {
		diff := mustCompare(t, d, tc.left, tc.right)
		rows := BuildSideBySideRows(diff.Lines)
		if len(rows) > len(diff.Lines) {
			t.Errorf("SBS rows (%d) > diff lines (%d) for left=%q right=%q",
				len(rows), len(diff.Lines), tc.left, tc.right)
		}
	}
}

func TestSBSLineNumbersPresent(t *testing.T) {
	d := New()
	diff := mustCompare(t, d, "a\nb\nc\n", "a\nX\nc\n")
	rows := BuildSideBySideRows(diff.Lines)

	for _, row := range rows {
		switch row.Type {
		case SBSEqual:
			if row.LeftLineNum <= 0 || row.RightLineNum <= 0 {
				t.Errorf("Equal row should have both line nums >0, got L=%d R=%d", row.LeftLineNum, row.RightLineNum)
			}
		case SBSDelete:
			if row.LeftLineNum <= 0 {
				t.Errorf("Delete row LeftLineNum should be >0, got %d", row.LeftLineNum)
			}
			if row.RightLineNum != -1 {
				t.Errorf("Delete row RightLineNum should be -1, got %d", row.RightLineNum)
			}
		case SBSInsert:
			if row.LeftLineNum != -1 {
				t.Errorf("Insert row LeftLineNum should be -1, got %d", row.LeftLineNum)
			}
			if row.RightLineNum <= 0 {
				t.Errorf("Insert row RightLineNum should be >0, got %d", row.RightLineNum)
			}
		case SBSModified:
			if row.LeftLineNum <= 0 {
				t.Errorf("Modified row LeftLineNum should be >0, got %d", row.LeftLineNum)
			}
			if row.RightLineNum <= 0 {
				t.Errorf("Modified row RightLineNum should be >0, got %d", row.RightLineNum)
			}
		}
	}
}

// ---- CompareFiles (io.Reader path) ------------------------------------------

func TestCompareFiles(t *testing.T) {
	d := New()
	left := strings.NewReader("hello\nworld\n")
	right := strings.NewReader("hello\nearth\n")
	diff, err := d.CompareFiles("l", "r", left, right)
	if err != nil {
		t.Fatalf("CompareFiles error: %v", err)
	}
	_, ins, del := diff.GetStats()
	if ins == 0 || del == 0 {
		t.Errorf("expected insertions and deletions, got ins=%d del=%d", ins, del)
	}
}
