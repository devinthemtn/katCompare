package ui

import (
	"fmt"
	"sort"
	"strings"

	"golang-fileCmp/internal/differ"
	"golang-fileCmp/internal/file"

	"github.com/charmbracelet/lipgloss"
)

// renderFileSelectView renders the file selection interface
func (m *Model) renderFileSelectView() string {
	var b strings.Builder

	// Title
	title := titleStyle.Width(m.windowWidth - 2).Render("File Comparison Tool")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Input fields - adapt width to terminal
	maxInputWidth := m.windowWidth - 20 // Leave space for labels and padding
	if maxInputWidth < 30 {
		maxInputWidth = 30
	}

	leftLabel := "Left Path: "
	rightLabel := "Right Path: "

	// Truncate input display if too long
	leftDisplay := m.inputLeft
	rightDisplay := m.inputRight
	if len(leftDisplay) > maxInputWidth {
		leftDisplay = "..." + leftDisplay[len(leftDisplay)-maxInputWidth+3:]
	}
	if len(rightDisplay) > maxInputWidth {
		rightDisplay = "..." + rightDisplay[len(rightDisplay)-maxInputWidth+3:]
	}

	var leftInput, rightInput string
	if m.focus == focusInputLeft {
		leftInput = focusedInputStyle.Width(maxInputWidth).Render(leftDisplay + "│")
		rightInput = inputStyle.Width(maxInputWidth).Render(rightDisplay)
	} else if m.focus == focusInputRight {
		leftInput = inputStyle.Width(maxInputWidth).Render(leftDisplay)
		rightInput = focusedInputStyle.Width(maxInputWidth).Render(rightDisplay + "│")
	} else {
		leftInput = inputStyle.Width(maxInputWidth).Render(leftDisplay)
		rightInput = inputStyle.Width(maxInputWidth).Render(rightDisplay)
	}

	b.WriteString(leftLabel + leftInput)
	b.WriteString("\n")

	// Show suggestions for left path if focused and available
	if m.focus == focusInputLeft && m.showSuggestions && len(m.leftSuggestions) > 0 {
		b.WriteString(m.renderSuggestions(m.leftSuggestions, m.leftSuggIndex))
		b.WriteString("\n")
	}

	b.WriteString(rightLabel + rightInput)
	b.WriteString("\n")

	// Show suggestions for right path if focused and available
	if m.focus == focusInputRight && m.showSuggestions && len(m.rightSuggestions) > 0 {
		b.WriteString(m.renderSuggestions(m.rightSuggestions, m.rightSuggIndex))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Buttons - part of the Tab focus cycle (Left Path -> Right Path -> View Diff -> Quit)
	var quitButton string
	if m.focus == focusQuitBtn {
		quitButton = focusedButtonStyle.Render("Quit")
	} else {
		quitButton = buttonStyle.Render("Quit")
	}

	if m.canViewDiff() {
		var viewDiffButton string
		if m.focus == focusViewDiffBtn {
			viewDiffButton = focusedButtonStyle.Render("View Diff")
		} else {
			viewDiffButton = buttonStyle.Render("View Diff")
		}
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, viewDiffButton, "  ", quitButton))
	} else {
		b.WriteString(quitButton)
	}
	b.WriteString("\n")

	// Status information - truncate paths if too long
	if m.leftFile != nil {
		leftPath := m.leftPath
		if len(leftPath) > maxInputWidth {
			leftPath = "..." + leftPath[len(leftPath)-maxInputWidth+3:]
		}
		b.WriteString(fmt.Sprintf("✓ Left: %s ", leftPath))
		if m.leftFile.IsDir {
			b.WriteString("(directory)")
		} else {
			b.WriteString("(file)")
		}
		b.WriteString("\n")
	}

	if m.rightFile != nil {
		rightPath := m.rightPath
		if len(rightPath) > maxInputWidth {
			rightPath = "..." + rightPath[len(rightPath)-maxInputWidth+3:]
		}
		b.WriteString(fmt.Sprintf("✓ Right: %s ", rightPath))
		if m.rightFile.IsDir {
			b.WriteString("(directory)")
		} else {
			b.WriteString("(file)")
		}
		b.WriteString("\n")
	}

	if len(m.allFiles) > 0 {
		commonCount := len(m.commonFiles)
		totalCount := len(m.allFiles)
		uniqueCount := totalCount - commonCount

		if commonCount > 0 && uniqueCount > 0 {
			b.WriteString(fmt.Sprintf("\nFound %d files (%d common, %d unique):\n", totalCount, commonCount, uniqueCount))
		} else if commonCount > 0 {
			b.WriteString(fmt.Sprintf("\nFound %d common files:\n", commonCount))
		} else {
			b.WriteString(fmt.Sprintf("\nFound %d unique files:\n", uniqueCount))
		}

		if m.selectedFile != "" {
			selectedPath := m.displayNameForFile(m.selectedFile)

			// Truncate path if too long for display
			maxPathWidth := m.windowWidth - 16 // Leave space for "► Selected: "
			if maxPathWidth < 20 {
				maxPathWidth = 20
			}
			if len(selectedPath) > maxPathWidth {
				selectedPath = "..." + selectedPath[len(selectedPath)-(maxPathWidth-3):]
			}

			// Show selected file on one line
			b.WriteString(selectedFileStyle.Render("► Selected: " + selectedPath))
			b.WriteString("\n")

			// Show help instruction on separate line to avoid width conflicts
			var helpText string
			maxHelpWidth := m.windowWidth - 6 // Conservative margin
			if maxHelpWidth < 20 {
				maxHelpWidth = 20
			}

			if m.windowWidth < 60 {
				helpText = "Press Ctrl+D to compare"
			} else {
				helpText = "Press Ctrl+D to compare, or ↑/↓ to select different file"
			}

			// Truncate help text if still too long
			if len(helpText) > maxHelpWidth {
				helpText = helpText[:maxHelpWidth-3] + "..."
			}

			b.WriteString(helpStyle.Render("  " + helpText))
			b.WriteString("\n")
		}

		// Filter bar
		if m.filterActive {
			filterDisplay := m.filterQuery + "│"
			b.WriteString(focusedInputStyle.Width(m.windowWidth-4).Render("Filter: " + filterDisplay))
			b.WriteString("\n")
		}

		b.WriteString(m.renderFileList())
	}

	// Error message
	if m.errorMsg != "" {
		b.WriteString("\n")
		errorMsg := m.errorMsg
		if len(errorMsg) > m.windowWidth-20 {
			errorMsg = errorMsg[:m.windowWidth-23] + "..."
		}
		b.WriteString(errorStyle.Width(m.windowWidth - 4).Render("Error: " + errorMsg))
		b.WriteString("\n")
	}

	// Help text - adapt to width
	b.WriteString("\n")
	var helpText string
	if m.filterActive {
		helpText = "Type to filter • Backspace: delete • Esc: clear filter • ↑↓: Navigate • Ctrl+D: Compare"
	} else if m.showSuggestions {
		if m.windowWidth > 80 {
			helpText = "↑↓: Navigate suggestions • Tab: Next suggestion • Enter: Accept • Esc: Cancel"
		} else {
			helpText = "↑↓: Navigate • Tab: Next • Enter: Accept • Esc: Cancel"
		}
	} else {
		if len(m.allFiles) > 0 {
			// When files are loaded, emphasize the comparison functionality
			if m.windowWidth > 100 {
				helpText = "Tab: Switch input/View Diff/Quit • Enter: Load path or activate button • ↑↓: Navigate files • Ctrl+D: Compare • /: Filter • ?: Help • Q: Quit"
			} else if m.windowWidth > 80 {
				helpText = "Tab: Switch/Diff/Quit • Enter: Load/Activate • ↑↓: Navigate • Ctrl+D: Compare • /: Filter • ?: Help • Q: Quit"
			} else if m.windowWidth > 60 {
				helpText = "Tab:Switch/Diff/Quit Enter:Load/Activate ↑↓:Nav Ctrl+D:Compare /:Filter ?:Help Q:Quit"
			} else {
				helpText = "↑↓:Select Ctrl+D:Compare /:Filter Q:Quit"
			}
		} else {
			// When no files are loaded, emphasize the input functionality
			if m.windowWidth > 80 {
				helpText = "Tab: Switch input/Quit button • Enter: Load path • ↑↓: Navigate suggestions • ?: Help • Q: Quit"
			} else if m.windowWidth > 60 {
				helpText = "Tab: Switch/Quit • Enter: Load • ↑↓: Navigate • ?: Help • Q: Quit"
			} else {
				helpText = "Tab:Switch Enter:Load ?:Help Q:Quit"
			}
		}
	}
	b.WriteString(helpStyle.Width(m.windowWidth - 2).Render(helpText))

	return b.String()
}

