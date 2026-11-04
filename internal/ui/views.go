package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"katcompare/internal/config"
	"katcompare/internal/differ"
	"katcompare/internal/file"
	"katcompare/internal/merge"

	"github.com/charmbracelet/lipgloss"
)

// ---- Browse: the file list beside the diff, merge or conflict pane ----------

// renderBrowseView draws the two panes, or just the focused one when narrow.
func (m *Model) renderBrowseView() string {
	h := m.bodyHeight()
	filesFocused := m.viewMode == ViewModeFileSelect

	var body string
	if m.wide() {
		files := box(m.filesPaneTitle(), m.renderFileRows(m.filesPaneWidth()-2), m.filesPaneWidth(), h, filesFocused)
		title, lines := m.renderRightPane(m.rightPaneWidth()-2, m.paneInnerHeight())
		right := box(title, lines, m.rightPaneWidth(), h, !filesFocused)
		body = lipgloss.JoinHorizontal(lipgloss.Top, files, right)
	} else if filesFocused {
		body = box(m.filesPaneTitle(), m.renderFileRows(m.windowWidth-2), m.windowWidth, h, true)
	} else {
		title, lines := m.renderRightPane(m.windowWidth-2, m.paneInnerHeight())
		body = box(title, lines, m.windowWidth, h, true)
	}
	return m.frame(body, m.browseKeys())
}

func (m *Model) browseKeys() string {
	w := m.windowWidth
	help := key{"?", "help"}
	switch m.viewMode {
	case ViewModeMerge:
		return renderKeys(w, key{"Space", "line"}, key{"Enter", "block"}, key{"]/[", "change"}, key{"t", "target"},
			key{"a/n", "all/none"}, key{"s", "save"}, key{"r", "reload"}, key{"Esc", "back"}, key{"h/l", "scroll"}, help)
	case ViewModeConflict:
		return renderKeys(w, key{"j/k", "conflict"}, key{"o/t", "ours/theirs"}, key{"b/B", "both"}, key{"s", "save"},
			key{"A", "save & mark resolved"}, key{"u/U", "undo/all"}, key{"O/T", "all ours/theirs"},
			key{"Esc", "back"}, key{"e", "edit"}, key{"R", "merge tool"}, help)
	case ViewModeDiff:
		return renderKeys(w, key{"j/k", "line"}, key{"]/[", "change"}, key{"n/p", "file"}, key{"s", "view"}, key{"m", "merge"},
			key{"Tab", "files"}, key{"e", "edit"}, key{"D", "diff tool"}, key{"h/l", "scroll"}, key{"r", "reload"}, help)
	}
	if m.filterTyping {
		return renderKeys(w, key{"Type", "to filter"}, key{"↑↓", "move"}, key{"Enter", "keep filter"}, key{"Esc", "clear"}, help)
	}
	keys := []key{{"j/k", "move"}, {"Enter", "open diff"}, {"m", "merge"}, {"/", "filter"}}
	if m.filterQuery != "" {
		keys = append(keys, key{"Esc", "clear filter"})
	}
	keys = append(keys, m.comparisonKeys()...)
	return renderKeys(w, append(keys, key{"o", "paths"}, key{"Ctrl+G", "git"}, key{"q", "quit"}, help)...)
}

// comparisonKeys returns the whole-comparison keys that can work right now.
func (m *Model) comparisonKeys() []key {
	var keys []key
	if !m.gitMode && m.hasUniqueFiles() {
		keys = append(keys, key{"c", "copy"})
	}
	keys = append(keys, key{"r", "reload"})
	if !m.gitMode || m.rightRef != "" {
		keys = append(keys, key{"x", "swap"})
	}
	if !m.gitMode && m.leftFile != nil && m.rightFile != nil && m.leftFile.IsDir && m.rightFile.IsDir {
		keys = append(keys, key{"F", "folder diff"})
	}
	return keys
}

