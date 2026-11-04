package ui

import (
	"fmt"
	"os"
	"path/filepath"

	"katcompare/internal/conflict"
	"katcompare/internal/external"
	"katcompare/internal/git"
	"katcompare/internal/merge"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// The conflict view (ViewModeConflict) resolves git conflict markers in the
// selected working-tree file block by block: ours, theirs or both.

var (
	statusStyle         = lipgloss.NewStyle().Foreground(greenColor).Bold(true)
	conflictOursStyle   = lipgloss.NewStyle().Foreground(redColor)
	conflictTheirsStyle = lipgloss.NewStyle().Foreground(blueColor)
	conflictResultStyle = lipgloss.NewStyle().Foreground(greenColor)
	conflictLabelStyle  = lipgloss.NewStyle().Bold(true)
	conflictStateStyle  = lipgloss.NewStyle().Foreground(yellowColor).Bold(true)
)

var choiceNames = map[conflict.Choice]string{
	conflict.Ours:           "ours",
	conflict.Theirs:         "theirs",
	conflict.OursThenTheirs: "both (ours first)",
	conflict.TheirsThenOurs: "both (theirs first)",
}

// conflictPath is the on-disk path of the selected conflicted file.
func (m *Model) conflictPath() string {
	return filepath.Join(m.gitRoot, m.selectedFile)
}

// openConflictView parses the selected file's conflict markers and shows
// the conflict view.
func (m *Model) openConflictView() {
	if err := m.loadConflicts(); err != nil {
		m.errorMsg = err.Error()
		return
	}
	m.conflictIdx = 0
	m.viewMode = ViewModeConflict
}

// loadConflicts (re)reads the selected file, dropping unsaved choices.
func (m *Model) loadConflicts() error {
	content, err := os.ReadFile(m.conflictPath())
	if err != nil {
		return err
	}
	m.conflictFile = conflict.Parse(string(content))
	m.conflictChoices = make([]conflict.Choice, len(m.conflictFile.Blocks))
	m.conflictIdx = min(m.conflictIdx, max(0, len(m.conflictChoices)-1))
	return nil
}

// saveConflicts writes the chosen resolutions to the file; with mark it also
// requires every conflict to be resolved and stages the file (git add).
func (m *Model) saveConflicts(mark bool) {
	remaining := conflict.Remaining(len(m.conflictFile.Blocks), m.conflictChoices)
	if mark && remaining > 0 {
		m.errorMsg = fmt.Sprintf("%d conflict(s) still unresolved", remaining)
		return
	}
	if !mark && remaining == len(m.conflictChoices) {
		m.errorMsg = "Nothing chosen yet — use o/t/b on a conflict first"
		return
	}
	path := m.conflictPath()
	report, err := merge.SaveVerified(path, m.conflictFile.Render(m.conflictChoices))
	if err != nil {
		m.errorMsg = "Failed to save: " + err.Error()
		return
	}
	m.tracker.Record(path, &merge.Record{Report: report, Kind: merge.KindConflicts})
	m.statusWarn = false
	issues := issueSuffix(report)
	if mark {
		if err := git.MarkResolved(m.gitRoot, m.selectedFile); err != nil {
			m.errorMsg = err.Error()
			return
		}
		m.viewMode = ViewModeDiff
		m.reloadComparison()
		m.statusMsg = fmt.Sprintf("Resolved %s and staged it (git add).%s", m.selectedFile, issues)
		m.statusWarn = issues != ""
		return
	}
	m.reloadComparison()
	if err := m.loadConflicts(); err != nil {
		m.errorMsg = err.Error()
		return
	}
	if remaining > 0 {
		m.statusMsg = fmt.Sprintf("Saved, %s left.%s", plural(remaining, "conflict"), issues)
	} else {
		m.statusMsg = "Saved, no conflicts left. Press A to mark it resolved." + issues
	}
	m.statusWarn = issues != ""
}

// chooseConflict sets the focused block's choice and, unless undoing, moves
// to the next unresolved block.
func (m *Model) chooseConflict(c conflict.Choice) {
	if len(m.conflictChoices) == 0 {
		return
	}
	m.conflictChoices[m.conflictIdx] = c
	if c == conflict.Unresolved {
		return
	}
	for i := m.conflictIdx + 1; i < len(m.conflictChoices); i++ {
		if m.conflictChoices[i] == conflict.Unresolved {
			m.conflictIdx = i
			return
		}
	}
}

func (m *Model) handleConflictKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.clearMessages()
	n := len(m.conflictChoices)
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		m.viewMode = ViewModeDiff
		m.ensureDiffLoaded()
	case "down", "j", "]":
		if m.conflictIdx < n-1 {
			m.conflictIdx++
		}
	case "up", "k", "[":
		if m.conflictIdx > 0 {
			m.conflictIdx--
		}
	case "o":
		m.chooseConflict(conflict.Ours)
	case "t":
		m.chooseConflict(conflict.Theirs)
	case "b":
		m.chooseConflict(conflict.OursThenTheirs)
	case "B":
		m.chooseConflict(conflict.TheirsThenOurs)
	case "u":
		m.chooseConflict(conflict.Unresolved)
	case "U":
		for i := range m.conflictChoices {
			m.conflictChoices[i] = conflict.Unresolved
		}
	case "O", "T":
		c := conflict.Ours
		if msg.String() == "T" {
			c = conflict.Theirs
		}
		for i := range m.conflictChoices {
			m.conflictChoices[i] = c
		}
	case "s":
		m.saveConflicts(false)
	case "A":
		m.saveConflicts(true)
	case "e":
		line := 0
		if n > 0 {
			line = m.conflictFile.Blocks[m.conflictIdx].StartLine
		}
		cmd, err := external.EditorCommand(m.conflictPath(), line, false)
		if err != nil {
			m.errorMsg = err.Error()
			return m, nil
		}
		return m, runExternal("edit", cmd)
	case "R":
		return m, m.resolveConflict()
	case "?", "f1":
		m.openHelp()
	}
	return m, nil
}

