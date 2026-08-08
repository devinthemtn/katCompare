#!/bin/bash

# Test Demo Script for File Comparison TUI Tool
# This script demonstrates the various ways to use the filecmp tool

echo "🚀 File Comparison TUI Tool - Demo Script"
echo "========================================="
echo

# Check if the binary exists
if [ ! -f "./filecmp" ]; then
    echo "❌ filecmp binary not found. Building..."
    go build -o filecmp
    if [ $? -ne 0 ]; then
        echo "❌ Failed to build filecmp"
        exit 1
    fi
    echo "✅ Built filecmp successfully"
fi

echo "📁 Available test files:"
echo "- examples/file1.txt"
echo "- examples/file2.txt"
echo "- examples/project-v1/ (directory)"
echo "- examples/project-v2/ (directory)"
echo

echo "🔧 Testing help output:"
echo "Command: ./filecmp --help"
echo "----------------------------------------"
./filecmp --help
echo

echo "📋 Demo Commands Available:"
echo
echo "1. Compare two files:"
echo "   ./filecmp examples/file1.txt examples/file2.txt"
echo
echo "2. Compare two directories:"
echo "   ./filecmp examples/project-v1 examples/project-v2"
echo
echo "3. Start with one file loaded:"
echo "   ./filecmp examples/file1.txt"
echo
echo "4. Start with interactive mode:"
echo "   ./filecmp"
echo

echo "💡 Interactive Controls (once in TUI):"
echo "- Tab: Switch between input fields"
echo "- Enter: Load entered path"
echo "- ↑/↓: Navigate file list"
echo "- Ctrl+D: Start diff comparison"
echo "- j/k: Navigate diff lines (vim-style)"
echo "- n/p: Next/previous file"
echo "- g/G: Go to top/bottom"
echo "- Esc: Go back"
echo "- ?: Show help"
echo "- Q/Ctrl+C: Quit"
echo

echo "🎨 Color Legend:"
echo "- Green background: Added lines (+)"
echo "- Blue background: Deleted lines (-)"
echo "- Gray text: Unchanged lines"
echo

echo "🎯 Quick Test - File Differences:"
echo "File1 content preview:"
head -3 examples/file1.txt | sed 's/^/  | /'
echo "  | ..."
echo
echo "File2 content preview:"
head -3 examples/file2.txt | sed 's/^/  | /'
echo "  | ..."
echo

echo "📊 Directory Comparison Preview:"
echo "project-v1 files:"
find examples/project-v1 -type f | sed 's/^/  - /'
echo
echo "project-v2 files:"
find examples/project-v2 -type f | sed 's/^/  - /'
echo

echo "🚀 Ready to run! Try one of these commands:"
echo
echo "  # Compare files directly:"
echo "  ./filecmp examples/file1.txt examples/file2.txt"
echo
echo "  # Compare project directories:"
echo "  ./filecmp examples/project-v1 examples/project-v2"
echo
echo "  # Interactive mode:"
echo "  ./filecmp"
echo

echo "✨ Enjoy comparing files with visual diffs!"
