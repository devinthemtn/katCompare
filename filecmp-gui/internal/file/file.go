package file

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FileInfo represents information about a file or directory
type FileInfo struct {
	Path     string
	Name     string
	IsDir    bool
	Size     int64
	Content  string
	Children []*FileInfo
}

// Manager handles file operations
type Manager struct{}

// New creates a new file manager
func New() *Manager {
	return &Manager{}
}

// LoadPath loads a file or directory and returns FileInfo
func (m *Manager) LoadPath(path string) (*FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat path %s: %w", path, err)
	}

	fileInfo := &FileInfo{
		Path:  path,
		Name:  filepath.Base(path),
		IsDir: info.IsDir(),
		Size:  info.Size(),
	}

	if info.IsDir() {
		children, err := m.loadDirectory(path)
		if err != nil {
			return nil, fmt.Errorf("failed to load directory %s: %w", path, err)
		}
		fileInfo.Children = children
	} else {
		content, err := m.readFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", path, err)
		}
		fileInfo.Content = content
	}

	return fileInfo, nil
}

// loadDirectory recursively loads directory contents
func (m *Manager) loadDirectory(dirPath string) ([]*FileInfo, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	var children []*FileInfo
	for _, entry := range entries {
		// Skip hidden files and directories
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		childPath := filepath.Join(dirPath, entry.Name())
		childInfo, err := m.LoadPath(childPath)
		if err != nil {
			// Log error but continue with other files
			continue
		}
		children = append(children, childInfo)
	}

	return children, nil
}

// readFile reads the content of a file
func (m *Manager) readFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// GetReader returns an io.Reader for the file content
func (fi *FileInfo) GetReader() io.Reader {
	return strings.NewReader(fi.Content)
}

// IsTextFile checks if the file appears to be a text file based on its extension
func (fi *FileInfo) IsTextFile() bool {
	if fi.IsDir {
		return false
	}

	ext := strings.ToLower(filepath.Ext(fi.Path))
	textExts := []string{
		".txt", ".md", ".go", ".js", ".ts", ".py", ".java", ".c", ".cpp", ".h", ".hpp",
		".css", ".html", ".htm", ".xml", ".json", ".yaml", ".yml", ".toml", ".csv",
		".sh", ".bash", ".zsh", ".fish", ".ps1", ".bat", ".cmd", ".sql",
		".php", ".rb", ".rs", ".swift", ".kt", ".cs", ".vb", ".fs",
		".pl", ".pm", ".r", ".R", ".m", ".scala", ".clj", ".hs", ".elm",
		".dockerfile", ".gitignore", ".gitattributes", ".editorconfig",
		".makefile", ".cmake", ".ninja", ".gradle", ".pom",
		".ini", ".cfg", ".conf", ".config", ".properties", ".env",
	}

	for _, textExt := range textExts {
		if ext == textExt {
			return true
		}
	}

	// Check for files without extensions that are commonly text
	name := strings.ToLower(fi.Name)
	textFiles := []string{
		"readme", "license", "changelog", "authors", "contributors",
		"makefile", "dockerfile", "gemfile", "rakefile", "procfile",
	}

	for _, textFile := range textFiles {
		if name == textFile {
			return true
		}
	}

	return false
}

// GetAllFiles recursively gets all files from a FileInfo tree
func (fi *FileInfo) GetAllFiles() []*FileInfo {
	var files []*FileInfo

	if !fi.IsDir {
		files = append(files, fi)
		return files
	}

	for _, child := range fi.Children {
		files = append(files, child.GetAllFiles()...)
	}

	return files
}

// GetTextFiles returns only text files from the file tree
func (fi *FileInfo) GetTextFiles() []*FileInfo {
	allFiles := fi.GetAllFiles()
	var textFiles []*FileInfo

	for _, file := range allFiles {
		if file.IsTextFile() {
			textFiles = append(textFiles, file)
		}
	}

	return textFiles
}

// FileSource indicates which side a file comes from
type FileSource int

const (
	SourceBoth  FileSource = iota // File exists in both directories
	SourceLeft                    // File exists only in left directory
	SourceRight                   // File exists only in right directory
)

// FileComparison represents a file that may exist on one or both sides
type FileComparison struct {
	RelativePath string
	LeftFile     *FileInfo // nil if file doesn't exist on left
	RightFile    *FileInfo // nil if file doesn't exist on right
	Source       FileSource
}

// relativeKey returns the key used to match a file between the left and right
// trees: its path relative to root. When root is a single file being compared
// directly (not a directory), this is always "." on both sides - that's what
// pairs the two files together as a single comparison regardless of whether
// their actual filenames match.
func relativeKey(root, file *FileInfo) (string, bool) {
	rel, err := filepath.Rel(root.Path, file.Path)
	if err != nil {
		return "", false
	}
	return rel, true
}

// FindCommonFiles finds files that exist in both file trees with the same relative path
func FindCommonFiles(left, right *FileInfo) map[string][2]*FileInfo {
	leftFiles := make(map[string]*FileInfo)
	rightFiles := make(map[string]*FileInfo)

	// Get all text files and create relative path maps
	for _, file := range left.GetTextFiles() {
		if rel, ok := relativeKey(left, file); ok {
			leftFiles[rel] = file
		}
	}

	for _, file := range right.GetTextFiles() {
		if rel, ok := relativeKey(right, file); ok {
			rightFiles[rel] = file
		}
	}

	// Find common files
	common := make(map[string][2]*FileInfo)
	for relPath, leftFile := range leftFiles {
		if rightFile, exists := rightFiles[relPath]; exists {
			common[relPath] = [2]*FileInfo{leftFile, rightFile}
		}
	}

	return common
}

// FindAllFiles finds all files from both directories, including unique files
func FindAllFiles(left, right *FileInfo) map[string]*FileComparison {
	leftFiles := make(map[string]*FileInfo)
	rightFiles := make(map[string]*FileInfo)

	// Get all text files and create relative path maps
	for _, file := range left.GetTextFiles() {
		if rel, ok := relativeKey(left, file); ok {
			leftFiles[rel] = file
		}
	}

	for _, file := range right.GetTextFiles() {
		if rel, ok := relativeKey(right, file); ok {
			rightFiles[rel] = file
		}
	}

	// Create comprehensive file comparison map
	allFiles := make(map[string]*FileComparison)

	// Add all left files
	for relPath, leftFile := range leftFiles {
		comparison := &FileComparison{
			RelativePath: relPath,
			LeftFile:     leftFile,
			Source:       SourceLeft,
		}
		allFiles[relPath] = comparison
	}

	// Process right files
	for relPath, rightFile := range rightFiles {
		if existing, exists := allFiles[relPath]; exists {
			// File exists on both sides
			existing.RightFile = rightFile
			existing.Source = SourceBoth
		} else {
			// File only exists on right side
			comparison := &FileComparison{
				RelativePath: relPath,
				RightFile:    rightFile,
				Source:       SourceRight,
			}
			allFiles[relPath] = comparison
		}
	}

	return allFiles
}