// ---- File list --------------------------------------------------------------

func (m *Model) filesPaneTitle() string {
	files := m.getSortedFiles()
	for i, f := range files {
		if f == m.selectedFile {
			return fmt.Sprintf("Files %d/%d", i+1, len(files))
		}
	}
	return "Files"
}

// renderFileRows draws the file list's rows, keeping the selection in view.
func (m *Model) renderFileRows(width int) []string {
	var rows []string
	height := m.paneInnerHeight()
	if m.filterTyping || m.filterQuery != "" {
		cursor := ""
		if m.filterTyping {
			cursor = "│"
		}
		rows = append(rows, truncatePath(" / "+sanitizeLine(m.filterQuery)+cursor, width))
		height = max(1, height-1)
	}

	files := m.getSortedFiles()
	if len(files) == 0 {
		if m.filterQuery != "" {
			return append(rows, helpStyle.Render(" No files match"))
		}
		if m.hiddenIdentical > 0 {
			return append(rows, helpStyle.Render(fmt.Sprintf(" All %s identical", plural(m.hiddenIdentical, "file"))))
		}
		return append(rows, helpStyle.Render(" No text files found"))
	}

	selected := 0
	for i, f := range files {
		if f == m.selectedFile {
			selected = i
			break
		}
	}
	m.fileListScroll = scrollFor(m.fileListScroll, selected, height, len(files))
	end := min(m.fileListScroll+height, len(files))
	for i := m.fileListScroll; i < end; i++ {
		rows = append(rows, m.renderFileRow(files[i], width, i == selected))
	}
	return rows
}

// fileBadge returns the row's icon and status, in short and full forms.
func (m *Model) fileBadge(relPath string) (icon string, style lipgloss.Style, short, full string) {
	comp := m.allFiles[relPath]
	if m.conflicts[relPath] {
		return "⚠", conflictStateStyle, "conflict", "conflict"
	}
	if rec := m.statuses[relPath]; rec != nil {
		if rec.State() == merge.StateSafe {
			return "◆", statusStyle, "safe", "merged · safe"
		}
		return "▲", warnStyle, "check", "merged · check"
	}
	switch comp.Source {
	case file.SourceLeft:
		return "◄", leftOnlyStyle, "left", "left only"
	case file.SourceRight:
		return "►", rightOnlyStyle, "right", "right only"
	}
	if m.identical[relPath] {
		return "✓", identicalStyle, "", ""
	}
	return "✗", differentStyle, "", ""
}

// fileSizes describes a row's size(s).
func fileSizes(comp *file.FileComparison) string {
	switch comp.Source {
	case file.SourceLeft:
		return formatFileSize(comp.LeftFile.Size)
	case file.SourceRight:
		return formatFileSize(comp.RightFile.Size)
	}
	if comp.LeftFile.Size == comp.RightFile.Size {
		return formatFileSize(comp.LeftFile.Size)
	}
	return formatFileSize(comp.LeftFile.Size) + " → " + formatFileSize(comp.RightFile.Size)
}

// renderFileRow draws "▶ ✗ name   size  badge", dropping the size and then the
// long form of the badge as space runs out.
func (m *Model) renderFileRow(relPath string, width int, selected bool) string {
	icon, style, short, full := m.fileBadge(relPath)
	right := short
	if width >= 44 {
		right = full
		if width >= 56 {
			size := fileSizes(m.allFiles[relPath])
			if right == "" {
				right = size
			} else {
				right = fmt.Sprintf("%s  %s", size, right)
			}
		}
	}

	nameWidth := width - 6 // marker, icon, spaces around them and at the end
	if right != "" {
		nameWidth -= lipgloss.Width(right) + 1
	}
	name := fitCell(truncatePath(sanitizeLine(m.displayNameForFile(relPath)), max(1, nameWidth)), max(1, nameWidth))
	if right != "" {
		right = " " + right
	}

	if selected {
		rowStyle := selectedDimStyle
		if m.viewMode == ViewModeFileSelect {
			rowStyle = selectedFileStyle
		}
		return rowStyle.Render(fitCell(" ▶ "+icon+" "+name+right+" ", width))
	}
	return "   " + style.Render(icon) + " " + name + style.Render(right) + " "
}

