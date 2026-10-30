package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"filecmp-gui/internal/differ"
	"filecmp-gui/internal/external"
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

	gitMode   bool
	gitRoot   string
	leftRef   string
	rightRef  string
	conflicts map[string]bool

	// Paths passed on the command line, pre-filled into the UI on startup.
	initialPaths []string
}

func NewApp(initialPaths []string) *App {
	return &App{
		initialPaths: initialPaths,
		fileManager:  file.New(),
		differ:       differ.New(),
		merger:       merge.New(),
		allFiles:     make(map[string]*file.FileComparison),
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
	Conflicted  bool   `json:"conflicted"`
}

type ComparisonResult struct {
	LeftLabel  string      `json:"leftLabel"`
	RightLabel string      `json:"rightLabel"`
	GitMode    bool        `json:"gitMode"`
	Dirs       bool        `json:"dirs"` // two directories (not git), so both can open in a diff tool
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

	// Whether each side is a real file on disk that can be opened in an editor.
	LeftEditable  bool `json:"leftEditable"`
	RightEditable bool `json:"rightEditable"`
	Conflicted    bool `json:"conflicted"`
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

// GetInitialPaths returns the left/right paths given on the command line
// (zero, one or two entries).
func (a *App) GetInitialPaths() []string {
	return a.initialPaths
}

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
	a.conflicts = nil
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
	conflicts := make(map[string]bool)

	for _, fs := range statuses {
		if fs.Conflicted {
			conflicts[fs.Path] = true
		}
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
	a.gitRoot = root
	a.leftRef = leftRef
	a.rightRef = rightRef
	a.conflicts = conflicts
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
			Conflicted:  a.conflicts[relPath],
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
		Dirs:       !a.gitMode && a.leftFile.IsDir && a.rightFile.IsDir,
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

		LeftEditable:  a.editablePath(comp, "left") != "",
		RightEditable: a.editablePath(comp, "right") != "",
		Conflicted:    a.conflicts[relPath],
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

// ---- External tools ----

type ToolInfo struct {
	DiffToolConfigured  bool `json:"diffToolConfigured"`
	MergeToolConfigured bool `json:"mergeToolConfigured"`
	CanDirDiff          bool `json:"canDirDiff"`
}

// GetToolInfo reports which external tools are available, so the UI can
// explain what's missing instead of failing on click.
func (a *App) GetToolInfo() ToolInfo {
	return ToolInfo{
		DiffToolConfigured:  external.HasDiffTool(true),
		MergeToolConfigured: external.HasMergeTool(true),
		CanDirDiff:          external.CanDirDiff(true),
	}
}

// editablePath returns the on-disk path for one side of a comparison, or ""
// when that side is missing or isn't a real file (a git ref, or the working
// tree path when comparing ref to ref).
func (a *App) editablePath(comp *file.FileComparison, side string) string {
	var f *file.FileInfo
	if side == "left" {
		f = comp.LeftFile
	} else {
		f = comp.RightFile
	}
	if f == nil || strings.HasPrefix(f.Path, "git:") {
		return ""
	}
	if a.gitMode && (side == "left" || a.rightRef != "") {
		return ""
	}
	return f.Path
}

// startDetached runs cmd without waiting for it, reaping it in the background.
func startDetached(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch %s: %w", cmd.Args[0], err)
	}
	go cmd.Wait() //nolint:errcheck
	return nil
}

// OpenInEditor opens one side ("left" | "right") of relPath in the user's
// editor, at line when > 0.
func (a *App) OpenInEditor(relPath, side string, line int) error {
	comp, ok := a.allFiles[relPath]
	if !ok {
		return fmt.Errorf("file not found: %s", relPath)
	}
	path := a.editablePath(comp, side)
	if path == "" {
		return fmt.Errorf("the %s side isn't a file on disk", side)
	}
	cmd, err := external.EditorCommand(path, line, true)
	if err != nil {
		return err
	}
	return startDetached(cmd)
}

// OpenInDiffTool shows relPath in the external diff tool. In files mode the
// files are passed directly, so they don't need to be in a git repository.
func (a *App) OpenInDiffTool(relPath string) error {
	comp, ok := a.allFiles[relPath]
	if !ok {
		return fmt.Errorf("file not found: %s", relPath)
	}
	req := external.DiffRequest{}
	if a.gitMode {
		req.GitRoot = a.gitRoot
		req.LeftRef = a.leftRef
		req.RightRef = a.rightRef
		req.RelPath = relPath
	} else {
		if comp.LeftFile != nil {
			req.LeftPath = comp.LeftFile.Path
		}
		if comp.RightFile != nil {
			req.RightPath = comp.RightFile.Path
		}
	}
	cmd, err := external.DiffToolCommand(req, true)
	if err != nil {
		return err
	}
	return startDetached(cmd)
}

// OpenDirsInDiffTool opens both compared directories in the diff tool.
func (a *App) OpenDirsInDiffTool() error {
	if a.gitMode || a.leftFile == nil || a.rightFile == nil || !a.leftFile.IsDir || !a.rightFile.IsDir {
		return fmt.Errorf("only available when comparing two directories")
	}
	cmd, err := external.DirDiffCommand(a.leftFile.Path, a.rightFile.Path, true)
	if err != nil {
		return err
	}
	return startDetached(cmd)
}

// ResolveConflict runs git mergetool on a conflicted file. It returns once
// the tool has launched; when it exits a "conflict-resolved" event is sent
// so the frontend can reload.
func (a *App) ResolveConflict(relPath string) error {
	if !a.gitMode || !a.conflicts[relPath] {
		return fmt.Errorf("%s has no merge conflicts", relPath)
	}
	cmd, err := external.MergeToolCommand(a.gitRoot, relPath, true)
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch git mergetool: %w", err)
	}
	go func() {
		err := cmd.Wait()
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		wailsRuntime.EventsEmit(a.ctx, "conflict-resolved", relPath, msg)
	}()
	return nil
}
