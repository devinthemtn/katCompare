package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"katcompare/internal/differ"
	"katcompare/internal/file"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestDialogStartDir(t *testing.T) {
	wd := t.TempDir()
	t.Chdir(wd)
	dir := t.TempDir()
	f := filepath.Join(dir, "f.txt")
	writeFile(t, f, "x\n")

	cases := []struct{ name, in, want string }{
		{"empty uses the current directory", "", wd},
		{"blank uses the current directory", "   ", wd},
		{"a directory", dir, dir},
		{"a file opens in its folder", f, dir},
		{"a missing file in an existing folder", filepath.Join(dir, "new.txt"), dir},
		{"a missing folder falls back", filepath.Join(dir, "nope", "x.txt"), wd},
		{"a bare missing name falls back", "nope.txt", wd},
	}
	for _, c := range cases {
		if got := dialogStartDir(c.in); got != c.want {
			t.Errorf("%s: dialogStartDir(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func TestInitialPaths(t *testing.T) {
	wd := t.TempDir()
	t.Chdir(wd)
	abs := filepath.Join(t.TempDir(), "b")

	got := initialPaths([]string{"-psn_0_123", "a", abs, "c"})
	want := []string{filepath.Join(wd, "a"), abs}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("initialPaths = %v, want %v (flags skipped, at most two)", got, want)
	}
	if got := initialPaths(nil); got != nil {
		t.Errorf("initialPaths(nil) = %v, want nil", got)
	}
	if got := NewApp(want).GetInitialPaths(); !reflect.DeepEqual(got, want) {
		t.Errorf("GetInitialPaths = %v, want %v", got, want)
	}
}

func TestVersion(t *testing.T) {
	saved := version
	t.Cleanup(func() { version = saved })

	version = "v9.9.9"
	if got := resolveVersion(); got != "v9.9.9" {
		t.Errorf("resolveVersion with a stamped version = %q", got)
	}
	if got := NewApp(nil).GetVersion(); got != "v9.9.9" {
		t.Errorf("GetVersion = %q", got)
	}

	version = "dev"
	if got := resolveVersion(); got == "" || got == "(devel)" {
		t.Errorf("resolveVersion without a stamp = %q, want dev or a module version", got)
	}
}

func TestDisplayNameForFile(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	lf, rf := filepath.Join(left, "a.txt"), filepath.Join(right, "b.txt")
	writeFile(t, lf, "1\n")
	writeFile(t, rf, "2\n")

	a := NewApp(nil)
	if _, err := a.LoadComparison(lf, rf); err != nil {
		t.Fatal(err)
	}
	if got := a.displayNameForFile("."); got != "a.txt ↔ b.txt" {
		t.Errorf("two files with different names: %q", got)
	}

	same := filepath.Join(right, "a.txt")
	writeFile(t, same, "2\n")
	if _, err := a.LoadComparison(lf, same); err != nil {
		t.Fatal(err)
	}
	if got := a.displayNameForFile("."); got != "a.txt" {
		t.Errorf("two files with the same name: %q", got)
	}

	lfi, rfi := &file.FileInfo{Name: "l.txt"}, &file.FileInfo{Name: "r.txt"}
	a.allFiles = map[string]*file.FileComparison{".": {Source: file.SourceLeft, LeftFile: lfi}}
	if got := a.displayNameForFile("."); got != "l.txt" {
		t.Errorf("left only: %q", got)
	}
	a.allFiles["."] = &file.FileComparison{Source: file.SourceRight, RightFile: rfi}
	if got := a.displayNameForFile("."); got != "r.txt" {
		t.Errorf("right only: %q", got)
	}
	if got := a.displayNameForFile("dir/x.txt"); got != "dir/x.txt" {
		t.Errorf("a path in a folder comparison: %q", got)
	}
	delete(a.allFiles, ".")
	if got := a.displayNameForFile("."); got != "." {
		t.Errorf("unknown file: %q", got)
	}
}

func TestTypeStrings(t *testing.T) {
	sources := map[file.FileSource]string{
		file.SourceBoth: "both", file.SourceLeft: "left", file.SourceRight: "right", file.FileSource(99): "both",
	}
	for s, want := range sources {
		if got := sourceString(s); got != want {
			t.Errorf("sourceString(%v) = %q, want %q", s, got, want)
		}
	}
	rows := map[differ.SideBySideRowType]string{
		differ.SBSInsert: "insert", differ.SBSDelete: "delete", differ.SBSModified: "modified", differ.SBSEqual: "equal",
	}
	for r, want := range rows {
		if got := sbsTypeString(r); got != want {
			t.Errorf("sbsTypeString(%v) = %q, want %q", r, got, want)
		}
	}
}

func TestCopyFiles(t *testing.T) {
	left, right := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(left, "both.txt"), "l\n")
	writeFile(t, filepath.Join(right, "both.txt"), "r\n")
	writeFile(t, filepath.Join(left, "sub", "left-only.txt"), "from left\n")
	writeFile(t, filepath.Join(right, "right-only.txt"), "from right\n")

	a := NewApp(nil)
	if _, err := a.CopyFiles([]string{"x"}, "to-right"); err == nil {
		t.Error("copy with no comparison loaded should fail")
	}
	if _, err := a.LoadComparison(left, right); err != nil {
		t.Fatal(err)
	}

	leftOnly := filepath.Join("sub", "left-only.txt")
	msg, err := a.CopyFiles([]string{leftOnly, "both.txt", "missing.txt", "right-only.txt"}, "to-right")
	if err != nil {
		t.Fatal(err)
	}
	if msg != "Copied 1 file(s), skipped 3" {
		t.Errorf("to-right: %q", msg)
	}
	if got, _ := os.ReadFile(filepath.Join(right, leftOnly)); string(got) != "from left\n" {
		t.Errorf("copied to the right = %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(right, "both.txt")); string(got) != "r\n" {
		t.Errorf("a file on both sides was overwritten: %q", got)
	}

	msg, err = a.CopyFiles([]string{"right-only.txt"}, "to-left")
	if err != nil || msg != "Copied 1 file(s), skipped 0" {
		t.Errorf("to-left: %q, %v", msg, err)
	}
	if got, _ := os.ReadFile(filepath.Join(left, "right-only.txt")); string(got) != "from right\n" {
		t.Errorf("copied to the left = %q", got)
	}

	// A file standing where the destination folder should be.
	if _, err := a.LoadComparison(left, right); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(left, "blocked", "f.txt"), "x\n")
	writeFile(t, filepath.Join(right, "blocked"), "a file, not a folder\n")
	if _, err := a.LoadComparison(left, right); err != nil {
		t.Fatal(err)
	}
	msg, err = a.CopyFiles([]string{filepath.Join("blocked", "f.txt")}, "to-right")
	if err != nil || !strings.Contains(msg, "1 error(s)") {
		t.Errorf("blocked copy: %q, %v", msg, err)
	}
}

func TestCopyFilesRefusedInGitMode(t *testing.T) {
	conflictRepo(t)
	a := NewApp(nil)
	if _, err := a.LoadGitComparison("HEAD", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CopyFiles([]string{"f.txt"}, "to-right"); err == nil {
		t.Error("copy in git mode should fail")
	}
}

func TestListGitRefs(t *testing.T) {
	a := NewApp(nil)
	if _, err := a.ListGitRefs(); err == nil {
		t.Error("ListGitRefs outside a repository should fail")
	}

	conflictRepo(t)
	opts, err := a.ListGitRefs()
	if err != nil {
		t.Fatal(err)
	}
	refs := map[string]bool{}
	for _, o := range opts {
		refs[o.Ref] = true
		if o.Label == "" || o.Group == "" {
			t.Errorf("option without a label or group: %+v", o)
		}
	}
	for _, want := range []string{"", "HEAD", "main", "other"} {
		if !refs[want] {
			t.Errorf("ref %q missing from %v", want, opts)
		}
	}
}

func TestSuggestPaths(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	for _, name := range []string{"Alpha.txt", "alpine.txt", "beta.txt", ".hidden"} {
		writeFile(t, filepath.Join(dir, name), "")
	}
	if err := os.Mkdir(filepath.Join(dir, "album"), 0755); err != nil {
		t.Fatal(err)
	}
	sep := string(filepath.Separator)

	if got := NewApp(nil).SuggestPaths(""); got != nil {
		t.Errorf("empty input: %v", got)
	}
	a := NewApp(nil)
	if got, want := a.SuggestPaths("al"), []string{"Alpha.txt", "album" + sep, "alpine.txt"}; !reflect.DeepEqual(got, want) {
		t.Errorf("relative prefix, any case: %v, want %v", got, want)
	}
	if got, want := a.SuggestPaths(".h"), []string{".hidden"}; !reflect.DeepEqual(got, want) {
		t.Errorf("hidden files when asked for: %v, want %v", got, want)
	}
	all := a.SuggestPaths(dir + sep)
	if len(all) != 4 || all[0] != filepath.Join(dir, "Alpha.txt") {
		t.Errorf("folder listing (hidden left out): %v", all)
	}
	if got := a.SuggestPaths(filepath.Join(dir, "missing", "x")); got != nil {
		t.Errorf("missing folder: %v", got)
	}

	for i := range 12 {
		writeFile(t, filepath.Join(dir, "many", "f"+string(rune('a'+i))), "")
	}
	if got := a.SuggestPaths(filepath.Join(dir, "many") + sep); len(got) != 8 {
		t.Errorf("got %d suggestions, want at most 8", len(got))
	}
}

func TestConflictErrors(t *testing.T) {
	a, _ := mergeDirs(t)
	if _, err := a.GetConflicts("f.txt"); err == nil {
		t.Error("GetConflicts outside git mode should fail")
	}
	if err := a.ResolveConflict("f.txt"); err == nil {
		t.Error("ResolveConflict outside git mode should fail")
	}

	conflictRepo(t)
	if _, err := a.LoadGitComparison("HEAD", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SaveConflicts("f.txt", []string{"ours"}, false); err == nil ||
		!strings.Contains(err.Error(), "changed on disk") {
		t.Errorf("wrong number of choices: %v", err)
	}
}