// formatFileSize formats file size in human readable format
func formatFileSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%dB", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(size)/1024)
	} else if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1fMB", float64(size)/(1024*1024))
	}
	return fmt.Sprintf("%.1fGB", float64(size)/(1024*1024*1024))
}

// ---- Right pane -------------------------------------------------------------

// renderRightPane returns the title and lines of the diff, merge or conflict
// pane, for an inner size of w by h.
func (m *Model) renderRightPane(w, h int) (string, []string) {
	switch m.viewMode {
	case ViewModeMerge:
		return m.renderMergePane(w)
	case ViewModeConflict:
		return m.renderConflictPane(w, h)
	}
	return m.renderDiffPane(w)
}

// renderStatusBanner is the selected file's last save this session, or "".
func (m *Model) renderStatusBanner(w int) string {
	rec := m.statuses[m.selectedFile]
	if rec == nil {
		return ""
	}
	backup := filepath.Base(rec.Report.Backup)
	var details []string
	title, style := "◆ Merged safely to "+rec.Target, statusStyle
	if rec.State() == merge.StateSafe {
		if rec.Kind == merge.KindMerge {
			details = append(details, fmt.Sprintf("%d applied, %d skipped", rec.Applied, rec.Skipped))
		} else {
			details = append(details, "no conflicts left")
		}
	} else {
		title, style = "▲ Check this merge to "+rec.Target, warnStyle
		details = append(details, rec.Report.Issues...)
	}
	details = append(details, backup+" kept")
	if rec.State() == merge.StateSafe && m.conflicts[m.selectedFile] {
		details = append(details, "A stages it")
	}
	return style.Render(truncate(sanitizeLine(" "+title+"  "+strings.Join(details, " · ")), w))
}

func (m *Model) renderDiffPane(w int) (string, []string) {
	if m.selectedFile == "" {
		return "Diff", []string{helpStyle.Render(" Select a file to see its differences")}
	}
	name := sanitizeLine(m.displayNameForFile(m.selectedFile))
	if m.currentDiff == nil || m.diffFile != m.selectedFile {
		return name, []string{helpStyle.Render(" Loading…")}
	}

	_, inserted, deleted := m.currentDiff.GetStats()
	mode := "Unified"
	if m.diffViewMode == DiffViewSideBySide {
		mode = "Side-by-side"
	}
	title := fmt.Sprintf("%s · %s · +%d -%d", name, mode, inserted, deleted)
	if inserted == 0 && deleted == 0 {
		title = fmt.Sprintf("%s · %s · identical", name, mode)
	}
	if m.conflicts[m.selectedFile] {
		title = fmt.Sprintf("%s · CONFLICT: m resolves here, R uses your merge tool", name)
	}
	if m.hScrollOffset > 0 {
		title += fmt.Sprintf(" · →%d", m.hScrollOffset)
	}
	title += m.ignoringNote()

	var lines []string
	if banner := m.renderStatusBanner(w); banner != "" {
		lines = append(lines, banner)
	}
	if m.diffViewMode == DiffViewSideBySide {
		lines = append(lines, m.renderSideBySideRows(w, m.diffViewHeight())...)
	} else {
		lines = append(lines, m.renderUnifiedRows(w, m.diffViewHeight())...)
	}
	return title, lines
}

// ignoringNote says what the diff is ignoring, so that a file showing as the
// same can't be mistaken for one with identical bytes.
func (m *Model) ignoringNote() string {
	if ignoring := m.diffOpts.Describe(); ignoring != "" {
		return " · ignoring " + ignoring
	}
	return ""
}

