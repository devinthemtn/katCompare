package main

import (
	"fmt"
	"log"
	"os"

	"golang-fileCmp/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Create the model
	model := ui.New()

	// Handle command line arguments
	args := os.Args[1:]

	// Show usage if help is requested (must check before SetLeftPath)
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help") {
		showUsage()
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
	} else {
		if len(args) >= 1 {
			model.SetLeftPath(args[0])
		}
		if len(args) >= 2 {
			model.SetRightPath(args[1])
		}
	}

	// Create the program
	p := tea.NewProgram(model, tea.WithAltScreen())

	// Run the program
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

func showUsage() {
	fmt.Printf(`File Comparison TUI Tool

Usage:
  %s [left_path] [right_path]
  %s --git [left_ref] [right_ref]

Arguments:
  left_path   Path to left file or directory (optional)
  right_path  Path to right file or directory (optional)

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

Interactive Controls:
  Tab              Switch between input fields / Navigate suggestions
  Enter            Load entered path / Accept suggestion
  ↑/↓              Navigate file list / Navigate suggestions
  /                Filter file list by name
  Esc              Clear suggestions / filter
  Ctrl+D           Start diff comparison
  s                Switch view (Unified / Side-by-Side)
  h/l or ←/→       Horizontal scroll in side-by-side view
  j/k              Navigate diff (vim-style)
  n/p              Next/previous file
  g/G              Go to top/bottom
  m                Enter merge mode
  Esc              Go back
  ?                Show help
  Q/Ctrl+C         Quit

Colors:
  Blue background:  Added lines (+)
  Red background:   Deleted lines (-)
  Gray text:        Unchanged lines

`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
}
