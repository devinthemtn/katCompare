package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang-fileCmp/internal/differ"
	"golang-fileCmp/internal/file"
	"golang-fileCmp/internal/git"
	"golang-fileCmp/internal/merge"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ViewMode represents the current view state
type ViewMode int

const (
	ViewModeFileSelect ViewMode = iota
	ViewModeDiff
	ViewModeMerge
	ViewModeCopy
	ViewModeHelp
)

// DiffViewMode represents the diff display mode
type DiffViewMode int

const (
	DiffViewUnified DiffViewMode = iota
	DiffViewSideBySide
)

// fileSelectFocus represents which control has keyboard focus on the file select screen
type fileSelectFocus int

const (
	focusInputLeft fileSelectFocus = iota
	focusInputRight
	focusViewDiffBtn
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
	commonFiles    map[string][2]*file.FileInfo
	allFiles       map[string]*file.FileComparison
	selectedFile   string
	fileListScroll int

	// Diff view
	currentDiff   *differ.FileDiff
	sbsRows       []differ.SideBySideRow // precomputed side-by-side rows (cached from currentDiff)
	scrollOffset  int
	hScrollOffset int // horizontal scroll for side-by-side view
	cursor        int
	diffViewMode  DiffViewMode

	// Merge view
	changeSelection *merge.ChangeSelection
	mergeTarget     string // "left" or "right"
	mergePreview    string

	// Copy view
	copySelection map[string]bool // Maps relative path to whether to copy
	copyTarget    string          // "to-left" or "to-right"

	// Services
	fileManager *file.Manager
	differ      *differ.Differ
	merger      *merge.Merger

	// UI state
	inputLeft   string
	inputRight  string
	focus       fileSelectFocus
	showingHelp bool
	errorMsg    string

	// Path suggestions
	leftSuggestions  []string
	rightSuggestions []string
	leftSuggIndex    int
	rightSuggIndex   int
	showSuggestions  bool

	// File list filter
	filterQuery string
	filterActive bool
}

// Styles for the UI
var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			Bold(true)

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#555555")).
			Padding(0, 1)

	inputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color("#FFFFFF")).
			Padding(0, 1)

	focusedInputStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#000000")).
				Background(lipgloss.Color("#FFFF00")).
				Padding(0, 1)

	// Single-line "badge" buttons - deliberately not bordered, since a border
	// costs two extra rows and the file-select screen is already tight on
	// vertical space (the file list box reserves most of the terminal height).
	buttonStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DDDDDD")).
			Background(lipgloss.Color("#444444")).
			Bold(true).
			Padding(0, 2)

	focusedButtonStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#000000")).
				Background(lipgloss.Color("#FFFF00")).
				Bold(true).
				Padding(0, 2)

	equalLineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	insertLineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#0000FF")).
			Bold(true)

	deleteLineStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#FF0000")).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#FF0000")).
			Padding(0, 1).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Italic(true)

	fileListStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Padding(1)

	selectedFileStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#7D56F4")).
				Foreground(lipgloss.Color("#FFFFFF")).
				Bold(true)

	suggestionStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#888888")).
			Padding(0, 1).
			MaxWidth(80)

	selectedSuggestionStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#DDDDDD")).
				Foreground(lipgloss.Color("#000000"))

	// Empty cell in side-by-side view (the blank side of an insert or delete row)
	emptyDiffStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#444444")).
			Background(lipgloss.Color("#111111"))

	// Merge mode styles
	mergeHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FAFAFA")).
				Background(lipgloss.Color("#00AA00")).
				Padding(0, 1).
				Bold(true)

	selectedChangeStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#FFFF00")).
				Foreground(lipgloss.Color("#000000")).
				Bold(true)

	unselectedChangeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#666666")).
				Strikethrough(true)

	previewStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#00AA00")).
			Padding(1)
)