// cursorMark is the marker for a row, hidden while the file list has focus.
func (m *Model) cursorMark(i int) string {
	if i == m.cursor && m.viewMode != ViewModeFileSelect {
		return "▶ "
	}
	return "  "
}

func (m *Model) renderUnifiedRows(w, height int) []string {
	diffLines := m.currentDiff.Lines
	if len(diffLines) == 0 {
		return []string{helpStyle.Render(" Both files are empty")}
	}
	m.scrollOffset = scrollFor(m.scrollOffset, m.cursor, height, len(diffLines))
	end := min(m.scrollOffset+height, len(diffLines))

	var rows []string
	for i := m.scrollOffset; i < end; i++ {
		line := diffLines[i]
		content := scrollRight(sanitizeLine(line.Content), m.hScrollOffset)
		sign, style := " ", equalLineStyle
		switch line.Type {
		case differ.DiffInsert:
			sign, style = "+", insertLineStyle
		case differ.DiffDelete:
			sign, style = "-", deleteLineStyle
		}
		rows = append(rows, style.Render(fitCell(fmt.Sprintf("%s%4d %s %s", m.cursorMark(i), line.LineNum, sign, content), w)))
	}
	return rows
}

func (m *Model) renderSideBySideRows(w, height int) []string {
	if len(m.sbsRows) == 0 {
		return []string{helpStyle.Render(" Both files are empty")}
	}
	m.scrollOffset = scrollFor(m.scrollOffset, m.cursor, height, len(m.sbsRows))
	end := min(m.scrollOffset+height, len(m.sbsRows))

	leftWidth := (w - 1) / 2
	rightWidth := w - 1 - leftWidth
	lineNum := func(n int) string {
		if n > 0 {
			return fmt.Sprintf("%4d", n)
		}
		return "    "
	}

	var rows []string
	for i := m.scrollOffset; i < end; i++ {
		row := m.sbsRows[i]
		lc := scrollRight(sanitizeLine(row.LeftContent), m.hScrollOffset)
		rc := scrollRight(sanitizeLine(row.RightContent), m.hScrollOffset)
		left := fitCell(fmt.Sprintf("%s%s %s", m.cursorMark(i), lineNum(row.LeftLineNum), lc), leftWidth)
		right := fitCell(fmt.Sprintf("%s %s", lineNum(row.RightLineNum), rc), rightWidth)

		leftStyle, rightStyle := equalLineStyle, equalLineStyle
		switch row.Type {
		case differ.SBSDelete:
			leftStyle, rightStyle = deleteLineStyle, emptyDiffStyle
		case differ.SBSInsert:
			leftStyle, rightStyle = emptyDiffStyle, insertLineStyle
		case differ.SBSModified:
			leftStyle, rightStyle = deleteLineStyle, insertLineStyle
		}
		rows = append(rows, leftStyle.Render(left)+paneBorderStyle.Render("│")+rightStyle.Render(right))
	}
	return rows
}

// ---- Merge pane -------------------------------------------------------------

// saveEffect says what saving the checked changes does to the target. A
// checked change moves the target towards the other side, so a checked
// insertion adds a line to the left file but removes one from the right file.
func saveEffect(selIns, selDel int, target, name string) string {
	adds, removes := selDel, selIns
	if target == "left" {
		adds, removes = selIns, selDel
	}
	switch {
	case adds > 0 && removes > 0:
		return fmt.Sprintf("saving adds %s and removes %s in %s", plural(adds, "line"), plural(removes, "line"), name)
	case adds > 0:
		return fmt.Sprintf("saving adds %s to %s", plural(adds, "line"), name)
	}
	return fmt.Sprintf("saving removes %s from %s", plural(removes, "line"), name)
}