// renderFileList renders the list of all files (common and unique)
func (m *Model) renderFileList() string {
	if len(m.allFiles) == 0 {
		return ""
	}

	// Define styles for file status indicators
	identicalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Bold(true) // Green
	differentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Bold(true) // Red
	leftOnlyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#0066FF")).Bold(true)  // Blue
	rightOnlyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6600")).Bold(true) // Orange
	sizeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))                 // Gray

	// Calculate available space for file list
	usedHeight := 15 // Approximate height used by title, inputs, status, help
	if m.showSuggestions {
		usedHeight += 10 // More space used by suggestions
	}
	if m.filterActive {
		usedHeight += 2 // Space for filter bar
	}
	if m.errorMsg != "" {
		usedHeight += 3 // Space for error message
	}

	availableHeight := m.windowHeight - usedHeight
	if availableHeight < 5 {
		availableHeight = 5
	}

	// Get all files, sorted, then filtered
	allFiles := make([]string, 0, len(m.allFiles))
	for relPath := range m.allFiles {
		allFiles = append(allFiles, relPath)
	}
	sort.Strings(allFiles)

	if m.filterActive && m.filterQuery != "" {
		query := strings.ToLower(m.filterQuery)
		filtered := allFiles[:0]
		for _, f := range allFiles {
			if strings.Contains(strings.ToLower(f), query) {
				filtered = append(filtered, f)
			}
		}
		allFiles = filtered
	}

	// Find current selection index
	selectedIndex := 0
	for i, relPath := range allFiles {
		if relPath == m.selectedFile {
			selectedIndex = i
			break
		}
	}

	// Calculate scroll offset to keep selected item visible
	startIndex := 0
	if len(allFiles) > availableHeight {
		startIndex = selectedIndex - availableHeight/2
		if startIndex < 0 {
			startIndex = 0
		}
		if startIndex+availableHeight > len(allFiles) {
			startIndex = len(allFiles) - availableHeight
		}
	}

	endIndex := startIndex + availableHeight
	if endIndex > len(allFiles) {
		endIndex = len(allFiles)
	}

	// Calculate max width for file display
	maxFileWidth := m.windowWidth - 8 // Account for borders and padding

	var items []string
	for i := startIndex; i < endIndex; i++ {
		relPath := allFiles[i]
		fileComparison := m.allFiles[relPath]

		var statusIndicator, sizeInfo, sourceInfo string

		switch fileComparison.Source {
		case file.SourceBoth:
			leftFile := fileComparison.LeftFile
			rightFile := fileComparison.RightFile

			// Check if files are identical
			isIdentical := leftFile.Content == rightFile.Content
			if isIdentical {
				statusIndicator = identicalStyle.Render("✓")
			} else {
				statusIndicator = differentStyle.Render("✗")
			}

			// Format file size (show left file size, or both if different)
			if leftFile.Size == rightFile.Size {
				sizeInfo = sizeStyle.Render(fmt.Sprintf("(%s)", formatFileSize(leftFile.Size)))
			} else {
				sizeInfo = sizeStyle.Render(fmt.Sprintf("(L:%s R:%s)", formatFileSize(leftFile.Size), formatFileSize(rightFile.Size)))
			}
			sourceInfo = ""

		case file.SourceLeft:
			leftFile := fileComparison.LeftFile
			statusIndicator = leftOnlyStyle.Render("◄")
			sizeInfo = sizeStyle.Render(fmt.Sprintf("(%s)", formatFileSize(leftFile.Size)))
			sourceInfo = leftOnlyStyle.Render(" [LEFT ONLY]")

		case file.SourceRight:
			rightFile := fileComparison.RightFile
			statusIndicator = rightOnlyStyle.Render("►")
			sizeInfo = sizeStyle.Render(fmt.Sprintf("(%s)", formatFileSize(rightFile.Size)))
			sourceInfo = rightOnlyStyle.Render(" [RIGHT ONLY]")
		}

		// Truncate filename if too long.
		// Use lipgloss.Width to measure rendered strings — len() counts ANSI escape bytes too.
		displayPath := m.displayNameForFile(relPath)
		baseWidth := 4 + lipgloss.Width(sizeInfo) + lipgloss.Width(sourceInfo)
		if len(displayPath) > maxFileWidth-baseWidth {
			maxPathWidth := maxFileWidth - baseWidth - 3 // 3 for "..."
			if maxPathWidth > 0 {
				displayPath = "..." + displayPath[len(displayPath)-maxPathWidth:]
			}
		}

		fileDisplay := fmt.Sprintf("%s %s %s%s", statusIndicator, displayPath, sizeInfo, sourceInfo)

		if relPath == m.selectedFile {
			items = append(items, selectedFileStyle.Width(maxFileWidth).Render("► "+fileDisplay))
		} else {
			items = append(items, "  "+fileDisplay)
		}
	}

	content := strings.Join(items, "\n")
	contentHeight := len(items)

	// Add scroll indicator if needed
	if len(allFiles) > availableHeight {
		scrollInfo := fmt.Sprintf("\nShowing %d-%d of %d files", startIndex+1, endIndex, len(allFiles))
		content += helpStyle.Render(scrollInfo)
		contentHeight += 2 // blank line + scroll indicator line
	}

	// Size the box to its actual content instead of always reserving the
	// full available height - a fixed-height box padded with blank lines
	// pushes the rest of the screen (title, path inputs) past the terminal
	// height and off-screen once files are loaded.
	if contentHeight < 1 {
		contentHeight = 1
	}
	return fileListStyle.Width(m.windowWidth - 4).Height(contentHeight).Render(content)
}

