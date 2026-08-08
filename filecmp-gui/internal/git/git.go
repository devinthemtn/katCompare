package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// FindRoot returns the absolute path of the git repository root,
// or an error if the current directory is not inside a git repo.
func FindRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("not inside a git repository")
	}
	return strings.TrimSpace(string(out)), nil
}

// FileStatus describes a single file in a diff.
type FileStatus struct {
	Path   string
	Status byte // 'M' modified, 'A' added, 'D' deleted, 'R' renamed
}

// ChangedFiles returns files that differ between leftRef and the target.
// If rightRef is empty the target is the working tree (staged + unstaged changes).
// If rightRef is non-empty the target is that git ref.
func ChangedFiles(root, leftRef, rightRef string) ([]FileStatus, error) {
	var args []string
	if rightRef == "" {
		// Compare ref against working tree: staged + unstaged combined
		// Use leftRef...HEAD equivalent: just diff leftRef vs working tree
		args = []string{"-C", root, "diff", "--name-status", leftRef}
	} else {
		args = []string{"-C", root, "diff", "--name-status", leftRef, rightRef}
	}

	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %w", err)
	}

	var staged []byte
	if rightRef == "" {
		// Also include files that are staged but not yet in a commit vs leftRef
		staged, _ = exec.Command("git", "-C", root, "diff", "--cached", "--name-status", leftRef).Output()
	}

	seen := make(map[string]bool)
	var files []FileStatus

	for _, chunk := range [][]byte{out, staged} {
		for _, line := range strings.Split(strings.TrimSpace(string(chunk)), "\n") {
			if line == "" {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			// Rename lines look like "R100\told\tnew" — use the destination (last field)
			path := parts[len(parts)-1]
			status := parts[0][0]
			if status == 'R' {
				status = 'M' // treat rename as modified for display purposes
			}
			if !seen[path] {
				seen[path] = true
				files = append(files, FileStatus{Path: path, Status: status})
			}
		}
	}

	return files, nil
}

// FileAtRef returns the content of path at the given git ref.
// Returns an error if the file does not exist at that ref.
func FileAtRef(root, ref, path string) (string, error) {
	out, err := exec.Command("git", "-C", root, "show", fmt.Sprintf("%s:%s", ref, path)).Output()
	if err != nil {
		return "", fmt.Errorf("%s not found at %s", path, ref)
	}
	return string(out), nil
}

// ReadWorkingTreeFile reads the file from disk at root/path.
func ReadWorkingTreeFile(root, path string) (string, error) {
	data, err := os.ReadFile(root + "/" + path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
