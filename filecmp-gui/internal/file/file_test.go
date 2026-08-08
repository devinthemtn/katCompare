package file

import (
	"os"
	"path/filepath"
	"testing"
)

// ---- IsTextFile -------------------------------------------------------------

func TestIsTextFileKnownExtensions(t *testing.T) {
	textExts := []string{
		".go", ".js", ".ts", ".py", ".java", ".c", ".cpp", ".h",
		".rs", ".swift", ".kt", ".cs", ".php", ".rb",
		".html", ".htm", ".css", ".xml", ".json", ".yaml", ".yml", ".toml",
		".sh", ".bash", ".zsh", ".txt", ".md", ".csv",
		".conf", ".ini", ".cfg", ".sql",
	}
	for _, ext := range textExts {
		fi := &FileInfo{Path: "file" + ext, Name: "file" + ext}
		if !fi.IsTextFile() {
			t.Errorf("expected %s to be a text file", ext)
		}
	}
}

func TestIsTextFileBinaryExtensions(t *testing.T) {
	binaryExts := []string{".exe", ".bin", ".jpg", ".png", ".zip", ".tar", ".pdf", ".mp4"}
	for _, ext := range binaryExts {
		fi := &FileInfo{Path: "file" + ext, Name: "file" + ext}
		if fi.IsTextFile() {
			t.Errorf("expected %s to NOT be a text file", ext)
		}
	}
}

func TestIsTextFileSpecialNames(t *testing.T) {
	specialNames := []string{"README", "LICENSE", "Makefile", "Dockerfile", "Gemfile"}
	for _, name := range specialNames {
		fi := &FileInfo{Path: name, Name: name}
		if !fi.IsTextFile() {
			t.Errorf("expected %s to be a text file", name)
		}
	}
}

func TestIsTextFileDirectory(t *testing.T) {
	fi := &FileInfo{Path: "/some/dir", Name: "dir", IsDir: true}
	if fi.IsTextFile() {
		t.Error("directory should not be a text file")
	}
}

// ---- FindAllFiles categorisation -------------------------------------------

func makeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for relPath, content := range files {
		full := filepath.Join(root, relPath)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestFindAllFilesSourceBoth(t *testing.T) {
	m := New()
	leftRoot := makeTree(t, map[string]string{
		"common.go": "package main\n",
	})
	rightRoot := makeTree(t, map[string]string{
		"common.go": "package main // changed\n",
	})

	leftInfo, _ := m.LoadPath(leftRoot)
	rightInfo, _ := m.LoadPath(rightRoot)
	all := FindAllFiles(leftInfo, rightInfo)

	comp, ok := all["common.go"]
	if !ok {
		t.Fatal("common.go not found in FindAllFiles result")
	}
	if comp.Source != SourceBoth {
		t.Errorf("common.go: expected SourceBoth, got %v", comp.Source)
	}
	if comp.LeftFile == nil || comp.RightFile == nil {
		t.Error("common.go: both LeftFile and RightFile should be non-nil")
	}
}

func TestFindAllFilesSourceLeft(t *testing.T) {
	m := New()
	leftRoot := makeTree(t, map[string]string{
		"only-left.go": "package main\n",
	})
	rightRoot := makeTree(t, map[string]string{
		"other.go": "package main\n",
	})

	leftInfo, _ := m.LoadPath(leftRoot)
	rightInfo, _ := m.LoadPath(rightRoot)
	all := FindAllFiles(leftInfo, rightInfo)

	comp, ok := all["only-left.go"]
	if !ok {
		t.Fatal("only-left.go not found")
	}
	if comp.Source != SourceLeft {
		t.Errorf("only-left.go: expected SourceLeft, got %v", comp.Source)
	}
	if comp.LeftFile == nil {
		t.Error("only-left.go: LeftFile should be non-nil")
	}
	if comp.RightFile != nil {
		t.Error("only-left.go: RightFile should be nil")
	}
}

func TestFindAllFilesSourceRight(t *testing.T) {
	m := New()
	leftRoot := makeTree(t, map[string]string{
		"other.go": "package main\n",
	})
	rightRoot := makeTree(t, map[string]string{
		"only-right.go": "package main\n",
	})

	leftInfo, _ := m.LoadPath(leftRoot)
	rightInfo, _ := m.LoadPath(rightRoot)
	all := FindAllFiles(leftInfo, rightInfo)

	comp, ok := all["only-right.go"]
	if !ok {
		t.Fatal("only-right.go not found")
	}
	if comp.Source != SourceRight {
		t.Errorf("only-right.go: expected SourceRight, got %v", comp.Source)
	}
	if comp.RightFile == nil {
		t.Error("only-right.go: RightFile should be non-nil")
	}
	if comp.LeftFile != nil {
		t.Error("only-right.go: LeftFile should be nil")
	}
}

func TestFindAllFilesMixed(t *testing.T) {
	m := New()
	leftRoot := makeTree(t, map[string]string{
		"shared.go":    "package main\n",
		"left-only.go": "package main\n",
	})
	rightRoot := makeTree(t, map[string]string{
		"shared.go":     "package main // v2\n",
		"right-only.go": "package main\n",
	})

	leftInfo, _ := m.LoadPath(leftRoot)
	rightInfo, _ := m.LoadPath(rightRoot)
	all := FindAllFiles(leftInfo, rightInfo)

	if len(all) != 3 {
		t.Errorf("expected 3 entries, got %d: %v", len(all), keys(all))
	}
	if all["shared.go"].Source != SourceBoth {
		t.Errorf("shared.go: expected SourceBoth")
	}
	if all["left-only.go"].Source != SourceLeft {
		t.Errorf("left-only.go: expected SourceLeft")
	}
	if all["right-only.go"].Source != SourceRight {
		t.Errorf("right-only.go: expected SourceRight")
	}
}

func TestFindAllFilesSkipsHiddenFiles(t *testing.T) {
	m := New()
	leftRoot := makeTree(t, map[string]string{
		"visible.go": "package main\n",
		".hidden.go": "package main\n",
	})
	rightRoot := makeTree(t, map[string]string{
		"visible.go": "package main\n",
	})

	leftInfo, _ := m.LoadPath(leftRoot)
	rightInfo, _ := m.LoadPath(rightRoot)
	all := FindAllFiles(leftInfo, rightInfo)

	if _, found := all[".hidden.go"]; found {
		t.Error(".hidden.go should be excluded")
	}
	if _, found := all["visible.go"]; !found {
		t.Error("visible.go should be included")
	}
}

func TestFindCommonFilesOnlyReturnsShared(t *testing.T) {
	m := New()
	leftRoot := makeTree(t, map[string]string{
		"shared.go":    "a\n",
		"left-only.go": "b\n",
	})
	rightRoot := makeTree(t, map[string]string{
		"shared.go":     "a\n",
		"right-only.go": "c\n",
	})

	leftInfo, _ := m.LoadPath(leftRoot)
	rightInfo, _ := m.LoadPath(rightRoot)
	common := FindCommonFiles(leftInfo, rightInfo)

	if len(common) != 1 {
		t.Errorf("expected 1 common file, got %d: %v", len(common), keys2(common))
	}
	if _, ok := common["shared.go"]; !ok {
		t.Error("shared.go should be in common files")
	}
}

// ---- LoadPath ---------------------------------------------------------------

func TestLoadPathFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := New()
	fi, err := m.LoadPath(path)
	if err != nil {
		t.Fatalf("LoadPath error: %v", err)
	}
	if fi.IsDir {
		t.Error("file should not be IsDir")
	}
	if fi.Content != "hello\n" {
		t.Errorf("content mismatch: got %q", fi.Content)
	}
}

func TestLoadPathDirectory(t *testing.T) {
	dir := makeTree(t, map[string]string{
		"a.go": "package main\n",
		"b.go": "package main\n",
	})

	m := New()
	fi, err := m.LoadPath(dir)
	if err != nil {
		t.Fatalf("LoadPath directory error: %v", err)
	}
	if !fi.IsDir {
		t.Error("directory should be IsDir")
	}
	if len(fi.Children) < 2 {
		t.Errorf("expected at least 2 children, got %d", len(fi.Children))
	}
}

func TestLoadPathNonExistent(t *testing.T) {
	m := New()
	_, err := m.LoadPath("/nonexistent/path/does/not/exist.txt")
	if err == nil {
		t.Error("expected error for non-existent path")
	}
}

// ---- helpers ----------------------------------------------------------------

func keys(m map[string]*FileComparison) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func keys2(m map[string][2]*FileInfo) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