// New creates a new model
func New() *Model {
	return &Model{
		viewMode:        ViewModeFileSelect,
		fileManager:     file.New(),
		differ:          differ.New(),
		merger:          merge.New(),
		focus:           focusInputLeft,
		commonFiles:     make(map[string][2]*file.FileInfo),
		allFiles:        make(map[string]*file.FileComparison),
		leftSuggIndex:   -1,
		rightSuggIndex:  -1,
		showSuggestions: false,
		fileListScroll:  0,
		mergeTarget:   "left",
		copySelection: make(map[string]bool),
		copyTarget:    "to-right",
		diffViewMode:  DiffViewUnified,
	}
}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
		m.windowHeight = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	}

	return m, nil
}

// View renders the current view
func (m *Model) View() string {
	switch m.viewMode {
	case ViewModeFileSelect:
		return m.renderFileSelectView()
	case ViewModeDiff:
		return m.renderDiffView()
	case ViewModeMerge:
		return m.renderMergeView()
	case ViewModeCopy:
		return m.renderCopyView()
	case ViewModeHelp:
		return m.renderHelpView()
	default:
		return "Unknown view mode"
	}
}

// handleKeyPress processes keyboard input
func (m *Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.viewMode {
	case ViewModeFileSelect:
		return m.handleFileSelectKeys(msg)
	case ViewModeDiff:
		return m.handleDiffKeys(msg)
	case ViewModeMerge:
		return m.handleMergeKeys(msg)
	case ViewModeCopy:
		return m.handleCopyKeys(msg)
	case ViewModeHelp:
		return m.handleHelpKeys(msg)
	}
	return m, nil
}

// canViewDiff reports whether there's a selected file the diff view can open
func (m *Model) canViewDiff() bool {
	return len(m.allFiles) > 0 && m.selectedFile != ""
}

// openDiffForSelected loads the diff for the currently selected file (or the
// first file if none is selected yet) and switches to the diff view.
func (m *Model) openDiffForSelected() {
	if len(m.allFiles) == 0 {
		return
	}
	if m.selectedFile == "" {
		files := m.getSortedFiles()
		if len(files) > 0 {
			m.selectedFile = files[0]
		}
	}
	if m.selectedFile != "" {
		m.loadDiff()
		m.viewMode = ViewModeDiff
	}
}

