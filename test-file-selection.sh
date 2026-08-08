#!/bin/bash

# Test script to verify file selection behavior in golang-fileCmp
echo "=========================================="
echo "  File Selection Behavior Test"
echo "=========================================="
echo

# Create test directory
mkdir -p file-selection-test
cd file-selection-test

# Create multiple test files with different content
echo "Creating test files..."

cat > version1.txt << 'EOF'
This is version 1
Line 2 unchanged
Line 3 to be modified
Line 4 unchanged
EOF

cat > version2.txt << 'EOF'
This is version 2
Line 2 unchanged
Line 3 has been modified
Line 4 unchanged
New line added
EOF

cat > config.old << 'EOF'
# Old configuration
port=8080
debug=false
feature_x=disabled
EOF

cat > config.new << 'EOF'
# New configuration
port=9000
debug=true
feature_x=enabled
feature_y=enabled
EOF

cat > data.csv << 'EOF'
name,age,city
John,25,NYC
Jane,30,LA
EOF

cat > data_updated.csv << 'EOF'
name,age,city,country
John,25,NYC,USA
Jane,30,LA,USA
Bob,35,Chicago,USA
EOF

echo "✅ Created test files:"
echo "   - version1.txt / version2.txt"
echo "   - config.old / config.new"
echo "   - data.csv / data_updated.csv"
echo

echo "Test 1: File selection with command line arguments"
echo "=================================================="
echo "This should load both files and auto-select the first alphabetically"
echo "Expected: First file should be highlighted with ▶ symbol"
echo "Instructions:"
echo "1. Check that a file is highlighted with ▶"
echo "2. Check the 'Selected: filename' line appears"
echo "3. Press Ctrl+D to verify the correct file is compared"
echo "4. Press Q to quit and continue to next test"
echo
echo "Press Enter to start test 1..."
read

../filecmp version1.txt version2.txt

echo
echo "Test 2: File navigation with arrow keys"
echo "========================================"
echo "This should allow you to navigate between files"
echo "Instructions:"
echo "1. Use ↑/↓ arrows to navigate through files"
echo "2. Watch the ▶ symbol move between files"
echo "3. Watch the 'Selected: filename' line update"
echo "4. Press Ctrl+D with different files selected"
echo "5. Use n/p in diff view to switch between files"
echo "6. Press Q to quit and continue"
echo
echo "Press Enter to start test 2..."
read

# Create a directory comparison test
mkdir -p dir1 dir2

cp version1.txt dir1/
cp config.old dir1/config
cp data.csv dir1/

cp version2.txt dir2/version1.txt
cp config.new dir2/config
cp data_updated.csv dir2/data.csv

../filecmp dir1 dir2

echo
echo "Test 3: Interactive file loading"
echo "================================"
echo "This tests loading files interactively"
echo "Instructions:"
echo "1. Tab to switch between input fields"
echo "2. Type 'config.old' in left field, press Enter"
echo "3. Type 'config.new' in right field, press Enter"
echo "4. Check that config.old or config.new is selected"
echo "5. Press Ctrl+D to compare"
echo "6. Press Q to quit"
echo
echo "Press Enter to start test 3..."
read

../filecmp

echo
echo "=========================================="
echo "  Test Complete!"
echo "=========================================="
echo
echo "Expected Behaviors Verified:"
echo "✅ Files auto-selected when loaded via command line"
echo "✅ Arrow keys navigate and update selection"
echo "✅ Selected file indicator (▶) shows current choice"
echo "✅ 'Selected: filename' status line updates"
echo "✅ Ctrl+D compares the currently selected file"
echo "✅ n/p keys switch files in diff view"
echo "✅ Interactive loading shows file selection"
echo

echo "Common Issues to Check:"
echo "❓ Is the ▶ symbol next to the file you expect?"
echo "❓ Does the 'Selected: filename' line match the highlighted file?"
echo "❓ When you press Ctrl+D, does it compare the right file?"
echo "❓ Do arrow keys properly change which file is selected?"
echo

# Cleanup
cd ..
echo "Clean up test files? (y/N)"
read -n 1 response
echo
if [[ $response =~ ^[Yy]$ ]]; then
    rm -rf file-selection-test
    echo "🗑️  Test files cleaned up"
else
    echo "📁 Test files kept in ./file-selection-test/"
fi

echo "File selection test completed! 🎯"
