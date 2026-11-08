package file

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
	if got, _ := fi.Text(); got != "hello\n" {
		t.Errorf("content mismatch: got %q", got)
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

func TestIsTextFileUnknownExtensionSniffsContent(t *testing.T) {
	fi := &FileInfo{Path: "notes.log", Name: "notes.log", Content: "plain text\nline two\n"}
	if !fi.IsTextFile() {
		t.Error("expected text content with unknown extension to be a text file")
	}

	fi = &FileInfo{Path: "data.dat", Name: "data.dat", Content: "\x7fELF\x00\x01\x02"}
	if fi.IsTextFile() {
		t.Error("expected content with NUL bytes to NOT be a text file")
	}
}

// loadedNames loads root with opts and returns its files' paths, relative to it.
func loadedNames(t *testing.T, root string, opts Options) map[string]bool {
	t.Helper()
	m := New()
	m.SetOptions(opts)
	info, err := m.LoadPath(root)
	if err != nil {
		t.Fatal(err)
	}
	names := make(map[string]bool)
	for _, f := range info.GetAllFiles() {
		rel, _ := filepath.Rel(root, f.Path)
		names[filepath.ToSlash(rel)] = true
	}
	return names
}

func ignoreTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, rel := range []string{
		"a.txt", "b.log", ".env", ".github/ci.yml", ".git/config", "src/.git/config",
		"node_modules/x/y.js", "src/node_modules/z.js", "src/c.txt", "src/d.log",
		"docs/generated/g.md", "docs/manual.md", "src/docs/generated/h.md", "build", "src/build/out.txt",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestLoadOptions(t *testing.T) {
	root := ignoreTree(t)
	base := []string{"a.txt", "b.log", "build", "docs/generated/g.md", "docs/manual.md", "node_modules/x/y.js",
		"src/build/out.txt", "src/c.txt", "src/d.log", "src/docs/generated/h.md", "src/node_modules/z.js"}
	without := func(remove ...string) []string {
		var out []string
		for _, name := range base {
			keep := true
			for _, r := range remove {
				if name == r {
					keep = false
				}
			}
			if keep {
				out = append(out, name)
			}
		}
		return out
	}
	cases := []struct {
		name string
		opts Options
		want []string
	}{
		{"default skips dotfiles", Options{}, base},
		{"a name matches at any depth", Options{Ignore: []string{"node_modules"}}, without("node_modules/x/y.js", "src/node_modules/z.js")},
		{"a wildcard matches at any depth", Options{Ignore: []string{"*.log"}}, without("b.log", "src/d.log")},
		{"a slash matches from the root", Options{Ignore: []string{"docs/generated"}}, without("docs/generated/g.md")},
		{"a leading slash matches from the root", Options{Ignore: []string{"/a.txt", "/c.txt"}}, without("a.txt")},
		{"a trailing slash matches folders only", Options{Ignore: []string{"build/"}}, without("src/build/out.txt")},
		{"a wildcard in a path", Options{Ignore: []string{"src/*.log"}}, without("src/d.log")},
		{"show hidden", Options{ShowHidden: true}, append(append([]string{}, base...), ".env", ".github/ci.yml")},
		{"show hidden can still ignore", Options{ShowHidden: true, Ignore: []string{".env"}}, append(append([]string{}, base...), ".github/ci.yml")},
	}
	for _, c := range cases {
		got := loadedNames(t, root, c.opts)
		want := make(map[string]bool)
		for _, name := range c.want {
			want[name] = true
		}
		for name := range want {
			if !got[name] {
				t.Errorf("%s: %s is missing", c.name, name)
			}
		}
		for name := range got {
			if !want[name] {
				t.Errorf("%s: %s should be left out", c.name, name)
			}
		}
	}
}

func TestIgnoredFoldersAreNotRead(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read anything")
	}
	root := t.TempDir()
	secret := filepath.Join(root, "locked")
	if err := os.Mkdir(secret, 0755); err != nil {
		t.Fatal(err)
	}
	blocked := filepath.Join(secret, "f.txt")
	if err := os.WriteFile(blocked, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ok.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	// A FIFO blocks forever if opened, so reaching it would hang this test.
	if err := syscallMkfifo(filepath.Join(secret, "pipe")); err != nil {
		t.Skip(err)
	}
	got := loadedNames(t, root, Options{Ignore: []string{"locked"}})
	if !got["ok.txt"] || len(got) != 1 {
		t.Errorf("got %v", got)
	}
}

func TestNamedPathsAreNeverIgnored(t *testing.T) {
	root := ignoreTree(t)
	m := New()
	m.SetOptions(Options{Ignore: []string{"*.log", "node_modules"}})

	f, err := m.LoadPath(filepath.Join(root, "b.log"))
	if text, _ := f.Text(); err != nil || text != "x\n" {
		t.Errorf("a named file: %+v, %v", f, err)
	}
	dir, err := m.LoadPath(filepath.Join(root, "node_modules"))
	if err != nil || len(dir.GetAllFiles()) != 1 {
		t.Errorf("a named folder: %v files, %v", len(dir.GetAllFiles()), err)
	}
	hidden, err := New().LoadPath(filepath.Join(root, ".env"))
	if text, _ := hidden.Text(); err != nil || text != "x\n" {
		t.Errorf("a named dotfile: %+v, %v", hidden, err)
	}
}

func TestBinaryFilesAreNotReadInFull(t *testing.T) {
	root := t.TempDir()
	huge := filepath.Join(root, "huge.bin")
	f, err := os.Create(huge)
	if err != nil {
		t.Fatal(err)
	}
	// Sparse, so it's instant to make, but reading it all would take 64 GB.
	if err := f.Truncate(64 << 30); err != nil {
		t.Skipf("can't make a sparse file here: %v", err)
	}
	f.Close()
	for name, content := range map[string]string{
		"text.txt":  "hello\n",
		"notes":     "no extension, but text\n",
		"data.txt":  "known extension\x00with a NUL\n",
		"late.bin":  strings.Repeat("x", sniffSize) + "\x00 after the part that's checked\n",
		"empty.dat": "",
		"nul.dat":   "a\x00b",
		"Makefile":  "all:\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	info, err := New().LoadPath(root)
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]string)
	for _, fi := range info.GetTextFiles() {
		got[fi.Name], _ = fi.Text()
	}
	want := map[string]string{
		"text.txt": "hello\n",
		"notes":    "no extension, but text\n",
		"data.txt": "known extension\x00with a NUL\n",
		"late.bin": strings.Repeat("x", sniffSize) + "\x00 after the part that's checked\n",
		"Makefile": "all:\n",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("text files = %v", got)
	}
}

func TestLoadKeepsTreeOrderAndSkipsUnreadable(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 200; i++ {
		name := filepath.Join(root, fmt.Sprintf("d%d", i%5), fmt.Sprintf("f%03d.txt", i))
		if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(fmt.Sprintf("content %d\n", i)), 0644); err != nil {
			t.Fatal(err)
		}
	}
	first, err := New().LoadPath(root)
	if err != nil {
		t.Fatal(err)
	}
	second, _ := New().LoadPath(root)
	a, b := first.GetAllFiles(), second.GetAllFiles()
	if len(a) != 200 || len(a) != len(b) {
		t.Fatalf("%d and %d files", len(a), len(b))
	}
	for i := range a {
		textA, _ := a[i].Text()
		textB, _ := b[i].Text()
		if a[i].Path != b[i].Path || textA == "" || textA != textB {
			t.Fatalf("file %d differs between loads: %s %s", i, a[i].Path, b[i].Path)
		}
	}

	if os.Geteuid() == 0 {
		return
	}
	locked := filepath.Join(root, "d0", "locked.txt")
	if err := os.WriteFile(locked, []byte("x"), 0000); err != nil {
		t.Fatal(err)
	}
	info, err := New().LoadPath(root)
	if err != nil {
		t.Fatalf("an unreadable file inside a folder shouldn't fail the load: %v", err)
	}
	for _, fi := range info.GetAllFiles() {
		if fi.Path == locked {
			t.Error("the unreadable file should be left out")
		}
	}
	if _, err := New().LoadPath(locked); err == nil {
		t.Error("naming an unreadable file should be an error")
	}
}