// formatFileSize formats file size in human readable format
func formatFileSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%dB", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(size)/1024)
	} else if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1fMB", float64(size)/(1024*1024))
	} else {
		return fmt.Sprintf("%.1fGB", float64(size)/(1024*1024*1024))
	}
}

// renderDiffView renders the diff comparison view
func (m *Model) renderDiffView() string {
	if m.currentDiff == nil {
		return "No diff loaded"
	}

	var b strings.Builder

	// Header with file names - truncate if too long
	leftFile := m.currentDiff.LeftFile
	rightFile := m.currentDiff.RightFile
	maxFileNameWidth := (m.windowWidth - 4) / 2 // Split width for both names
	if len(leftFile) > maxFileNameWidth {
		leftFile = "..." + leftFile[len(leftFile)-maxFileNameWidth+3:]
	}
	if len(rightFile) > maxFileNameWidth {
		rightFile = "..." + rightFile[len(rightFile)-maxFileNameWidth+3:]
	}

	var viewModeIndicator string
	if m.diffViewMode == DiffViewSideBySide {
		viewModeIndicator = " [Side-by-Side]"
	} else {
		viewModeIndicator = " [Unified]"
	}

	header := fmt.Sprintf("%s vs %s%s", leftFile, rightFile, viewModeIndicator)
	b.WriteString(headerStyle.Width(m.windowWidth).Render(header))
	b.WriteString("\n\n")

	// Stats
	equal, inserted, deleted := m.currentDiff.GetStats()
	var stats string
	if m.windowWidth > 60 {
		stats = fmt.Sprintf("Lines: %d equal, %d inserted (+), %d deleted (-)", equal, inserted, deleted)
	} else {
		stats = fmt.Sprintf("%d equal, %d added, %d deleted", equal, inserted, deleted)
	}
	b.WriteString(helpStyle.Width(m.windowWidth).Render(stats))
	b.WriteString("\n\n")

	// Diff content based on view mode
	if m.diffViewMode == DiffViewSideBySide {
		b.WriteString(m.renderSideBySideContent())
	} else {
		b.WriteString(m.renderDiffContent())
	}

	// Navigation help - adapt to width and view mode
	b.WriteString("\n")
	var helpText string
	if m.diffViewMode == DiffViewSideBySide {
		if m.windowWidth > 80 {
			helpText = "↑↓/j/k: Navigate • h/l: Left/Right • g/G: Top/Bottom • s: Switch view • n/p: Next/Prev file • m: Merge • Esc: Back • ?: Help • Q: Quit"
		} else if m.windowWidth > 60 {
			helpText = "↑↓/j/k: Navigate • h/l: Left/Right • s: Switch view • n/p: All files • m: Merge • Esc: Back • ?: Help • Q: Quit"
		} else {
			helpText = "↑↓:Nav h/l:L/R s:View n/p:Files m:Merge Esc:Back ?:Help Q:Quit"
		}
	} else {
		if m.windowWidth > 80 {
			helpText = "↑↓/j/k: Navigate • g/G: Top/Bottom • s: Switch view • n/p: Next/Prev file • m: Merge • Esc: Back • ?: Help • Q: Quit"
		} else if m.windowWidth > 60 {
			helpText = "↑↓/j/k: Navigate • g/G: Top/Bottom • s: Switch view • n/p: All files • m: Merge • Esc: Back • ?: Help • Q: Quit"
		} else {
			helpText = "↑↓:Nav g/G:Top/Bot s:View n/p:Files m:Merge Esc:Back ?:Help Q:Quit"
		}
	}
	b.WriteString(helpStyle.Width(m.windowWidth).Render(helpText))

	return b.String()
}

