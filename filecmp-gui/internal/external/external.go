// Package external launches tools outside the app: the user's editor, an
// external diff tool and git's merge tool for resolving conflicts.
package external

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// NoDiffToolHint is shown when no diff tool is configured or detected.
const NoDiffToolHint = "no diff tool configured — run `git config --global diff.tool meld` (or kdiff3, vscode, nvimdiff, ...)"

// NoMergeToolHint is shown when no merge tool is configured or detected.
const NoMergeToolHint = "no merge tool configured — run `git config --global merge.tool meld` (or kdiff3, vscode, nvimdiff, ...)"

// ---- Editor -----------------------------------------------------------------

// editorEnvVars are checked in order to find the user's editor.
var editorEnvVars = []string{"FILECMP_EDITOR", "VISUAL", "EDITOR"}

// terminalEditors always need a terminal to run.
var terminalEditors = map[string]bool{
	"vi": true, "vim": true, "nvim": true, "nano": true, "micro": true,
	"hx": true, "helix": true, "kak": true, "joe": true, "ne": true,
	"mg": true, "ed": true, "pico": true,
}

// plusLineEditors take "+N file" to jump to a line.
var plusLineEditors = map[string]bool{
	"vi": true, "vim": true, "nvim": true, "gvim": true, "nano": true,
	"emacs": true, "emacsclient": true, "micro": true, "kak": true,
	"joe": true, "pico": true,
}

// colonLineEditors take "file:N" to jump to a line.
var colonLineEditors = map[string]bool{
	"zed": true, "subl": true, "hx": true, "helix": true,
}

// EditorCommand returns a command that opens path in the user's editor,
// jumping to line when it is > 0 and the editor supports it. The editor is
// taken from FILECMP_EDITOR, then $VISUAL, then $EDITOR, falling back to the
// OS default opener. When gui is true (no terminal attached) editors that
// need a terminal are skipped.
func EditorCommand(path string, line int, gui bool) (*exec.Cmd, error) {
	for _, env := range editorEnvVars {
		fields := strings.Fields(os.Getenv(env))
		if len(fields) == 0 {
			continue
		}
		if gui && NeedsTerminal(fields) {
			continue
		}
		if _, err := exec.LookPath(fields[0]); err != nil {
			continue
		}
		args := append(fields[1:len(fields):len(fields)], editorFileArgs(fields[0], path, line)...)
		return exec.Command(fields[0], args...), nil
	}
	return OSOpenCommand(path), nil
}

// NeedsTerminal reports whether an editor command line (program + args)
// must run inside a terminal.
func NeedsTerminal(fields []string) bool {
	if len(fields) == 0 {
		return false
	}
	name := toolBase(fields[0])
	if terminalEditors[name] {
		return true
	}
	if name == "emacs" || name == "emacsclient" {
		for _, f := range fields[1:] {
			if f == "-t" || f == "-nw" || f == "--tty" || f == "--no-window-system" {
				return true
			}
		}
	}
	return false
}

// editorFileArgs builds the trailing arguments that open path at line.
func editorFileArgs(program, path string, line int) []string {
	if line <= 0 {
		return []string{path}
	}
	name := toolBase(program)
	switch {
	case plusLineEditors[name]:
		return []string{"+" + strconv.Itoa(line), path}
	case name == "code" || name == "codium":
		return []string{"-g", fmt.Sprintf("%s:%d", path, line)}
	case colonLineEditors[name]:
		return []string{fmt.Sprintf("%s:%d", path, line)}
	}
	return []string{path}
}

// OSOpenCommand returns the platform's "open with default app" command.
func OSOpenCommand(path string) *exec.Cmd {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", path)
	case "windows":
		return exec.Command("cmd", "/c", "start", "", path)
	}
	return exec.Command("xdg-open", path)
}

// ---- Tool resolution --------------------------------------------------------

// knownTool describes how to launch a diff tool directly, without git.
type knownTool struct {
	argv     []string // program and leading args; left/right paths are appended
	terminal bool     // needs a terminal
	dirs     bool     // can compare whole directories
	extcmd   bool     // not a git builtin; git mode passes argv via --extcmd
}

// knownTools maps git's tool names to direct launch commands.
var knownTools = map[string]knownTool{
	"meld":     {argv: []string{"meld"}, dirs: true},
	"kdiff3":   {argv: []string{"kdiff3"}, dirs: true},
	"vscode":   {argv: []string{"code", "--wait", "--diff"}},
	"zed":      {argv: []string{"zed", "--wait", "--diff"}, dirs: true, extcmd: true},
	"gvimdiff": {argv: []string{"gvim", "-d"}},
	"nvimdiff": {argv: []string{"nvim", "-d"}, terminal: true},
	"vimdiff":  {argv: []string{"vim", "-d"}, terminal: true},
	"bc":       {argv: []string{"bcompare"}, dirs: true},
	"bc3":      {argv: []string{"bcompare"}, dirs: true},
	"bc4":      {argv: []string{"bcompare"}, dirs: true},
	"opendiff": {argv: []string{"opendiff"}},
	"emerge":   {argv: []string{"emacs"}},
}

