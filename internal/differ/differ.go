package differ

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// DiffType represents the type of difference
type DiffType int

const (
	DiffEqual DiffType = iota
	DiffInsert
	DiffDelete
)

// DiffLine represents a single line in the diff output
type DiffLine struct {
	Type         DiffType
	Content      string
	LineNum      int // Left line num for equal/delete; right line num for insert (unified view)
	LeftLineNum  int // Left file line number; -1 if not applicable
	RightLineNum int // Right file line number; -1 if not applicable
}

// SideBySideRowType represents the type of an aligned side-by-side row
type SideBySideRowType int

const (
	SBSEqual    SideBySideRowType = iota
	SBSInsert                     // new line on right only
	SBSDelete                     // removed line on left only
	SBSModified                   // adjacent delete+insert pair (changed line)
)

// SideBySideRow is one aligned row in a side-by-side diff view
type SideBySideRow struct {
	Type         SideBySideRowType
	LeftContent  string
	RightContent string
	LeftLineNum  int // -1 if no left line
	RightLineNum int // -1 if no right line
}

// BuildSideBySideRows converts diff lines into aligned side-by-side rows.
// Adjacent Delete+Insert pairs are collapsed into a single Modified row so
// changed lines appear side-by-side rather than on separate rows.
func BuildSideBySideRows(lines []DiffLine) []SideBySideRow {
	rows := make([]SideBySideRow, 0, len(lines))
	i := 0
	for i < len(lines) {
		line := lines[i]
		switch line.Type {
		case DiffEqual:
			rows = append(rows, SideBySideRow{
				Type:         SBSEqual,
				LeftContent:  line.Content,
				RightContent: line.Content,
				LeftLineNum:  line.LeftLineNum,
				RightLineNum: line.RightLineNum,
			})
			i++
		case DiffDelete:
			if i+1 < len(lines) && lines[i+1].Type == DiffInsert {
				rows = append(rows, SideBySideRow{
					Type:         SBSModified,
					LeftContent:  line.Content,
					RightContent: lines[i+1].Content,
					LeftLineNum:  line.LeftLineNum,
					RightLineNum: lines[i+1].RightLineNum,
				})
				i += 2
			} else {
				rows = append(rows, SideBySideRow{
					Type:         SBSDelete,
					LeftContent:  line.Content,
					RightContent: "",
					LeftLineNum:  line.LeftLineNum,
					RightLineNum: -1,
				})
				i++
			}
		case DiffInsert:
			rows = append(rows, SideBySideRow{
				Type:         SBSInsert,
				LeftContent:  "",
				RightContent: line.Content,
				LeftLineNum:  -1,
				RightLineNum: line.RightLineNum,
			})
			i++
		}
	}
	return rows
}

// FileDiff represents the complete diff between two files
type FileDiff struct {
	LeftFile  string
	RightFile string
	Lines     []DiffLine
}

// Differ handles file comparison operations
type Differ struct{}

// New creates a new Differ instance
func New() *Differ {
	return &Differ{}
}

// CompareFiles compares two files and returns a structured diff
func (d *Differ) CompareFiles(leftPath, rightPath string, leftContent, rightContent io.Reader) (*FileDiff, error) {
	leftLines, err := readLines(leftContent)
	if err != nil {
		return nil, fmt.Errorf("error reading left file: %w", err)
	}

	rightLines, err := readLines(rightContent)
	if err != nil {
		return nil, fmt.Errorf("error reading right file: %w", err)
	}

	return d.compareLines(leftPath, rightPath, leftLines, rightLines), nil
}

// CompareStrings compares two strings and returns a structured diff
func (d *Differ) CompareStrings(leftPath, rightPath, leftContent, rightContent string) *FileDiff {
	leftLines := strings.Split(leftContent, "\n")
	rightLines := strings.Split(rightContent, "\n")
	return d.compareLines(leftPath, rightPath, leftLines, rightLines)
}

