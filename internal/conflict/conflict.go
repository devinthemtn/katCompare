// Package conflict parses git conflict markers in a working-tree file and
// writes the file back with some or all of the conflicts resolved.
package conflict

import (
	"strings"
)

// Choice is how one conflict block is resolved.
type Choice int

const (
	Unresolved     Choice = iota
	Ours                  // keep the "ours" side (between <<<<<<< and =======)
	Theirs                // keep the "theirs" side (between ======= and >>>>>>>)
	OursThenTheirs        // keep both, ours first
	TheirsThenOurs        // keep both, theirs first
)

// ParseChoice converts the names used by the front ends ("ours", "theirs",
// "both", "both-theirs-first") to a Choice; anything else is Unresolved.
func ParseChoice(s string) Choice {
	switch s {
	case "ours":
		return Ours
	case "theirs":
		return Theirs
	case "both":
		return OursThenTheirs
	case "both-theirs-first":
		return TheirsThenOurs
	}
	return Unresolved
}

// String is the inverse of ParseChoice ("" for Unresolved).
func (c Choice) String() string {
	switch c {
	case Ours:
		return "ours"
	case Theirs:
		return "theirs"
	case OursThenTheirs:
		return "both"
	case TheirsThenOurs:
		return "both-theirs-first"
	}
	return ""
}

// Block is one conflict: the lines between the markers and the marker labels.
type Block struct {
	StartLine   int // 1-based line of the <<<<<<< marker
	OursLabel   string
	BaseLabel   string // diff3/zdiff3 style only
	TheirsLabel string
	Ours        []string
	Base        []string // nil unless the file uses diff3 style
	Theirs      []string
	raw         []string // the block exactly as in the file, markers included
}

// Lines returns the block's lines for a choice; for Unresolved it returns the
// original block with its markers.
func (b *Block) Lines(c Choice) []string {
	switch c {
	case Ours:
		return b.Ours
	case Theirs:
		return b.Theirs
	case OursThenTheirs:
		return append(append([]string{}, b.Ours...), b.Theirs...)
	case TheirsThenOurs:
		return append(append([]string{}, b.Theirs...), b.Ours...)
	}
	return b.raw
}

// segment is either plain text lines or a conflict block.
type segment struct {
	plain []string
	block *Block
}

// File is a parsed file: plain text interleaved with conflict blocks.
type File struct {
	Blocks          []*Block
	segments        []segment
	eol             string // "\r\n" or "\n"
	trailingNewline bool
}

const (
	markerOurs   = "<<<<<<<"
	markerBase   = "|||||||"
	markerSep    = "======="
	markerTheirs = ">>>>>>>"
)

// isMarker reports whether line is the given 7-character marker, alone or
// followed by a space and a label, and returns the label.
func isMarker(line, marker string) (string, bool) {
	if !strings.HasPrefix(line, marker) {
		return "", false
	}
	rest := line[len(marker):]
	if rest == "" {
		return "", true
	}
	if rest[0] == ' ' {
		return strings.TrimSpace(rest), true
	}
	return "", false
}

// Parse splits content into plain text and conflict blocks. Markers that
// don't form a complete block are kept as plain text.
func Parse(content string) *File {
	f := &File{eol: "\n"}
	if strings.Contains(content, "\r\n") {
		f.eol = "\r\n"
	}
	f.trailingNewline = strings.HasSuffix(content, "\n")
	body := strings.TrimSuffix(content, "\n")
	if f.eol == "\r\n" {
		body = strings.TrimSuffix(content, "\r\n")
	}
	var lines []string
	if content != "" {
		lines = strings.Split(body, f.eol)
	}

	var plain []string
	flushPlain := func() {
		if len(plain) > 0 {
			f.segments = append(f.segments, segment{plain: plain})
			plain = nil
		}
	}

	for i := 0; i < len(lines); i++ {
		label, ok := isMarker(lines[i], markerOurs)
		if !ok {
			plain = append(plain, lines[i])
			continue
		}
		b, end := parseBlock(lines, i, label)
		if b == nil {
			plain = append(plain, lines[i])
			continue
		}
		flushPlain()
		f.Blocks = append(f.Blocks, b)
		f.segments = append(f.segments, segment{block: b})
		i = end
	}
	flushPlain()
	return f
}

// parseBlock parses the block whose <<<<<<< marker is lines[start]. It
// returns the block and the index of its >>>>>>> line, or nil when the block
// isn't complete.
func parseBlock(lines []string, start int, oursLabel string) (*Block, int) {
	b := &Block{StartLine: start + 1, OursLabel: oursLabel}
	section := 0 // 0 ours, 1 base, 2 theirs
	for j := start + 1; j < len(lines); j++ {
		line := lines[j]
		if _, ok := isMarker(line, markerOurs); ok {
			return nil, 0 // nested/garbled
		}
		if label, ok := isMarker(line, markerBase); ok && section == 0 {
			section = 1
			b.BaseLabel = label
			b.Base = []string{}
			continue
		}
		if line == markerSep && section < 2 {
			section = 2
			continue
		}
		if label, ok := isMarker(line, markerTheirs); ok && section == 2 {
			b.TheirsLabel = label
			b.raw = append([]string{}, lines[start:j+1]...)
			return b, j
		}
		switch section {
		case 0:
			b.Ours = append(b.Ours, line)
		case 1:
			b.Base = append(b.Base, line)
		default:
			b.Theirs = append(b.Theirs, line)
		}
	}
	return nil, 0
}

// Render writes the file back with each block resolved by choices[i]
// (missing entries count as Unresolved, which keeps the original markers).
func (f *File) Render(choices []Choice) string {
	var out []string
	bi := 0
	for _, seg := range f.segments {
		if seg.block == nil {
			out = append(out, seg.plain...)
			continue
		}
		c := Unresolved
		if bi < len(choices) {
			c = choices[bi]
		}
		out = append(out, seg.block.Lines(c)...)
		bi++
	}
	s := strings.Join(out, f.eol)
	if f.trailingNewline && len(out) > 0 {
		s += f.eol
	}
	return s
}

// Context returns up to n plain lines before and after block i.
func (f *File) Context(i, n int) (before, after []string) {
	bi := -1
	for si, seg := range f.segments {
		if seg.block == nil {
			continue
		}
		bi++
		if bi != i {
			continue
		}
		if si > 0 && f.segments[si-1].block == nil {
			p := f.segments[si-1].plain
			before = p[max(0, len(p)-n):]
		}
		if si+1 < len(f.segments) && f.segments[si+1].block == nil {
			p := f.segments[si+1].plain
			after = p[:min(n, len(p))]
		}
		return before, after
	}
	return nil, nil
}

// Remaining counts the blocks left unresolved by choices.
func Remaining(blocks int, choices []Choice) int {
	n := 0
	for i := 0; i < blocks; i++ {
		if i >= len(choices) || choices[i] == Unresolved {
			n++
		}
	}
	return n
}