// handleFileSelectKeys handles keys in file selection mode
func (m *Model) handleFileSelectKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "tab":
		// If suggestions are showing and we have suggestions, cycle through them
		onInput := m.focus == focusInputLeft || m.focus == focusInputRight
		if onInput && m.showSuggestions {
			if m.focus == focusInputLeft && len(m.leftSuggestions) > 0 {
				m.leftSuggIndex = (m.leftSuggIndex + 1) % len(m.leftSuggestions)
				return m, nil
			} else if m.focus == focusInputRight && len(m.rightSuggestions) > 0 {
				m.rightSuggIndex = (m.rightSuggIndex + 1) % len(m.rightSuggestions)
				return m, nil
			}
		}
		// Cycle focus: left input -> right input -> view diff button (if available) -> quit button -> left input
		switch m.focus {
		case focusInputLeft:
			m.focus = focusInputRight
		case focusInputRight:
			if m.canViewDiff() {
				m.focus = focusViewDiffBtn
			} else {
				m.focus = focusQuitBtn
			}
		case focusViewDiffBtn:
			m.focus = focusQuitBtn
		case focusQuitBtn:
			m.focus = focusInputLeft
		}
		m.clearSuggestions()
		return m, nil

	case "enter":
		// If the quit button is focused, quit the app
		if m.focus == focusQuitBtn {
			return m, tea.Quit
		}

		// If the view diff button is focused, open the diff for the selected file
		if m.focus == focusViewDiffBtn {
			m.openDiffForSelected()
			return m, nil
		}

		// If suggestions are showing, accept the selected suggestion. But if the
		// suggestion is already an exact match for what's typed (e.g. the user
		// typed a complete, valid filename), accepting it would be a no-op -
		// fall through and load the path instead of silently swallowing Enter.
		if m.showSuggestions {
			if m.focus == focusInputLeft && len(m.leftSuggestions) > 0 && m.leftSuggIndex >= 0 {
				suggestion := m.leftSuggestions[m.leftSuggIndex]
				if suggestion != m.inputLeft {
					m.inputLeft = suggestion
					m.clearSuggestions()
					return m, nil
				}
			} else if m.focus == focusInputRight && len(m.rightSuggestions) > 0 && m.rightSuggIndex >= 0 {
				suggestion := m.rightSuggestions[m.rightSuggIndex]
				if suggestion != m.inputRight {
					m.inputRight = suggestion
					m.clearSuggestions()
					return m, nil
				}
			}
		}

		// Load the path
		if m.focus == focusInputLeft {
			if m.inputLeft != "" {
				m.leftPath = m.inputLeft
				m.loadLeftPath()
			}
		} else if m.focus == focusInputRight {
			if m.inputRight != "" {
				m.rightPath = m.inputRight
				m.loadRightPath()
			}
		}
		m.clearSuggestions()
		m.updateCommonFiles()
		return m, nil

	case "ctrl+d":
		m.openDiffForSelected()
		return m, nil

	case "?":
		m.viewMode = ViewModeHelp
		return m, nil

	case "backspace":
		if m.filterActive {
			if len(m.filterQuery) > 0 {
				m.filterQuery = m.filterQuery[:len(m.filterQuery)-1]
			}
			return m, nil
		}
		if m.focus == focusInputLeft {
			if len(m.inputLeft) > 0 {
				m.inputLeft = m.inputLeft[:len(m.inputLeft)-1]
				m.updateSuggestions()
			}
		} else if m.focus == focusInputRight {
			if len(m.inputRight) > 0 {
				m.inputRight = m.inputRight[:len(m.inputRight)-1]
				m.updateSuggestions()
			}
		}
		return m, nil

	case "esc":
		if m.filterActive {
			m.filterActive = false
			m.filterQuery = ""
			return m, nil
		}
		m.clearSuggestions()
		return m, nil

	case "up":
		if m.showSuggestions {
			if m.focus == focusInputLeft && len(m.leftSuggestions) > 0 {
				if m.leftSuggIndex <= 0 {
					m.leftSuggIndex = len(m.leftSuggestions) - 1
				} else {
					m.leftSuggIndex--
				}
				return m, nil
			} else if m.focus == focusInputRight && len(m.rightSuggestions) > 0 {
				if m.rightSuggIndex <= 0 {
					m.rightSuggIndex = len(m.rightSuggestions) - 1
				} else {
					m.rightSuggIndex--
				}
				return m, nil
			}
		}
		if len(m.allFiles) > 0 {
			m.selectPreviousFile()
			// Only auto-load diff if we're in file selection mode
			// In diff mode, user needs to press Ctrl+D or navigate with n/p
		}
		return m, nil

	case "down":
		if m.showSuggestions {
			if m.focus == focusInputLeft && len(m.leftSuggestions) > 0 {
				m.leftSuggIndex = (m.leftSuggIndex + 1) % len(m.leftSuggestions)
				return m, nil
			} else if m.focus == focusInputRight && len(m.rightSuggestions) > 0 {
				m.rightSuggIndex = (m.rightSuggIndex + 1) % len(m.rightSuggestions)
				return m, nil
			}
		}
		if len(m.allFiles) > 0 {
			m.selectNextFile()
			// Only auto-load diff if we're in file selection mode
			// In diff mode, user needs to press Ctrl+D or navigate with n/p
		}
		return m, nil

	case "/":
		// Only steal "/" for the file-list filter once files are loaded and
		// neither path input is focused; otherwise it's a normal path character.
		if len(m.allFiles) > 0 && m.focus != focusInputLeft && m.focus != focusInputRight {
			m.filterActive = true
			m.filterQuery = ""
			return m, nil
		}
		fallthrough

	default:
		if m.filterActive {
			// Route all printable characters to the filter query
			if len(msg.String()) == 1 {
				m.filterQuery += msg.String()
			}
			return m, nil
		}
		if m.focus == focusQuitBtn {
			// Space also activates the focused quit button, like enter
			if msg.String() == " " {
				return m, tea.Quit
			}
			return m, nil
		}
		if m.focus == focusViewDiffBtn {
			// Space also activates the focused view diff button, like enter
			if msg.String() == " " {
				m.openDiffForSelected()
			}
			return m, nil
		}
		// Add character to appropriate input
		if len(msg.String()) == 1 {
			if m.focus == focusInputLeft {
				m.inputLeft += msg.String()
			} else if m.focus == focusInputRight {
				m.inputRight += msg.String()
			}
			m.updateSuggestions()
		}
		return m, nil
	}
}

