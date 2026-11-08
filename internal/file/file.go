package file

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

// FileInfo represents information about a file or directory
type FileInfo struct {
	Path  string
	Name  string
	IsDir bool
	Size  int64
	// Content is the text of a file that isn't read from disk, such as one from
	// git. A file loaded from a directory leaves it empty: use Text.
	Content  string
	Children []*FileInfo

	onDisk bool // the text stays on disk until Text asks for it
	empty  bool // an empty file with no name marking it as text isn't listed
	binary bool // found to be binary when loaded, so it isn't listed
	failed bool // couldn't be read; removed from the tree when loading finishes
}

// Text returns the file's text, reading it from disk if that's where it is. It
// isn't kept, so a folder's files don't all stay in memory: hold on to what's
// returned for as long as it's on screen.
func (fi *FileInfo) Text() (string, error) {
	if !fi.onDisk {
		return fi.Content, nil
	}
	f, err := os.Open(fi.Path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	// A Builder sized in advance holds the file once, where reading to a byte
	// slice and converting holds it twice at the peak.
	var content strings.Builder
	content.Grow(int(fi.Size) + 1)
	if _, err := io.Copy(&content, f); err != nil {
		return "", err
	}
	return content.String(), nil
}

// Options says what to leave out when loading a directory.
type Options struct {
	// Ignore lists names and wildcards to skip. One without a slash matches at
	// any depth; one with a slash matches from the loaded directory. A trailing
	// slash matches directories only.
	Ignore     []string
	ShowHidden bool // include dotfiles (.git is always skipped)
}

// skips reports whether a directory entry is left out. rel is its slash-separated
// path from the loaded directory.
func (o Options) skips(name, rel string, isDir bool) bool {
	if name == ".git" || (strings.HasPrefix(name, ".") && !o.ShowHidden) {
		return true
	}
	for _, pattern := range o.Ignore {
		dirOnly := strings.HasSuffix(pattern, "/")
		pattern = strings.TrimSuffix(pattern, "/")
		if dirOnly && !isDir {
			continue
		}
		target := name
		if strings.Contains(pattern, "/") {
			target = rel
			pattern = strings.TrimPrefix(pattern, "/")
		}
		if ok, _ := path.Match(pattern, target); ok {
			return true
		}
	}
	return false
}

// Manager handles file operations
type Manager struct {
	opts Options
}

// New creates a new file manager
func New() *Manager {
	return &Manager{}
}

// SetOptions sets what later loads leave out.
func (m *Manager) SetOptions(opts Options) {
	m.opts = opts
}

// maxParallelReads limits how many files are read at once. It helps most on
// slow paths such as network drives or \\wsl.localhost.
const maxParallelReads = 16

// LoadPath loads a file or directory and returns FileInfo. A path named here is
// always loaded: Options only applies to what's found inside a directory.
func (m *Manager) LoadPath(path string) (*FileInfo, error) {
	var files []*FileInfo
	root, err := m.scan(path, "", &files)
	if err != nil {
		return nil, err
	}

	jobs := make(chan *FileInfo)
	var wg sync.WaitGroup
	for i := 0; i < maxParallelReads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for fi := range jobs {
				fi.failed = fi.read() != nil
			}
		}()
	}
	for _, fi := range files {
		jobs <- fi
	}
	close(jobs)
	wg.Wait()

	if root.failed {
		return nil, fmt.Errorf("failed to read file %s: %w", path, root.read())
	}
	root.removeFailed()
	return root, nil
}

// removeFailed drops files that couldn't be read, as if they weren't there.
func (fi *FileInfo) removeFailed() {
	kept := fi.Children[:0]
	for _, child := range fi.Children {
		if !child.failed {
			child.removeFailed()
			kept = append(kept, child)
		}
	}
	fi.Children = kept
}

// scan builds the tree for filePath without reading any file's content, adding
// each file to files. rel is its slash-separated path from the root of the load.
func (m *Manager) scan(filePath, rel string, files *[]*FileInfo) (*FileInfo, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat path %s: %w", filePath, err)
	}

	fileInfo := &FileInfo{
		Path:  filePath,
		Name:  filepath.Base(filePath),
		IsDir: info.IsDir(),
		Size:  info.Size(),
	}

	if info.IsDir() {
		children, err := m.loadDirectory(filePath, rel, files)
		if err != nil {
			return nil, fmt.Errorf("failed to load directory %s: %w", filePath, err)
		}
		fileInfo.Children = children
	} else {
		*files = append(*files, fileInfo)
	}

	return fileInfo, nil
}

// loadDirectory recursively loads directory contents
func (m *Manager) loadDirectory(dirPath, rel string, files *[]*FileInfo) ([]*FileInfo, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	var children []*FileInfo
	for _, entry := range entries {
		childPath := filepath.Join(dirPath, entry.Name())
		childRel := path.Join(rel, entry.Name())

		// Decided before reading anything, so an ignored folder costs nothing.
		isDir := entry.IsDir()
		if entry.Type()&os.ModeSymlink != 0 {
			if info, err := os.Stat(childPath); err == nil {
				isDir = info.IsDir()
			}
		}
		if m.opts.skips(entry.Name(), childRel, isDir) {
			continue
		}

		childInfo, err := m.scan(childPath, childRel, files)
		if err != nil {
			// Log error but continue with other files
			continue
		}
		children = append(children, childInfo)
	}

	return children, nil
}