func (m *Model) renderMergePane(w int) (string, []string) {
	name := sanitizeLine(m.displayNameForFile(m.selectedFile))
	targetName := sanitizeLine(filepath.Base(m.editablePath(m.mergeTarget)))
	title := "Merge · " + name + m.ignoringNote()

	var lines []string
	if banner := m.renderStatusBanner(w); banner != "" {
		lines = append(lines, banner)
	}

	selIns, totIns, selDel, totDel := 0, 0, 0, 0
	if m.changeSelection != nil {
		selIns, totIns, selDel, totDel = m.changeSelection.GetSelectedStats(m.currentDiff)
	}
	var state string
	switch {
	case m.targetDisk == merge.DiskMissing:
		state = warnStyle.Render(" ▲ " + targetName + " no longer exists · r reloads")
	case m.targetDisk == merge.DiskChanged:
		state = warnStyle.Render(" ▲ Changed on disk · r reloads, s overwrites")
	case selIns+selDel > 0:
		state = unsavedStyle.Render(" ○ Unsaved · " + saveEffect(selIns, selDel, m.mergeTarget, targetName))
	default:
		state = statusStyle.Render(" ● Saved · matches disk")
	}
	lines = append(lines, state)
	lines = append(lines, keyActionStyle.Render(fmt.Sprintf(" Target: %s (%s) · insertions %d/%d · deletions %d/%d",
		strings.ToUpper(m.mergeTarget), targetName, selIns, totIns, selDel, totDel)))

	return title, append(lines, m.renderMergeRows(w, m.mergeViewHeight())...)
}

func (m *Model) renderMergeRows(w, height int) []string {
	diffLines := m.currentDiff.Lines
	m.scrollOffset = scrollFor(m.scrollOffset, m.cursor, height, len(diffLines))
	end := min(m.scrollOffset+height, len(diffLines))

	var rows []string
	for i := m.scrollOffset; i < end; i++ {
		line := diffLines[i]
		content := scrollRight(sanitizeLine(line.Content), m.hScrollOffset)
		check, sign, style := "   ", " ", equalLineStyle
		if line.Type != differ.DiffEqual {
			var selected bool
			unticked := unselectedDeleteStyle
			if line.Type == differ.DiffInsert {
				sign, style, unticked = "+", insertLineStyle, unselectedInsertStyle
				selected = m.changeSelection != nil && m.changeSelection.IsInsertionSelected(i)
			} else {
				sign, style = "-", deleteLineStyle
				selected = m.changeSelection != nil && m.changeSelection.IsDeletionSelected(i)
			}
			check = "[✓]"
			if !selected {
				check, style = "[ ]", unticked
			}
		}
		rows = append(rows, style.Render(fitCell(fmt.Sprintf("%s%s%4d %s %s", m.cursorMark(i), check, line.LineNum, sign, content), w)))
	}
	return rows
}

// ---- Copy -------------------------------------------------------------------