// compareLines performs a true line-level diff using LCS.
// Each DiffLine corresponds to exactly one source line — no partial-line
// chunks, no spurious empty entries.
func (d *Differ) compareLines(leftPath, rightPath string, leftLines, rightLines []string) *FileDiff {
	diff := &FileDiff{
		LeftFile:  leftPath,
		RightFile: rightPath,
		Lines:     make([]DiffLine, 0),
	}

	// Drop the trailing empty string that strings.Split produces for
	// content ending in "\n" (e.g. "a\nb\n" → ["a","b",""])
	if len(leftLines) > 0 && leftLines[len(leftLines)-1] == "" {
		leftLines = leftLines[:len(leftLines)-1]
	}
	if len(rightLines) > 0 && rightLines[len(rightLines)-1] == "" {
		rightLines = rightLines[:len(rightLines)-1]
	}

	m, n := len(leftLines), len(rightLines)
	matches := lcsMatches(leftLines, rightLines)

	leftLineNum := 1
	rightLineNum := 1
	li := 0
	ri := 0

	for _, match := range matches {
		leftIdx, rightIdx := match[0], match[1]

		// Lines on the left before this match are deletions
		for li < leftIdx {
			diff.Lines = append(diff.Lines, DiffLine{
				Type:         DiffDelete,
				Content:      leftLines[li],
				LineNum:      leftLineNum,
				LeftLineNum:  leftLineNum,
				RightLineNum: -1,
			})
			li++
			leftLineNum++
		}

		// Lines on the right before this match are insertions
		for ri < rightIdx {
			diff.Lines = append(diff.Lines, DiffLine{
				Type:         DiffInsert,
				Content:      rightLines[ri],
				LineNum:      rightLineNum,
				LeftLineNum:  -1,
				RightLineNum: rightLineNum,
			})
			ri++
			rightLineNum++
		}

		// The matched line (equal on both sides)
		diff.Lines = append(diff.Lines, DiffLine{
			Type:         DiffEqual,
			Content:      leftLines[li],
			LineNum:      leftLineNum,
			LeftLineNum:  leftLineNum,
			RightLineNum: rightLineNum,
		})
		li++
		ri++
		leftLineNum++
		rightLineNum++
	}

	// Remaining left lines are deletions
	for li < m {
		diff.Lines = append(diff.Lines, DiffLine{
			Type:         DiffDelete,
			Content:      leftLines[li],
			LineNum:      leftLineNum,
			LeftLineNum:  leftLineNum,
			RightLineNum: -1,
		})
		li++
		leftLineNum++
	}

	// Remaining right lines are insertions
	for ri < n {
		diff.Lines = append(diff.Lines, DiffLine{
			Type:         DiffInsert,
			Content:      rightLines[ri],
			LineNum:      rightLineNum,
			LeftLineNum:  -1,
			RightLineNum: rightLineNum,
		})
		ri++
		rightLineNum++
	}

	return diff
}

// lcsMatches returns the matched (leftIndex, rightIndex) pairs from the
// Longest Common Subsequence of left and right.
func lcsMatches(left, right []string) [][2]int {
	m, n := len(left), len(right)
	if m == 0 || n == 0 {
		return nil
	}

	// dp[i][j] = LCS length of left[:i] and right[:j]
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if left[i-1] == right[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to collect matched pairs
	matches := make([][2]int, 0, dp[m][n])
	i, j := m, n
	for i > 0 && j > 0 {
		if left[i-1] == right[j-1] {
			matches = append([][2]int{{i - 1, j - 1}}, matches...)
			i--
			j--
		} else if dp[i-1][j] >= dp[i][j-1] {
			i--
		} else {
			j--
		}
	}
	return matches
}

// readLines reads all lines from a reader
func readLines(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return lines, nil
}

// GetStats returns statistics about the diff
func (fd *FileDiff) GetStats() (int, int, int) {
	equal, inserted, deleted := 0, 0, 0

	for _, line := range fd.Lines {
		switch line.Type {
		case DiffEqual:
			equal++
		case DiffInsert:
			inserted++
		case DiffDelete:
			deleted++
		}
	}

	return equal, inserted, deleted
}