// renderDiffContent renders the actual diff lines with syntax highlighting
func (m *Model) renderDiffContent() string {
	if m.currentDiff == nil || len(m.currentDiff.Lines) == 0 {
		return "No differences found"
	}

	var b strings.Builder
	maxVisible := m.windowHeight - 10 // Account for header, stats, and help text
	if maxVisible < 5 {
		maxVisible = 5
	}

	start := m.scrollOffset
	end := start + maxVisible
	if end > len(m.currentDiff.Lines) {
		end = len(m.currentDiff.Lines)
	}

	for i := start; i < end; i++ {
		line := m.currentDiff.Lines[i]
		prefix := "  "

		if i == m.cursor {
			prefix = "▶ "
		}

		lineNum := fmt.Sprintf("%4d", line.LineNum)
		content := line.Content

		// Truncate long lines to fit screen width
		maxContentWidth := m.windowWidth - 12 // Account for prefix, line numbers, and padding
		if maxContentWidth < 20 {
			maxContentWidth = 20
		}
		if len(content) > maxContentWidth {
			content = content[:maxContentWidth-3] + "..."
		}

		var renderedLine string
		lineText := fmt.Sprintf("%s%s %s", prefix, lineNum, content)

		switch line.Type {
		case differ.DiffEqual:
			renderedLine = equalLineStyle.Width(m.windowWidth - 2).Render(lineText)
		case differ.DiffInsert:
			renderedLine = insertLineStyle.Width(m.windowWidth - 2).Render(fmt.Sprintf("%s%s +%s", prefix, lineNum, content))
		case differ.DiffDelete:
			renderedLine = deleteLineStyle.Width(m.windowWidth - 2).Render(fmt.Sprintf("%s%s -%s", prefix, lineNum, content))
		}

		b.WriteString(renderedLine)
		b.WriteString("\n")
	}

	// Show scroll indicator if needed
	if len(m.currentDiff.Lines) > maxVisible {
		var scrollInfo string
		if m.windowWidth > 50 {
			scrollInfo = fmt.Sprintf("Showing %d-%d of %d lines", start+1, end, len(m.currentDiff.Lines))
		} else {
			scrollInfo = fmt.Sprintf("%d-%d of %d", start+1, end, len(m.currentDiff.Lines))
		}
		b.WriteString(helpStyle.Width(m.windowWidth).Render(scrollInfo))
		b.WriteString("\n")
	}

	return b.String()
}

