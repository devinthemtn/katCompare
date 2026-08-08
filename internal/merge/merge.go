package merge

import (
	"fmt"
	"strings"

	"golang-fileCmp/internal/differ"
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
