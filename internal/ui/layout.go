package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Every screen is a one-line header, a body, a message line and a key line.
// The sizes below are the only place that arithmetic lives: the renderers and
// the key handlers both use them, so they can't disagree about what's visible.

// wideWidth is the narrowest terminal that shows the file list and the diff
// side by side. Below it, one pane shows at a time.
const wideWidth = 100

func (m *Model) wide() bool { return m.windowWidth >= wideWidth }

// bodyHeight is the rows between the header and the message line.
func (m *Model) bodyHeight() int { return max(1, m.windowHeight-3) }

// paneInnerHeight is the rows inside a bordered pane.
func (m *Model) paneInnerHeight() int { return max(1, m.bodyHeight()-2) }

// filesPaneWidth is the outer width of the file list.
func (m *Model) filesPaneWidth() int {
	if !m.wide() {
		return m.windowWidth
	}
	return min(max(28, m.windowWidth*30/100), 60)
}

// rightPaneWidth is the outer width of the diff, merge or conflict pane.
func (m *Model) rightPaneWidth() int {
	if !m.wide() {
		return m.windowWidth
	}
	return m.windowWidth - m.filesPaneWidth()
}

// bannerRows is the rows the selected file's merge status takes in the diff.
func (m *Model) bannerRows() int {
	if m.statuses[m.selectedFile] != nil {
		return 1
	}
	return 0
}

// diffViewHeight is the number of rows of lines the current view shows.
func (m *Model) diffViewHeight() int {
	if m.viewMode == ViewModeMerge {
		return m.mergeViewHeight()
	}
	return max(1, m.paneInnerHeight()-m.bannerRows())
}

// mergeViewHeight is the same for the merge view, which has two toolbar rows.
func (m *Model) mergeViewHeight() int {
	return max(1, m.paneInnerHeight()-m.bannerRows()-2)
}

// scrollFor returns an offset close to offset that keeps cursor visible.
func scrollFor(offset, cursor, height, total int) int {
	if cursor < offset {
		offset = cursor
	}
	if cursor >= offset+height {
		offset = cursor - height + 1
	}
	return max(0, min(offset, total-height))
}

// box draws a bordered pane of exactly w by h cells with title in its top
// border. Lines are clipped; the focused pane gets the accent color.
func box(title string, lines []string, w, h int, focused bool) string {
	if w < 4 || h < 2 {
		return ""
	}
	border := paneBorderStyle
	if focused {
		border = paneFocusedBorderStyle
	}
	inner := w - 2

	title = truncate(title, inner-3)
	top := "╭─ " + title + " "
	top = border.Render(top + strings.Repeat("─", max(0, w-1-lipgloss.Width(top))) + "╮")

	out := make([]string, 0, h)
	out = append(out, top)
	side := border.Render("│")
	for i := 0; i < h-2; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		if lipgloss.Width(line) > inner {
			line = truncate(line, inner)
		}
		out = append(out, side+pad(line, inner)+side)
	}
	out = append(out, border.Render("╰"+strings.Repeat("─", inner)+"╯"))
	return strings.Join(out, "\n")
}

// frame assembles a screen.
func (m *Model) frame(body string, keys string) string {
	lines := strings.Split(body, "\n")
	for len(lines) < m.bodyHeight() {
		lines = append(lines, "")
	}
	if len(lines) > m.bodyHeight() {
		lines = lines[:m.bodyHeight()]
	}
	return strings.Join([]string{m.renderHeader(), strings.Join(lines, "\n"), m.renderMessage(), keys}, "\n")
}

// renderHeader is the top line: what's being compared and how it came out.
func (m *Model) renderHeader() string {
	w := m.windowWidth
	summary := ""
	if n := len(m.allFiles); n > 0 {
		summary = fmt.Sprintf("%s · %d differ", plural(n, "file"), m.differCount)
	}
	text := " katCompare"
	if m.leftFile != nil && m.rightFile != nil {
		room := max(4, (w-lipgloss.Width(summary)-lipgloss.Width(text)-10)/2)
		text += "  " + truncatePath(sanitizeLine(m.sideLabel(m.leftPath)), room) +
			" ↔ " + truncatePath(sanitizeLine(m.sideLabel(m.rightPath)), room)
	}
	if summary != "" {
		gap := max(1, w-lipgloss.Width(text)-lipgloss.Width(summary)-1)
		text += strings.Repeat(" ", gap) + summary
	}
	return headerBarStyle.Render(fitCell(text, w))
}

// sideLabel shortens one compared root for the header. Paths lose all but
// their last two parts; git refs and "working tree" are shown as they are.
func (m *Model) sideLabel(path string) string {
	if m.gitMode {
		return path
	}
	path = filepath.Clean(path)
	dir, base := filepath.Split(path)
	parent := filepath.Base(dir)
	if parent == "." || parent == string(filepath.Separator) || dir == "" {
		return path
	}
	return filepath.Join(parent, base)
}

// renderMessage is the line above the keys: an error, a warning or a result.
func (m *Model) renderMessage() string {
	w := m.windowWidth
	switch {
	case m.errorMsg != "":
		return errorStyle.Render(truncate(" "+sanitizeLine(m.errorMsg), w))
	case m.statusMsg != "" && m.statusWarn:
		return warnStyle.Render(truncate(" "+sanitizeLine(m.statusMsg), w))
	case m.statusMsg != "":
		return statusStyle.Render(truncate(" "+sanitizeLine(m.statusMsg), w))
	}
	return ""
}

// key is one entry of a key line.
type key struct{ keys, action string }

// renderKeys draws as many keys as fit in w cells. The last one, the way to
// the help screen, always stays.
func renderKeys(w int, keys ...key) string {
	render := func(k key) string {
		return keyNameStyle.Render(k.keys) + keyActionStyle.Render(" "+k.action)
	}
	if len(keys) == 0 {
		return ""
	}
	last := render(keys[len(keys)-1])
	room := w - lipgloss.Width(last) - 1
	var parts []string
	for _, k := range keys[:len(keys)-1] {
		part := render(k)
		if lipgloss.Width(part)+2 > room {
			break
		}
		parts = append(parts, part)
		room -= lipgloss.Width(part) + 2
	}
	parts = append(parts, last)
	return " " + strings.Join(parts, "  ")
}

// plural formats a count, as in "1 file" or "2 files".
func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