// renderHelpView renders the help screen
func (m *Model) renderHelpView() string {
	var b strings.Builder

	title := titleStyle.Render("Help - File Comparison Tool")
	b.WriteString(title)
	b.WriteString("\n\n")

	help := `File Selection Mode:
  Tab              Switch between left/right input fields (or cycle suggestions)
  Enter            Load the entered path / Accept selected suggestion
  ↑/↓              Navigate through common files list / Navigate suggestions
  Esc              Clear path suggestions
  Ctrl+D           Start comparing selected files
  ?                Show this help screen
  Q/Ctrl+C         Quit application

Path Suggestions:
  Type any path    Smart suggestions appear automatically
  ↑/↓              Navigate through suggestions
  Tab              Cycle to next suggestion
  Enter            Accept selected suggestion
  Esc              Cancel suggestions

Diff View Mode:
  ↑/↓ or j/k       Navigate through diff lines
  g                Go to top of diff
  G                Go to bottom of diff
  s                Switch view mode (Unified ↔ Side-by-Side)
  n                Next common file
  p                Previous common file
  m                Enter merge mode
  c                Enter copy mode (for unique files)
  Esc              Return to file selection
  ?                Show this help screen
  Q/Ctrl+C         Quit application

Side-by-Side View Mode:
  ↑/↓ or j/k       Navigate through diff lines
  h/l or ←/→       Visual focus left/right (for reference)
  g                Go to top of diff
  G                Go to bottom of diff
  s                Switch to Unified view mode
  n                Next common file
  p                Previous common file
  m                Enter merge mode
  c                Enter copy mode (for unique files)
  Esc              Return to file selection
  ?                Show this help screen
  Q/Ctrl+C         Quit application

Merge Mode:
  ↑/↓ or j/k       Navigate through diff lines
  Space/Enter      Toggle selection of current change
  t                Switch merge target (left/right)
  a                Select all changes
  n                Select no changes
  s                Save merged result to file
  Esc              Return to diff view
  ?                Show this help screen
  Q/Ctrl+C         Quit application

Copy Mode (Directory Comparison Only):
  ↑/↓ or j/k       Navigate through unique files
  Space/Enter      Toggle selection of current file to copy
  t                Switch copy target (to-left/to-right)
  a                Select all unique files
  n                Select no files
  s                Copy selected files to target directory
  Esc              Return to file selection
  ?                Show this help screen
  Q/Ctrl+C         Quit application

Color Legend:
  `

	b.WriteString(help)

	// Color examples
	b.WriteString(insertLineStyle.Render("Blue background: Added lines (+)"))
	b.WriteString("\n")
	b.WriteString(deleteLineStyle.Render("Red background: Deleted lines (-)"))
	b.WriteString("\n")
	b.WriteString(equalLineStyle.Render("Gray text: Unchanged lines"))
	b.WriteString("\n")
	b.WriteString(selectedChangeStyle.Render("Yellow background: Selected changes (merge mode)"))
	b.WriteString("\n")
	b.WriteString(unselectedChangeStyle.Render("Strikethrough: Unselected changes (merge mode)"))
	b.WriteString("\n\n")

	fileIndicators := `File Status Indicators:
  `
	b.WriteString(fileIndicators)

	// File status examples
	identicalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Bold(true)
	differentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Bold(true)
	leftOnlyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#0066FF")).Bold(true)
	rightOnlyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6600")).Bold(true)

	b.WriteString(identicalStyle.Render("✓ Green checkmark: Identical files (same content)"))
	b.WriteString("\n")
	b.WriteString(differentStyle.Render("✗ Red X: Different files (content differs)"))
	b.WriteString("\n")
	b.WriteString(leftOnlyStyle.Render("◄ Blue arrow: File exists only in LEFT directory"))
	b.WriteString("\n")
	b.WriteString(rightOnlyStyle.Render("► Orange arrow: File exists only in RIGHT directory"))
	b.WriteString("\n\n")

	instructions := `Instructions:
1. Enter paths to files or directories in the input fields
   - Path suggestions appear automatically as you type
   - Use ↑/↓ or Tab to navigate through suggestions
   - Press Enter to accept a suggestion or Esc to dismiss
2. Press Enter to load each path
3. Use ↑/↓ to select which file to compare (shows ALL files, not just common ones)
4. Press Ctrl+D to start comparing selected file
5. Navigate through the diff using arrow keys or j/k
6. Use n/p to switch between different files
7. Press 'm' in diff view to enter merge mode
8. In merge mode, select which changes to apply and press 's' to save
9. Press 'c' in file selection to enter copy mode for unique files
10. In copy mode, select files to copy and press 's' to copy them

Merge Workflow:
- Enter merge mode from diff view with 'm'
- Navigate with ↑/↓ or j/k through changes
- Toggle individual changes with Space/Enter
- Use 'a' to select all or 'n' to select none
- Switch target file with 't' (left or right)
- Save merged result with 's' - creates .merged file
- Apply changes in either direction (left-to-right or right-to-left)

Copy Workflow (for directories with unique files):
- Enter copy mode from file selection with 'c'
- Navigate with ↑/↓ or j/k through unique files
- Toggle individual files with Space/Enter
- Use 'a' to select all or 'n' to select none
- Switch copy direction with 't' (to-left or to-right)
- Copy selected files with 's' - files are copied to target directory

File Comparison Notes:
- Shows ALL files from both directories, not just common ones
- Files unique to one side show [LEFT ONLY] or [RIGHT ONLY] tags
- Unique files can still be "compared" (shown as all additions or deletions)
- Unique files can be easily copied between directories using copy mode
- Only text files are included (detects common extensions automatically)
- Path suggestions include files and directories from your current working directory`

	b.WriteString(instructions)
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("Press Esc or ? to return to previous view"))

	return b.String()
}

