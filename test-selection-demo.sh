#!/bin/bash

# Simple test for file selection behavior
echo "Creating test files for file selection demo..."

# Create test directory
mkdir -p selection-test
cd selection-test

# Create file A
cat > fileA.txt << 'EOF'
Line 1 - Original
Line 2 - Keep this
Line 3 - Will be changed
Line 4 - Original ending
EOF

# Create file B
cat > fileB.txt << 'EOF'
Line 1 - Original
Line 2 - Keep this
Line 3 - This was changed
Line 4 - Original ending
Line 5 - New addition
EOF

# Create file C
cat > fileC.txt << 'EOF'
Different content entirely
This file has completely different text
For testing multiple file selection
EOF

# Create file D
cat > fileD.txt << 'EOF'
Different content entirely
This file has completely different text
Modified version for comparison
EOF

echo "✅ Created test files in selection-test/"
echo "   - fileA.txt (original)"
echo "   - fileB.txt (modified A)"
echo "   - fileC.txt (different content)"
echo "   - fileD.txt (modified C)"
echo ""

echo "Testing file selection:"
echo "1. Running: ../filecmp fileA.txt fileB.txt"
echo "2. Check that one file is highlighted with ►"
echo "3. Use ↑/↓ to change selection"
echo "4. Press Ctrl+D to compare selected file"
echo "5. Press Q to quit"
echo ""
echo "Press Enter to start..."
read

../filecmp fileA.txt fileB.txt

echo ""
echo "Now testing with different files:"
echo "Press Enter to continue..."
read

../filecmp fileC.txt fileD.txt

echo ""
echo "Demo complete!"
cd ..
rm -rf selection-test
echo "Cleaned up test files."
