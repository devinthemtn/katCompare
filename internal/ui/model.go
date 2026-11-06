package ui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"katcompare/internal/config"
	"katcompare/internal/conflict"
	"katcompare/internal/differ"
	"katcompare/internal/external"
	"katcompare/internal/file"
	"katcompare/internal/git"
	"katcompare/internal/merge"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ViewMode represents the current view state
type ViewMode int

const (
	// ViewModeFileSelect and ViewModeDiff are the two panes of the main screen,
	// and say which has focus. Merge and conflict views use the diff's pane.
	ViewModeFileSelect ViewMode = iota
	ViewModeDiff
	ViewModeMerge
	ViewModeCopy
	ViewModeHelp
	ViewModeGitSelect
	ViewModeConflict
	ViewModeOpen // choosing the paths to compare
)

// DiffViewMode represents the diff display mode
type DiffViewMode int

const (
	DiffViewUnified DiffViewMode = iota
	DiffViewSideBySide
)

// fileSelectFocus represents which control has keyboard focus on the open screen
type fileSelectFocus int

const (
	focusInputLeft fileSelectFocus = iota
	focusInputRight
	focusViewDiffBtn
	focusGitBtn
	focusQuitBtn
)

// Model represents the main application state
type Model struct {
	// Application state
	viewMode     ViewMode
	windowWidth  int
	windowHeight int

	// File selection
	leftPath       string
	rightPath      string
	leftFile       *file.FileInfo
	rightFile      *file.FileInfo
	allFiles       map[string]*file.FileComparison
	selectedFile   string
	fileListScroll int
	sorted         []string
	sortedFor      sortedKey

	// Diff view
	diffFile      string // the file currentDiff belongs to
	diffCache     map[string]*cachedDiff
	diffOrder     []string    // the cache's files, oldest first
	openDiff      *cachedDiff // the file on screen
	diffErr       error       // why the selected file couldn't be read
	diffGen       int         // counts comparison loads; older background results are dropped
	previewSeq    int         // identifies the latest delayed diff load
	currentDiff   *differ.FileDiff
	sbsRows       []differ.SideBySideRow // precomputed side-by-side rows (cached from currentDiff)
	scrollOffset  int
	hScrollOffset int // horizontal scroll for side-by-side view
	cursor        int
	diffViewMode  DiffViewMode

	// Merge view
	changeSelection *merge.ChangeSelection
	mergeTarget     string // "left" or "right"
	targetDisk      string // the target on disk vs as loaded: merge.Disk*
	overwriteArmed  bool   // warned about outside edits; the next s overwrites

	// Conflict view: resolving git conflict markers in the selected file
	conflictFile    *conflict.File
	conflictChoices []conflict.Choice
	conflictIdx     int

	// Copy view
	copySelection map[string]bool // Maps relative path to whether to copy
	copyTarget    string          // "to-left" or "to-right"

	// Merge status: saves made this session, and what they mean for each file
	tracker     *merge.Tracker
	statuses    map[string]*merge.Record
	identical   map[string]bool // files on both sides with no differences to show
	differCount int

	// Settings for the current comparison, read when it's loaded and kept until
	// the next one, so the diff on screen is always the one a merge saves.
	diffOpts        differ.Options
	hideIdentical   bool
	hiddenIdentical int    // files left out of the list by hide_identical
	configWarning   string // why the config file couldn't be used

	// Services
	fileManager *file.Manager
	differ      *differ.Differ
	merger      *merge.Merger

	// UI state
	inputLeft  string
	inputRight string
	focus      fileSelectFocus
	helpScroll int
	helpReturn ViewMode // the view to go back to from help
	errorMsg   string
	statusMsg  string // result message, shown until the next key press
	statusWarn bool   // statusMsg reports a problem to look at

	// Path suggestions
	leftSuggestions  []string
	rightSuggestions []string
	leftSuggIndex    int
	rightSuggIndex   int
	showSuggestions  bool

	// File list filter
	filterQuery  string
	filterTyping bool

	// Git comparison (for reloading and launching git difftool/mergetool)
	gitMode   bool
	gitRoot   string
	leftRef   string
	rightRef  string
	conflicts map[string]bool

	// Git ref picker (ViewModeGitSelect)
	gitRefs      []git.RefOption
	gitSelInput  [2]string // left, right; right "" means the working tree
	gitSelFocus  int       // 0 = left ref, 1 = right ref
	gitSelIndex  int       // highlighted option in the filtered list, -1 for none
	gitSelEdited bool      // the focused input was typed in, so filter the list by it
}

// externalDoneMsg is sent when an external editor, diff tool or merge tool
// launched with tea.ExecProcess exits.
type externalDoneMsg struct {
	kind string // "edit" | "diff" | "merge"
	err  error
}

// sortedKey says what a kept list of sorted files was made from. diffGen changes
// on every load, and the count catches rows pruned afterwards.
type sortedKey struct {
	query string
	count int
	gen   int
}

// previewMsg loads the selected file's diff after a short delay, so moving
// through a list of large files doesn't diff each one on the way past.
type previewMsg struct{ seq int }

// diffReadyMsg delivers a diff computed in the background.
type diffReadyMsg struct {
	relPath string
	gen     int // the comparison it was computed from
	cached  *cachedDiff
}

// previewDelay and previewNoDelayBytes control when a diff load waits. Up to the
// limit, reading and diffing a file takes well under the time between key
// repeats. Past it, a slow disk could be noticeable, so it runs in the background.
const (
	previewDelay        = 80 * time.Millisecond
	previewNoDelayBytes = 128 * 1024
)

// cachedDiff is a file's two sides and their diff. The file on screen stays in
// openDiff, so the disk check and a save work from what's shown. Others are kept
// up to maxCachedBytes, so moving back and forth through a list stays quick.
type cachedDiff struct {
	left, right string
	diff        *differ.FileDiff
	rows        []differ.SideBySideRow
	err         error
	stamp       string // which version of the files this is
}

// diffStamp identifies the versions of a comparison's files on disk.
func diffStamp(comp *file.FileComparison) string {
	var left, right string
	if comp.LeftFile != nil {
		left = comp.LeftFile.Stamp()
	}
	if comp.RightFile != nil {
		right = comp.RightFile.Stamp()
	}
	return left + "|" + right
}

// maxCachedBytes limits the text held in the diff cache, on top of the open file.
var maxCachedBytes = 64 << 20

// cacheDiff keeps a diff, dropping the oldest ones past the limit.
func (m *Model) cacheDiff(relPath string, cached *cachedDiff) {
	if m.diffCache == nil {
		m.diffCache = make(map[string]*cachedDiff)
	}
	if _, exists := m.diffCache[relPath]; !exists {
		m.diffOrder = append(m.diffOrder, relPath)
	}
	m.diffCache[relPath] = cached
	total := 0
	for _, c := range m.diffCache {
		total += len(c.left) + len(c.right)
	}
	for total > maxCachedBytes && len(m.diffOrder) > 0 {
		oldest := m.diffOrder[0]
		m.diffOrder = m.diffOrder[1:]
		if c := m.diffCache[oldest]; c != nil {
			total -= len(c.left) + len(c.right)
			delete(m.diffCache, oldest)
		}
	}
}

// Version is the build version shown in the help screen; set by main.
var Version = "dev"