func (m *Model) renderCopyView() string {
	uniqueFiles := m.getUniqueFiles()
	w, height := m.windowWidth, m.paneInnerHeight()

	selected, eligible := 0, 0
	for _, relPath := range uniqueFiles {
		if m.canCopy(relPath) {
			eligible++
			if m.copySelection[relPath] {
				selected++
			}
		}
	}
	direction := "left → right"
	if m.copyTarget == "to-left" {
		direction = "right → left"
	}
	title := fmt.Sprintf("Copy %s · %d of %s selected", direction, selected, plural(eligible, "file"))

	m.scrollOffset = scrollFor(m.scrollOffset, m.cursor, height, len(uniqueFiles))
	end := min(m.scrollOffset+height, len(uniqueFiles))
	var rows []string
	for i := m.scrollOffset; i < end; i++ {
		relPath := uniqueFiles[i]
		comp := m.allFiles[relPath]
		cursor := "  "
		if i == m.cursor {
			cursor = "▶ "
		}
		icon, source, info := "◄", "left only", comp.LeftFile
		if comp.Source == file.SourceRight {
			icon, source, info = "►", "right only", comp.RightFile
		}
		check := "[-]"
		if m.canCopy(relPath) {
			check = "[ ]"
			if m.copySelection[relPath] {
				check = "[✓]"
			}
		}
		suffix := fmt.Sprintf(" %s  %s", formatFileSize(info.Size), source)
		nameWidth := max(1, w-2-len(cursor)-4-2-lipgloss.Width(suffix))
		row := fmt.Sprintf("%s%s %s %s%s", cursor, check, icon, fitCell(truncatePath(sanitizeLine(relPath), nameWidth), nameWidth), suffix)
		switch {
		case !m.canCopy(relPath):
			row = helpStyle.Render(row)
		case m.copySelection[relPath]:
			row = statusStyle.Render(row)
		}
		rows = append(rows, row)
	}

	return m.frame(box(title, rows, w, m.bodyHeight(), true), renderKeys(w,
		key{"Space", "toggle"}, key{"t", "direction"}, key{"a/n", "all/none"}, key{"s", "copy"}, key{"Esc", "back"}, key{"?", "help"}))
}

// ---- Open screen ------------------------------------------------------------

func (m *Model) renderOpenView() string {
	w := m.windowWidth
	inputWidth := max(10, w-14)
	var lines []string

	field := func(label, value string, focused bool) string {
		if focused {
			return label + focusedInputStyle.Render(fitCell(" "+truncatePath(sanitizeLine(value)+"│", inputWidth-2)+" ", inputWidth))
		}
		return label + inputStyle.Render(fitCell(" "+truncatePath(sanitizeLine(value), inputWidth-2)+" ", inputWidth))
	}
	suggestions := func(items []string, selected int) {
		if !m.showSuggestions || len(items) == 0 {
			return
		}
		lines = append(lines, m.renderSuggestions(items, selected)...)
	}

	lines = append(lines, "", keyActionStyle.Render(" Choose two files or folders to compare."), "")
	lines = append(lines, field(" Left    ", m.inputLeft, m.focus == focusInputLeft))
	if m.focus == focusInputLeft {
		suggestions(m.leftSuggestions, m.leftSuggIndex)
	}
	lines = append(lines, field(" Right   ", m.inputRight, m.focus == focusInputRight))
	if m.focus == focusInputRight {
		suggestions(m.rightSuggestions, m.rightSuggIndex)
	}
	lines = append(lines, "")

	button := func(label string, focused bool) string {
		if focused {
			return focusedButtonStyle.Render(label)
		}
		return buttonStyle.Render(label)
	}
	lines = append(lines, "          "+button("Compare", m.focus == focusViewDiffBtn)+"  "+button("Quit", m.focus == focusQuitBtn))
	lines = append(lines, "", keyActionStyle.Render(" Or press Ctrl+G to compare git branches and commits."))

	keys := []key{{"Tab", "next"}, {"Enter", "accept / compare"}, {"↑↓", "suggestions"}, {"Ctrl+G", "git"}}
	if len(m.allFiles) > 0 {
		keys = append(keys, key{"Esc", "back"})
	} else {
		keys = append(keys, key{"Esc", "close list"})
	}
	keys = append(keys, key{"Ctrl+C", "quit"}, key{"F1", "help"})
	return m.frame(strings.Join(lines, "\n"), renderKeys(w, keys...))
}

// renderSuggestions returns the rows of the path suggestion list.
func (m *Model) renderSuggestions(suggestions []string, selected int) []string {
	w := m.windowWidth
	room := max(1, min(6, m.bodyHeight()-12))
	start := 0
	if selected >= room {
		start = selected - room + 1
	}
	end := min(start+room, len(suggestions))

	var rows []string
	for i := start; i < end; i++ {
		text := truncatePath(sanitizeLine(suggestions[i]), max(1, w-14))
		if i == selected {
			rows = append(rows, "          "+selectedSuggestionStyle.Render(fitCell("→ "+text, max(1, w-12))))
		} else {
			rows = append(rows, "            "+text)
		}
	}
	if hidden := len(suggestions) - end + start; hidden > 0 {
		rows = append(rows, helpStyle.Render(fmt.Sprintf("            … %d more", hidden)))
	}
	return rows
}

