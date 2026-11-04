package ui

import (
	"fmt"
	"strings"

	"katcompare/internal/git"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// openGitSelect switches to the git ref picker, pre-filled with the current
// git comparison (or HEAD vs the working tree).
func (m *Model) openGitSelect() {
	root, err := git.FindRoot()
	if err != nil {
		m.errorMsg = err.Error()
		return
	}
	refs, err := git.ListRefs(root, true)
	if err != nil {
		m.errorMsg = err.Error()
		return
	}

	m.gitRefs = refs
	if m.gitMode {
		m.gitSelInput = [2]string{m.leftRef, m.rightRef}
	} else {
		m.gitSelInput = [2]string{"HEAD", ""}
	}
	m.gitSelFocus = 0
	m.gitSelIndex = -1
	m.gitSelEdited = false
	m.errorMsg = ""
	m.viewMode = ViewModeGitSelect
}

// gitSelOptions returns the picker options for the focused input, filtered
// by what was typed into it. The working tree is only offered on the right.
func (m *Model) gitSelOptions() []git.RefOption {
	query := ""
	if m.gitSelEdited {
		query = strings.ToLower(m.gitSelInput[m.gitSelFocus])
	}
	var opts []git.RefOption
	for _, r := range m.gitRefs {
		if r.Ref == "" && m.gitSelFocus == 0 {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(r.Label), query) {
			continue
		}
		opts = append(opts, r)
	}
	return opts
}

// handleGitSelectKeys handles keys in the git ref picker
func (m *Model) handleGitSelectKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.errorMsg = ""
	opts := m.gitSelOptions()

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		if m.gitSelIndex >= 0 {
			m.gitSelIndex = -1
		} else if len(m.allFiles) > 0 {
			m.viewMode = ViewModeFileSelect
		} else {
			m.viewMode = ViewModeOpen
		}

	case "tab", "shift+tab":
		m.gitSelFocus = 1 - m.gitSelFocus
		m.gitSelIndex = -1
		m.gitSelEdited = false

	case "up":
		m.gitSelIndex = max(-1, m.gitSelIndex-1)
	case "down":
		m.gitSelIndex = min(len(opts)-1, m.gitSelIndex+1)
	case "pgup":
		m.gitSelIndex = max(0, m.gitSelIndex-m.gitSelListHeight())
	case "pgdown":
		m.gitSelIndex = min(len(opts)-1, m.gitSelIndex+m.gitSelListHeight())

	case "enter":
		// Enter accepts the highlighted option; with nothing highlighted it compares
		if m.gitSelIndex >= 0 && m.gitSelIndex < len(opts) {
			m.gitSelInput[m.gitSelFocus] = opts[m.gitSelIndex].Ref
			m.gitSelIndex = -1
			m.gitSelEdited = false
			if m.gitSelFocus == 0 {
				m.gitSelFocus = 1
			}
			return m, nil
		}
		m.runGitSelect()

	case "ctrl+d":
		m.runGitSelect()

	case "backspace":
		m.gitSelInput[m.gitSelFocus] = dropLastRune(m.gitSelInput[m.gitSelFocus])
		m.gitSelEdited = true
		m.gitSelIndex = -1

	case "ctrl+u":
		m.gitSelInput[m.gitSelFocus] = ""
		m.gitSelEdited = true
		m.gitSelIndex = -1

	default:
		if text, ok := textOf(msg); ok {
			m.gitSelInput[m.gitSelFocus] += text
			m.gitSelEdited = true
			m.gitSelIndex = -1
		}
	}
	return m, nil
}

// runGitSelect loads the comparison chosen in the picker and returns to the
// file list, or stays in the picker showing the error.
func (m *Model) runGitSelect() {
	left := strings.TrimSpace(m.gitSelInput[0])
	if left == "" {
		left = "HEAD"
	}
	right := strings.TrimSpace(m.gitSelInput[1])
	if err := m.LoadGitComparison(left, right); err != nil {
		m.errorMsg = err.Error()
		return
	}
	m.ShowFileList()
}

// gitSelListHeight is the number of dropdown rows that fit on screen.
func (m *Model) gitSelListHeight() int {
	return max(3, m.windowHeight-12)
}

// renderGitSelectView renders the git ref picker: two ref inputs with a
// dropdown of detected refs under the focused one.
func (m *Model) renderGitSelectView() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(fitCell(" katCompare  Compare git refs", m.windowWidth)))
	b.WriteString("\n\n")

	inputWidth := max(30, m.windowWidth-20)
	labels := [2]string{"Left ref:   ", "Right ref:  "}
	for side := 0; side < 2; side++ {
		display := m.gitSelInput[side]
		if display == "" && side == 1 && m.gitSelFocus != 1 {
			display = "(working tree)"
		}
		var input string
		if m.gitSelFocus == side {
			input = focusedInputStyle.Render(fitCell(" "+truncatePath(sanitizeLine(display)+"│", inputWidth-2)+" ", inputWidth))
		} else {
			input = inputStyle.Render(fitCell(" "+truncatePath(sanitizeLine(display), inputWidth-2)+" ", inputWidth))
		}
		b.WriteString(labels[side] + input)
		b.WriteString("\n")
		if m.gitSelFocus == side {
			b.WriteString(m.renderGitSelDropdown())
			b.WriteString("\n")
		}
	}

	if m.errorMsg != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render(truncate(" "+sanitizeLine(m.errorMsg), m.windowWidth)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(renderKeys(m.windowWidth, key{"Type", "ref or filter"}, key{"↑↓", "choose"}, key{"Enter", "accept / compare"},
		key{"Tab", "side"}, key{"Ctrl+U", "clear"}, key{"Esc", "back"}))

	return b.String()
}

// renderGitSelDropdown renders the grouped, scrolled option list for the
// focused input, keeping the highlighted option in view.
func (m *Model) renderGitSelDropdown() string {
	opts := m.gitSelOptions()
	if len(opts) == 0 {
		return suggestionStyle.Render(helpStyle.Render("No matching refs - Enter compares the typed ref"))
	}

	groupStyle := lipgloss.NewStyle().Foreground(accentColor).Bold(true)
	labelWidth := max(20, m.windowWidth-12)

	// Flatten into rows with a header before each group; remember which row
	// holds the highlighted option.
	var rows []string
	selRow := 0
	group := ""
	for i, o := range opts {
		if o.Group != group {
			group = o.Group
			rows = append(rows, groupStyle.Render(group))
		}
		label := truncate(sanitizeLine(o.Label), labelWidth)
		if i == m.gitSelIndex {
			selRow = len(rows)
			rows = append(rows, selectedSuggestionStyle.Render("▶ "+label))
		} else {
			rows = append(rows, "  "+label)
		}
	}

	height := m.gitSelListHeight()
	start := 0
	if len(rows) > height {
		start = min(max(0, selRow-height/2), len(rows)-height)
	}
	end := min(start+height, len(rows))
	body := strings.Join(rows[start:end], "\n")
	if len(rows) > height {
		body += "\n" + helpStyle.Render(fmt.Sprintf("(%d-%d of %d rows)", start+1, end, len(rows)))
	}
	return suggestionStyle.MaxWidth(m.windowWidth).Render(body)
}
