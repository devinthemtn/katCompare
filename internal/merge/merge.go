package merge

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"katcompare/internal/differ"
)

// ChangeSelection represents which changes to apply
type ChangeSelection struct {
	ApplyInsertions map[int]bool // Maps line index to whether to apply insertion
	ApplyDeletions  map[int]bool // Maps line index to whether to apply deletion
}

// MergeResult represents the result of merging changes
type MergeResult struct {
	Content string
	Applied int
	Skipped int
}

// Merger handles merging operations
type Merger struct{}

// New creates a new Merger instance
func New() *Merger {
	return &Merger{}
}

// NewChangeSelection creates a new ChangeSelection with all changes selected by default
func NewChangeSelection(diff *differ.FileDiff) *ChangeSelection {
	selection := &ChangeSelection{
		ApplyInsertions: make(map[int]bool),
		ApplyDeletions:  make(map[int]bool),
	}

	// By default, select all changes
	for i, line := range diff.Lines {
		switch line.Type {
		case differ.DiffInsert:
			selection.ApplyInsertions[i] = true
		case differ.DiffDelete:
			selection.ApplyDeletions[i] = true
		}
	}

	return selection
}

// ToggleInsertion toggles whether to apply an insertion at the given line index
func (cs *ChangeSelection) ToggleInsertion(lineIndex int) {
	if current, exists := cs.ApplyInsertions[lineIndex]; exists {
		cs.ApplyInsertions[lineIndex] = !current
	}
}

// ToggleDeletion toggles whether to apply a deletion at the given line index
func (cs *ChangeSelection) ToggleDeletion(lineIndex int) {
	if current, exists := cs.ApplyDeletions[lineIndex]; exists {
		cs.ApplyDeletions[lineIndex] = !current
	}
}

// IsInsertionSelected returns whether an insertion at the given line index is selected
func (cs *ChangeSelection) IsInsertionSelected(lineIndex int) bool {
	return cs.ApplyInsertions[lineIndex]
}

// IsDeletionSelected returns whether a deletion at the given line index is selected
func (cs *ChangeSelection) IsDeletionSelected(lineIndex int) bool {
	return cs.ApplyDeletions[lineIndex]
}

// SelectAll selects all changes
func (cs *ChangeSelection) SelectAll(diff *differ.FileDiff) {
	for i, line := range diff.Lines {
		switch line.Type {
		case differ.DiffInsert:
			cs.ApplyInsertions[i] = true
		case differ.DiffDelete:
			cs.ApplyDeletions[i] = true
		}
	}
}

// SelectNone deselects all changes
func (cs *ChangeSelection) SelectNone(diff *differ.FileDiff) {
	for i, line := range diff.Lines {
		switch line.Type {
		case differ.DiffInsert:
			cs.ApplyInsertions[i] = false
		case differ.DiffDelete:
			cs.ApplyDeletions[i] = false
		}
	}
}

// ApplyToLeft applies selected changes to create a merged version starting from the left file
func (m *Merger) ApplyToLeft(diff *differ.FileDiff, selection *ChangeSelection) *MergeResult {
	var result strings.Builder
	applied := 0
	skipped := 0

	for i, line := range diff.Lines {
		switch line.Type {
		case differ.DiffEqual:
			// Always include equal lines
			result.WriteString(line.Content)
			result.WriteString("\n")

		case differ.DiffDelete:
			// Only include deleted lines if NOT selected for deletion
			if !selection.IsDeletionSelected(i) {
				result.WriteString(line.Content)
				result.WriteString("\n")
				skipped++
			} else {
				applied++
			}

		case differ.DiffInsert:
			// Only include inserted lines if selected for insertion
			if selection.IsInsertionSelected(i) {
				result.WriteString(line.Content)
				result.WriteString("\n")
				applied++
			} else {
				skipped++
			}
		}
	}

	// Remove trailing newline if present
	content := result.String()
	if strings.HasSuffix(content, "\n") {
		content = content[:len(content)-1]
	}

	return &MergeResult{
		Content: content,
		Applied: applied,
		Skipped: skipped,
	}
}

// ApplyToRight applies selected changes to create a merged version starting from the right file
func (m *Merger) ApplyToRight(diff *differ.FileDiff, selection *ChangeSelection) *MergeResult {
	var result strings.Builder
	applied := 0
	skipped := 0

	for i, line := range diff.Lines {
		switch line.Type {
		case differ.DiffEqual:
			// Always include equal lines
			result.WriteString(line.Content)
			result.WriteString("\n")

		case differ.DiffInsert:
			// Only include inserted lines if NOT selected for insertion
			if !selection.IsInsertionSelected(i) {
				result.WriteString(line.Content)
				result.WriteString("\n")
				skipped++
			} else {
				applied++
			}

		case differ.DiffDelete:
			// Only include deleted lines if selected for deletion
			if selection.IsDeletionSelected(i) {
				result.WriteString(line.Content)
				result.WriteString("\n")
				applied++
			} else {
				skipped++
			}
		}
	}

	// Remove trailing newline if present
	content := result.String()
	if strings.HasSuffix(content, "\n") {
		content = content[:len(content)-1]
	}

	return &MergeResult{
		Content: content,
		Applied: applied,
		Skipped: skipped,
	}
}

