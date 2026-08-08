package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ---- FindRoot ---------------------------------------------------------------

func TestFindRootInsideRepo(t *testing.T) {
	// The test binary runs inside the module, which is a git repo.
	root, err := FindRoot()
	if err != nil {
		t.Fatalf("FindRoot failed: %v", err)
	}
	if root == "" {
		t.Error("FindRoot returned empty string")
	}
	// The root must contain a .git directory or file.
	info, err := os.Stat(filepath.Join(root, ".git"))
	if err != nil {
		t.Errorf("expected .git at %s: %v", root, err)
	}
	_ = info
}

func TestFindRootOutsideRepo(t *testing.T) {
	// Change to /tmp which should not be a git repo.
	orig, _ := os.Getwd()
	defer os.Chdir(orig) //nolint:errcheck

	if err := os.Chdir(os.TempDir()); err != nil {
		t.Skip("could not chdir to tmpdir")
	}
	_, err := FindRoot()
	if err == nil {
		t.Error("expected error when not in a git repo")
	}
}

// ---- FileAtRef --------------------------------------------------------------

func TestFileAtRefKnownFile(t *testing.T) {
	root, err := FindRoot()
	if err != nil {
		t.Skip("not in a git repo:", err)
	}

	// go.mod should exist at HEAD in this repository.
	content, err := FileAtRef(root, "HEAD", "go.mod")
	if err != nil {
		t.Fatalf("FileAtRef(HEAD, go.mod) failed: %v", err)
	}
	if !strings.Contains(content, "golang-fileCmp") {
		t.Errorf("go.mod at HEAD should contain module name, got: %q", content[:min(len(content), 200)])
	}
}

func TestFileAtRefMissingFile(t *testing.T) {
	root, err := FindRoot()
	if err != nil {
		t.Skip("not in a git repo:", err)
	}

	_, err = FileAtRef(root, "HEAD", "this_file_definitely_does_not_exist_xyz.txt")
	if err == nil {
		t.Error("expected error for non-existent file at HEAD")
	}
}

func TestFileAtRefInvalidRef(t *testing.T) {
	root, err := FindRoot()
	if err != nil {
		t.Skip("not in a git repo:", err)
	}

	_, err = FileAtRef(root, "not-a-real-ref-12345", "go.mod")
	if err == nil {
		t.Error("expected error for invalid ref")
	}
}

// ---- ChangedFiles -----------------------------------------------------------

func TestChangedFilesVsWorkingTree(t *testing.T) {
	root, err := FindRoot()
	if err != nil {
		t.Skip("not in a git repo:", err)
	}

	files, err := ChangedFiles(root, "HEAD", "")
	if err != nil {
		t.Fatalf("ChangedFiles(HEAD, working tree) failed: %v", err)
	}

	// Each file path must be non-empty and Status must be a recognised byte.
	for _, f := range files {
		if f.Path == "" {
			t.Error("got a FileStatus with empty Path")
		}
		switch f.Status {
		case 'M', 'A', 'D':
			// valid
		default:
			t.Errorf("unexpected status byte %q for file %s", f.Status, f.Path)
		}
	}
}

func TestChangedFilesRefToRef(t *testing.T) {
	root, err := FindRoot()
	if err != nil {
		t.Skip("not in a git repo:", err)
	}

	// Count how many commits exist — skip if fewer than 2.
	out, err := exec.Command("git", "-C", root, "rev-list", "--count", "HEAD").Output()
	if err != nil || strings.TrimSpace(string(out)) == "1" {
		t.Skip("need at least 2 commits for ref-to-ref test")
	}

	files, err := ChangedFiles(root, "HEAD~1", "HEAD")
	if err != nil {
		t.Fatalf("ChangedFiles(HEAD~1, HEAD) failed: %v", err)
	}

	// If there are results, validate them.
	for _, f := range files {
		if f.Path == "" {
			t.Error("got a FileStatus with empty Path")
		}
	}
}

func TestChangedFilesNoDuplicates(t *testing.T) {
	root, err := FindRoot()
	if err != nil {
		t.Skip("not in a git repo:", err)
	}

	files, err := ChangedFiles(root, "HEAD", "")
	if err != nil {
		t.Fatalf("ChangedFiles failed: %v", err)
	}

	seen := make(map[string]bool)
	for _, f := range files {
		if seen[f.Path] {
			t.Errorf("duplicate path in ChangedFiles result: %s", f.Path)
		}
		seen[f.Path] = true
	}
}

// ---- ReadWorkingTreeFile ----------------------------------------------------

func TestReadWorkingTreeFile(t *testing.T) {
	root, err := FindRoot()
	if err != nil {
		t.Skip("not in a git repo:", err)
	}

	content, err := ReadWorkingTreeFile(root, "go.mod")
	if err != nil {
		t.Fatalf("ReadWorkingTreeFile(go.mod) failed: %v", err)
	}
	if !strings.Contains(content, "golang-fileCmp") {
		t.Errorf("go.mod should contain module name, got: %q", content[:min(len(content), 200)])
	}
}

func TestReadWorkingTreeFileMissing(t *testing.T) {
	root, err := FindRoot()
	if err != nil {
		t.Skip("not in a git repo:", err)
	}

	_, err = ReadWorkingTreeFile(root, "nonexistent_xyz_abc.txt")
	if err == nil {
		t.Error("expected error for missing working tree file")
	}
}

// ---- helper -----------------------------------------------------------------

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