// handleDiffKeys handles keys in diff view mode
func (m *Model) handleDiffKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "esc":
		m.viewMode = ViewModeFileSelect
		return m, nil

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.scrollOffset {
				m.scrollOffset = m.cursor
			}
		}
		return m, nil

	case "down", "j":
		maxLines := m.maxDiffLines()
		if m.cursor < maxLines-1 {
			m.cursor++
			maxVisible := m.windowHeight - 10
			if m.cursor >= m.scrollOffset+maxVisible {
				m.scrollOffset = m.cursor - maxVisible + 1
			}
		}
		return m, nil

	case "left", "h":
		if m.diffViewMode == DiffViewSideBySide && m.hScrollOffset > 0 {
			m.hScrollOffset--
		}
		return m, nil

	case "right", "l":
		if m.diffViewMode == DiffViewSideBySide {
			m.hScrollOffset++
		}
		return m, nil

	case "g":
		m.cursor = 0
		m.scrollOffset = 0
		return m, nil

	case "G":
		maxLines := m.maxDiffLines()
		if maxLines > 0 {
			m.cursor = maxLines - 1
			maxVisible := m.windowHeight - 10
			m.scrollOffset = max(0, m.cursor-maxVisible+1)
		}
		return m, nil

	case "n":
		m.selectNextFile()
		m.loadDiff()
		return m, nil

	case "p":
		m.selectPreviousFile()
		m.loadDiff()
		return m, nil

	case "?":
		m.viewMode = ViewModeHelp
		return m, nil

	case "s":
		// Toggle between unified and side-by-side diff view; reset position
		if m.diffViewMode == DiffViewUnified {
			m.diffViewMode = DiffViewSideBySide
		} else {
			m.diffViewMode = DiffViewUnified
		}
		m.cursor = 0
		m.scrollOffset = 0
		m.hScrollOffset = 0
		return m, nil

	case "m":
		// Enter merge mode if we have a diff loaded
		if m.currentDiff != nil {
			// Check if this is a valid file for merging
			if m.selectedFile != "" {
				if fileComparison, exists := m.allFiles[m.selectedFile]; exists {
					if fileComparison.Source == file.SourceBoth {
						m.initializeMergeMode()
						m.viewMode = ViewModeMerge
					} else {
						// Cannot merge files that only exist on one side
						if fileComparison.Source == file.SourceLeft {
							m.errorMsg = "Cannot merge: File exists only in LEFT directory"
						} else {
							m.errorMsg = "Cannot merge: File exists only in RIGHT directory"
						}
					}
				}
			}
		}
		return m, nil

	case "c":
		// Enter copy mode if we have files loaded
		if len(m.allFiles) > 0 && m.hasUniqueFiles() {
			m.initializeCopyMode()
			m.viewMode = ViewModeCopy
		}
		return m, nil
	}

	return m, nil
}

