package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"filecmp-gui/internal/differ"
	"filecmp-gui/internal/file"
	"filecmp-gui/internal/git"
	"filecmp-gui/internal/merge"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails-bound backend. It holds the currently loaded comparison
// so the frontend can request diffs / trigger merges / copies by relative
// path without re-sending file content on every call.
type App struct {
	ctx context.Context

	fileManager *file.Manager
	differ      *differ.Differ
	merger      *merge.Merger

	leftPath  string
	rightPath string
	leftFile  *file.FileInfo
	rightFile *file.FileInfo
	allFiles  map[string]*file.FileComparison

	gitMode bool
	leftRef string
}

func NewApp() *App {
	return &App{
		fileManager: file.New(),
		differ:      differ.New(),
		merger:      merge.New(),
		allFiles:    make(map[string]*file.FileComparison),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// ---- DTOs exposed to the frontend ----

type FileEntry struct {
	RelPath     string `json:"relPath"`
	DisplayName string `json:"displayName"`
	Source      string `json:"source"` // "both" | "left" | "right"
	Identical   bool   `json:"identical"`
}

type ComparisonResult struct {
	LeftLabel  string      `json:"leftLabel"`
	RightLabel string      `json:"rightLabel"`
	GitMode    bool        `json:"gitMode"`
	Files      []FileEntry `json:"files"`
}

type DiffLineDTO struct {
	Type         string `json:"type"` // "equal" | "insert" | "delete"
	Content      string `json:"content"`
	LeftLineNum  int    `json:"leftLineNum"`
	RightLineNum int    `json:"rightLineNum"`
}

type SideBySideRowDTO struct {
	Type         string `json:"type"` // "equal" | "insert" | "delete" | "modified"
	LeftContent  string `json:"leftContent"`
	RightContent string `json:"rightContent"`
	LeftLineNum  int    `json:"leftLineNum"`
	RightLineNum int    `json:"rightLineNum"`
}

type DiffResult struct {
	RelPath     string             `json:"relPath"`
	DisplayName string             `json:"displayName"`
	Source      string             `json:"source"`
	LeftLabel   string             `json:"leftLabel"`
	RightLabel  string             `json:"rightLabel"`
	Lines       []DiffLineDTO      `json:"lines"`
	SideBySide  []SideBySideRowDTO `json:"sideBySide"`
	Equal       int                `json:"equal"`
	Inserted    int                `json:"inserted"`
	Deleted     int                `json:"deleted"`
	CanMerge    bool               `json:"canMerge"`
}

type MergeSaveRequest struct {
	RelPath            string `json:"relPath"`
	Target             string `json:"target"` // "left" | "right"
	SelectedInsertions []int  `json:"selectedInsertions"`
	SelectedDeletions  []int  `json:"selectedDeletions"`
}

// ---- Dialogs ----

func (a *App) SelectDirectory() (string, error) {
	return wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select Directory",
	})
}

func (a *App) SelectFile() (string, error) {
	return wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select File",
	})
}

// ---- Loading comparisons ----

// LoadComparison loads left/right paths (files or directories) and returns
// the merged file list (common files + files unique to either side).
func (a *App) LoadComparison(leftPath, rightPath string) (*ComparisonResult, error) {
	if leftPath == "" || rightPath == "" {
		return nil, fmt.Errorf("both a left and right path are required")
	}

	leftFile, err := a.fileManager.LoadPath(leftPath)
	if err != nil {
		return nil, fmt.Errorf("left path: %w", err)
	}
	rightFile, err := a.fileManager.LoadPath(rightPath)
	if err != nil {
		return nil, fmt.Errorf("right path: %w", err)
	}

	a.gitMode = false
	a.leftPath = leftPath
	a.rightPath = rightPath
	a.leftFile = leftFile
	a.rightFile = rightFile
	a.allFiles = file.FindAllFiles(leftFile, rightFile)

	if len(a.allFiles) == 0 {
		return nil, fmt.Errorf("no comparable text files found under the given paths")
	}

	return a.buildComparisonResult(), nil
}

