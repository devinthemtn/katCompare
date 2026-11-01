package ui

import (
	"testing"

	"golang-fileCmp/internal/differ"
)

func TestJumpToChange(t *testing.T) {
	E, I, D := differ.DiffEqual, differ.DiffInsert, differ.DiffDelete
	// Change blocks start at rows 2 and 6.
	types := []differ.DiffType{E, E, D, I, E, E, I, E}
	lines := make([]differ.DiffLine, len(types))
	for i, ty := range types {
		lines[i] = differ.DiffLine{Type: ty}
	}
	m := &Model{currentDiff: &differ.FileDiff{Lines: lines}, windowHeight: 30}

	steps := []struct {
		dir     int
		want    int
		wantErr bool
	}{
		{1, 2, false},
		{1, 6, false},
		{1, 6, true}, // no more below: cursor stays
		{-1, 2, false},
		{-1, 2, true},
	}
	for i, s := range steps {
		m.errorMsg = ""
		m.jumpToChange(s.dir)
		if m.cursor != s.want || (m.errorMsg != "") != s.wantErr {
			t.Fatalf("step %d: cursor=%d err=%q, want cursor=%d err=%v", i, m.cursor, m.errorMsg, s.want, s.wantErr)
		}
	}

	// Side-by-side uses its own rows: the delete+insert pair is one row.
	m.diffViewMode = DiffViewSideBySide
	m.sbsRows = differ.BuildSideBySideRows(lines)
	m.cursor = 0
	m.jumpToChange(1)
	if m.sbsRows[m.cursor].Type == differ.SBSEqual || m.cursor != 2 {
		t.Fatalf("side-by-side: cursor=%d, want 2 (first change)", m.cursor)
	}
	m.jumpToChange(1)
	if m.cursor != 5 {
		t.Fatalf("side-by-side: cursor=%d, want 5 (second change)", m.cursor)
	}
}