// handleMergeKeys handles keys in merge view mode
func (m *Model) handleMergeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "esc":
		m.viewMode = ViewModeDiff
		return m, nil

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.scrollOffset {
				m.scrollOffset = m.cursor
			}
		}
		return m, nil

	case "down", "j":
		if m.currentDiff != nil && m.cursor < len(m.currentDiff.Lines)-1 {
			m.cursor++
			maxVisible := m.windowHeight - 15 // Account for header and footer
			if m.cursor >= m.scrollOffset+maxVisible {
				m.scrollOffset = m.cursor - maxVisible + 1
			}
		}
		return m, nil

	case " ", "enter":
		// Toggle selection of current change
		if m.currentDiff != nil && m.cursor < len(m.currentDiff.Lines) {
			line := m.currentDiff.Lines[m.cursor]
			switch line.Type {
			case differ.DiffInsert:
				m.changeSelection.ToggleInsertion(m.cursor)
			case differ.DiffDelete:
				m.changeSelection.ToggleDeletion(m.cursor)
			}
			m.updateMergePreview()
		}
		return m, nil

	case "a":
		// Select all changes
		m.changeSelection.SelectAll(m.currentDiff)
		m.updateMergePreview()
		return m, nil

	case "n":
		// Select no changes
		m.changeSelection.SelectNone(m.currentDiff)
		m.updateMergePreview()
		return m, nil

	case "t":
		// Toggle merge target (left/right)
		if m.mergeTarget == "left" {
			m.mergeTarget = "right"
		} else {
			m.mergeTarget = "left"
		}
		m.updateMergePreview()
		return m, nil

	case "s":
		// Save merged result
		return m, m.saveMergedFile()

	case "?":
		m.viewMode = ViewModeHelp
		return m, nil
	}

	return m, nil
}

// handleCopyKeys handles keys in copy view mode
func (m *Model) handleCopyKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit

	case "esc":
		m.viewMode = ViewModeFileSelect
		return m, nil

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			if m.cursor < m.scrollOffset {
				m.scrollOffset = m.cursor
			}
		}
		return m, nil

	case "down", "j":
		uniqueFiles := m.getUniqueFiles()
		if m.cursor < len(uniqueFiles)-1 {
			m.cursor++
			maxVisible := m.windowHeight - 12 // Account for header and footer
			if m.cursor >= m.scrollOffset+maxVisible {
				m.scrollOffset = m.cursor - maxVisible + 1
			}
		}
		return m, nil

	case " ", "enter":
		// Toggle selection of current unique file
		uniqueFiles := m.getUniqueFiles()
		if m.cursor < len(uniqueFiles) {
			relPath := uniqueFiles[m.cursor]
			m.copySelection[relPath] = !m.copySelection[relPath]
		}
		return m, nil

	case "a":
		// Select all unique files
		uniqueFiles := m.getUniqueFiles()
		for _, relPath := range uniqueFiles {
			m.copySelection[relPath] = true
		}
		return m, nil

	case "n":
		// Select no files
		uniqueFiles := m.getUniqueFiles()
		for _, relPath := range uniqueFiles {
			m.copySelection[relPath] = false
		}
		return m, nil

	case "t":
		// Toggle copy target (to-left/to-right)
		if m.copyTarget == "to-left" {
			m.copyTarget = "to-right"
		} else {
			m.copyTarget = "to-left"
		}
		return m, nil

	case "s":
		// Execute copy operation
		return m, m.executeCopyOperation()

	case "?":
		m.viewMode = ViewModeHelp
		return m, nil
	}

	return m, nil
}

// handleHelpKeys handles keys in help view mode
func (m *Model) handleHelpKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "?":
		if m.viewMode == ViewModeMerge {
			m.viewMode = ViewModeMerge
		} else if m.viewMode == ViewModeCopy {
			m.viewMode = ViewModeCopy
		} else if len(m.commonFiles) > 0 && m.currentDiff != nil {
			m.viewMode = ViewModeDiff
		} else {
			m.viewMode = ViewModeFileSelect
		}
		return m, nil
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
	m.errorMsg = ""
}

func (m *Model) loadRightPath() {
	rightFile, err := m.fileManager.LoadPath(m.rightPath)
	if err != nil {
		m.errorMsg = fmt.Sprintf("Error loading right path: %s", err.Error())
		return
	}
	m.rightFile = rightFile
	m.errorMsg = ""
}

