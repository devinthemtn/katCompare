package main

import (
	"fmt"
	"golang-fileCmp/internal/differ"
	"golang-fileCmp/internal/file"
)

func main() {
	fmt.Println("🧪 Testing File Comparison Engine")
	fmt.Println("=================================")
	fmt.Println()

	// Initialize services
	fileManager := file.New()
	diffEngine := differ.New()

	// Test 1: Simple file comparison
	fmt.Println("📝 Test 1: File Comparison")
	fmt.Println("--------------------------")

	file1Path := "examples/file1.txt"
	file2Path := "examples/file2.txt"

	file1, err := fileManager.LoadPath(file1Path)
	if err != nil {
		fmt.Printf("❌ Error loading %s: %v\n", file1Path, err)
		return
	}

	file2, err := fileManager.LoadPath(file2Path)
	if err != nil {
		fmt.Printf("❌ Error loading %s: %v\n", file2Path, err)
		return
	}

	// Compare the files
	diff := diffEngine.CompareStrings(file1.Path, file2.Path, file1.Content, file2.Content)

	// Show statistics
	equal, inserted, deleted := diff.GetStats()
	fmt.Printf("📊 Diff Statistics:\n")
	fmt.Printf("   Equal lines: %d\n", equal)
	fmt.Printf("   Inserted lines: %d (shown in green)\n", inserted)
	fmt.Printf("   Deleted lines: %d (shown in blue)\n", deleted)
	fmt.Printf("   Total diff lines: %d\n", len(diff.Lines))
	fmt.Println()

	// Show sample diff output
	fmt.Println("🔍 Sample Diff Output:")
	fmt.Println("----------------------")

	maxLines := 15 // Show first 15 lines
	for i, line := range diff.Lines {
		if i >= maxLines {
			fmt.Printf("... (%d more lines)\n", len(diff.Lines)-maxLines)
			break
		}

		var symbol string
		var colorDesc string
		switch line.Type {
		case differ.DiffEqual:
			symbol = " "
			colorDesc = "[GRAY]"
		case differ.DiffInsert:
			symbol = "+"
			colorDesc = "[GREEN]"
		case differ.DiffDelete:
			symbol = "-"
			colorDesc = "[BLUE]"
		}

		fmt.Printf("%s %4d %s %s\n", colorDesc, line.LineNum, symbol, line.Content)
	}

	fmt.Println()

	// Test 2: Directory comparison
	fmt.Println("📂 Test 2: Directory Comparison")
	fmt.Println("-------------------------------")

	dir1Path := "examples/project-v1"
	dir2Path := "examples/project-v2"

	dir1, err := fileManager.LoadPath(dir1Path)
	if err != nil {
		fmt.Printf("❌ Error loading %s: %v\n", dir1Path, err)
		return
	}

	dir2, err := fileManager.LoadPath(dir2Path)
	if err != nil {
		fmt.Printf("❌ Error loading %s: %v\n", dir2Path, err)
		return
	}

	// Find common files
	commonFiles := file.FindCommonFiles(dir1, dir2)

	fmt.Printf("🔗 Found %d common files:\n", len(commonFiles))
	for relPath, filePair := range commonFiles {
		leftFile := filePair[0]
		rightFile := filePair[1]

		fmt.Printf("   📄 %s\n", relPath)

		// Quick diff statistics for each file
		fileDiff := diffEngine.CompareStrings(leftFile.Path, rightFile.Path, leftFile.Content, rightFile.Content)
		equal, inserted, deleted := fileDiff.GetStats()

		if inserted > 0 || deleted > 0 {
			fmt.Printf("      Changes: +%d/-%d lines (=%d unchanged)\n", inserted, deleted, equal)
		} else {
			fmt.Printf("      No changes detected\n")
		}
	}

	fmt.Println()

	// Test 3: File type detection
	fmt.Println("🔍 Test 3: File Type Detection")
	fmt.Println("------------------------------")

	testFiles := []string{
		"examples/file1.txt",
		"examples/project-v1/config.yaml",
		"examples/project-v1/README.md",
	}

	for _, testFile := range testFiles {
		info, err := fileManager.LoadPath(testFile)
		if err != nil {
			continue
		}

		if info.IsTextFile() {
			fmt.Printf("✅ %s - Text file (will be compared)\n", testFile)
		} else {
			fmt.Printf("❌ %s - Binary file (will be skipped)\n", testFile)
		}
	}

	fmt.Println()
	fmt.Println("✅ All tests completed successfully!")
	fmt.Println("🚀 Ready to use the TUI application:")
	fmt.Println("   ./filecmp examples/file1.txt examples/file2.txt")
	fmt.Println("   ./filecmp examples/project-v1 examples/project-v2")
}