func TestLoadingKeepsNoFileText(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "f.txt")
	if err := os.WriteFile(path, []byte("original\n"), 0644); err != nil {
		t.Fatal(err)
	}
	info, err := New().LoadPath(root)
	if err != nil {
		t.Fatal(err)
	}
	fi := info.GetTextFiles()[0]
	if fi.Content != "" {
		t.Errorf("Content = %q, want the text left on disk", fi.Content)
	}

	// Text reads what's there now.
	if err := os.WriteFile(path, []byte("edited since\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got, err := fi.Text(); err != nil || got != "edited since\n" {
		t.Errorf("Text = %q, %v", got, err)
	}

	// A file gone since is an error, not empty text.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if got, err := fi.Text(); err == nil {
		t.Errorf("Text = %q, want an error for a deleted file", got)
	}

	// Text not from disk is returned as it is.
	inline := &FileInfo{Path: "git:HEAD:f", Name: "f", Content: "from git\n"}
	if got, err := inline.Text(); err != nil || got != "from git\n" {
		t.Errorf("inline Text = %q, %v", got, err)
	}
}

func TestIdentical(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	big := strings.Repeat("a long line of text for several chunks\n", 8000) // over 64 KB
	pairs := map[string][2]string{
		"same.txt":      {"same\n", "same\n"},
		"empty.txt":     {"", ""},
		"size.txt":      {"short\n", "longer text\n"},
		"content.txt":   {"abc\n", "abd\n"},
		"crlf.txt":      {"a\r\nb\r\n", "a\nb\n"},
		"big-same.txt":  {big, big},
		"big-diff.txt":  {big + "x\n", big + "y\n"},
		"big-early.txt": {"x" + big, "y" + big},
	}
	for name, p := range pairs {
		if err := os.WriteFile(filepath.Join(left, name), []byte(p[0]), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(right, name), []byte(p[1]), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(left, "only.txt"), []byte("o\n"), 0644); err != nil {
		t.Fatal(err)
	}
	l, _ := New().LoadPath(left)
	r, _ := New().LoadPath(right)
	files := FindAllFiles(l, r)

	got := Identical(files, nil)
	want := map[string]bool{"same.txt": true, "empty.txt": true, "big-same.txt": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("bytes: %v", got)
	}

	sameIgnoringCase := func(a, b string) bool { return strings.EqualFold(a, b) }
	if got := Identical(files, sameIgnoringCase); got["size.txt"] || !got["same.txt"] {
		t.Errorf("with a comparison: %v", got)
	}

	// With a comparison, different sizes can still be the same: no size shortcut.
	ignoreCR := func(a, b string) bool { return strings.ReplaceAll(a, "\r", "") == strings.ReplaceAll(b, "\r", "") }
	if got := Identical(files, ignoreCR); !got["crlf.txt"] {
		t.Errorf("ignoring CR: %v", got)
	}

	// Inline text, as in git mode.
	inline := map[string]*FileComparison{
		"a": {Source: SourceBoth, LeftFile: &FileInfo{Content: "x"}, RightFile: &FileInfo{Content: "x"}},
		"b": {Source: SourceBoth, LeftFile: &FileInfo{Content: "x"}, RightFile: &FileInfo{Content: "y"}},
	}
	if got := Identical(inline, nil); !got["a"] || got["b"] {
		t.Errorf("inline: %v", got)
	}

	// A file that can't be read isn't identical, so it stays listed.
	os.Remove(filepath.Join(left, "same.txt"))
	if got := Identical(files, nil); got["same.txt"] {
		t.Error("a deleted file counted as identical")
	}
}

// Files of different sizes are known to differ without being read.
func TestIdenticalDoesNotReadFilesOfDifferentSizes(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read anything")
	}
	left, right := t.TempDir(), t.TempDir()
	lPath, rPath := filepath.Join(left, "f.txt"), filepath.Join(right, "f.txt")
	if err := os.WriteFile(lPath, []byte("short\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rPath, []byte("much longer\n"), 0644); err != nil {
		t.Fatal(err)
	}
	l, _ := New().LoadPath(left)
	r, _ := New().LoadPath(right)
	files := FindAllFiles(l, r)
	// Replaced by something that blocks forever if opened.
	os.Remove(lPath)
	if err := syscallMkfifo(lPath); err != nil {
		t.Skip(err)
	}
	if Identical(files, nil)["f.txt"] {
		t.Error("files of different sizes reported as identical")
	}
}

func TestHasBinarySide(t *testing.T) {
	side := func(name, content string) *FileInfo { return &FileInfo{Path: name, Name: name, Content: content} }
	cases := []struct {
		name string
		comp FileComparison
		want bool
	}{
		{"text both", FileComparison{LeftFile: side("a", "x\n"), RightFile: side("a", "y\n")}, false},
		{"binary left", FileComparison{LeftFile: side("a.png", "\x89PNG\x00"), RightFile: side("a.png", "x")}, true},
		{"binary right only", FileComparison{RightFile: side("a.dat", "\x00\x01")}, true},
		{"NUL past the sniffed start", FileComparison{LeftFile: side("a", strings.Repeat("x", sniffSize)+"\x00")}, false},
		{"text name keeps it", FileComparison{LeftFile: side("a.txt", "\x00")}, false},
		{"empty", FileComparison{LeftFile: side("a", "")}, false},
	}
	for _, c := range cases {
		if got := c.comp.HasBinarySide(); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