// Stamp identifies the version of the file on disk, by its size and time, so a
// caller can tell whether text it read earlier is still current. It's empty for
// text that isn't on disk, such as from git.
func (fi *FileInfo) Stamp() string {
	if !fi.onDisk {
		return ""
	}
	info, err := os.Stat(fi.Path)
	if err != nil {
		return "gone"
	}
	return fmt.Sprintf("%d/%d", info.Size(), info.ModTime().UnixNano())
}

// sniffSize is how much of a file decides whether it's binary, as in git.
const sniffSize = 8000

// sniffBuffers saves allocating a buffer for each file in a large folder.
var sniffBuffers = sync.Pool{New: func() any { return make([]byte, sniffSize) }}

// read checks the file can be read and whether it's text, from its start alone.
func (fi *FileInfo) read() error {
	f, err := os.Open(fi.Path)
	if err != nil {
		return err
	}
	defer f.Close()

	head := sniffBuffers.Get().([]byte)
	defer sniffBuffers.Put(head)
	n, err := io.ReadFull(f, head)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return err
	}
	fi.onDisk = true
	fi.empty = n == 0
	fi.binary = !fi.hasTextName() && bytes.IndexByte(head[:n], 0) >= 0
	return nil
}

// HasBinarySide reports whether either side held in memory, like a file from
// git, is binary by the rule a folder comparison uses to leave a file out: a
// NUL byte near the start, unless the name marks it as text.
func (c *FileComparison) HasBinarySide() bool {
	for _, fi := range []*FileInfo{c.LeftFile, c.RightFile} {
		if fi == nil || fi.onDisk || fi.hasTextName() {
			continue
		}
		head := fi.Content[:min(len(fi.Content), sniffSize)]
		if strings.IndexByte(head, 0) >= 0 {
			return true
		}
	}
	return false
}

// IsTextFile checks if the file appears to be a text file. Known extensions and
// names are accepted directly; anything else is sniffed for binary content.
func (fi *FileInfo) IsTextFile() bool {
	if fi.IsDir || fi.binary {
		return false
	}
	if fi.hasTextName() {
		return true
	}
	if fi.onDisk {
		return !fi.empty
	}
	return looksLikeText(fi.Content)
}

// hasTextName reports whether the file's extension or name marks it as text.
func (fi *FileInfo) hasTextName() bool {

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

// looksLikeText reports whether content appears to be text, using the same
// heuristic as git: no NUL bytes within the first 8000 bytes.
func looksLikeText(content string) bool {
	if content == "" {
		return false
	}
	sniff := content
	if len(sniff) > 8000 {
		sniff = sniff[:8000]
	}
	return !strings.Contains(sniff, "\x00")
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

// Identical finds which files on both sides have no differences, comparing up
// to maxParallelReads at once and keeping none of their text. same says whether
// two texts count as the same when something is being ignored. Leave it nil to
// compare bytes, where files of different sizes needn't be read at all.
func Identical(files map[string]*FileComparison, same func(left, right string) bool) map[string]bool {
	type result struct {
		relPath   string
		identical bool
	}
	jobs := make(chan string)
	results := make(chan result)
	var wg sync.WaitGroup
	for i := 0; i < maxParallelReads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for relPath := range jobs {
				comp := files[relPath]
				results <- result{relPath, isIdentical(comp.LeftFile, comp.RightFile, same)}
			}
		}()
	}
	go func() {
		for relPath, comp := range files {
			if comp.Source == SourceBoth {
				jobs <- relPath
			}
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	identical := make(map[string]bool)
	for r := range results {
		if r.identical {
			identical[r.relPath] = true
		}
	}
	return identical
}

// isIdentical compares two files. One that can't be read counts as different, so
// that it stays in the list and shows the error when it's opened.
func isIdentical(left, right *FileInfo, same func(left, right string) bool) bool {
	if same == nil && left.onDisk && right.onDisk {
		return left.Size == right.Size && sameBytes(left.Path, right.Path)
	}
	l, err := left.Text()
	if err != nil {
		return false
	}
	r, err := right.Text()
	if err != nil {
		return false
	}
	if same != nil {
		return same(l, r)
	}
	return l == r
}

// compareBuffers saves allocating buffers for each file in a large folder.
var compareBuffers = sync.Pool{New: func() any { return make([]byte, 64*1024) }}

// sameBytes compares two files on disk, stopping at the first difference.
func sameBytes(leftPath, rightPath string) bool {
	l, err := os.Open(leftPath)
	if err != nil {
		return false
	}
	defer l.Close()
	r, err := os.Open(rightPath)
	if err != nil {
		return false
	}
	defer r.Close()

	leftBuf := compareBuffers.Get().([]byte)
	defer compareBuffers.Put(leftBuf)
	rightBuf := compareBuffers.Get().([]byte)
	defer compareBuffers.Put(rightBuf)
	for {
		n, leftErr := io.ReadFull(l, leftBuf)
		m, rightErr := io.ReadFull(r, rightBuf)
		if n != m || !bytes.Equal(leftBuf[:n], rightBuf[:m]) {
			return false
		}
		leftDone := leftErr == io.EOF || leftErr == io.ErrUnexpectedEOF
		rightDone := rightErr == io.EOF || rightErr == io.ErrUnexpectedEOF
		if leftDone || rightDone {
			return leftDone && rightDone
		}
		if leftErr != nil || rightErr != nil {
			return false
		}
	}
}
