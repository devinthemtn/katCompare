package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"katcompare/internal/config"
	"katcompare/internal/conflict"
	"katcompare/internal/differ"
	"katcompare/internal/external"
	"katcompare/internal/file"
	"katcompare/internal/git"
	"katcompare/internal/merge"
	"katcompare/internal/proc"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// App is the Wails service bound to the frontend. It holds the currently loaded comparison
// so the frontend can request diffs / trigger merges / copies by relative
// path without re-sending file content on every call.
type App struct {
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

	// Saves made this session.
	merges *merge.Tracker

	// Settings for the current comparison, read when it's loaded and kept until
	// the next one. Merge selections are line numbers, so the diff a save
	// recomputes has to come out the same as the one on screen.
	diffOpts      differ.Options
	hideIdentical bool

	// Files read recently, oldest first. Frontend calls can overlap, so access
	// goes through openMu.
	openMu    sync.Mutex
	opened    []*openedFile
	openToken int

	identical map[string]bool // files with no differences to show

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
		merges:       merge.NewTracker(),
	}
}

// conflictResolvedEvent is sent when a git mergetool launched by
// ResolveConflict exits. Data: [relPath, errMsg].
const conflictResolvedEvent = "conflict-resolved"

func init() {
	application.RegisterEvent[[]string](conflictResolvedEvent)
}

// ---- DTOs exposed to the frontend ----

type FileEntry struct {
	RelPath     string `json:"relPath"`
	DisplayName string `json:"displayName"`
	Source      string `json:"source"` // "both" | "left" | "right"
	Identical   bool   `json:"identical"`
	Conflicted  bool   `json:"conflicted"`

	MergeStatus *MergeStatusDTO `json:"mergeStatus,omitempty"`
}

// MergeStatusDTO describes the last save of a file this session, for as long
// as the file on disk still matches that save.
type MergeStatusDTO struct {
	State   string   `json:"state"`   // "safe" | "check"
	Kind    string   `json:"kind"`    // "merge" | "conflicts"
	Target  string   `json:"target"`  // name of the file saved to
	Backup  string   `json:"backup"`  // name of its backup
	Applied int      `json:"applied"` // merge saves only
	Skipped int      `json:"skipped"` // merge saves only
	Issues  []string `json:"issues"`  // why State is "check"
}

type ComparisonResult struct {
	HiddenIdentical int         `json:"hiddenIdentical"` // files left out by hide_identical
	LeftLabel       string      `json:"leftLabel"`
	RightLabel      string      `json:"rightLabel"`
	GitMode         bool        `json:"gitMode"`
	Dirs            bool        `json:"dirs"` // two directories (not git), so both can open in a diff tool
	Files           []FileEntry `json:"files"`
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
	RelPath     string        `json:"relPath"`
	DisplayName string        `json:"displayName"`
	Source      string        `json:"source"`
	LeftLabel   string        `json:"leftLabel"`
	RightLabel  string        `json:"rightLabel"`
	Lines       []DiffLineDTO `json:"lines"`
	Equal       int           `json:"equal"`
	Inserted    int           `json:"inserted"`
	Deleted     int           `json:"deleted"`
	CanMerge    bool          `json:"canMerge"`

	// Whether each side is a real file on disk that can be opened in an editor.
	LeftEditable  bool `json:"leftEditable"`
	RightEditable bool `json:"rightEditable"`
	Conflicted    bool `json:"conflicted"`

	MergeStatus *MergeStatusDTO `json:"mergeStatus,omitempty"`
	Ignoring    string          `json:"ignoring"` // what the diff ignores, or ""
	Token       int             `json:"token"`    // pass back to SaveMerge
}

type GitRefOption struct {
	Ref   string `json:"ref"` // "" means the working tree
	Label string `json:"label"`
	Group string `json:"group"`
}

type MergeSaveRequest struct {
	RelPath            string `json:"relPath"`
	Target             string `json:"target"` // "left" | "right"
	SelectedInsertions []int  `json:"selectedInsertions"`
	SelectedDeletions  []int  `json:"selectedDeletions"`
	Token              int    `json:"token"` // from the DiffResult the selections were made on
}

// ---- Dialogs ----

