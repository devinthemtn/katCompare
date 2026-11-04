package ui

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

// Everything here measures in terminal cells, not bytes, so that cutting text
// can't split a character or misjudge a wide one.

const tabWidth = 4

// sanitizeLine makes file content safe to draw: tabs become spaces, carriage
// returns go, and other control characters and invalid bytes become visible
// placeholders, so a file can't move the cursor or change the colors.
func sanitizeLine(s string) string {
	s = strings.ToValidUTF8(s, "�")
	var b strings.Builder
	col := 0
	for _, r := range s {
		switch {
		case r == '\t':
			n := tabWidth - col%tabWidth
			b.WriteString(strings.Repeat(" ", n))
			col += n
		case r == '\r':
		case unicode.IsControl(r):
			b.WriteRune('·')
			col++
		default:
			b.WriteRune(r)
			col += ansi.StringWidth(string(r))
		}
	}
	return b.String()
}

// truncate cuts s to at most w cells, marking the cut.
func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return ansi.Truncate(s, w, "…")
}

// truncatePath cuts s to at most w cells from the left, since the end of a
// path says the most about it.
func truncatePath(s string, w int) string {
	if w <= 0 {
		return ""
	}
	width := ansi.StringWidth(s)
	if width <= w {
		return s
	}
	return truncate(ansi.TruncateLeft(s, width-w+1, "…"), w)
}

// pad fills s with spaces up to w cells.
func pad(s string, w int) string {
	if n := w - ansi.StringWidth(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

// fitCell makes s exactly w cells wide.
func fitCell(s string, w int) string {
	return pad(truncate(s, w), w)
}

// scrollRight drops the first n cells of s.
func scrollRight(s string, n int) string {
	if n <= 0 {
		return s
	}
	width := ansi.StringWidth(s)
	if n >= width {
		return ""
	}
	return ansi.Cut(s, n, width)
}

// dropLastRune removes the last character of s.
func dropLastRune(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return string(r[:len(r)-1])
}

// fit clips rendered text to w cells by h lines. Every view passes through it,
// so nothing a view draws can scroll the terminal or wrap.
func fit(s string, w, h int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[:max(0, h)]
	}
	for i, line := range lines {
		if ansi.StringWidth(line) > w {
			lines[i] = ansi.Truncate(line, w, "")
		}
	}
	return strings.Join(lines, "\n")
}
