package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"katcompare/internal/conflict"
	"katcompare/internal/external"
	"katcompare/internal/git"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// The conflict view (ViewModeConflict) resolves git conflict markers in the
// selected working-tree file block by block: ours, theirs or both.

var (
	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#34D399")).
			Bold(true)
	conflictOursStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171"))
	conflictTheirsStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#60A5FA"))
	conflictResultStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399"))
	conflictLabelStyle  = lipgloss.NewStyle().Bold(true)
	conflictStateStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700")).Bold(true)
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
	info, err := os.Stat(path)
	if err != nil {
		m.errorMsg = err.Error()
		return
	}
	if err := os.WriteFile(path, []byte(m.conflictFile.Render(m.conflictChoices)), info.Mode().Perm()); err != nil {
		m.errorMsg = "Failed to save: " + err.Error()
		return
	}
	if mark {
		if err := git.MarkResolved(m.gitRoot, m.selectedFile); err != nil {
			m.errorMsg = err.Error()
			return
		}
		m.viewMode = ViewModeDiff
		m.reloadComparison()
		m.statusMsg = fmt.Sprintf("Resolved %s and staged it (git add)", m.selectedFile)
		return
	}
	m.reloadComparison()
	if err := m.loadConflicts(); err != nil {
		m.errorMsg = err.Error()
		return
	}
	if remaining > 0 {
		m.statusMsg = fmt.Sprintf("Saved — %d conflict(s) left", remaining)
	} else {
		m.statusMsg = "Saved — no conflicts left; press A to mark it resolved"
	}
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
	m.errorMsg, m.statusMsg = "", ""
	n := len(m.conflictChoices)
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		m.viewMode = ViewModeDiff
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
	case "?":
		m.viewMode = ViewModeHelp
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

func (m *Model) renderConflictView() string {
	var b strings.Builder
	f := m.conflictFile
	n := len(f.Blocks)
	resolved := n - conflict.Remaining(n, m.conflictChoices)

	header := fmt.Sprintf("Resolve conflicts — %s  [%d/%d resolved]", m.selectedFile, resolved, n)
	b.WriteString(mergeHeaderStyle.Width(m.windowWidth).Render(header))
	b.WriteString("\n\n")
	used := 3

	if m.errorMsg != "" {
		b.WriteString(errorStyle.Render(m.errorMsg) + "\n\n")
		used += 2
	} else if m.statusMsg != "" {
		b.WriteString(statusStyle.Render(m.statusMsg) + "\n\n")
		used += 2
	}

	if n == 0 {
		b.WriteString("No conflict markers left in this file, but git still lists it as conflicted.\n")
		b.WriteString("Press A to mark it resolved (git add), or Esc to go back.\n\n")
	} else {
		// Block list (a window around the focused block)
		listMax := min(n, 6)
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
			line := fmt.Sprintf("%s#%d  line %d  ours %d / theirs %d lines  ", cursor, i+1, blk.StartLine, len(blk.Ours), len(blk.Theirs))
			b.WriteString(line + state + "\n")
		}
		b.WriteString("\n")
		used += listMax + 1

		// Focused block in detail
		avail := max(6, m.windowHeight-used-3)
		b.WriteString(m.renderConflictDetail(m.conflictIdx, avail))
	}

	b.WriteString("\n")
	var help string
	switch {
	case m.windowWidth > 140:
		help = "j/k ]/[: Conflict • o: Ours • t: Theirs • b/B: Both • u: Undo • O/T: All ours/theirs • s: Save • A: Save & mark resolved • e: Edit • R: Merge tool • Esc: Back"
	case m.windowWidth > 60:
		help = "j/k: Move • o/t/b/B: Ours/Theirs/Both • u: Undo • s: Save • A: Resolve • Esc: Back"
	default:
		help = "o/t/b:Pick u:Undo s:Save A:Resolve Esc:Back"
	}
	b.WriteString(helpStyle.Width(m.windowWidth).Render(help))
	return b.String()
}

// renderConflictDetail shows block i: context, then either both sides or the
// chosen result, within at most maxLines lines.
func (m *Model) renderConflictDetail(i, maxLines int) string {
	f := m.conflictFile
	blk := f.Blocks[i]
	width := max(20, m.windowWidth-4)
	var out []string
	add := func(style lipgloss.Style, prefix string, lines []string) {
		for _, l := range lines {
			l = strings.ReplaceAll(l, "\t", "    ")
			if len(l) > width-2 {
				l = l[:width-5] + "..."
			}
			out = append(out, style.Render(prefix+l))
		}
	}
	before, after := f.Context(i, 2)
	add(equalLineStyle, "  ", before)
	if c := m.conflictChoices[i]; c != conflict.Unresolved {
		out = append(out, conflictLabelStyle.Render("── Result: "+choiceNames[c]+" ──"))
		add(conflictResultStyle, "  ", blk.Lines(c))
	} else {
		out = append(out, conflictLabelStyle.Render("── Ours ("+blk.OursLabel+") ──"))
		add(conflictOursStyle, "  ", blk.Ours)
		out = append(out, conflictLabelStyle.Render("── Theirs ("+blk.TheirsLabel+") ──"))
		add(conflictTheirsStyle, "  ", blk.Theirs)
	}
	add(equalLineStyle, "  ", after)
	if len(out) > maxLines {
		hidden := len(out) - maxLines + 1
		out = append(out[:maxLines-1], helpStyle.Render(fmt.Sprintf("  … %d more line(s) — press e to open in your editor", hidden)))
	}
	return strings.Join(out, "\n") + "\n"
}