// dialogStartDir picks the folder a file dialog opens in: the folder of the
// path already entered for that side, or else the current directory (the one
// the app was launched from, e.g. a WSL directory via katcmp-win).
func dialogStartDir(current string) string {
	if current = strings.TrimSpace(current); current != "" {
		if info, err := os.Stat(current); err == nil {
			if info.IsDir() {
				return current
			}
			return filepath.Dir(current)
		}
		if dir := filepath.Dir(current); dir != "." {
			if info, err := os.Stat(dir); err == nil && info.IsDir() {
				return dir
			}
		}
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return ""
}

// SelectDirectory opens a folder picker starting near current (the path in
// the input box, may be empty).
func (a *App) SelectDirectory(current string) (string, error) {
	return application.Get().Dialog.OpenFile().
		SetTitle("Select Directory").
		SetDirectory(dialogStartDir(current)).
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(true).
		PromptForSingleSelection()
}

// SelectFile opens a file picker starting near current (the path in the
// input box, may be empty).
func (a *App) SelectFile(current string) (string, error) {
	return application.Get().Dialog.OpenFile().
		SetTitle("Select File").
		SetDirectory(dialogStartDir(current)).
		CanChooseFiles(true).
		PromptForSingleSelection()
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

	a.applyConfig()
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

// ListGitRefs returns the refs offered in the Git mode dropdowns: common
// refs, branches, tags and recent commits of the repository the app runs in.
// It includes the working tree (Ref "") for the right side to offer.
func (a *App) ListGitRefs() ([]GitRefOption, error) {
	root, err := git.FindRoot()
	if err != nil {
		return nil, err
	}
	refs, err := git.ListRefs(root, true)
	if err != nil {
		return nil, err
	}
	opts := make([]GitRefOption, len(refs))
	for i, r := range refs {
		opts[i] = GitRefOption{Ref: r.Ref, Label: r.Label, Group: r.Group}
	}
	return opts, nil
}

// LoadGitComparison loads a diff between two git refs, or a ref and the
// working tree when rightRef is empty.
func (a *App) LoadGitComparison(leftRef, rightRef string) (*ComparisonResult, error) {
	a.applyConfig()
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
		statuses = git.WithPaths(statuses, a.merges.SavedUnder(root))
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

// applyConfig reads the config file's settings for the next comparison.
func (a *App) applyConfig() {
	cfg, _ := config.Load() // GetSettings reports any problem with the file
	a.fileManager.SetOptions(cfg.FileOptions())
	a.diffOpts = cfg.DifferOptions()
	a.differ = differ.NewWithOptions(a.diffOpts)
	a.hideIdentical = cfg.Compare.HideIdentical
	a.openMu.Lock()
	a.opened = nil
	a.openMu.Unlock()
}

func (a *App) buildComparisonResult() *ComparisonResult {
	var same func(string, string) bool
	if a.diffOpts.Describe() != "" {
		same = a.diffOpts.Same
	}
	a.identical = file.Identical(a.allFiles, same)

	entries := make([]FileEntry, 0, len(a.allFiles))
	backups := a.merges.Backups()
	// Comparing two single files always shows them.
	canHide := a.hideIdentical && (a.gitMode || (a.leftFile != nil && a.rightFile != nil && a.leftFile.IsDir && a.rightFile.IsDir))
	hidden := 0
	for relPath, comp := range a.allFiles {
		if a.isMergeBackup(comp, backups) {
			continue
		}
		entry := FileEntry{
			RelPath:     relPath,
			DisplayName: a.displayNameForFile(relPath),
			Source:      sourceString(comp.Source),
			Conflicted:  a.conflicts[relPath],
			MergeStatus: a.mergeStatusFor(comp),
		}
		if comp.Source == file.SourceBoth {
			entry.Identical = a.identical[relPath]
			if canHide && entry.Identical && entry.MergeStatus == nil && !entry.Conflicted {
				hidden++
				continue
			}
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

		HiddenIdentical: hidden,
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
	opened, err := a.openFile(relPath)
	if err != nil {
		return nil, err
	}
	comp := opened.comp
	equal, inserted, deleted := opened.diff.GetStats()

	return &DiffResult{
		RelPath:     relPath,
		DisplayName: a.displayNameForFile(relPath),
		Source:      sourceString(comp.Source),
		LeftLabel:   opened.diff.LeftFile,
		RightLabel:  opened.diff.RightFile,
		Lines:       toDiffLineDTOs(opened.diff.Lines),
		Equal:       equal,
		Inserted:    inserted,
		Deleted:     deleted,
		CanMerge:    comp.Source == file.SourceBoth,

		LeftEditable:  a.editablePath(comp, "left") != "",
		RightEditable: a.editablePath(comp, "right") != "",
		Conflicted:    a.conflicts[relPath],
		MergeStatus:   a.mergeStatusFor(comp),
		Ignoring:      a.diffOpts.Describe(),
		Token:         opened.token,
	}, nil
}

// GetSideBySide returns the side-by-side rows of an open file. It's separate
// from GetDiff so that the page only gets the text of the view in use.
func (a *App) GetSideBySide(relPath string) ([]SideBySideRowDTO, error) {
	opened := a.findOpened(relPath)
	if opened == nil {
		return nil, fmt.Errorf("%s isn't open", relPath)
	}
	return toSideBySideDTOs(differ.BuildSideBySideRows(opened.diff.Lines)), nil
}

// openedFile is a file as shown: its two sides and their diff. A save works from
// these, so what it writes always matches what was on screen.
type openedFile struct {
	relPath     string
	comp        *file.FileComparison
	left, right string
	diff        *differ.FileDiff
	token       int // says which reading of the file this is
}

// maxOpenedFiles is how many recent files are kept. Only the latest is on screen,
// but the page's calls can overlap, so an older file's call can finish after a
// newer one's. Keeping a few means that can't push out the file on screen.
const maxOpenedFiles = 4

// findOpened returns the kept copy of relPath, or nil.
func (a *App) findOpened(relPath string) *openedFile {
	a.openMu.Lock()
	defer a.openMu.Unlock()
	for _, opened := range a.opened {
		if opened.relPath == relPath {
			return opened
		}
	}
	return nil
}

// openFile reads relPath as it is now, and keeps it.
func (a *App) openFile(relPath string) (*openedFile, error) {
	comp, ok := a.allFiles[relPath]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", relPath)
	}
	left, right, leftLabel, rightLabel := "", "", "<file not found>", "<file not found>"
	var err error
	if comp.LeftFile != nil {
		leftLabel = comp.LeftFile.Path
		if left, err = comp.LeftFile.Text(); err != nil {
			return nil, fmt.Errorf("couldn't read %s, which may have changed since it was loaded. Compare again to refresh: %w", relPath, err)
		}
	}
	if comp.RightFile != nil {
		rightLabel = comp.RightFile.Path
		if right, err = comp.RightFile.Text(); err != nil {
			return nil, fmt.Errorf("couldn't read %s, which may have changed since it was loaded. Compare again to refresh: %w", relPath, err)
		}
	}
	opened := &openedFile{
		relPath: relPath,
		comp:    comp,
		left:    left,
		right:   right,
		diff:    a.differ.CompareStrings(leftLabel, rightLabel, left, right),
	}

	a.openMu.Lock()
	defer a.openMu.Unlock()
	a.openToken++
	opened.token = a.openToken
	kept := a.opened[:0]
	for _, other := range a.opened {
		if other.relPath != relPath {
			kept = append(kept, other)
		}
	}
	a.opened = append(kept, opened)
	if len(a.opened) > maxOpenedFiles {
		a.opened = a.opened[1:]
	}
	return opened, nil
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

// SaveMerge recomputes the diff for relPath, applies the selected changes to
// the target side and saves the result over that side's file, keeping the
// previous version as "<file>.orig". The target must be a real file (not a
// git ref).
func (a *App) SaveMerge(req MergeSaveRequest) (string, error) {
	comp, ok := a.allFiles[req.RelPath]
	if !ok || comp.Source != file.SourceBoth {
		return "", fmt.Errorf("cannot merge: file must exist on both sides")
	}
	if req.Target != "left" && req.Target != "right" {
		return "", fmt.Errorf("invalid merge target: %s", req.Target)
	}
	targetPath := a.editablePath(comp, req.Target)
	if targetPath == "" {
		return "", fmt.Errorf("the %s side is a git revision, not a file on disk", req.Target)
	}

	// Selections are line numbers in the diff on screen, so a save must use that
	// diff. Working one out afresh could give lines different numbers.
	opened := a.findOpened(req.RelPath)
	if opened == nil || (req.Token != 0 && req.Token != opened.token) {
		return "", fmt.Errorf("%s is no longer open as shown, so nothing was saved. Select it and try again", req.RelPath)
	}
	diff := opened.diff

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
	if req.Target == "left" {
		result = a.merger.ApplyToLeft(diff, selection)
	} else {
		result = a.merger.ApplyToRight(diff, selection)
	}

	report, err := merge.SaveMerged(targetPath, result)
	if err != nil {
		return "", err
	}
	a.merges.Record(targetPath, &merge.Record{Report: report, Kind: merge.KindMerge, Applied: result.Applied, Skipped: result.Skipped})
	return fmt.Sprintf("Saved %s (%d change(s) applied, %d skipped); previous version in %s.",
		targetPath, result.Applied, result.Skipped, filepath.Base(report.Backup)) + issueSuffix(report), nil
}

// CheckTargetOnDisk compares one side's file with what this comparison loaded
// and returns "unchanged", "changed" or "missing".
func (a *App) CheckTargetOnDisk(relPath, target string) (string, error) {
	comp, ok := a.allFiles[relPath]
	if !ok || comp.Source != file.SourceBoth {
		return "", fmt.Errorf("%s isn't on both sides", relPath)
	}
	path := a.editablePath(comp, target)
	if path == "" {
		return "", fmt.Errorf("the %s side is not a file on disk", target)
	}
	// Only a file that's been opened has text to compare with.
	opened := a.findOpened(relPath)
	if opened == nil {
		return "", fmt.Errorf("%s isn't open", relPath)
	}
	loaded := opened.right
	if target == "left" {
		loaded = opened.left
	}
	return merge.DiskState(path, loaded)
}

// ---- Merge status ----

func mergeStatusDTO(r *merge.Record) *MergeStatusDTO {
	return &MergeStatusDTO{
		State:   r.State(),
		Kind:    r.Kind,
		Target:  r.Target,
		Backup:  filepath.Base(r.Report.Backup),
		Applied: r.Applied,
		Skipped: r.Skipped,
		Issues:  r.Report.Issues,
	}
}

func issueSuffix(report *merge.SaveReport) string {
	if len(report.Issues) == 0 {
		return ""
	}
	return " Check: " + strings.Join(report.Issues, "; ") + "."
}

// mergeStatusFor returns the status of a comparison's saved sides, or nil.
func (a *App) mergeStatusFor(comp *file.FileComparison) *MergeStatusDTO {
	rec := a.merges.StatusOf(a.editablePath(comp, "left"), a.editablePath(comp, "right"))
	if rec == nil {
		return nil
	}
	return mergeStatusDTO(rec)
}

// isMergeBackup reports whether every file in comp is a backup from backups,
// so that saving a merge doesn't add a row to the file list.
func (a *App) isMergeBackup(comp *file.FileComparison, backups map[string]bool) bool {
	var paths []string
	if comp.LeftFile != nil {
		paths = append(paths, a.editablePath(comp, "left"))
	}
	if comp.RightFile != nil {
		paths = append(paths, a.editablePath(comp, "right"))
	}
	return merge.AllBackups(paths, backups)
}

// ---- Conflicts ----

// ConflictBlockDTO is one conflict in a working-tree file.
type ConflictBlockDTO struct {
	StartLine   int      `json:"startLine"`
	OursLabel   string   `json:"oursLabel"`
	TheirsLabel string   `json:"theirsLabel"`
	BaseLabel   string   `json:"baseLabel"`
	Ours        []string `json:"ours"`
	Base        []string `json:"base"`
	Theirs      []string `json:"theirs"`
	Before      []string `json:"before"` // a few unchanged lines above, for context
	After       []string `json:"after"`
}

// conflictFile returns the on-disk path of a conflicted file in git mode.
func (a *App) conflictFile(relPath string) (string, error) {
	if !a.gitMode || !a.conflicts[relPath] {
		return "", fmt.Errorf("%s has no merge conflicts", relPath)
	}
	return filepath.Join(a.gitRoot, relPath), nil
}

// GetConflicts parses the conflict markers in a conflicted working-tree file.
func (a *App) GetConflicts(relPath string) ([]ConflictBlockDTO, error) {
	path, err := a.conflictFile(relPath)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f := conflict.Parse(string(content))
	blocks := make([]ConflictBlockDTO, len(f.Blocks))
	for i, b := range f.Blocks {
		before, after := f.Context(i, 3)
		blocks[i] = ConflictBlockDTO{
			StartLine: b.StartLine, OursLabel: b.OursLabel, TheirsLabel: b.TheirsLabel, BaseLabel: b.BaseLabel,
			Ours: b.Ours, Base: b.Base, Theirs: b.Theirs, Before: before, After: after,
		}
	}
	return blocks, nil
}

// SaveConflicts writes the file with each block resolved by choices[i]
// ("ours", "theirs", "both", "both-theirs-first", or "" to leave it). With
// markResolved, and no conflicts left, it also stages the file (git add).
// The file list should be reloaded afterwards.
func (a *App) SaveConflicts(relPath string, choices []string, markResolved bool) (string, error) {
	path, err := a.conflictFile(relPath)
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	f := conflict.Parse(string(content))
	if len(choices) != len(f.Blocks) {
		return "", fmt.Errorf("%s changed on disk (now %d conflicts, expected %d) — reload and try again",
			relPath, len(f.Blocks), len(choices))
	}
	parsed := make([]conflict.Choice, len(choices))
	for i, c := range choices {
		parsed[i] = conflict.ParseChoice(c)
	}
	remaining := conflict.Remaining(len(f.Blocks), parsed)
	if markResolved && remaining > 0 {
		return "", fmt.Errorf("%d conflict(s) still unresolved", remaining)
	}
	report, err := merge.SaveVerified(path, f.Render(parsed))
	if err != nil {
		return "", fmt.Errorf("failed to save %s: %w", relPath, err)
	}
	a.merges.Record(path, &merge.Record{Report: report, Kind: merge.KindConflicts})
	if markResolved {
		if err := git.MarkResolved(a.gitRoot, relPath); err != nil {
			return "", err
		}
		return fmt.Sprintf("Resolved %s and staged it (git add).", relPath) + issueSuffix(report), nil
	}
	if remaining > 0 {
		return fmt.Sprintf("Saved %s — %d conflict(s) left.", relPath, remaining) + issueSuffix(report), nil
	}
	return fmt.Sprintf("Saved %s — no conflicts left; mark it resolved to stage it.", relPath) + issueSuffix(report), nil
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
		text, err := srcFile.Text()
		if err == nil {
			err = os.WriteFile(dstPath, []byte(text), 0644)
		}
		if err != nil {
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

// Settings are the app's preferences, from the config file.
type Settings struct {
	AutoCollapsePaths  bool    `json:"autoCollapsePaths"`
	DiffFontSize       float64 `json:"diffFontSize"`
	DefaultView        string  `json:"defaultView"`
	DefaultMergeTarget string  `json:"defaultMergeTarget"`
	ConfigPath         string  `json:"configPath"`
	ConfigError        string  `json:"configError"` // why the file couldn't be used
}

// GetSettings reads the config file. It's called when the settings are needed,
// so a change applies without restarting the app.
func (a *App) GetSettings() Settings {
	var s Settings
	s.ConfigPath, _ = config.Path()
	cfg, err := config.Load()
	if err != nil {
		s.ConfigError = err.Error()
	}
	s.AutoCollapsePaths = cfg.GUI.AutoCollapsePaths
	s.DiffFontSize = cfg.GUI.DiffFontSize
	s.DefaultView = cfg.View.DefaultView
	s.DefaultMergeTarget = cfg.View.DefaultMergeTarget
	return s
}

// GetVersion returns the build version, shown in the help dialog.
func (a *App) GetVersion() string {
	return version
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
// GUI mode only launches GUI tools, so any console of their own (git, cmd,
// sh wrappers on Windows) is hidden.
func startDetached(cmd *exec.Cmd) error {
	proc.HideConsole(cmd)
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
	cfg, _ := config.Load() // read each time, so a change needs no restart
	cmd, err := external.ConfiguredEditorCommand(cfg.Tools.Editor, path, line, true)
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
	proc.HideConsole(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to launch git mergetool: %w", err)
	}
	go func() {
		err := cmd.Wait()
		msg := ""
		if err != nil {
			msg = err.Error()
		}
		application.Get().Event.Emit(conflictResolvedEvent, []string{relPath, msg})
	}()
	return nil
}