// Styles for the UI. Colors adapt to light and dark terminals.
var (
	accentColor = lipgloss.Color("#7D56F4")
	mutedColor  = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	greenColor  = lipgloss.AdaptiveColor{Light: "#047857", Dark: "#34D399"}
	yellowColor = lipgloss.AdaptiveColor{Light: "#A16207", Dark: "#FACC15"}
	blueColor   = lipgloss.AdaptiveColor{Light: "#1D4ED8", Dark: "#60A5FA"}
	orangeColor = lipgloss.AdaptiveColor{Light: "#C2410C", Dark: "#FB923C"}
	redColor    = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#F87171"}

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(accentColor).
			Bold(true)

	headerBarStyle = titleStyle

	paneBorderStyle        = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#9CA3AF", Dark: "#4B5563"})
	paneFocusedBorderStyle = lipgloss.NewStyle().Foreground(accentColor).Bold(true)

	inputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#111827", Dark: "#F3F4F6"}).
			Background(lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#374151"})

	focusedInputStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(accentColor)

	buttonStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#111827", Dark: "#E5E7EB"}).
			Background(lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#374151"}).
			Bold(true).
			Padding(0, 2)

	focusedButtonStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(accentColor).
				Bold(true).
				Padding(0, 2)

	equalLineStyle = lipgloss.NewStyle().Foreground(mutedColor)

	insertLineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#1E3A8A", Dark: "#DBEAFE"}).
			Background(lipgloss.AdaptiveColor{Light: "#DBEAFE", Dark: "#1E3A5F"})

	deleteLineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#7F1D1D", Dark: "#FECACA"}).
			Background(lipgloss.AdaptiveColor{Light: "#FEE2E2", Dark: "#5B1F24"})

	errorStyle = lipgloss.NewStyle().Foreground(redColor).Bold(true)
	warnStyle  = lipgloss.NewStyle().Foreground(yellowColor).Bold(true)

	helpStyle = lipgloss.NewStyle().Foreground(mutedColor).Italic(true)

	keyNameStyle   = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
	keyActionStyle = lipgloss.NewStyle().Foreground(mutedColor)

	selectedFileStyle = lipgloss.NewStyle().
				Background(accentColor).
				Foreground(lipgloss.Color("#FFFFFF")).
				Bold(true)

	selectedDimStyle = lipgloss.NewStyle().
				Background(lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#374151"}).
				Foreground(lipgloss.AdaptiveColor{Light: "#111827", Dark: "#F3F4F6"})

	selectedSuggestionStyle = selectedDimStyle

	suggestionStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(mutedColor).
			Padding(0, 1)

	// Empty cell in side-by-side view (the blank side of an insert or delete row)
	emptyDiffStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Background(lipgloss.AdaptiveColor{Light: "#F3F4F6", Dark: "#1F2937"})

	unsavedStyle = lipgloss.NewStyle().Foreground(blueColor).Bold(true)

	// A ticked change has its insert or delete background. An unticked one loses
	// the background but keeps a matching text color, so it stays easy to read.
	unselectedInsertStyle = lipgloss.NewStyle().Foreground(blueColor)
	unselectedDeleteStyle = lipgloss.NewStyle().Foreground(redColor)

	// File list
	identicalStyle = lipgloss.NewStyle().Foreground(greenColor)
	differentStyle = lipgloss.NewStyle().Foreground(redColor).Bold(true)
	leftOnlyStyle  = lipgloss.NewStyle().Foreground(blueColor).Bold(true)
	rightOnlyStyle = lipgloss.NewStyle().Foreground(orangeColor).Bold(true)
)

// New creates a new model
func New() *Model {
	m := &Model{
		viewMode:       ViewModeOpen,
		fileManager:    file.New(),
		differ:         differ.New(),
		merger:         merge.New(),
		tracker:        merge.NewTracker(),
		focus:          focusInputLeft,
		allFiles:       make(map[string]*file.FileComparison),
		leftSuggIndex:  -1,
		rightSuggIndex: -1,
		mergeTarget:    "left",
		targetDisk:     merge.DiskUnchanged,
		copySelection:  make(map[string]bool),
		copyTarget:     "to-right",
		diffViewMode:   DiffViewUnified,
	}
	m.applyConfig()
	cfg, _ := config.Load()
	m.mergeTarget = cfg.View.DefaultMergeTarget
	if cfg.View.DefaultView == config.ViewSideBySide {
		m.diffViewMode = DiffViewSideBySide
	}
	return m
}

// applyConfig reads the config file's settings for the next comparison.
func (m *Model) applyConfig() {
	cfg, err := config.Load()
	m.configWarning = ""
	if err != nil {
		m.configWarning = "Using default settings. Problem in config.toml: " + err.Error()
	}
	m.fileManager.SetOptions(cfg.FileOptions())
	m.diffOpts = cfg.DifferOptions()
	m.differ = differ.NewWithOptions(m.diffOpts)
	m.hideIdentical = cfg.Compare.HideIdentical
}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		first := m.windowWidth == 0
		m.windowWidth = msg.Width
		m.windowHeight = msg.Height
		if m.browsing() {
			m.ensureDiffLoaded()
		}
		if first && m.currentDiff != nil {
			// Positioned before the size was known: put the change in view.
			m.scrollOffset = max(0, m.cursor-m.diffViewHeight()/4)
		}
		return m, nil

	case tea.FocusMsg:
		// Back from another window: files may have changed.
		m.refreshStatuses()
		if m.viewMode == ViewModeMerge {
			m.refreshTargetDisk()
		}
		return m, nil

	case tea.KeyMsg:
		// Keys pressed quickly can arrive as one message. Only a text field
		// wants them together.
		if msg.Type == tea.KeyRunes && !msg.Paste && len(msg.Runes) > 1 && !m.typingText() {
			var cmds []tea.Cmd
			for _, r := range msg.Runes {
				_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}, Alt: msg.Alt})
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
		return m.handleKeyPress(msg)

	case externalDoneMsg:
		m.handleExternalDone(msg)
		return m, nil

	case previewMsg:
		if msg.seq == m.previewSeq && m.browsing() && m.diffFile != m.selectedFile {
			return m, m.computeDiffCmd(m.selectedFile)
		}
		return m, nil

	case diffReadyMsg:
		if msg.gen != m.diffGen {
			return m, nil
		}
		m.cacheDiff(msg.relPath, msg.cached)
		if msg.relPath == m.selectedFile && m.browsing() {
			m.ensureDiffLoaded()
		}
		return m, nil

	case copyDoneMsg:
		m.viewMode = ViewModeFileSelect
		m.reloadComparison()
		if msg.err != "" {
			m.errorMsg = msg.err
		}
		m.statusMsg = msg.text
		return m, nil

	case saveDoneMsg:
		if msg.err != nil {
			m.errorMsg = msg.err.Error()
			return m, nil
		}
		m.tracker.Record(msg.path, msg.record)
		// The target file changed: show the refreshed diff, with nothing left
		// to save.
		m.reloadComparison()
		if m.viewMode == ViewModeMerge {
			m.changeSelection.SelectNone(m.currentDiff)
			m.refreshTargetDisk()
		}
		m.statusMsg = msg.text
		m.statusWarn = len(msg.record.Report.Issues) > 0
		return m, nil
	}

	return m, nil
}

// typingText reports whether key presses are going into a text field.
func (m *Model) typingText() bool {
	switch m.viewMode {
	case ViewModeOpen:
		return m.focusedInput() != nil
	case ViewModeGitSelect:
		return true
	case ViewModeFileSelect:
		return m.filterTyping
	}
	return false
}

// browsing reports whether the file list and a right-hand pane are on screen.
func (m *Model) browsing() bool {
	switch m.viewMode {
	case ViewModeFileSelect, ViewModeDiff, ViewModeMerge, ViewModeConflict:
		return true
	}
	return false
}

// View renders the current view
func (m *Model) View() string {
	// Layout depends on the terminal size, which arrives in the first
	// WindowSizeMsg - render nothing until then.
	if m.windowWidth == 0 {
		return ""
	}

	var out string
	switch m.viewMode {
	case ViewModeOpen:
		out = m.renderOpenView()
	case ViewModeFileSelect, ViewModeDiff, ViewModeMerge, ViewModeConflict:
		out = m.renderBrowseView()
	case ViewModeCopy:
		out = m.renderCopyView()
	case ViewModeHelp:
		out = m.renderHelpView()
	case ViewModeGitSelect:
		out = m.renderGitSelectView()
	default:
		out = "Unknown view mode"
	}
	return fit(out, m.windowWidth, m.windowHeight)
}

// handleKeyPress processes keyboard input
func (m *Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.viewMode {
	case ViewModeOpen:
		return m.handleOpenKeys(msg)
	case ViewModeFileSelect:
		return m.handleFilesKeys(msg)
	case ViewModeDiff:
		return m.handleDiffKeys(msg)
	case ViewModeMerge:
		return m.handleMergeKeys(msg)
	case ViewModeCopy:
		return m.handleCopyKeys(msg)
	case ViewModeHelp:
		return m.handleHelpKeys(msg)
	case ViewModeGitSelect:
		return m.handleGitSelectKeys(msg)
	case ViewModeConflict:
		return m.handleConflictKeys(msg)
	}
	return m, nil
}

// clearMessages ends the messages shown until the next key press.
func (m *Model) clearMessages() {
	m.errorMsg, m.statusMsg, m.statusWarn = "", "", false
}

// textOf returns the text a key press types, whether a single key or a paste,
// and whether it typed anything at all.
func textOf(msg tea.KeyMsg) (string, bool) {
	if msg.Alt || (msg.Type != tea.KeyRunes && msg.Type != tea.KeySpace) {
		return "", false
	}
	return strings.TrimRight(strings.ReplaceAll(string(msg.Runes), "\r", ""), "\n"), true
}

func (m *Model) openHelp() {
	m.helpReturn = m.viewMode
	m.helpScroll = 0
	m.viewMode = ViewModeHelp
}