// autodetectOrder lists tools tried, in order, when none is configured.
var autodetectOrder = []string{"meld", "kdiff3", "vscode", "zed", "emerge", "nvimdiff", "vimdiff"}

// hasGit reports whether git is on PATH.
func hasGit() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// gitConfigGet reads a git config value. dir is the directory to run git in
// (so repo-local config applies when it's inside a repo); empty means the
// current directory. Returns "" if git is missing or the key is unset.
func gitConfigGet(dir, key string) string {
	if !hasGit() {
		return ""
	}
	cmd := exec.Command("git", "config", "--get", key)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// isTerminalTool reports whether a git tool name is known to need a terminal.
func isTerminalTool(name string) bool {
	t, ok := knownTools[name]
	return ok && t.terminal
}

// resolveTool picks the tool to use for kind ("diff" or "merge"): the
// configured <kind>.guitool (gui only) or <kind>.tool, else the first
// installed tool from autodetectOrder. Terminal-only tools are skipped when
// gui is true. configured reports whether the name came from git config.
func resolveTool(dir, kind string, gui bool) (name string, configured bool) {
	keys := []string{kind + ".tool"}
	if gui {
		keys = []string{kind + ".guitool", kind + ".tool"}
	}
	for _, key := range keys {
		n := gitConfigGet(dir, key)
		if n == "" || (gui && isTerminalTool(n)) {
			continue
		}
		return n, true
	}
	for _, n := range autodetectOrder {
		t := knownTools[n]
		if gui && t.terminal {
			continue
		}
		if _, err := exec.LookPath(t.argv[0]); err == nil {
			return n, false
		}
	}
	return "", false
}

// HasDiffTool reports whether a diff tool is configured or can be detected.
func HasDiffTool(gui bool) bool {
	name, _ := resolveTool("", "diff", gui)
	return name != ""
}

// HasMergeTool reports whether a merge tool is configured or can be detected.
func HasMergeTool(gui bool) bool {
	name, _ := resolveTool("", "merge", gui)
	return name != ""
}

// ---- Diff tool --------------------------------------------------------------

// DiffRequest describes a pair of files to show in an external diff tool.
// Set GitRoot for git mode (refs + relative path); otherwise LeftPath and
// RightPath are plain files on disk, with "" meaning that side is missing.
type DiffRequest struct {
	GitRoot  string
	LeftRef  string
	RightRef string // "" = working tree
	RelPath  string

	LeftPath  string
	RightPath string
}

// DiffToolCommand returns a command that opens req in the external diff tool.
func DiffToolCommand(req DiffRequest, gui bool) (*exec.Cmd, error) {
	if req.GitRoot != "" {
		return gitDiffToolCommand(req, gui)
	}
	return filesDiffToolCommand(req, gui)
}

func gitDiffToolCommand(req DiffRequest, gui bool) (*exec.Cmd, error) {
	if !hasGit() {
		return nil, fmt.Errorf("git is not installed")
	}
	name, configured := resolveTool(req.GitRoot, "diff", gui)
	if name == "" {
		return nil, fmt.Errorf(NoDiffToolHint)
	}
	toolArg := "--tool=" + name
	if t, ok := knownTools[name]; ok && t.extcmd && !configured {
		toolArg = "--extcmd=" + strings.Join(t.argv, " ")
	}
	args := []string{"-C", req.GitRoot, "difftool", "-y", toolArg, req.LeftRef}
	if req.RightRef != "" {
		args = append(args, req.RightRef)
	}
	args = append(args, "--", req.RelPath)
	return exec.Command("git", args...), nil
}

// filesDiffToolCommand handles plain files, which need not be in any repo.
func filesDiffToolCommand(req DiffRequest, gui bool) (*exec.Cmd, error) {
	left, err := absOrDevNull(req.LeftPath)
	if err != nil {
		return nil, err
	}
	right, err := absOrDevNull(req.RightPath)
	if err != nil {
		return nil, err
	}
	// Run git (and read its config) from the file's own directory, not the
	// app's cwd, so an unrelated or broken repo there can't interfere.
	dir := neutralDir(left, right)

	name, configured := resolveTool(dir, "diff", gui)
	if name == "" {
		return nil, fmt.Errorf(NoDiffToolHint)
	}

	// A configured tool goes through git, which knows every builtin tool and
	// any difftool.<name>.cmd. --no-index works outside a repository.
	if configured && hasGit() {
		cmd := exec.Command("git", "difftool", "--no-index", "-y", "--tool="+name, "--", left, right)
		cmd.Dir = dir
		return cmd, nil
	}

	cmd, err := directToolCommand(dir, name, left, right)
	if err != nil {
		return nil, err
	}
	cmd.Dir = dir
	return cmd, nil
}

// DirDiffCommand opens two whole directories in the diff tool. git's
// --dir-diff can't be combined with --no-index, so the tool is launched
// directly; only tools that can compare directories are allowed.
func DirDiffCommand(leftDir, rightDir string, gui bool) (*exec.Cmd, error) {
	left, err := filepath.Abs(leftDir)
	if err != nil {
		return nil, err
	}
	right, err := filepath.Abs(rightDir)
	if err != nil {
		return nil, err
	}
	dir := neutralDir(left, right)
	name, _ := resolveTool(dir, "diff", gui)
	if name == "" {
		return nil, fmt.Errorf(NoDiffToolHint)
	}
	if !SupportsDirDiff(dir, name) {
		return nil, fmt.Errorf("diff tool %q can't compare whole directories (meld, kdiff3 and Beyond Compare can)", name)
	}
	cmd, err := directToolCommand(dir, name, left, right)
	if err != nil {
		return nil, err
	}
	cmd.Dir = dir
	return cmd, nil
}

// CanDirDiff reports whether the resolved diff tool can compare directories.
func CanDirDiff(gui bool) bool {
	name, _ := resolveTool("", "diff", gui)
	return name != "" && SupportsDirDiff("", name)
}

// SupportsDirDiff reports whether tool name can compare two directories.
// Custom difftool.<name>.cmd tools are assumed to handle whatever they're
// given.
func SupportsDirDiff(dir, name string) bool {
	if t, ok := knownTools[name]; ok {
		return t.dirs
	}
	return gitConfigGet(dir, "difftool."+name+".cmd") != ""
}

// directToolCommand launches a diff tool without git: a difftool.<name>.cmd
// template (run through the shell with $LOCAL/$REMOTE set, as git does) or
// a known tool's command line.
func directToolCommand(dir, name, left, right string) (*exec.Cmd, error) {
	if tmpl := gitConfigGet(dir, "difftool."+name+".cmd"); tmpl != "" {
		cmd := shellCommand(tmpl)
		cmd.Env = append(os.Environ(), "LOCAL="+left, "REMOTE="+right)
		return cmd, nil
	}
	t, ok := knownTools[name]
	if !ok {
		return nil, fmt.Errorf("don't know how to launch diff tool %q without git", name)
	}
	if name == "emerge" {
		return exec.Command("emacs", "--eval", fmt.Sprintf("(ediff-files %s %s)", strconv.Quote(left), strconv.Quote(right))), nil
	}
	args := append(t.argv[1:len(t.argv):len(t.argv)], left, right)
	return exec.Command(t.argv[0], args...), nil
}

func shellCommand(script string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("sh"); err != nil {
			return exec.Command("cmd", "/c", script)
		}
	}
	return exec.Command("sh", "-c", script)
}