// ---- Help -------------------------------------------------------------------

func (m *Model) renderHelpView() string {
	lines := m.helpLines()
	start := min(m.helpScroll, m.helpMaxScroll())
	end := min(start+m.helpPageHeight(), len(lines))
	return m.frame(strings.Join(lines[start:end], "\n"), renderKeys(m.windowWidth,
		key{"j/k", "scroll"}, key{"PgUp/PgDn", "page"}, key{"g/G", "top/bottom"},
		key{fmt.Sprintf("%d-%d of %d", start+1, end, len(lines)), ""}, key{"Esc", "back"}))
}

// helpPageHeight is the number of help lines that fit on screen.
func (m *Model) helpPageHeight() int { return m.bodyHeight() }

// helpMaxScroll is the largest useful help scroll offset.
func (m *Model) helpMaxScroll() int {
	return max(0, len(m.helpLines())-m.helpPageHeight())
}

// helpLines returns the help body split into lines.
func (m *Model) helpLines() []string {
	return strings.Split(m.renderHelpBody(), "\n")
}

const helpText = `Getting started
  katcmp left right      Compare two files or folders
  katcmp --git           Compare HEAD with the working tree
  katcmp                 Choose paths on the open screen

  The file list and the diff sit side by side at 100 columns or wider.
  In a narrower terminal, one shows at a time with the same keys.

File list
  j/k or ↑/↓             Move; the diff follows the selection
  g/G, PgUp/PgDn         Top/bottom, page up/down
  Enter, Tab or Ctrl+D   Open the diff, on its first change
  /                      Filter by name (Enter keeps it, Esc clears it)
  m                      Merge the selected file
  c                      Copy files that exist on only one side
  r                      Reload from disk
  x                      Swap left and right
  F                      Open both folders in your diff tool
  o                      Change the paths
  Ctrl+G                 Compare git branches and commits
  q                      Quit

Diff
  j/k or ↑/↓             Move a line
  g/G, PgUp/PgDn         Top/bottom, page up/down
  ] / [                  Next/previous change
  h/l or ←/→             Scroll sideways
  s                      Switch between unified and side-by-side
  n / p                  Next/previous file
  Tab or Esc             Back to the file list
  m                      Merge (on a git conflict: resolve it)
  e / E                  Edit the right / left file at this line
  D                      Open the file pair in your diff tool
  R                      Resolve a git conflict in your merge tool
  r / x / c              Reload / swap / copy

Merge
  Tick the changes to apply, choose a target and save over that file. A ticked
  change makes the target match the other side, so with target RIGHT a ticked
  insertion is removed from the right file. The status line says what saving
  will do.

  Space                  Tick the change on this line
  Enter                  Tick the whole block
  ] / [                  Next/previous change
  t                      Switch the target between left and right
  a / n                  Tick all / none
  s                      Save (the old version is kept as .orig)
  r                      Reload from disk
  h/l or ←/→             Scroll sideways
  Esc                    Back to the diff

  ● Saved                Nothing is ticked and the file matches this view
  ○ Unsaved              Ticked changes aren't in the file yet
  ▲ Changed on disk      The file changed after it was loaded. r reloads;
                         s warns, then a second s replaces those edits

Conflicts (m on a git conflict)
  j/k or ] / [           Move between conflicts
  o / t                  Take ours / theirs
  b / B                  Take both, ours / theirs first
  u / U                  Undo this choice / every choice
  O / T                  Take ours / theirs for every conflict
  s                      Save the choices; the rest keep their markers
  A                      Save and mark resolved (git add)
  e / R                  Edit the file / use your merge tool
  Esc                    Back to the diff

Copy (folders only)
  Space or Enter         Tick a file
  t                      Switch the direction
  a / n                  Tick all / none
  s                      Copy the ticked files
  Esc                    Back to the file list

Open screen
  Type or paste          Paths are suggested as you type
  ↑/↓ or Tab             Choose a suggestion
  Enter                  Accept it, then move on and compare
  Shift+Tab              Previous field
  Ctrl+G                 Compare git branches and commits
  F1                     Help (? is an ordinary character here)
  Ctrl+C                 Quit

Git picker (Ctrl+G)
  Type                   Enter any ref, or filter the list
  ↑/↓, PgUp/PgDn         Choose from branches, tags and recent commits
  Enter                  Accept the choice / compare when none is chosen
  Tab                    Switch between the left and right ref
  Ctrl+U                 Clear the ref (right blank = working tree)
  Esc                    Clear the choice, then go back

File list symbols
`