// openPaths goes to the open screen.
func (m *Model) openPaths() {
	if m.gitMode {
		m.inputLeft, m.inputRight = "", ""
	}
	m.focus = focusInputLeft
	m.clearSuggestions()
	m.viewMode = ViewModeOpen
}

// focusedInput returns the text field with focus, or nil on a button.
func (m *Model) focusedInput() *string {
	switch m.focus {
	case focusInputLeft:
		return &m.inputLeft
	case focusInputRight:
		return &m.inputRight
	}
	return nil
}

// handleOpenKeys handles keys on the open screen. Anything typed goes into the
// focused field, so ordinary keys like q are only commands on the buttons.
func (m *Model) handleOpenKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.clearMessages()
	input := m.focusedInput()

	if text, ok := textOf(msg); ok && input != nil {
		*input += text
		m.updateSuggestions()
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "q":
		if input == nil {
			return m, tea.Quit
		}
	case "?", "f1":
		m.openHelp()
	case "ctrl+g", "f2": // F2 because Zellij, for one, keeps Ctrl+G for itself
		m.openGitSelect()
	case "ctrl+d":
		m.comparePaths()
	case "tab":
		if !m.cycleSuggestion(1) {
			m.moveFocus(1)
		}
	case "shift+tab":
		m.moveFocus(-1)
	case "up":
		m.cycleSuggestion(-1)
	case "down":
		m.cycleSuggestion(1)
	case "backspace":
		if input != nil && *input != "" {
			*input = dropLastRune(*input)
			m.updateSuggestions()
		}
	case "esc":
		if m.showSuggestions {
			m.clearSuggestions()
		} else if len(m.allFiles) > 0 {
			m.viewMode = ViewModeFileSelect
		}
	case " ":
		return m.pressOpenButton()
	case "enter":
		if input == nil {
			return m.pressOpenButton()
		}
		if suggestion, ok := m.currentSuggestion(); ok && suggestion != *input {
			*input = suggestion
			m.clearSuggestions()
			return m, nil
		}
		m.clearSuggestions()
		other := m.inputRight
		if m.focus == focusInputRight {
			other = m.inputLeft
		}
		if *input != "" && other != "" {
			m.comparePaths()
		} else if *input != "" {
			m.moveFocus(1)
			if m.focus == focusViewDiffBtn { // on the right, with the left empty
				m.focus = focusInputLeft
			}
		}
	}
	return m, nil
}

func (m *Model) pressOpenButton() (tea.Model, tea.Cmd) {
	switch m.focus {
	case focusQuitBtn:
		return m, tea.Quit
	case focusViewDiffBtn:
		m.comparePaths()
	case focusGitBtn:
		m.openGitSelect()
	}
	return m, nil
}

// moveFocus moves through left field, right field, Compare, Git and Quit.
func (m *Model) moveFocus(dir int) {
	const controls = 5
	m.focus = fileSelectFocus((int(m.focus) + dir + controls) % controls)
	m.clearSuggestions()
}

// suggestionList returns the focused field's suggestions and their index.
func (m *Model) suggestionList() ([]string, *int) {
	if m.focus == focusInputRight {
		return m.rightSuggestions, &m.rightSuggIndex
	}
	return m.leftSuggestions, &m.leftSuggIndex
}

func (m *Model) currentSuggestion() (string, bool) {
	if !m.showSuggestions || m.focusedInput() == nil {
		return "", false
	}
	items, index := m.suggestionList()
	if *index < 0 || *index >= len(items) {
		return "", false
	}
	return items[*index], true
}

// cycleSuggestion moves through the suggestions, and reports whether there were
// any. A single suggestion has nowhere to cycle to, so Tab completes it instead.
func (m *Model) cycleSuggestion(dir int) bool {
	input := m.focusedInput()
	if !m.showSuggestions || input == nil {
		return false
	}
	items, index := m.suggestionList()
	if len(items) == 0 {
		return false
	}
	if len(items) == 1 {
		*input = items[0]
		m.clearSuggestions()
		return true
	}
	*index = (*index + dir + len(items)) % len(items)
	return true
}

// comparePaths loads the two paths on the open screen and moves on to the file
// list, or stays with an error.
func (m *Model) comparePaths() {
	left, right := strings.TrimSpace(m.inputLeft), strings.TrimSpace(m.inputRight)
	m.clearSuggestions()
	if left == "" || right == "" {
		m.errorMsg = "Enter both a left and a right path first"
		return
	}
	m.applyConfig()
	leftFile, err := m.fileManager.LoadPath(left)
	if err != nil {
		m.errorMsg = fmt.Sprintf("Left path: %s", err)
		return
	}
	rightFile, err := m.fileManager.LoadPath(right)
	if err != nil {
		m.errorMsg = fmt.Sprintf("Right path: %s", err)
		return
	}
	m.leftPath, m.rightPath, m.leftFile, m.rightFile = left, right, leftFile, rightFile
	m.filterQuery, m.filterTyping = "", false
	m.updateCommonFiles()
	m.OpenDiffIfFiles()
}

// selectionChanged loads the newly selected file's diff for the right-hand
// pane: at once if it's quick, otherwise after a short delay.
func (m *Model) selectionChanged() tea.Cmd {
	if !m.wide() || m.selectedFile == "" {
		return nil
	}
	m.previewSeq++
	comp, ok := m.allFiles[m.selectedFile]
	if !ok {
		return nil
	}
	size := int64(0)
	if comp.LeftFile != nil {
		size += comp.LeftFile.Size
	}
	if comp.RightFile != nil {
		size += comp.RightFile.Size
	}
	if _, cached := m.diffCache[m.selectedFile]; cached || size < previewNoDelayBytes {
		m.ensureDiffLoaded()
		return nil
	}
	seq := m.previewSeq
	return tea.Tick(previewDelay, func(time.Time) tea.Msg { return previewMsg{seq: seq} })
}

// ensureDiffLoaded loads the selected file's diff unless it's already showing.
func (m *Model) ensureDiffLoaded() {
	if m.selectedFile != "" && (m.currentDiff == nil || m.diffFile != m.selectedFile) {
		m.loadDiff()
	}
}

// focusDiff moves focus to the diff pane.
func (m *Model) focusDiff() {
	if m.selectedFile == "" {
		return
	}
	m.ensureDiffLoaded()
	m.viewMode = ViewModeDiff
}

// syncSelectionToFilter keeps the selection inside the filtered list.
func (m *Model) syncSelectionToFilter() {
	files := m.getSortedFiles()
	for _, f := range files {
		if f == m.selectedFile {
			return
		}
	}
	m.selectedFile = ""
	if len(files) > 0 {
		m.selectedFile = files[0]
	}
}

// moveSelection moves the file list's selection by delta rows, stopping at the
// ends.
func (m *Model) moveSelection(delta int) tea.Cmd {
	files := m.getSortedFiles()
	if len(files) == 0 {
		return nil
	}
	index := 0
	for i, f := range files {
		if f == m.selectedFile {
			index = i
			break
		}
	}
	index = max(0, min(index+delta, len(files)-1))
	if files[index] == m.selectedFile {
		return nil
	}
	m.selectedFile = files[index]
	return m.selectionChanged()
}

// handleFilesKeys handles keys when the file list has focus.
func (m *Model) handleFilesKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.clearMessages()

	if m.filterTyping {
		if text, ok := textOf(msg); ok {
			m.filterQuery += text
			m.syncSelectionToFilter()
			return m, m.selectionChanged()
		}
		switch msg.String() {
		case "enter":
			m.filterTyping = false
			return m, nil
		case "backspace":
			m.filterQuery = dropLastRune(m.filterQuery)
			m.syncSelectionToFilter()
			return m, m.selectionChanged()
		case "esc":
			m.clearFilter()
			return m, m.selectionChanged()
		case "ctrl+c":
			return m, tea.Quit
		case "up":
			return m, m.moveSelection(-1)
		case "down":
			return m, m.moveSelection(1)
		case "ctrl+d":
			m.filterTyping = false
			m.focusDiff()
		}
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "?", "f1":
		m.openHelp()
	case "up", "k":
		return m, m.moveSelection(-1)
	case "down", "j":
		return m, m.moveSelection(1)
	case "pgup":
		return m, m.moveSelection(-m.paneInnerHeight())
	case "pgdown":
		return m, m.moveSelection(m.paneInnerHeight())
	case "g", "home":
		return m, m.moveSelection(-len(m.allFiles))
	case "G", "end":
		return m, m.moveSelection(len(m.allFiles))
	case "enter", "tab", "ctrl+d":
		m.focusDiff()
	case "/":
		m.filterQuery, m.filterTyping = "", true
	case "esc":
		m.clearFilter()
		return m, m.selectionChanged()
	case "o":
		m.openPaths()
	case "ctrl+g", "f2": // F2 because Zellij, for one, keeps Ctrl+G for itself
		m.openGitSelect()
	case "r":
		m.reload()
	case "x":
		m.swapSides()
	case "c":
		m.enterCopyMode()
	case "F":
		return m, m.openDirsInDiffTool()
	case "m", "s", "e", "E", "D", "R", "n", "p":
		// Keys that act on the selected file work from here too.
		if m.selectedFile != "" {
			m.ensureDiffLoaded()
			return m.handleDiffKeys(msg)
		}
	case "]", "[":
		if m.selectedFile != "" {
			m.focusDiff()
			return m.handleDiffKeys(msg)
		}
	}
	return m, nil
}