// afterExternalInConflictView refreshes the view after an editor or merge
// tool ran, leaving it when the file is no longer conflicted.
func (m *Model) afterExternalInConflictView() {
	if !m.conflicts[m.selectedFile] {
		m.viewMode = ViewModeDiff
		m.loadDiff()
		if m.errorMsg == "" {
			m.statusMsg = m.selectedFile + " is no longer conflicted"
		}
		return
	}
	if err := m.loadConflicts(); err != nil {
		m.errorMsg = err.Error()
	}
}

// renderConflictPane returns the title and lines of the conflict pane.
func (m *Model) renderConflictPane(w, h int) (string, []string) {
	f := m.conflictFile
	n := len(f.Blocks)
	resolved := n - conflict.Remaining(n, m.conflictChoices)
	title := fmt.Sprintf("Resolve conflicts · %s  [%d/%d resolved]", sanitizeLine(m.selectedFile), resolved, n)

	var lines []string
	if banner := m.renderStatusBanner(w); banner != "" {
		lines = append(lines, banner)
	}
	if n == 0 {
		return title, append(lines,
			" No conflict markers left, but git still lists this file as conflicted.",
			" Press A to mark it resolved (git add), or Esc to go back.")
	}

	// Block list (a window around the focused block)
	listMax := min(n, max(1, h/3))
	start := max(0, min(m.conflictIdx-listMax/2, n-listMax))
	for i := start; i < start+listMax; i++ {
		blk := f.Blocks[i]
		cursor := "  "
		if i == m.conflictIdx {
			cursor = "▶ "
		}
		state := conflictStateStyle.Render("unresolved")
		if c := m.conflictChoices[i]; c != conflict.Unresolved {
			state = statusStyle.Render(choiceNames[c])
		}
		lines = append(lines, fmt.Sprintf("%s#%d  line %d  ours %d / theirs %d lines  ", cursor, i+1, blk.StartLine, len(blk.Ours), len(blk.Theirs))+state)
	}
	lines = append(lines, "")

	// Focused block in detail
	return title, append(lines, m.renderConflictDetail(m.conflictIdx, w, max(3, h-len(lines)))...)
}

// renderConflictDetail shows block i: context, then either both sides or the
// chosen result, within at most maxLines lines.
func (m *Model) renderConflictDetail(i, width, maxLines int) []string {
	f := m.conflictFile
	blk := f.Blocks[i]
	var out []string
	add := func(style lipgloss.Style, prefix string, lines []string) {
		for _, l := range lines {
			out = append(out, style.Render(truncate(prefix+sanitizeLine(l), width)))
		}
	}
	label := func(text string) {
		out = append(out, conflictLabelStyle.Render(truncate(sanitizeLine(text), width)))
	}
	before, after := f.Context(i, 2)
	add(equalLineStyle, "  ", before)
	if c := m.conflictChoices[i]; c != conflict.Unresolved {
		label("── Result: " + choiceNames[c] + " ──")
		add(conflictResultStyle, "  ", blk.Lines(c))
	} else {
		label("── Ours (" + blk.OursLabel + ") ──")
		add(conflictOursStyle, "  ", blk.Ours)
		label("── Theirs (" + blk.TheirsLabel + ") ──")
		add(conflictTheirsStyle, "  ", blk.Theirs)
	}
	add(equalLineStyle, "  ", after)
	if len(out) > maxLines {
		hidden := len(out) - maxLines + 1
		out = append(out[:maxLines-1], helpStyle.Render(truncate(fmt.Sprintf("  … %d more line(s). Press e to open your editor", hidden), width)))
	}
	return out
}