// renderSuggestions renders path suggestions
func (m *Model) renderSuggestions(suggestions []string, selectedIndex int) string {
	if len(suggestions) == 0 {
		return ""
	}

	// Limit suggestions based on available space
	maxSuggestions := 6
	availableHeight := m.windowHeight - 20
	if availableHeight > 0 && availableHeight < maxSuggestions {
		maxSuggestions = availableHeight
	}
	if maxSuggestions < 3 {
		maxSuggestions = 3
	}

	displaySuggestions := suggestions
	if len(displaySuggestions) > maxSuggestions {
		displaySuggestions = displaySuggestions[:maxSuggestions]
	}

	maxSuggestionWidth := m.windowWidth - 10
	if maxSuggestionWidth < 30 {
		maxSuggestionWidth = 30
	}

	var suggestionLines []string
	for i, suggestion := range displaySuggestions {
		// Truncate suggestion if too long
		displaySuggestion := suggestion
		if len(displaySuggestion) > maxSuggestionWidth {
			displaySuggestion = "..." + displaySuggestion[len(displaySuggestion)-maxSuggestionWidth+3:]
		}

		if i == selectedIndex && selectedIndex < len(displaySuggestions) {
			suggestionLines = append(suggestionLines, selectedSuggestionStyle.Width(maxSuggestionWidth).Render("  → "+displaySuggestion))
		} else {
			suggestionLines = append(suggestionLines, "    "+displaySuggestion)
		}
	}

	content := strings.Join(suggestionLines, "\n")
	if len(suggestions) > maxSuggestions {
		content += helpStyle.Render(fmt.Sprintf("\n... and %d more", len(suggestions)-maxSuggestions))
	}
	return suggestionStyle.Width(m.windowWidth - 4).Render(content)
}

// renderMergeView renders the merge interface
func (m *Model) renderMergeView() string {
	if m.currentDiff == nil {
		return "No diff loaded for merging"
	}

	var b strings.Builder

	// Header
	header := fmt.Sprintf("Merge Mode - Target: %s", strings.ToUpper(m.mergeTarget))
	b.WriteString(mergeHeaderStyle.Width(m.windowWidth).Render(header))
	b.WriteString("\n\n")

	// Statistics
	if m.changeSelection != nil {
		selIns, totIns, selDel, totDel := m.changeSelection.GetSelectedStats(m.currentDiff)
		stats := fmt.Sprintf("Selected: %d/%d insertions, %d/%d deletions", selIns, totIns, selDel, totDel)
		b.WriteString(helpStyle.Width(m.windowWidth).Render(stats))
		b.WriteString("\n\n")
	}

	// Diff content with selection indicators
	b.WriteString(m.renderMergeContent())

	// Help text
	b.WriteString("\n")
	var helpText string
	if m.windowWidth > 80 {
		helpText = "Space/Enter: Toggle • t: Switch target • a: Select all • n: Select none • s: Save • Esc: Back • ?: Help"
	} else if m.windowWidth > 60 {
		helpText = "Space: Toggle • t: Target • a: All • n: None • s: Save • Esc: Back"
	} else {
		helpText = "Space:Toggle t:Target a:All n:None s:Save Esc:Back"
	}
	b.WriteString(helpStyle.Width(m.windowWidth).Render(helpText))

	return b.String()
}

// renderCopyView renders the copy interface for copying unique files
func (m *Model) renderCopyView() string {
	if len(m.allFiles) == 0 {
		return "No files loaded for copying"
	}

	uniqueFiles := m.getUniqueFiles()
	if len(uniqueFiles) == 0 {
		return "No unique files to copy"
	}

	var b strings.Builder

	// Header
	var targetDesc string
	if m.copyTarget == "to-right" {
		targetDesc = "to RIGHT directory"
	} else {
		targetDesc = "to LEFT directory"
	}
	header := fmt.Sprintf("Copy Mode - Target: %s", targetDesc)
	b.WriteString(mergeHeaderStyle.Width(m.windowWidth).Render(header))
	b.WriteString("\n\n")

	// Statistics
	selectedCount := 0
	totalCount := len(uniqueFiles)
	for _, relPath := range uniqueFiles {
		if m.copySelection[relPath] {
			selectedCount++
		}
	}
	stats := fmt.Sprintf("Selected: %d/%d unique files to copy", selectedCount, totalCount)
	b.WriteString(helpStyle.Width(m.windowWidth).Render(stats))
	b.WriteString("\n\n")

	// Copy content with selection indicators
	b.WriteString(m.renderCopyContent())

	// Help text
	b.WriteString("\n")
	var helpText string
	if m.windowWidth > 80 {
		helpText = "Space/Enter: Toggle • t: Switch target • a: Select all • n: Select none • s: Copy files • Esc: Back • ?: Help"
	} else if m.windowWidth > 60 {
		helpText = "Space: Toggle • t: Target • a: All • n: None • s: Copy • Esc: Back"
	} else {
		helpText = "Space:Toggle t:Target a:All n:None s:Copy Esc:Back"
	}
	b.WriteString(helpStyle.Width(m.windowWidth).Render(helpText))

	return b.String()
}