// clearFilter removes the filter, selecting a file again if none matched.
func (m *Model) clearFilter() {
	m.filterQuery, m.filterTyping = "", false
	m.syncSelectionToFilter()
}

// reload re-reads the comparison at the user's request.
func (m *Model) reload() {
	m.reloadComparison()
	if m.errorMsg == "" {
		m.statusMsg = "Reloaded from disk"
	}
}

// swapSides swaps left and right, keeping the selected file.
func (m *Model) swapSides() {
	if m.gitMode {
		if m.rightRef == "" {
			m.errorMsg = "Can't swap: the working tree can only be on the right"
			return
		}
		left, right := m.leftRef, m.rightRef
		m.leftRef, m.rightRef = right, left
		m.reloadComparison()
		if m.errorMsg != "" {
			m.leftRef, m.rightRef = left, right
			return
		}
		m.statusMsg = "Swapped left and right"
		return
	} else {
		if m.leftFile == nil || m.rightFile == nil {
			return
		}
		m.leftPath, m.rightPath = m.rightPath, m.leftPath
		m.inputLeft, m.inputRight = m.inputRight, m.inputLeft
		if m.mergeTarget == "left" {
			m.mergeTarget = "right"
		} else {
			m.mergeTarget = "left"
		}
	}
	m.reloadComparison()
	if m.selectedFile != "" {
		m.loadDiff()
	}
	if m.errorMsg == "" {
		m.statusMsg = "Swapped left and right"
	}
}

// openDirsInDiffTool opens both compared folders in the external diff tool.
func (m *Model) openDirsInDiffTool() tea.Cmd {
	if m.gitMode || m.leftFile == nil || m.rightFile == nil || !m.leftFile.IsDir || !m.rightFile.IsDir {
		m.errorMsg = "Opening folders in the diff tool needs two directories"
		return nil
	}
	cmd, err := external.DirDiffCommand(m.leftFile.Path, m.rightFile.Path, false)
	if err != nil {
		m.errorMsg = err.Error()
		return nil
	}
	return runExternal("diff", cmd)
}

// moveCursor moves the diff cursor by delta rows, stopping at the ends.
func (m *Model) moveCursor(delta int) {
	m.cursor = max(0, min(m.cursor+delta, m.maxDiffLines()-1))
}

// switchDiffView toggles unified and side-by-side, staying on the same line.
func (m *Model) switchDiffView() {
	line := m.cursorLineFor("right")
	if m.diffViewMode == DiffViewUnified {
		m.diffViewMode = DiffViewSideBySide
	} else {
		m.diffViewMode = DiffViewUnified
	}
	m.hScrollOffset = 0

	target := -1
	if line > 0 && m.diffViewMode == DiffViewSideBySide {
		for i, r := range m.sbsRows {
			if r.RightLineNum == line {
				target = i
				break
			}
		}
	} else if line > 0 && m.currentDiff != nil {
		for i, l := range m.currentDiff.Lines {
			if l.RightLineNum == line {
				target = i
				break
			}
		}
	}
	if target < 0 {
		m.moveToFirstChange()
		return
	}
	m.cursor = target
	m.scrollOffset = max(0, target-m.diffViewHeight()/4)
}

// enterMergeView opens the merge view or the conflict resolver for the file.
func (m *Model) enterMergeView() {
	if m.gitMode && m.conflicts[m.selectedFile] {
		m.openConflictView()
		return
	}
	comp, exists := m.allFiles[m.selectedFile]
	if !exists || m.currentDiff == nil {
		return
	}
	switch comp.Source {
	case file.SourceLeft:
		m.errorMsg = "Can't merge: this file exists only on the left. Use c to copy it"
		return
	case file.SourceRight:
		m.errorMsg = "Can't merge: this file exists only on the right. Use c to copy it"
		return
	}
	if m.identical[m.selectedFile] {
		m.errorMsg = "These files are identical, so there's nothing to merge"
		if ignoring := m.diffOpts.Describe(); ignoring != "" {
			m.errorMsg = "These files are the same when ignoring " + ignoring + ", so there's nothing to merge"
		}
		return
	}
	m.viewMode = ViewModeMerge
	m.initializeMergeMode()
}

// handleDiffKeys handles keys when the diff pane has focus
func (m *Model) handleDiffKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.clearMessages()
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "esc", "tab":
		m.viewMode = ViewModeFileSelect
		return m, nil

	case "up", "k":
		m.moveCursor(-1)
		return m, nil

	case "down", "j":
		m.moveCursor(1)
		return m, nil

	case "pgup":
		m.moveCursor(-m.diffViewHeight())
		return m, nil

	case "pgdown":
		m.moveCursor(m.diffViewHeight())
		return m, nil

	case "left", "h":
		m.hScrollOffset = max(0, m.hScrollOffset-4)
		return m, nil

	case "right", "l":
		m.hScrollOffset += 4
		return m, nil

	case "g", "home":
		m.cursor = 0
		m.scrollOffset = 0
		return m, nil

	case "G", "end":
		m.cursor = max(0, m.maxDiffLines()-1)
		return m, nil

	case "]":
		m.jumpToChange(1)
		return m, nil

	case "[":
		m.jumpToChange(-1)
		return m, nil

	case "n":
		m.selectNextFile()
		m.loadDiff()
		return m, nil

	case "p":
		m.selectPreviousFile()
		m.loadDiff()
		return m, nil

	case "?", "f1":
		m.openHelp()
		return m, nil

	case "/":
		m.viewMode = ViewModeFileSelect
		m.filterQuery, m.filterTyping = "", true
		return m, nil

	case "s":
		m.switchDiffView()
		return m, nil

	case "m":
		m.enterMergeView()
		return m, nil

	case "c":
		m.enterCopyMode()
		return m, nil

	case "r":
		m.reload()
		return m, nil

	case "x":
		m.swapSides()
		return m, nil

	case "F":
		return m, m.openDirsInDiffTool()

	case "o":
		m.openPaths()
		return m, nil

	case "ctrl+g", "f2": // F2 because Zellij, for one, keeps Ctrl+G for itself
		m.openGitSelect()
		return m, nil

	case "e":
		// Edit the right (newer / working tree) side, or the only real side
		side := "right"
		if m.editablePath(side) == "" {
			side = "left"
		}
		return m, m.openInEditor(side)

	case "E":
		return m, m.openInEditor("left")

	case "D":
		return m, m.openInDiffTool()

	case "R":
		return m, m.resolveConflict()
	}

	return m, nil
}

// ---- External tools ---------------------------------------------------------

// editablePath returns the on-disk path for one side of the selected file.
func (m *Model) editablePath(side string) string {
	comp, ok := m.allFiles[m.selectedFile]
	if !ok {
		return ""
	}
	return m.editablePathOf(comp, side)
}

// editablePathOf returns the on-disk path for one side of a comparison, or ""
// when that side is missing or isn't a real file (a git ref, or the working
// tree path when comparing ref to ref).
func (m *Model) editablePathOf(comp *file.FileComparison, side string) string {
	f := comp.RightFile
	if side == "left" {
		f = comp.LeftFile
	}
	if f == nil || strings.HasPrefix(f.Path, "git:") {
		return ""
	}
	if m.gitMode && (side == "left" || m.rightRef != "") {
		return ""
	}
	return f.Path
}

// pruneMergeBackups drops the .orig files of this session's saves from the
// list, so that saving a merge doesn't add a row, and it can't be copied.
func (m *Model) pruneMergeBackups() {
	backups := m.tracker.Backups()
	if len(backups) == 0 {
		return
	}
	for relPath, comp := range m.allFiles {
		var paths []string
		if comp.LeftFile != nil {
			paths = append(paths, m.editablePathOf(comp, "left"))
		}
		if comp.RightFile != nil {
			paths = append(paths, m.editablePathOf(comp, "right"))
		}
		if merge.AllBackups(paths, backups) {
			delete(m.allFiles, relPath)
		}
	}
}

