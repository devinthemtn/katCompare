#!/bin/bash

# Demo script for the merge functionality
echo "=== File Comparison Tool - Merge Demo ==="
echo ""

# Show the example files first
echo "Let's look at our example files:"
echo ""
echo "--- examples/file1.txt (original) ---"
cat examples/file1.txt
echo ""
echo "--- examples/file2.txt (modified) ---"
cat examples/file2.txt
echo ""

echo "Now let's run the file comparison tool to see the differences and merge them:"
echo ""
echo "Usage instructions:"
echo "1. The tool will start and load both files"
echo "2. Press Ctrl+D to start comparing"
echo "3. Navigate through the diff with ↑/↓ or j/k"
echo "4. Press 'm' to enter merge mode"
echo "5. In merge mode:"
echo "   - Use Space/Enter to toggle individual changes"
echo "   - Press 't' to switch merge target (left/right)"
echo "   - Press 'a' to select all changes or 'n' to select none"
echo "   - Press 's' to save the merged result"
echo "6. The merged file will be saved with a '.merged' extension"
echo ""

echo "Key features of merge mode:"
echo "- Yellow background = Selected changes that will be applied"
echo "- Strikethrough = Unselected changes that will be skipped"
echo "- Target 'LEFT' = Apply changes to file1 (start with file1, add/remove changes)"
echo "- Target 'RIGHT' = Apply changes to file2 (start with file2, add/remove changes)"
echo ""

echo "Starting the tool now..."
echo "Press any key to continue..."
read -n 1 -s

# Run the file comparison tool
./filecmp examples/file1.txt examples/file2.txt

echo ""
echo "=== Demo Complete ==="
echo ""
echo "If you saved a merged file, you can check the results:"
echo "ls -la examples/*.merged"
echo ""
echo "Example merge workflow:"
echo "1. Enter merge mode with 'm'"
echo "2. Select which changes to apply (some insertions, some deletions)"
echo "3. Switch target with 't' to see different merge results"
echo "4. Save with 's' to create a .merged file"
echo ""
echo "This allows you to:"
echo "- Cherry-pick specific changes from one file to another"
echo "- Create custom merged versions by selecting only the changes you want"
echo "- Apply changes in either direction (left-to-right or right-to-left)"