// absOrDevNull makes path absolute; an empty path (missing side) becomes the
// platform null device.
func absOrDevNull(path string) (string, error) {
	if path == "" {
		return os.DevNull, nil
	}
	return filepath.Abs(path)
}

// neutralDir picks a directory to run tools from: the left file's directory,
// or the right's when the left side is missing.
func neutralDir(left, right string) string {
	for _, p := range []string{left, right} {
		if p == os.DevNull {
			continue
		}
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.IsDir() {
			return p
		}
		return filepath.Dir(p)
	}
	return ""
}

// ---- Merge tool -------------------------------------------------------------

// MergeToolCommand returns a command that resolves the conflicts in relPath
// using git mergetool.
func MergeToolCommand(root, relPath string, gui bool) (*exec.Cmd, error) {
	if !hasGit() {
		return nil, fmt.Errorf("git is not installed")
	}
	name, _ := resolveTool(root, "merge", gui)
	if name == "" {
		return nil, fmt.Errorf(NoMergeToolHint)
	}
	return exec.Command("git", "-C", root, "mergetool", "-y", "--tool="+name, "--", relPath), nil
}

// ---- helpers ----------------------------------------------------------------

// toolBase returns a program's base name without a Windows .exe suffix.
func toolBase(program string) string {
	return strings.TrimSuffix(filepath.Base(program), ".exe")
}
