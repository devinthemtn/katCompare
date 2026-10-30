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
	Path       string
	Status     byte // 'M' modified, 'A' added, 'D' deleted, 'R' renamed
	Conflicted bool // unmerged: has conflicts from a merge/rebase in progress
}

// ConflictedFiles returns the unmerged (conflicted) paths in the working tree.
func ConflictedFiles(root string) ([]string, error) {
	out, err := exec.Command("git", "-C", root, "diff", "--name-only", "--diff-filter=U").Output()
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %w", err)
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
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
			if status == 'R' || status == 'U' {
				status = 'M' // treat rename/unmerged as modified for display purposes
			}
			if !seen[path] {
				seen[path] = true
				files = append(files, FileStatus{Path: path, Status: status})
			}
		}
	}

	if rightRef == "" {
		// Mark files with merge conflicts; they only exist in the working tree.
		conflicted, _ := ConflictedFiles(root)
		for _, path := range conflicted {
			found := false
			for i := range files {
				if files[i].Path == path {
					files[i].Conflicted = true
					found = true
				}
			}
			if !found {
				files = append(files, FileStatus{Path: path, Status: 'M', Conflicted: true})
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

// RefOption is one choice in a ref picker.
type RefOption struct {
	Ref   string // value passed to git; "" means the working tree
	Label string // text shown to the user
	Group string // "Common", "Branches", "Remote branches", "Tags" or "Commits"
}

// Ref picker groups, in display order.
const (
	GroupCommon   = "Common"
	GroupBranches = "Branches"
	GroupRemotes  = "Remote branches"
	GroupTags     = "Tags"
	GroupCommits  = "Commits"
)

// maxRefsPerGroup caps tags and commits so the picker stays usable on large repos.
const maxRefsPerGroup = 30

// ListRefs returns the refs worth offering in a picker: common relative refs
// (plus the working tree when includeWorkingTree is set), local and remote
// branches by most recent commit, recent tags and recent commits on HEAD.
func ListRefs(root string, includeWorkingTree bool) ([]RefOption, error) {
	if _, err := exec.Command("git", "-C", root, "rev-parse", "--verify", "-q", "HEAD").Output(); err != nil {
		return nil, fmt.Errorf("repository has no commits yet")
	}

	var refs []RefOption
	add := func(group, ref, label string) {
		refs = append(refs, RefOption{Ref: ref, Label: label, Group: group})
	}

	if includeWorkingTree {
		add(GroupCommon, "", "working tree (uncommitted changes)")
	}
	add(GroupCommon, "HEAD", "HEAD (last commit)")
	for i := 1; i <= 3; i++ {
		ref := fmt.Sprintf("HEAD~%d", i)
		if refExists(root, ref) {
			add(GroupCommon, ref, fmt.Sprintf("%s (%d commit%s back)", ref, i, plural(i)))
		}
	}
	if refExists(root, "@{upstream}") {
		add(GroupCommon, "@{upstream}", "@{upstream} (tracking branch)")
	}
	if refExists(root, "MERGE_HEAD") {
		add(GroupCommon, "MERGE_HEAD", "MERGE_HEAD (branch being merged)")
	}

	current := gitLines(root, "branch", "--show-current")
	for _, b := range gitLines(root, "for-each-ref", "--sort=-committerdate",
		"--format=%(refname:short)", "refs/heads") {
		label := b
		if len(current) > 0 && b == current[0] {
			label += " (current)"
		}
		add(GroupBranches, b, label)
	}
	for _, b := range gitLines(root, "for-each-ref", "--sort=-committerdate",
		"--format=%(refname:short)", "refs/remotes") {
		// Skip symbolic "origin/HEAD" entries (shown as just "origin")
		if !strings.Contains(b, "/") || strings.HasSuffix(b, "/HEAD") {
			continue
		}
		add(GroupRemotes, b, b)
	}
	for i, t := range gitLines(root, "for-each-ref", "--sort=-creatordate",
		"--format=%(refname:short)", "refs/tags") {
		if i >= maxRefsPerGroup {
			break
		}
		add(GroupTags, t, t)
	}
	for _, line := range gitLines(root, "log", fmt.Sprintf("-n%d", maxRefsPerGroup),
		"--format=%h%x09%s%x09%cr") {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		add(GroupCommits, parts[0], fmt.Sprintf("%s  %s (%s)", parts[0], parts[1], parts[2]))
	}

	return refs, nil
}

// refExists reports whether ref resolves to a commit.
func refExists(root, ref string) bool {
	return exec.Command("git", "-C", root, "rev-parse", "--verify", "-q", ref+"^{commit}").Run() == nil
}

// gitLines runs git in root and returns its non-empty output lines,
// or nil if the command fails.
func gitLines(root string, args ...string) []string {
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).Output()
	if err != nil {
		return nil
	}
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
