package main

import (
	"fmt"
	"log"
	"os"
	"runtime/debug"

	"katcompare/internal/git"
	"katcompare/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// version is set at build time with -ldflags "-X main.version=v1.2.3".
var version = "dev"

func main() {
	ui.Version = resolveVersion()
	git.TrustWSLPaths()

	// Create the model
	model := ui.New()

	// Handle command line arguments
	args := os.Args[1:]

	// Show usage/version if requested (must check before SetLeftPath)
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		showUsage()
		return
	}
	if len(args) > 0 && (args[0] == "-v" || args[0] == "--version") {
		fmt.Printf("katcmp %s\n", ui.Version)
		return
	}

	if len(args) >= 1 && args[0] == "--git" {
		// Git mode: compare refs or ref vs working tree
		leftRef := "HEAD"
		rightRef := ""
		if len(args) >= 2 {
			leftRef = args[1]
		}
		if len(args) >= 3 {
			rightRef = args[2]
		}
		if err := model.LoadGitComparison(leftRef, rightRef); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		model.ShowFileList()
	} else {
		if len(args) >= 1 {
			model.SetLeftPath(args[0])
			model.FocusRightPath()
		}
		if len(args) >= 2 {
			model.SetRightPath(args[1])
			model.OpenDiffIfFiles()
		}
	}

	// Adaptive colors ask the terminal for its background once. Do that now, as
	// the answer would otherwise arrive as key presses while the program runs.
	lipgloss.HasDarkBackground()

	// Create the program
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithReportFocus())

	// Run the program
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

// resolveVersion prefers the ldflags-stamped version, falling back to the
// module version recorded by "go install module@version".
func resolveVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

func showUsage() {
	fmt.Printf(`File Comparison TUI Tool %s

Usage:
  %s [left_path] [right_path]
  %s --git [left_ref] [right_ref]
  %s --version

Arguments:
  left_path   Path to left file or directory (optional)
  right_path  Path to right file or directory (optional)

Options:
  -h, --help                Show this help
  -v, --version             Print the version and exit

Git Mode:
  --git                     Compare HEAD against working tree
  --git <ref>               Compare <ref> against working tree
  --git <ref1> <ref2>       Compare two git refs

Examples:
  %s                           # Start with empty inputs
  %s file1.txt file2.txt       # Compare two files
  %s ./dir1 ./dir2             # Compare two directories
  %s --git                     # HEAD vs working tree
  %s --git HEAD~1              # Previous commit vs working tree
  %s --git HEAD~3 HEAD         # Three commits ago vs current HEAD

WSL:
  katcmp-win runs the Windows build from WSL; see katcmp-win --help. Use
  katcmp-win --wait when another program runs it as a diff tool.

Getting around (press ? in the app for every key):
  j/k or ↑/↓       Move through the files or the diff
  Enter or Tab     Open the selected file's diff
  Tab or Esc       Back to the file list
  ]/[              Next/previous change
  s                Switch view (Unified / Side-by-Side)
  m                Merge (on a git conflict: resolve it)
  /                Filter the file list by name
  c                Copy files that exist on only one side
  r / x            Reload / swap left and right
  o / Ctrl+G       Change the paths / compare git refs
  ?                Help (F1 on the open screen)
  q / Ctrl+C       Quit (Ctrl+C on the open screen, where q types a q)

Colors:
  Blue background:  Added lines (+)
  Red background:   Deleted lines (-)
  Gray text:        Unchanged lines

`, ui.Version, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
}
