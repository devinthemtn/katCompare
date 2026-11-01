package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// conflictRepo makes a repo whose merge left two conflicts in f.txt, and
// changes into it for the test.
func conflictRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		cmd.Run() //nolint:errcheck // the merge is expected to fail
	}
	write := func(s string) {
		if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte(s), 0644); err != nil {
			t.Fatal(err)
		}
	}
	run("init", "-q", "-b", "main")
	write("a\nbase1\nb\nc\nd\ne\nbase2\nf\n")
	run("add", ".")
	run("commit", "-qm", "base")
	run("checkout", "-qb", "other")
	write("a\nother1\nb\nc\nd\ne\nother2\nf\n")
	run("commit", "-qam", "other")
	run("checkout", "-q", "main")
	write("a\nmain1\nb\nc\nd\ne\nmain2\nf\n")
	run("commit", "-qam", "main")
	run("merge", "-q", "other")

	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) }) //nolint:errcheck
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestConflictEndpoints(t *testing.T) {
	dir := conflictRepo(t)
	a := NewApp(nil)
	if _, err := a.LoadGitComparison("HEAD", ""); err != nil {
		t.Fatal(err)
	}

	blocks, err := a.GetConflicts("f.txt")
	if err != nil || len(blocks) != 2 {
		t.Fatalf("GetConflicts = %d blocks, %v; want 2", len(blocks), err)
	}
	if blocks[0].OursLabel != "HEAD" || blocks[0].TheirsLabel != "other" || len(blocks[0].After) == 0 {
		t.Errorf("block 0 = %+v", blocks[0])
	}

	if _, err := a.SaveConflicts("f.txt", []string{"ours", ""}, true); err == nil {
		t.Fatal("mark resolved with an unresolved conflict should fail")
	}
	msg, err := a.SaveConflicts("f.txt", []string{"ours", ""}, false)
	if err != nil || !strings.Contains(msg, "1 conflict(s) left") {
		t.Fatalf("partial save: %q, %v", msg, err)
	}
	if blocks, _ = a.GetConflicts("f.txt"); len(blocks) != 1 {
		t.Fatalf("after partial save: %d blocks, want 1", len(blocks))
	}

	if _, err := a.SaveConflicts("f.txt", []string{"both"}, true); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "f.txt"))
	if want := "a\nmain1\nb\nc\nd\ne\nmain2\nother2\nf\n"; string(got) != want {
		t.Errorf("file = %q, want %q", got, want)
	}
	out, _ := exec.Command("git", "-C", dir, "diff", "--name-only", "--diff-filter=U").Output()
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("still unmerged after mark resolved: %s", out)
	}
}

func TestSaveMergeRefusesGitRevision(t *testing.T) {
	conflictRepo(t)
	a := NewApp(nil)
	if _, err := a.LoadGitComparison("HEAD", ""); err != nil {
		t.Fatal(err)
	}
	_, err := a.SaveMerge(MergeSaveRequest{RelPath: "f.txt", Target: "left"})
	if err == nil || !strings.Contains(err.Error(), "git revision") {
		t.Fatalf("SaveMerge to a git ref: err = %v, want refusal", err)
	}
	if matches, _ := filepath.Glob("git:*"); len(matches) > 0 {
		t.Errorf("stray files written: %v", matches)
	}
}