// refreshStatuses works out each file's merge status.
func (m *Model) refreshStatuses() {
	m.statuses = make(map[string]*merge.Record)
	for relPath, comp := range m.allFiles {
		if rec := m.tracker.StatusOf(m.editablePathOf(comp, "left"), m.editablePathOf(comp, "right")); rec != nil {
			m.statuses[relPath] = rec
		}
	}
}

// refreshIdentical works out which files have no differences to show, and the
// list's summary. It compares every file, so it's only for when they're loaded.
func (m *Model) refreshIdentical() {
	var same func(string, string) bool
	if m.diffOpts.Describe() != "" {
		same = m.diffOpts.Same
	}
	m.identical = file.Identical(m.allFiles, same)
	m.differCount = len(m.allFiles) - len(m.identical)
}

// pruneIdentical applies hide_identical. Comparing two single files always shows
// them, and a file with a merge status or a conflict keeps its row.
func (m *Model) pruneIdentical() {
	m.hiddenIdentical = 0
	if !m.hideIdentical || !(m.gitMode || (m.leftFile != nil && m.rightFile != nil && m.leftFile.IsDir && m.rightFile.IsDir)) {
		return
	}
	for relPath := range m.identical {
		if m.statuses[relPath] == nil && !m.conflicts[relPath] {
			delete(m.allFiles, relPath)
			m.hiddenIdentical++
		}
	}
}

// refreshTargetDisk compares the merge target on disk with what was loaded.
func (m *Model) refreshTargetDisk() {
	comp, ok := m.allFiles[m.selectedFile]
	if !ok || comp.Source != file.SourceBoth {
		return
	}
	path := m.editablePath(m.mergeTarget)
	if path == "" {
		return
	}
	if m.openDiff == nil || m.diffFile != m.selectedFile {
		return
	}
	loaded := m.openDiff.right
	if m.mergeTarget == "left" {
		loaded = m.openDiff.left
	}
	if state, err := merge.DiskState(path, loaded); err == nil {
		m.targetDisk = state
	}
	m.refreshStatuses()
}

// cursorLineFor returns the line number on side under the diff cursor,
// looking at nearby lines when the cursor is on a line that only exists on
// the other side. Returns 0 if there is none.
func (m *Model) cursorLineFor(side string) int {
	var nums []int
	if m.diffViewMode == DiffViewSideBySide {
		for _, r := range m.sbsRows {
			if side == "left" {
				nums = append(nums, r.LeftLineNum)
			} else {
				nums = append(nums, r.RightLineNum)
			}
		}
	} else if m.currentDiff != nil {
		for _, l := range m.currentDiff.Lines {
			if side == "left" {
				nums = append(nums, l.LeftLineNum)
			} else {
				nums = append(nums, l.RightLineNum)
			}
		}
	}
	for i := m.cursor; i < len(nums); i++ {
		if i >= 0 && nums[i] > 0 {
			return nums[i]
		}
	}
	for i := m.cursor - 1; i >= 0 && i < len(nums); i-- {
		if nums[i] > 0 {
			return nums[i]
		}
	}
	return 0
}

// runExternal suspends the TUI while cmd runs, so terminal editors and
// tools like nvimdiff can take over the screen.
func runExternal(kind string, cmd *exec.Cmd) tea.Cmd {
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return externalDoneMsg{kind: kind, err: err}
	})
}

// editorCommand opens path in the editor the config file names, or the usual
// one. It reads the file each time, so a change needs no restart.
func (m *Model) editorCommand(path string, line int) (*exec.Cmd, error) {
	cfg, _ := config.Load()
	return external.ConfiguredEditorCommand(cfg.Tools.Editor, path, line, false)
}

func (m *Model) openInEditor(side string) tea.Cmd {
	path := m.editablePath(side)
	if path == "" {
		m.errorMsg = fmt.Sprintf("The %s side isn't a file on disk", side)
		return nil
	}
	cmd, err := m.editorCommand(path, m.cursorLineFor(side))
	if err != nil {
		m.errorMsg = err.Error()
		return nil
	}
	return runExternal("edit", cmd)
}

func (m *Model) openInDiffTool() tea.Cmd {
	comp, ok := m.allFiles[m.selectedFile]
	if !ok {
		return nil
	}
	req := external.DiffRequest{}
	if m.gitMode {
		req.GitRoot = m.gitRoot
		req.LeftRef = m.leftRef
		req.RightRef = m.rightRef
		req.RelPath = m.selectedFile
	} else {
		// Plain files: passed straight to the tool, no git repo needed
		if comp.LeftFile != nil {
			req.LeftPath = comp.LeftFile.Path
		}
		if comp.RightFile != nil {
			req.RightPath = comp.RightFile.Path
		}
	}
	cmd, err := external.DiffToolCommand(req, false)
	if err != nil {
		m.errorMsg = err.Error()
		return nil
	}
	return runExternal("diff", cmd)
}

func (m *Model) resolveConflict() tea.Cmd {
	if !m.gitMode || !m.conflicts[m.selectedFile] {
		m.errorMsg = "This file has no merge conflicts"
		return nil
	}
	cmd, err := external.MergeToolCommand(m.gitRoot, m.selectedFile, false)
	if err != nil {
		m.errorMsg = err.Error()
		return nil
	}
	return runExternal("merge", cmd)
}

// handleExternalDone reports a tool's result and reloads, since the tool may
// have changed files on disk.
func (m *Model) handleExternalDone(msg externalDoneMsg) {
	errText := ""
	if msg.err != nil {
		var exitErr *exec.ExitError
		// git difftool --no-index exits 1 when the files differ, like diff
		if !(msg.kind == "diff" && errors.As(msg.err, &exitErr) && exitErr.ExitCode() == 1) {
			errText = msg.err.Error()
		}
	}
	if msg.kind != "diff" {
		m.reloadComparison()
		if m.viewMode == ViewModeConflict {
			m.afterExternalInConflictView()
		}
	}
	switch {
	case errText != "" && msg.kind == "merge":
		m.errorMsg = "Merge tool exited without resolving: " + errText
	case errText != "":
		m.errorMsg = fmt.Sprintf("External %s tool failed: %s", msg.kind, errText)
	}
}

// reloadComparison re-reads the compared files from disk (or git), keeping
// the selected file and diff position where possible.
func (m *Model) reloadComparison() {
	selected, shown := m.selectedFile, m.diffFile
	cursor, scroll, hScroll := m.cursor, m.scrollOffset, m.hScrollOffset

	m.applyConfig()
	if m.gitMode {
		if err := m.LoadGitComparison(m.leftRef, m.rightRef); err != nil {
			m.errorMsg = err.Error()
			m.viewMode = ViewModeFileSelect
			return
		}
	} else {
		m.loadLeftPath()
		m.loadRightPath()
		m.updateCommonFiles()
	}
	if _, ok := m.allFiles[selected]; ok {
		m.selectedFile = selected
	}
	m.syncSelectionToFilter()

	if !m.browsing() {
		m.currentDiff = nil
		return
	}
	if m.selectedFile == "" {
		m.currentDiff = nil
		if m.viewMode != ViewModeFileSelect {
			m.viewMode = ViewModeFileSelect
		}
		return
	}
	m.loadDiff()
	if m.selectedFile == shown {
		maxLines := m.maxDiffLines()
		cursor = min(cursor, max(0, maxLines-1))
		m.cursor, m.scrollOffset, m.hScrollOffset = cursor, min(scroll, cursor), hScroll
	}
	if m.viewMode == ViewModeMerge {
		if comp := m.allFiles[m.selectedFile]; comp.Source != file.SourceBoth || m.selectedFile != selected || m.currentDiff == nil {
			m.viewMode = ViewModeDiff
			m.errorMsg = "The file is no longer on both sides, so the merge has ended"
			return
		}
		m.changeSelection = merge.NewChangeSelection(m.currentDiff)
		m.targetDisk = merge.DiskUnchanged
		m.overwriteArmed = false
	}
}