const helpTextEnd = `
Merge status
  After each save, katcmp checks that the file on disk matches the result when
  read back, that the .orig backup matches the previous version, and that no
  <<<<<<< or >>>>>>> lines remain. The status lasts until you quit, and clears
  if the file changes. "Safe" doesn't mean the two sides now match, so unticked
  changes only show as "skipped". Only the latest backup is kept, so a second
  save overwrites it.

External tools
  Editor                 $KATCMP_EDITOR, then $VISUAL, then $EDITOR, then the
                         OS default (terminal editors are preferred here)
  Diff/merge tool        git config diff.tool / merge.tool (e.g. nvimdiff, meld)
                         Terminal tools are preferred here
  As a diff tool (WSL)   Other programs should run: katcmp-win --wait
                         (see katcmp-win --help)

Tool choices
  These are read from git config:
  Global                 ~/.gitconfig or ~/.config/git/config
                         (Windows: %USERPROFILE%\.gitconfig)
  Per repository         <repo>/.git/config (overrides global)
  Editor variables       Set in your shell profile (~/.bashrc, ~/.zshrc, ...)

Only text files are listed; other files are detected and skipped.`

// renderHelpBody renders the scrollable part of the help screen.
func (m *Model) renderHelpBody() string {
	var b strings.Builder
	b.WriteString(helpText)
	for _, row := range []struct {
		style lipgloss.Style
		icon  string
		text  string
	}{
		{identicalStyle, "✓", "Same on both sides"},
		{differentStyle, "✗", "Different"},
		{leftOnlyStyle, "◄", "Only on the left"},
		{rightOnlyStyle, "►", "Only on the right"},
		{conflictStateStyle, "⚠", "Has unresolved git merge conflicts"},
		{statusStyle, "◆", "Saved this session and passed its checks (merged · safe)"},
		{warnStyle, "▲", "Saved this session, but a check failed (merged · check)"},
	} {
		b.WriteString("  " + row.style.Render(row.icon) + " " + row.text + "\n")
	}
	b.WriteString("\nDiff colors\n")
	b.WriteString("  " + insertLineStyle.Render(" + Added line ") + "  " + deleteLineStyle.Render(" - Deleted line ") + "  " + unselectedInsertStyle.Render(" Unticked + ") + " " + unselectedDeleteStyle.Render(" Unticked - ") + "\n")
	b.WriteString(helpTextEnd)
	b.WriteString("\n\nSettings file\n")
	if path, err := config.Path(); err == nil {
		b.WriteString("  " + path + "\n")
	}
	b.WriteString("  Created the first time the app runs, and never changed after that. Edits\n")
	b.WriteString("  apply on your next comparison. It sets files to skip, what to ignore when\n")
	b.WriteString("  comparing, your editor, and the view and merge target to start with. The\n")
	b.WriteString("  file's comments explain each one, and the README lists them.")
	return b.String()
}