// renderCopyContent renders the unique files with selection indicators
func (m *Model) renderCopyContent() string {
	uniqueFiles := m.getUniqueFiles()
	if len(uniqueFiles) == 0 {
		return "No unique files found"
	}

	var b strings.Builder
	maxVisible := m.windowHeight - 12 // Account for header, stats, and help text
	if maxVisible < 5 {
		maxVisible = 5
	}

	start := m.scrollOffset
	end := start + maxVisible
	if end > len(uniqueFiles) {
		end = len(uniqueFiles)
	}

	for i := start; i < end; i++ {
		relPath := uniqueFiles[i]
		fileComp := m.allFiles[relPath]

		cursor := "  "
		if i == m.cursor {
			cursor = "▶ "
		}

		// Determine file source and status
		var statusIcon, sourceInfo string
		var fileInfo *file.FileInfo

		if fileComp.Source == file.SourceLeft {
			statusIcon = "◄"
			sourceInfo = "[LEFT ONLY]"
			fileInfo = fileComp.LeftFile
		} else {
			statusIcon = "►"
			sourceInfo = "[RIGHT ONLY]"
			fileInfo = fileComp.RightFile
		}

		// Check if this file should be copied based on target
		canCopy := false
		if m.copyTarget == "to-right" && fileComp.Source == file.SourceLeft {
			canCopy = true
		} else if m.copyTarget == "to-left" && fileComp.Source == file.SourceRight {
			canCopy = true
		}

		// Selection indicator
		var selectionIcon string
		if !canCopy {
			selectionIcon = "[-]" // Cannot copy (wrong direction)
		} else if m.copySelection[relPath] {
			selectionIcon = "[✓]" // Selected for copying
		} else {
			selectionIcon = "[ ]" // Not selected
		}

		// File size
		sizeInfo := fmt.Sprintf("(%s)", formatFileSize(fileInfo.Size))

		// Truncate filename if needed
		maxContentWidth := m.windowWidth - 25 // Account for cursor, icons, size, etc.
		if maxContentWidth < 20 {
			maxContentWidth = 20
		}
		displayPath := relPath
		if len(displayPath) > maxContentWidth {
			displayPath = displayPath[:maxContentWidth-3] + "..."
		}

		// Render line
		lineText := fmt.Sprintf("%s%s %s %s %s %s", cursor, selectionIcon, statusIcon, displayPath, sizeInfo, sourceInfo)

		var renderedLine string
		if !canCopy {
			// Grayed out for files that can't be copied in current direction
			renderedLine = helpStyle.Width(m.windowWidth - 2).Render(lineText)
		} else if m.copySelection[relPath] {
			// Selected files
			renderedLine = selectedChangeStyle.Width(m.windowWidth - 2).Render(lineText)
		} else {
			// Unselected files
			renderedLine = lineText
		}

		b.WriteString(renderedLine)
		b.WriteString("\n")
	}

	// Show scroll indicator if needed
	if len(uniqueFiles) > maxVisible {
		var scrollInfo string
		if m.windowWidth > 50 {
			scrollInfo = fmt.Sprintf("Showing %d-%d of %d unique files", start+1, end, len(uniqueFiles))
		} else {
			scrollInfo = fmt.Sprintf("%d-%d of %d", start+1, end, len(uniqueFiles))
		}
		b.WriteString(helpStyle.Width(m.windowWidth).Render(scrollInfo))
		b.WriteString("\n")
	}

	return b.String()
}

// renderSideBySideContent renders the diff in a two-column layout using
// precomputed SideBySideRows so that changed lines appear aligned side-by-side.
func (m *Model) renderSideBySideContent() string {
	if len(m.sbsRows) == 0 {
		return "No differences found"
	}

	var b strings.Builder
	maxVisible := m.windowHeight - 10 // account for header, stats, help
	if maxVisible < 5 {
		maxVisible = 5
	}

	sideWidth := (m.windowWidth - 5) / 2 // -5 for " │ " separator
	if sideWidth < 20 {
		sideWidth = 20
	}
	maxContentWidth := sideWidth - 8 // leave room for cursor (2) + line num (4) + space (1) + margin (1)
	if maxContentWidth < 10 {
		maxContentWidth = 10
	}

	start := m.scrollOffset
	end := start + maxVisible
	if end > len(m.sbsRows) {
		end = len(m.sbsRows)
	}

	for i := start; i < end; i++ {
		row := m.sbsRows[i]

		cursorStr := "  "
		if i == m.cursor {
			cursorStr = "▶ "
		}

		// Format line number gutters
		leftNumStr := "    "
		if row.LeftLineNum > 0 {
			leftNumStr = fmt.Sprintf("%4d", row.LeftLineNum)
		}
		rightNumStr := "    "
		if row.RightLineNum > 0 {
			rightNumStr = fmt.Sprintf("%4d", row.RightLineNum)
		}

		// Apply horizontal scroll then truncate to fit column
		lc := row.LeftContent
		rc := row.RightContent
		if m.hScrollOffset > 0 {
			if m.hScrollOffset < len(lc) {
				lc = lc[m.hScrollOffset:]
			} else {
				lc = ""
			}
			if m.hScrollOffset < len(rc) {
				rc = rc[m.hScrollOffset:]
			} else {
				rc = ""
			}
		}
		if len(lc) > maxContentWidth {
			lc = lc[:maxContentWidth-3] + "..."
		}
		if len(rc) > maxContentWidth {
			rc = rc[:maxContentWidth-3] + "..."
		}

		var leftSide, rightSide string
		switch row.Type {
		case differ.SBSEqual:
			leftSide = equalLineStyle.Width(sideWidth).Render(fmt.Sprintf("%s%s %s", cursorStr, leftNumStr, lc))
			rightSide = equalLineStyle.Width(sideWidth).Render(fmt.Sprintf("  %s %s", rightNumStr, rc))
		case differ.SBSDelete:
			leftSide = deleteLineStyle.Width(sideWidth).Render(fmt.Sprintf("%s%s %s", cursorStr, leftNumStr, lc))
			rightSide = emptyDiffStyle.Width(sideWidth).Render(fmt.Sprintf("  %s", rightNumStr))
		case differ.SBSInsert:
			leftSide = emptyDiffStyle.Width(sideWidth).Render(fmt.Sprintf("%s%s", cursorStr, leftNumStr))
			rightSide = insertLineStyle.Width(sideWidth).Render(fmt.Sprintf("  %s %s", rightNumStr, rc))
		case differ.SBSModified:
			leftSide = deleteLineStyle.Width(sideWidth).Render(fmt.Sprintf("%s%s %s", cursorStr, leftNumStr, lc))
			rightSide = insertLineStyle.Width(sideWidth).Render(fmt.Sprintf("  %s %s", rightNumStr, rc))
		}

		b.WriteString(leftSide + " │ " + rightSide)
		b.WriteString("\n")
	}

	if len(m.sbsRows) > maxVisible {
		var scrollInfo string
		if m.windowWidth > 50 {
			scrollInfo = fmt.Sprintf("Showing %d-%d of %d lines", start+1, end, len(m.sbsRows))
		} else {
			scrollInfo = fmt.Sprintf("%d-%d of %d", start+1, end, len(m.sbsRows))
		}
		b.WriteString(helpStyle.Width(m.windowWidth).Render(scrollInfo))
		b.WriteString("\n")
	}

	return b.String()
}