// handleMergeKeys handles keys in merge view mode
func (m *Model) handleMergeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.clearMessages()
	armed := m.overwriteArmed
	m.overwriteArmed = false

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "esc":
		m.viewMode = ViewModeDiff
		if m.diffViewMode == DiffViewSideBySide {
			// The cursor was counting unified lines.
			m.moveToFirstChange()
		}
		return m, nil

	case "up", "k":
		m.moveCursor(-1)
		return m, nil

	case "down", "j":
		m.moveCursor(1)
		return m, nil

	case "pgup":
		m.moveCursor(-m.mergeViewHeight())
		return m, nil

	case "pgdown":
		m.moveCursor(m.mergeViewHeight())
		return m, nil

	case "left", "h":
		m.hScrollOffset = max(0, m.hScrollOffset-4)
		return m, nil

	case "right", "l":
		m.hScrollOffset += 4
		return m, nil

	case "g", "home":
		m.cursor, m.scrollOffset = 0, 0
		return m, nil

	case "G", "end":
		m.cursor = max(0, m.maxDiffLines()-1)
		return m, nil

	case "enter":
		// Toggle the whole block of changes under the cursor
		m.changeSelection.ToggleBlock(m.currentDiff, m.cursor)
		m.refreshTargetDisk()
		return m, nil

	case "]":
		m.jumpToChange(1)
		return m, nil

	case "[":
		m.jumpToChange(-1)
		return m, nil

	case " ":
		// Toggle selection of current change
		if m.cursor < len(m.currentDiff.Lines) {
			switch m.currentDiff.Lines[m.cursor].Type {
			case differ.DiffInsert:
				m.changeSelection.ToggleInsertion(m.cursor)
			case differ.DiffDelete:
				m.changeSelection.ToggleDeletion(m.cursor)
			}
			m.refreshTargetDisk()
		}
		return m, nil

	case "a":
		m.changeSelection.SelectAll(m.currentDiff)
		m.refreshTargetDisk()
		return m, nil

	case "n":
		m.changeSelection.SelectNone(m.currentDiff)
		m.refreshTargetDisk()
		return m, nil

	case "t":
		// Toggle merge target (left/right); only real files can be saved to
		other := "left"
		if m.mergeTarget == "left" {
			other = "right"
		}
		if m.editablePath(other) == "" {
			m.errorMsg = fmt.Sprintf("The %s side is a git revision, not a file. Only the %s file can be changed", other, m.mergeTarget)
			return m, nil
		}
		m.mergeTarget = other
		m.refreshTargetDisk()
		return m, nil

	case "s":
		m.overwriteArmed = armed
		return m, m.saveMergedFile()

	case "r":
		m.reload()
		return m, nil

	case "?", "f1":
		m.openHelp()
		return m, nil
	}

	return m, nil
}

// handleCopyKeys handles keys in copy view mode
func (m *Model) handleCopyKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.clearMessages()
	uniqueFiles := m.getUniqueFiles()

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "esc":
		m.cursor, m.scrollOffset = 0, 0
		m.viewMode = ViewModeFileSelect
		if m.currentDiff != nil {
			m.moveToFirstChange()
		}

	case "up", "k":
		m.cursor = max(0, m.cursor-1)

	case "down", "j":
		m.cursor = min(m.cursor+1, max(0, len(uniqueFiles)-1))

	case " ", "enter":
		if m.cursor < len(uniqueFiles) && m.canCopy(uniqueFiles[m.cursor]) {
			relPath := uniqueFiles[m.cursor]
			m.copySelection[relPath] = !m.copySelection[relPath]
		}

	case "a":
		for _, relPath := range uniqueFiles {
			m.copySelection[relPath] = true
		}

	case "n":
		for _, relPath := range uniqueFiles {
			m.copySelection[relPath] = false
		}

	case "t":
		if m.copyTarget == "to-left" {
			m.copyTarget = "to-right"
		} else {
			m.copyTarget = "to-left"
		}

	case "s":
		return m, m.executeCopyOperation()

	case "?", "f1":
		m.openHelp()
	}
	return m, nil
}

// handleHelpKeys handles keys in help view mode
func (m *Model) handleHelpKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		m.helpScroll = max(0, m.helpScroll-1)
	case "down", "j":
		m.helpScroll = min(m.helpMaxScroll(), m.helpScroll+1)
	case "pgup", "ctrl+u":
		m.helpScroll = max(0, m.helpScroll-m.helpPageHeight())
	case "pgdown", "ctrl+d", " ":
		m.helpScroll = min(m.helpMaxScroll(), m.helpScroll+m.helpPageHeight())
	case "g", "home":
		m.helpScroll = 0
	case "G", "end":
		m.helpScroll = m.helpMaxScroll()
	case "esc", "?", "f1":
		m.helpScroll = 0
		m.viewMode = m.helpReturn
	}
	return m, nil
}

// Helper functions

func (m *Model) loadLeftPath() {
	leftFile, err := m.fileManager.LoadPath(m.leftPath)
	if err != nil {
		m.errorMsg = fmt.Sprintf("Error loading left path: %s", err.Error())
		return
	}
	m.leftFile = leftFile
}

func (m *Model) loadRightPath() {
	rightFile, err := m.fileManager.LoadPath(m.rightPath)
	if err != nil {
		m.errorMsg = fmt.Sprintf("Error loading right path: %s", err.Error())
		return
	}
	m.rightFile = rightFile
}

func (m *Model) updateCommonFiles() {
	if m.leftFile != nil && m.rightFile != nil {
		m.gitMode = false
		m.conflicts = nil
		m.diffCache, m.diffOrder = nil, nil
		m.diffGen++
		m.allFiles = file.FindAllFiles(m.leftFile, m.rightFile)
		m.pruneMergeBackups()
		m.refreshStatuses()
		m.refreshIdentical()
		m.pruneIdentical()
		m.fileListScroll = 0 // Reset scroll when files change
		m.syncSelectionToFilter()
	}
}