// GetSelectedStats returns statistics about selected changes
func (cs *ChangeSelection) GetSelectedStats(diff *differ.FileDiff) (int, int, int, int) {
	selectedInsertions := 0
	totalInsertions := 0
	selectedDeletions := 0
	totalDeletions := 0

	for i, line := range diff.Lines {
		switch line.Type {
		case differ.DiffInsert:
			totalInsertions++
			if cs.IsInsertionSelected(i) {
				selectedInsertions++
			}
		case differ.DiffDelete:
			totalDeletions++
			if cs.IsDeletionSelected(i) {
				selectedDeletions++
			}
		}
	}

	return selectedInsertions, totalInsertions, selectedDeletions, totalDeletions
}

// CreateMergePreview creates a preview of what the merged content would look like
func (m *Merger) CreateMergePreview(diff *differ.FileDiff, selection *ChangeSelection, targetSide string) string {
	var preview strings.Builder

	preview.WriteString(fmt.Sprintf("=== Merge Preview (applying to %s) ===\n\n", targetSide))

	selectedIns, totalIns, selectedDel, totalDel := selection.GetSelectedStats(diff)
	preview.WriteString(fmt.Sprintf("Changes to apply:\n"))
	preview.WriteString(fmt.Sprintf("  Insertions: %d/%d selected\n", selectedIns, totalIns))
	preview.WriteString(fmt.Sprintf("  Deletions:  %d/%d selected\n", selectedDel, totalDel))
	preview.WriteString(fmt.Sprintf("\n"))

	// Show first few lines of the result
	var result *MergeResult
	if targetSide == "left" {
		result = m.ApplyToLeft(diff, selection)
	} else {
		result = m.ApplyToRight(diff, selection)
	}

	lines := strings.Split(result.Content, "\n")
	maxLines := 10
	if len(lines) > maxLines {
		for i := 0; i < maxLines; i++ {
			preview.WriteString(fmt.Sprintf("%3d: %s\n", i+1, lines[i]))
		}
		preview.WriteString(fmt.Sprintf("... (%d more lines)\n", len(lines)-maxLines))
	} else {
		for i, line := range lines {
			preview.WriteString(fmt.Sprintf("%3d: %s\n", i+1, line))
		}
	}

	return preview.String()
}

// ToggleBlock toggles the whole block of adjacent changed lines containing
// lineIndex: every change in it becomes selected, or all are deselected when
// they already were. It does nothing on an unchanged line.
func (cs *ChangeSelection) ToggleBlock(diff *differ.FileDiff, lineIndex int) {
	lines := diff.Lines
	if lineIndex < 0 || lineIndex >= len(lines) || lines[lineIndex].Type == differ.DiffEqual {
		return
	}
	start, end := lineIndex, lineIndex
	for start > 0 && lines[start-1].Type != differ.DiffEqual {
		start--
	}
	for end+1 < len(lines) && lines[end+1].Type != differ.DiffEqual {
		end++
	}
	allOn := true
	for i := start; i <= end; i++ {
		if !cs.IsInsertionSelected(i) && !cs.IsDeletionSelected(i) {
			allOn = false
			break
		}
	}
	for i := start; i <= end; i++ {
		switch lines[i].Type {
		case differ.DiffInsert:
			cs.ApplyInsertions[i] = !allOn
		case differ.DiffDelete:
			cs.ApplyDeletions[i] = !allOn
		}
	}
}

// SaveWithBackup overwrites path with content, first copying the current
// file to path+".orig". The file keeps its permissions, and its trailing
// newline (the merge result has none) when it had one. It returns the
// backup path.
func SaveWithBackup(path, content string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory", path)
	}
	old, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	backup := path + ".orig"
	if err := os.WriteFile(backup, old, info.Mode().Perm()); err != nil {
		return "", fmt.Errorf("failed to write backup: %w", err)
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		if bytes.HasSuffix(old, []byte("\r\n")) {
			content += "\r\n"
		} else if bytes.HasSuffix(old, []byte("\n")) {
			content += "\n"
		}
	}
	if err := os.WriteFile(path, []byte(content), info.Mode().Perm()); err != nil {
		return "", fmt.Errorf("failed to save %s: %w", path, err)
	}
	return backup, nil
}