func (m *Model) renderMergeContent() string {
	if m.currentDiff == nil || len(m.currentDiff.Lines) == 0 {
		return "No differences found"
	}

	var b strings.Builder
	maxVisible := m.windowHeight - 12 // Account for header, stats, and help text
	if maxVisible < 5 {
		maxVisible = 5
	}

	start := m.scrollOffset
	end := start + maxVisible
	if end > len(m.currentDiff.Lines) {
		end = len(m.currentDiff.Lines)
	}

	for i := start; i < end; i++ {
		line := m.currentDiff.Lines[i]
		prefix := "  "
		cursor := "  "

		if i == m.cursor {
			cursor = "▶ "
		}

		lineNum := fmt.Sprintf("%4d", line.LineNum)
		content := line.Content

		// Truncate long lines to fit screen width
		maxContentWidth := m.windowWidth - 16 // Account for prefix, cursor, line numbers, and padding
		if maxContentWidth < 20 {
			maxContentWidth = 20
		}
		if len(content) > maxContentWidth {
			content = content[:maxContentWidth-3] + "..."
		}

		var renderedLine string
		var selected bool

		switch line.Type {
		case differ.DiffEqual:
			lineText := fmt.Sprintf("%s%s%s %s", cursor, prefix, lineNum, content)
			renderedLine = equalLineStyle.Width(m.windowWidth - 2).Render(lineText)

		case differ.DiffInsert:
			if m.changeSelection != nil {
				selected = m.changeSelection.IsInsertionSelected(i)
			}

			if selected {
				prefix = "[✓]"
				lineText := fmt.Sprintf("%s%s%s +%s", cursor, prefix, lineNum, content)
				renderedLine = selectedChangeStyle.Width(m.windowWidth - 2).Render(lineText)
			} else {
				prefix = "[ ]"
				lineText := fmt.Sprintf("%s%s%s +%s", cursor, prefix, lineNum, content)
				renderedLine = unselectedChangeStyle.Width(m.windowWidth - 2).Render(lineText)
			}

		case differ.DiffDelete:
			if m.changeSelection != nil {
				selected = m.changeSelection.IsDeletionSelected(i)
			}

			if selected {
				prefix = "[✓]"
				lineText := fmt.Sprintf("%s%s%s -%s", cursor, prefix, lineNum, content)
				renderedLine = selectedChangeStyle.Width(m.windowWidth - 2).Render(lineText)
			} else {
				prefix = "[ ]"
				lineText := fmt.Sprintf("%s%s%s -%s", cursor, prefix, lineNum, content)
				renderedLine = unselectedChangeStyle.Width(m.windowWidth - 2).Render(lineText)
			}
		}

		b.WriteString(renderedLine)
		b.WriteString("\n")
	}

	// Show scroll indicator if needed
	if len(m.currentDiff.Lines) > maxVisible {
		var scrollInfo string
		if m.windowWidth > 50 {
			scrollInfo = fmt.Sprintf("Showing %d-%d of %d lines", start+1, end, len(m.currentDiff.Lines))
		} else {
			scrollInfo = fmt.Sprintf("%d-%d of %d", start+1, end, len(m.currentDiff.Lines))
		}
		b.WriteString(helpStyle.Width(m.windowWidth).Render(scrollInfo))
		b.WriteString("\n")
	}

	return b.String()
}