// LoadGitComparison loads a diff between two git refs, or a ref and the
// working tree when rightRef is empty.
func (a *App) LoadGitComparison(leftRef, rightRef string) (*ComparisonResult, error) {
	if leftRef == "" {
		leftRef = "HEAD"
	}

	root, err := git.FindRoot()
	if err != nil {
		return nil, err
	}

	statuses, err := git.ChangedFiles(root, leftRef, rightRef)
	if err != nil {
		return nil, err
	}

	rightLabel := rightRef
	if rightLabel == "" {
		rightLabel = "working tree"
	}
	if len(statuses) == 0 {
		return nil, fmt.Errorf("no changes found between %s and %s", leftRef, rightLabel)
	}

	leftLabel := fmt.Sprintf("git:%s", leftRef)

	allFiles := make(map[string]*file.FileComparison)

	for _, fs := range statuses {
		comparison := &file.FileComparison{RelativePath: fs.Path}

		switch fs.Status {
		case 'A': // Added — only exists on the right (working tree or rightRef)
			var content string
			if rightRef == "" {
				content, err = git.ReadWorkingTreeFile(root, fs.Path)
			} else {
				content, err = git.FileAtRef(root, rightRef, fs.Path)
			}
			if err != nil {
				continue
			}
			comparison.RightFile = &file.FileInfo{
				Path:    filepath.Join(root, fs.Path),
				Name:    filepath.Base(fs.Path),
				Content: content,
				Size:    int64(len(content)),
			}
			comparison.Source = file.SourceRight

		case 'D': // Deleted — only exists at leftRef
			content, err := git.FileAtRef(root, leftRef, fs.Path)
			if err != nil {
				continue
			}
			comparison.LeftFile = &file.FileInfo{
				Path:    fmt.Sprintf("git:%s:%s", leftRef, fs.Path),
				Name:    filepath.Base(fs.Path),
				Content: content,
				Size:    int64(len(content)),
			}
			comparison.Source = file.SourceLeft

		default: // Modified — exists on both sides
			leftContent, err := git.FileAtRef(root, leftRef, fs.Path)
			if err != nil {
				continue
			}
			var rightContent string
			if rightRef == "" {
				rightContent, err = git.ReadWorkingTreeFile(root, fs.Path)
			} else {
				rightContent, err = git.FileAtRef(root, rightRef, fs.Path)
			}
			if err != nil {
				continue
			}
			comparison.LeftFile = &file.FileInfo{
				Path:    fmt.Sprintf("git:%s:%s", leftRef, fs.Path),
				Name:    filepath.Base(fs.Path),
				Content: leftContent,
				Size:    int64(len(leftContent)),
			}
			comparison.RightFile = &file.FileInfo{
				Path:    filepath.Join(root, fs.Path),
				Name:    filepath.Base(fs.Path),
				Content: rightContent,
				Size:    int64(len(rightContent)),
			}
			comparison.Source = file.SourceBoth
		}

		allFiles[fs.Path] = comparison
	}

	if len(allFiles) == 0 {
		return nil, fmt.Errorf("no comparable text files changed between %s and %s", leftRef, rightLabel)
	}

	a.gitMode = true
	a.leftRef = leftRef
	a.leftPath = leftLabel
	a.rightPath = rightLabel
	a.leftFile = &file.FileInfo{Path: leftLabel, Name: leftRef, IsDir: true}
	a.rightFile = &file.FileInfo{Path: rightLabel, Name: rightLabel, IsDir: true}
	a.allFiles = allFiles

	return a.buildComparisonResult(), nil
}

func (a *App) buildComparisonResult() *ComparisonResult {
	entries := make([]FileEntry, 0, len(a.allFiles))
	for relPath, comp := range a.allFiles {
		entry := FileEntry{
			RelPath:     relPath,
			DisplayName: a.displayNameForFile(relPath),
			Source:      sourceString(comp.Source),
		}
		if comp.Source == file.SourceBoth {
			entry.Identical = comp.LeftFile.Content == comp.RightFile.Content
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].RelPath < entries[j].RelPath })

	return &ComparisonResult{
		LeftLabel:  a.leftPath,
		RightLabel: a.rightPath,
		GitMode:    a.gitMode,
		Files:      entries,
	}
}