// displayNameForFile returns a human-readable label for a file-list entry.
// Entries are normally keyed by their path relative to the compared roots,
// but when both roots are single files (not directories) that key collapses
// to "." - in that case show the actual filename(s) instead.
func (m *Model) displayNameForFile(relPath string) string {
	if relPath != "." {
		return relPath
	}
	comparison, exists := m.allFiles[relPath]
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

// getSortedFiles returns the files the list shows, in order: those matching
// the filter, if there is one.
//
// It's called several times per frame, so the result is kept until the files or
// the filter change. Callers mustn't change the slice.
func (m *Model) getSortedFiles() []string {
	key := sortedKey{m.filterQuery, len(m.allFiles), m.diffGen}
	if m.sorted != nil && m.sortedFor == key {
		return m.sorted
	}
	query := strings.ToLower(m.filterQuery)
	files := make([]string, 0, len(m.allFiles))
	for relPath := range m.allFiles {
		if query == "" || strings.Contains(strings.ToLower(m.displayNameForFile(relPath)), query) {
			files = append(files, relPath)
		}
	}
	sort.Strings(files)
	m.sorted, m.sortedFor = files, key
	return files
}

func (m *Model) selectNextFile() {
	files := m.getSortedFiles()
	if len(files) == 0 {
		return
	}
	currentIndex := -1
	for i, f := range files {
		if f == m.selectedFile {
			currentIndex = i
			break
		}
	}
	if currentIndex == -1 || currentIndex == len(files)-1 {
		m.selectedFile = files[0]
	} else {
		m.selectedFile = files[currentIndex+1]
	}
}

func (m *Model) selectPreviousFile() {
	files := m.getSortedFiles()
	if len(files) == 0 {
		return
	}
	currentIndex := -1
	for i, f := range files {
		if f == m.selectedFile {
			currentIndex = i
			break
		}
	}
	if currentIndex <= 0 {
		m.selectedFile = files[len(files)-1]
	} else {
		m.selectedFile = files[currentIndex-1]
	}
}

// LoadGitComparison populates the model with a diff between two git states.
// leftRef is the older git ref (e.g. "HEAD", "HEAD~1", "abc1234").
// rightRef is the newer ref; pass "" to compare leftRef against the working tree.
func (m *Model) LoadGitComparison(leftRef, rightRef string) error {
	m.applyConfig()
	root, err := git.FindRoot()
	if err != nil {
		return err
	}

	statuses, err := git.ChangedFiles(root, leftRef, rightRef)
	if err != nil {
		return err
	}
	if rightRef == "" {
		statuses = git.WithPaths(statuses, m.tracker.SavedUnder(root))
	}
	if len(statuses) == 0 {
		return fmt.Errorf("no changes found between %s and %s", leftRef, func() string {
			if rightRef == "" {
				return "working tree"
			}
			return rightRef
		}())
	}

	rightLabel := rightRef
	if rightLabel == "" {
		rightLabel = "working tree"
	}

	// Set display paths (shown in the header / input fields)
	m.leftPath = fmt.Sprintf("git:%s", leftRef)
	m.rightPath = rightLabel
	m.inputLeft = m.leftPath
	m.inputRight = rightLabel

	// Synthetic directory-level FileInfo objects (used for display only)
	m.leftFile = &file.FileInfo{Path: m.leftPath, Name: leftRef, IsDir: true}
	m.rightFile = &file.FileInfo{Path: m.rightPath, Name: rightLabel, IsDir: true}

	m.allFiles = make(map[string]*file.FileComparison)
	m.selectedFile = ""

	m.gitMode = true
	m.gitRoot = root
	m.leftRef = leftRef
	m.rightRef = rightRef
	m.conflicts = make(map[string]bool)

	for _, fs := range statuses {
		if fs.Conflicted {
			m.conflicts[fs.Path] = true
		}
		comparison := &file.FileComparison{RelativePath: fs.Path}

		switch fs.Status {
		case 'A': // Added — only exists in right (working tree or rightRef)
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

		case 'D': // Deleted — only exists in leftRef
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

		default: // Modified — exists in both
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
			lf := &file.FileInfo{
				Path:    fmt.Sprintf("git:%s:%s", leftRef, fs.Path),
				Name:    filepath.Base(fs.Path),
				Content: leftContent,
				Size:    int64(len(leftContent)),
			}
			rf := &file.FileInfo{
				Path:    filepath.Join(root, fs.Path),
				Name:    filepath.Base(fs.Path),
				Content: rightContent,
				Size:    int64(len(rightContent)),
			}
			comparison.LeftFile = lf
			comparison.RightFile = rf
			comparison.Source = file.SourceBoth
		}

		m.allFiles[fs.Path] = comparison
	}

	m.diffCache, m.diffOrder = nil, nil
	m.diffGen++
	m.pruneMergeBackups()
	m.refreshStatuses()
	m.refreshIdentical()
	m.pruneIdentical()
	m.syncSelectionToFilter()

	return nil
}

// computeDiff reads one file's two sides and diffs them.
func computeDiff(d *differ.Differ, comp *file.FileComparison) *cachedDiff {
	cached := &cachedDiff{stamp: diffStamp(comp)}
	leftPath, rightPath := "<file not found>", "<file not found>"
	if comp.LeftFile != nil {
		leftPath = comp.LeftFile.Path
		if cached.left, cached.err = comp.LeftFile.Text(); cached.err != nil {
			return cached
		}
	}
	if comp.RightFile != nil {
		rightPath = comp.RightFile.Path
		if cached.right, cached.err = comp.RightFile.Text(); cached.err != nil {
			return cached
		}
	}
	cached.diff = d.CompareStrings(leftPath, rightPath, cached.left, cached.right)
	cached.rows = differ.BuildSideBySideRows(cached.diff.Lines)
	return cached
}

// computeDiffCmd diffs a file in the background.
func (m *Model) computeDiffCmd(relPath string) tea.Cmd {
	comp, ok := m.allFiles[relPath]
	if !ok {
		return nil
	}
	if _, cached := m.diffCache[relPath]; cached {
		m.ensureDiffLoaded()
		return nil
	}
	gen := m.diffGen
	d := differ.NewWithOptions(m.diffOpts)
	return func() tea.Msg {
		return diffReadyMsg{relPath: relPath, gen: gen, cached: computeDiff(d, comp)}
	}
}

func (m *Model) loadDiff() {
	if m.selectedFile == "" {
		m.errorMsg = "No file selected for comparison"
		return
	}

	fileComparison, exists := m.allFiles[m.selectedFile]
	if !exists {
		m.errorMsg = fmt.Sprintf("Selected file '%s' no longer exists", m.selectedFile)
		return
	}

	// A kept diff is only good if its files haven't changed since.
	cached, ok := m.diffCache[m.selectedFile]
	if !ok || cached.stamp != diffStamp(fileComparison) {
		cached = computeDiff(m.differ, fileComparison)
		if cached.err == nil {
			m.cacheDiff(m.selectedFile, cached)
		}
	}

	m.openDiff = cached
	m.diffErr = cached.err
	m.diffFile = m.selectedFile
	if cached.err != nil {
		m.currentDiff, m.sbsRows = nil, nil
		return
	}
	m.currentDiff = cached.diff
	m.sbsRows = cached.rows
	m.hScrollOffset = 0
	m.moveToFirstChange()
}

// moveToFirstChange puts the cursor on the first change, with some context
// above it, or at the top when the sides are the same.
func (m *Model) moveToFirstChange() {
	m.cursor, m.scrollOffset = 0, 0
	for i := 0; i < m.maxDiffLines(); i++ {
		if m.isChangeRow(i) {
			m.cursor = i
			m.scrollOffset = max(0, i-m.diffViewHeight()/4)
			return
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// usesSideBySideRows reports whether the cursor indexes sbsRows (the
// side-by-side diff) rather than the unified lines (also used by merge).
func (m *Model) usesSideBySideRows() bool {
	return m.diffViewMode == DiffViewSideBySide && m.viewMode != ViewModeMerge
}

// isChangeRow reports whether row i of the current diff view is a change.
func (m *Model) isChangeRow(i int) bool {
	if m.usesSideBySideRows() {
		return m.sbsRows[i].Type != differ.SBSEqual
	}
	return m.currentDiff.Lines[i].Type != differ.DiffEqual
}

// jumpToChange moves the cursor to the start of the next (dir > 0) or
// previous (dir < 0) block of changed lines, scrolling it into view a little
// below the top so some context shows above it.
func (m *Model) jumpToChange(dir int) {
	n := m.maxDiffLines()
	isStart := func(i int) bool {
		return m.isChangeRow(i) && (i == 0 || !m.isChangeRow(i-1))
	}
	target := -1
	for i := m.cursor + dir; i >= 0 && i < n; i += dir {
		if isStart(i) {
			target = i
			break
		}
	}
	if target < 0 {
		if dir > 0 {
			m.errorMsg = "No more changes below"
		} else {
			m.errorMsg = "No more changes above"
		}
		return
	}
	m.cursor = target
	height := m.diffViewHeight()
	m.scrollOffset = max(0, min(target-height/4, n-height))
}

func (m *Model) maxDiffLines() int {
	if m.usesSideBySideRows() {
		return len(m.sbsRows)
	}
	if m.currentDiff == nil {
		return 0
	}
	return len(m.currentDiff.Lines)
}

// SetLeftPath sets the left path and loads it
func (m *Model) SetLeftPath(path string) {
	m.inputLeft = path
	m.leftPath = path
	m.loadLeftPath()

	// If we already have a right file, update files and select first
	if m.rightFile != nil {
		m.updateCommonFiles()
		if len(m.allFiles) > 0 && m.selectedFile == "" {
			files := m.getSortedFiles()
			if len(files) > 0 {
				m.selectedFile = files[0]
			}
		}
	}
}

// SetRightPath sets the right path and loads it
func (m *Model) SetRightPath(path string) {
	m.inputRight = path
	m.rightPath = path
	m.loadRightPath()
	m.updateCommonFiles()

	// Ensure first file is selected when both paths are loaded
	if m.leftFile != nil && m.rightFile != nil && len(m.allFiles) > 0 && m.selectedFile == "" {
		files := m.getSortedFiles()
		if len(files) > 0 {
			m.selectedFile = files[0]
		}
	}
}

// OpenDiffIfFiles leaves the open screen once both paths have loaded: for two
// files straight to the diff, and for directories to the file list so the user
// can pick one.
func (m *Model) OpenDiffIfFiles() {
	if m.leftFile == nil || m.rightFile == nil {
		return
	}
	if !m.leftFile.IsDir && !m.rightFile.IsDir {
		m.focusDiff()
		if m.viewMode == ViewModeDiff {
			return
		}
	}
	m.viewMode = ViewModeFileSelect
	if m.selectedFile != "" {
		m.ensureDiffLoaded()
	}
}

// ShowFileList switches to the file list after LoadGitComparison.
func (m *Model) ShowFileList() {
	m.viewMode = ViewModeFileSelect
	m.filterQuery, m.filterTyping = "", false
	m.syncSelectionToFilter()
	m.ensureDiffLoaded()
}

// FocusRightPath moves the open screen's focus to the right path field.
func (m *Model) FocusRightPath() {
	m.focus = focusInputRight
}

// Path suggestion methods

func (m *Model) updateSuggestions() {
	if m.focus == focusInputLeft {
		m.leftSuggestions = m.generateSuggestions(m.inputLeft)
		m.leftSuggIndex = 0
		if len(m.leftSuggestions) == 0 {
			m.leftSuggIndex = -1
		}
	} else {
		m.rightSuggestions = m.generateSuggestions(m.inputRight)
		m.rightSuggIndex = 0
		if len(m.rightSuggestions) == 0 {
			m.rightSuggIndex = -1
		}
	}

	m.showSuggestions = len(m.leftSuggestions) > 0 || len(m.rightSuggestions) > 0
}

func (m *Model) clearSuggestions() {
	m.leftSuggestions = nil
	m.rightSuggestions = nil
	m.leftSuggIndex = -1
	m.rightSuggIndex = -1
	m.showSuggestions = false
}

func (m *Model) generateSuggestions(input string) []string {
	if len(input) == 0 {
		return nil
	}

	var suggestions []string

	// Determine the directory to search and the prefix to match
	var searchDir, prefix string

	if strings.HasSuffix(input, "/") || strings.HasSuffix(input, "\\") {
		// Input ends with separator, search in that directory
		searchDir = input
		prefix = ""
	} else {
		// Input is a partial path, split into directory and filename parts
		searchDir = filepath.Dir(input)
		prefix = filepath.Base(input)

		if searchDir == "." && !strings.Contains(input, "/") && !strings.Contains(input, "\\") {
			searchDir = ""
		}
	}

	// Handle empty or current directory
	if searchDir == "" || searchDir == "." {
		searchDir = "."
	}

	// Read directory contents
	entries, err := os.ReadDir(searchDir)
	if err != nil {
		return nil
	}

	// Filter and collect matching entries
	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files unless explicitly requested
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(prefix, ".") {
			continue
		}

		// Check if the name matches the prefix
		if prefix == "" || strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) {
			var suggestion string
			if searchDir == "." {
				suggestion = name
			} else {
				suggestion = filepath.Join(searchDir, name)
			}

			// Add trailing slash for directories
			if entry.IsDir() {
				suggestion += string(filepath.Separator)
			}

			suggestions = append(suggestions, suggestion)
		}
	}

	// Sort suggestions
	sort.Strings(suggestions)

	// Limit number of suggestions
	maxSuggestions := 8
	if len(suggestions) > maxSuggestions {
		suggestions = suggestions[:maxSuggestions]
	}

	return suggestions
}

// initializeMergeMode sets up merge mode with every change ticked, on the
// first change, with a target that's a real file.
func (m *Model) initializeMergeMode() {
	m.changeSelection = merge.NewChangeSelection(m.currentDiff)
	if m.editablePath(m.mergeTarget) == "" {
		if m.mergeTarget == "left" {
			m.mergeTarget = "right"
		} else {
			m.mergeTarget = "left"
		}
	}
	m.overwriteArmed = false
	m.targetDisk = merge.DiskUnchanged
	m.refreshTargetDisk()
	m.hScrollOffset = 0
	m.moveToFirstChange()
}

// saveDoneMsg reports the result of saving a merge.
type saveDoneMsg struct {
	path   string
	record *merge.Record
	text   string
	err    error
}

// issueSuffix reports what a save's checks found, if anything.
func issueSuffix(report *merge.SaveReport) string {
	if len(report.Issues) == 0 {
		return ""
	}
	return " Check: " + strings.Join(report.Issues, "; ") + "."
}

// saveMergedFile applies the selected changes to the target side and saves
// the result over that file, keeping the old version as "<file>.orig". If the
// file changed after it was loaded, the first s only warns.
func (m *Model) saveMergedFile() tea.Cmd {
	if m.currentDiff == nil || m.changeSelection == nil {
		return nil
	}
	targetPath := m.editablePath(m.mergeTarget)
	if targetPath == "" {
		m.errorMsg = fmt.Sprintf("The %s side is a git revision, not a file on disk", m.mergeTarget)
		return nil
	}
	name := filepath.Base(targetPath)

	m.refreshTargetDisk()
	armed := m.overwriteArmed
	m.overwriteArmed = false
	if m.targetDisk == merge.DiskMissing {
		m.errorMsg = fmt.Sprintf("%s no longer exists. Press r to reload", name)
		return nil
	}
	selIns, _, selDel, _ := m.changeSelection.GetSelectedStats(m.currentDiff)
	if selIns+selDel == 0 {
		m.errorMsg = "No changes are ticked, so there's nothing to save"
		return nil
	}
	if m.targetDisk == merge.DiskChanged && !armed {
		m.overwriteArmed = true
		m.errorMsg = fmt.Sprintf("%s changed on disk. Press s again to replace those edits (kept as %s.orig), or r to reload", name, name)
		return nil
	}

	var result *merge.MergeResult
	if m.mergeTarget == "left" {
		result = m.merger.ApplyToLeft(m.currentDiff, m.changeSelection)
	} else {
		result = m.merger.ApplyToRight(m.currentDiff, m.changeSelection)
	}
	return func() tea.Msg {
		report, err := merge.SaveMerged(targetPath, result)
		if err != nil {
			return saveDoneMsg{err: fmt.Errorf("error saving merged file: %w", err)}
		}
		return saveDoneMsg{
			path:   targetPath,
			record: &merge.Record{Report: report, Kind: merge.KindMerge, Applied: result.Applied, Skipped: result.Skipped},
			text: fmt.Sprintf("Saved %s (%d applied, %d skipped); previous version in %s.%s",
				name, result.Applied, result.Skipped, filepath.Base(report.Backup), issueSuffix(report)),
		}
	}
}

// hasUniqueFiles checks if there are any files that exist in only one directory
func (m *Model) hasUniqueFiles() bool {
	for _, fileComp := range m.allFiles {
		if fileComp.Source == file.SourceLeft || fileComp.Source == file.SourceRight {
			return true
		}
	}
	return false
}

// getUniqueFiles returns a sorted list of files that exist in only one directory
func (m *Model) getUniqueFiles() []string {
	var uniqueFiles []string
	for relPath, fileComp := range m.allFiles {
		if fileComp.Source == file.SourceLeft || fileComp.Source == file.SourceRight {
			uniqueFiles = append(uniqueFiles, relPath)
		}
	}

	sort.Strings(uniqueFiles)
	return uniqueFiles
}

// canCopy reports whether a one-sided file goes in the current direction.
func (m *Model) canCopy(relPath string) bool {
	comp := m.allFiles[relPath]
	if comp == nil {
		return false
	}
	if m.copyTarget == "to-right" {
		return comp.Source == file.SourceLeft
	}
	return comp.Source == file.SourceRight
}

// enterCopyMode opens the copy view, facing a direction that has files to copy.
func (m *Model) enterCopyMode() {
	if m.gitMode {
		m.errorMsg = "Copy isn't available when comparing against git"
		return
	}
	uniqueFiles := m.getUniqueFiles()
	if len(uniqueFiles) == 0 {
		m.errorMsg = "No files exist on only one side, so there's nothing to copy"
		return
	}
	eligible := false
	for _, relPath := range uniqueFiles {
		eligible = eligible || m.canCopy(relPath)
	}
	if !eligible {
		if m.copyTarget == "to-right" {
			m.copyTarget = "to-left"
		} else {
			m.copyTarget = "to-right"
		}
	}
	m.copySelection = make(map[string]bool)
	for _, relPath := range uniqueFiles {
		m.copySelection[relPath] = true
	}
	m.cursor, m.scrollOffset = 0, 0
	m.viewMode = ViewModeCopy
}

// copyDoneMsg reports the result of copying files.
type copyDoneMsg struct {
	text string
	err  string
}

// executeCopyOperation copies the ticked files going in the current direction.
func (m *Model) executeCopyOperation() tea.Cmd {
	type copyJob struct {
		rel, dst string
		src      *file.FileInfo
	}
	var jobs []copyJob
	skipped := 0
	for _, relPath := range m.getUniqueFiles() {
		if !m.copySelection[relPath] || !m.canCopy(relPath) {
			skipped++
			continue
		}
		comp := m.allFiles[relPath]
		if m.copyTarget == "to-right" {
			jobs = append(jobs, copyJob{relPath, filepath.Join(m.rightFile.Path, relPath), comp.LeftFile})
		} else {
			jobs = append(jobs, copyJob{relPath, filepath.Join(m.leftFile.Path, relPath), comp.RightFile})
		}
	}
	return func() tea.Msg {
		copied := 0
		var errs []string
		for _, job := range jobs {
			if err := os.MkdirAll(filepath.Dir(job.dst), 0755); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", job.rel, err))
				continue
			}
			text, err := job.src.Text()
			if err == nil {
				err = os.WriteFile(job.dst, []byte(text), 0644)
			}
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", job.rel, err))
				continue
			}
			copied++
		}
		msg := copyDoneMsg{text: fmt.Sprintf("Copy finished: %d copied, %d skipped", copied, skipped)}
		if len(errs) > 0 {
			msg.err = fmt.Sprintf("%s failed to copy; first: %s", plural(len(errs), "file"), errs[0])
		}
		return msg
	}
}