func (m *Model) updateCommonFiles() {
	if m.leftFile != nil && m.rightFile != nil {
		m.commonFiles = file.FindCommonFiles(m.leftFile, m.rightFile)
		m.allFiles = file.FindAllFiles(m.leftFile, m.rightFile)
		m.fileListScroll = 0 // Reset scroll when files change

		// Select first file by default if none selected
		if len(m.allFiles) > 0 && m.selectedFile == "" {
			files := m.getSortedFiles()
			if len(files) > 0 {
				m.selectedFile = files[0]
			}
		}

		// Ensure selected file still exists in the new file list
		if m.selectedFile != "" {
			if _, exists := m.allFiles[m.selectedFile]; !exists {
				files := m.getSortedFiles()
				if len(files) > 0 {
					m.selectedFile = files[0]
				} else {
					m.selectedFile = ""
				}
			}
		}
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

func (m *Model) getSortedFiles() []string {
	files := make([]string, 0, len(m.allFiles))
	for relPath := range m.allFiles {
		files = append(files, relPath)
	}

	// Sort files alphabetically for consistent ordering
	for i := 0; i < len(files)-1; i++ {
		for j := i + 1; j < len(files); j++ {
			if files[i] > files[j] {
				files[i], files[j] = files[j], files[i]
			}
		}
	}

	return files
}

func (m *Model) selectNextFile() {
	if len(m.allFiles) == 0 {
		return
	}

	files := m.getSortedFiles()

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
	if len(m.allFiles) == 0 {
		return
	}

	files := m.getSortedFiles()

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
	root, err := git.FindRoot()
	if err != nil {
		return err
	}

	statuses, err := git.ChangedFiles(root, leftRef, rightRef)
	if err != nil {
		return err
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
	m.commonFiles = make(map[string][2]*file.FileInfo)
	m.selectedFile = ""

	for _, fs := range statuses {
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
			m.commonFiles[fs.Path] = [2]*file.FileInfo{lf, rf}
		}

		m.allFiles[fs.Path] = comparison
	}

	// Auto-select first file
	if len(m.allFiles) > 0 {
		sorted := m.getSortedFiles()
		if len(sorted) > 0 {
			m.selectedFile = sorted[0]
		}
	}

	return nil
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

	var leftContent, rightContent, leftPath, rightPath string

	switch fileComparison.Source {
	case file.SourceBoth:
		leftFile := fileComparison.LeftFile
		rightFile := fileComparison.RightFile
		leftContent = leftFile.Content
		rightContent = rightFile.Content
		leftPath = leftFile.Path
		rightPath = rightFile.Path
	case file.SourceLeft:
		leftFile := fileComparison.LeftFile
		leftContent = leftFile.Content
		rightContent = ""
		leftPath = leftFile.Path
		rightPath = "<file not found>"
	case file.SourceRight:
		rightFile := fileComparison.RightFile
		leftContent = ""
		rightContent = rightFile.Content
		leftPath = "<file not found>"
		rightPath = rightFile.Path
	}

	diff := m.differ.CompareStrings(
		leftPath,
		rightPath,
		leftContent,
		rightContent,
	)

	m.currentDiff = diff
	m.sbsRows = differ.BuildSideBySideRows(diff.Lines)
	m.cursor = 0
	m.scrollOffset = 0
	m.hScrollOffset = 0
	m.errorMsg = "" // Clear any previous errors
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// maxDiffLines returns the total navigable lines for the current view mode.
func (m *Model) maxDiffLines() int {
	if m.diffViewMode == DiffViewSideBySide {
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

// initializeMergeMode sets up merge mode with default selections
func (m *Model) initializeMergeMode() {
	if m.currentDiff == nil {
		return
	}

	// Only initialize merge mode for files that exist on both sides
	if m.selectedFile != "" {
		if fileComparison, exists := m.allFiles[m.selectedFile]; exists && fileComparison.Source == file.SourceBoth {
			m.changeSelection = merge.NewChangeSelection(m.currentDiff)
			m.updateMergePreview()
			m.cursor = 0
			m.scrollOffset = 0
			m.errorMsg = "" // Clear any previous error
		}
	}
}

// updateMergePreview updates the merge preview text
func (m *Model) updateMergePreview() {
	if m.currentDiff == nil || m.changeSelection == nil {
		return
	}

	m.mergePreview = m.merger.CreateMergePreview(m.currentDiff, m.changeSelection, m.mergeTarget)
}

// saveMergedFile saves the merged result to a file
func (m *Model) saveMergedFile() tea.Cmd {
	return func() tea.Msg {
		if m.currentDiff == nil || m.changeSelection == nil {
			return nil
		}

		var result *merge.MergeResult
		var targetPath string

		if m.mergeTarget == "left" {
			result = m.merger.ApplyToLeft(m.currentDiff, m.changeSelection)
			targetPath = m.currentDiff.LeftFile + ".merged"
		} else {
			result = m.merger.ApplyToRight(m.currentDiff, m.changeSelection)
			targetPath = m.currentDiff.RightFile + ".merged"
		}

		err := os.WriteFile(targetPath, []byte(result.Content), 0644)
		if err != nil {
			return fmt.Sprintf("Error saving merged file: %s", err.Error())
		}

		return fmt.Sprintf("Saved merged result to %s (%d changes applied, %d skipped)",
			targetPath, result.Applied, result.Skipped)
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

	// Sort files alphabetically
	for i := 0; i < len(uniqueFiles)-1; i++ {
		for j := i + 1; j < len(uniqueFiles); j++ {
			if uniqueFiles[i] > uniqueFiles[j] {
				uniqueFiles[i], uniqueFiles[j] = uniqueFiles[j], uniqueFiles[i]
			}
		}
	}

	return uniqueFiles
}

// initializeCopyMode sets up copy mode with default selections
func (m *Model) initializeCopyMode() {
	m.copySelection = make(map[string]bool)

	// By default, select all unique files
	uniqueFiles := m.getUniqueFiles()
	for _, relPath := range uniqueFiles {
		m.copySelection[relPath] = true
	}

	m.cursor = 0
	m.scrollOffset = 0
}

// executeCopyOperation copies selected unique files
func (m *Model) executeCopyOperation() tea.Cmd {
	return func() tea.Msg {
		copiedCount := 0
		skippedCount := 0
		errorCount := 0
		var errors []string

		for relPath, shouldCopy := range m.copySelection {
			if !shouldCopy {
				skippedCount++
				continue
			}

			fileComp, exists := m.allFiles[relPath]
			if !exists {
				continue
			}

			var srcFile *file.FileInfo
			var dstPath string

			if m.copyTarget == "to-right" && fileComp.Source == file.SourceLeft {
				// Copy from left to right
				srcFile = fileComp.LeftFile
				dstPath = filepath.Join(m.rightFile.Path, relPath)
			} else if m.copyTarget == "to-left" && fileComp.Source == file.SourceRight {
				// Copy from right to left
				srcFile = fileComp.RightFile
				dstPath = filepath.Join(m.leftFile.Path, relPath)
			} else {
				// Wrong direction or file exists in both sides
				skippedCount++
				continue
			}

			// Create directory if needed
			dstDir := filepath.Dir(dstPath)
			if err := os.MkdirAll(dstDir, 0755); err != nil {
				errors = append(errors, fmt.Sprintf("Failed to create directory for %s: %v", relPath, err))
				errorCount++
				continue
			}

			// Copy file
			if err := os.WriteFile(dstPath, []byte(srcFile.Content), 0644); err != nil {
				errors = append(errors, fmt.Sprintf("Failed to copy %s: %v", relPath, err))
				errorCount++
				continue
			}

			copiedCount++
		}

		// Create result message
		var result strings.Builder
		result.WriteString(fmt.Sprintf("Copy operation completed: %d copied, %d skipped", copiedCount, skippedCount))

		if errorCount > 0 {
			result.WriteString(fmt.Sprintf(", %d errors", errorCount))
			if len(errors) > 0 {
				result.WriteString("\nErrors:\n")
				for _, err := range errors {
					result.WriteString("  - " + err + "\n")
				}
			}
		}

		return result.String()
	}
}