// displayNameForFile mirrors the TUI's logic: when comparing two single
// files directly, both are keyed by "." and we should show their actual
// filenames instead of a meaningless ".".
func (a *App) displayNameForFile(relPath string) string {
	if relPath != "." {
		return relPath
	}
	comparison, exists := a.allFiles[relPath]
	if !exists {
		return relPath
	}
	switch comparison.Source {
	case file.SourceBoth:
		if comparison.LeftFile.Name == comparison.RightFile.Name {
			return comparison.LeftFile.Name
		}
		return comparison.LeftFile.Name + " ↔ " + comparison.RightFile.Name
	case file.SourceLeft:
		return comparison.LeftFile.Name
	case file.SourceRight:
		return comparison.RightFile.Name
	}
	return relPath
}

func sourceString(s file.FileSource) string {
	switch s {
	case file.SourceBoth:
		return "both"
	case file.SourceLeft:
		return "left"
	case file.SourceRight:
		return "right"
	}
	return "both"
}

// ---- Diffing ----

func (a *App) GetDiff(relPath string) (*DiffResult, error) {
	comp, ok := a.allFiles[relPath]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", relPath)
	}

	var leftContent, rightContent, leftLabel, rightLabel string

	switch comp.Source {
	case file.SourceBoth:
		leftContent = comp.LeftFile.Content
		rightContent = comp.RightFile.Content
		leftLabel = comp.LeftFile.Path
		rightLabel = comp.RightFile.Path
	case file.SourceLeft:
		leftContent = comp.LeftFile.Content
		leftLabel = comp.LeftFile.Path
		rightLabel = "<file not found>"
	case file.SourceRight:
		rightContent = comp.RightFile.Content
		leftLabel = "<file not found>"
		rightLabel = comp.RightFile.Path
	}

	diff := a.differ.CompareStrings(leftLabel, rightLabel, leftContent, rightContent)
	sbsRows := differ.BuildSideBySideRows(diff.Lines)
	equal, inserted, deleted := diff.GetStats()

	return &DiffResult{
		RelPath:     relPath,
		DisplayName: a.displayNameForFile(relPath),
		Source:      sourceString(comp.Source),
		LeftLabel:   leftLabel,
		RightLabel:  rightLabel,
		Lines:       toDiffLineDTOs(diff.Lines),
		SideBySide:  toSideBySideDTOs(sbsRows),
		Equal:       equal,
		Inserted:    inserted,
		Deleted:     deleted,
		CanMerge:    comp.Source == file.SourceBoth,
	}, nil
}

func toDiffLineDTOs(lines []differ.DiffLine) []DiffLineDTO {
	out := make([]DiffLineDTO, len(lines))
	for i, l := range lines {
		out[i] = DiffLineDTO{
			Type:         diffTypeString(l.Type),
			Content:      l.Content,
			LeftLineNum:  l.LeftLineNum,
			RightLineNum: l.RightLineNum,
		}
	}
	return out
}

func toSideBySideDTOs(rows []differ.SideBySideRow) []SideBySideRowDTO {
	out := make([]SideBySideRowDTO, len(rows))
	for i, r := range rows {
		out[i] = SideBySideRowDTO{
			Type:         sbsTypeString(r.Type),
			LeftContent:  r.LeftContent,
			RightContent: r.RightContent,
			LeftLineNum:  r.LeftLineNum,
			RightLineNum: r.RightLineNum,
		}
	}
	return out
}

func diffTypeString(t differ.DiffType) string {
	switch t {
	case differ.DiffInsert:
		return "insert"
	case differ.DiffDelete:
		return "delete"
	}
	return "equal"
}

func sbsTypeString(t differ.SideBySideRowType) string {
	switch t {
	case differ.SBSInsert:
		return "insert"
	case differ.SBSDelete:
		return "delete"
	case differ.SBSModified:
		return "modified"
	}
	return "equal"
}

// ---- Merge ----

// SaveMerge recomputes the diff for relPath and writes a "<file>.merged"
// file containing the given selection of changes applied to the chosen
// target side. The original files are never overwritten.
func (a *App) SaveMerge(req MergeSaveRequest) (string, error) {
	comp, ok := a.allFiles[req.RelPath]
	if !ok || comp.Source != file.SourceBoth {
		return "", fmt.Errorf("cannot merge: file must exist on both sides")
	}

	diff := a.differ.CompareStrings(comp.LeftFile.Path, comp.RightFile.Path, comp.LeftFile.Content, comp.RightFile.Content)

	selection := &merge.ChangeSelection{
		ApplyInsertions: make(map[int]bool, len(req.SelectedInsertions)),
		ApplyDeletions:  make(map[int]bool, len(req.SelectedDeletions)),
	}
	for _, i := range req.SelectedInsertions {
		selection.ApplyInsertions[i] = true
	}
	for _, i := range req.SelectedDeletions {
		selection.ApplyDeletions[i] = true
	}

	var result *merge.MergeResult
	var targetPath string
	if req.Target == "left" {
		result = a.merger.ApplyToLeft(diff, selection)
		targetPath = diff.LeftFile + ".merged"
	} else if req.Target == "right" {
		result = a.merger.ApplyToRight(diff, selection)
		targetPath = diff.RightFile + ".merged"
	} else {
		return "", fmt.Errorf("invalid merge target: %s", req.Target)
	}

	if err := os.WriteFile(targetPath, []byte(result.Content), 0644); err != nil {
		return "", fmt.Errorf("failed to save merged file: %w", err)
	}

	return fmt.Sprintf("Saved merged result to %s (%d change(s) applied, %d skipped)", targetPath, result.Applied, result.Skipped), nil
}

// ---- Copy ----

// CopyFiles copies the given unique (single-side) files to the other side's
// directory. target is "to-left" or "to-right". Not available in git mode,
// since the compared roots there are synthetic.
func (a *App) CopyFiles(relPaths []string, target string) (string, error) {
	if a.gitMode {
		return "", fmt.Errorf("copy is not available when comparing against git")
	}
	if a.leftFile == nil || a.rightFile == nil {
		return "", fmt.Errorf("no comparison loaded")
	}

	copied, skipped, errCount := 0, 0, 0
	var errs []string

	for _, relPath := range relPaths {
		comp, ok := a.allFiles[relPath]
		if !ok {
			skipped++
			continue
		}

		var srcFile *file.FileInfo
		var dstPath string

		if target == "to-right" && comp.Source == file.SourceLeft {
			srcFile = comp.LeftFile
			dstPath = filepath.Join(a.rightFile.Path, relPath)
		} else if target == "to-left" && comp.Source == file.SourceRight {
			srcFile = comp.RightFile
			dstPath = filepath.Join(a.leftFile.Path, relPath)
		} else {
			skipped++
			continue
		}

		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			errCount++
			errs = append(errs, fmt.Sprintf("%s: %v", relPath, err))
			continue
		}
		if err := os.WriteFile(dstPath, []byte(srcFile.Content), 0644); err != nil {
			errCount++
			errs = append(errs, fmt.Sprintf("%s: %v", relPath, err))
			continue
		}
		copied++
	}

	msg := fmt.Sprintf("Copied %d file(s), skipped %d", copied, skipped)
	if errCount > 0 {
		msg += fmt.Sprintf(", %d error(s): %s", errCount, strings.Join(errs, "; "))
	}
	return msg, nil
}

// ---- Path autocomplete ----

func (a *App) SuggestPaths(input string) []string {
	if len(input) == 0 {
		return nil
	}

	var searchDir, prefix string
	if strings.HasSuffix(input, "/") || strings.HasSuffix(input, "\\") {
		searchDir = input
		prefix = ""
	} else {
		searchDir = filepath.Dir(input)
		prefix = filepath.Base(input)
		if searchDir == "." && !strings.Contains(input, "/") && !strings.Contains(input, "\\") {
			searchDir = ""
		}
	}
	if searchDir == "" || searchDir == "." {
		searchDir = "."
	}

	entries, err := os.ReadDir(searchDir)
	if err != nil {
		return nil
	}

	var suggestions []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(prefix, ".") {
			continue
		}
		if prefix == "" || strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) {
			var suggestion string
			if searchDir == "." {
				suggestion = name
			} else {
				suggestion = filepath.Join(searchDir, name)
			}
			if entry.IsDir() {
				suggestion += string(filepath.Separator)
			}
			suggestions = append(suggestions, suggestion)
		}
	}

	sort.Strings(suggestions)

	const maxSuggestions = 8
	if len(suggestions) > maxSuggestions {
		suggestions = suggestions[:maxSuggestions]
	}
	return suggestions
}
