#!/bin/bash

# Simple test to verify the width fix for selected file status
echo "=========================================="
echo "  Width Fix Verification Test"
echo "=========================================="
echo ""

# Create test directory and files
mkdir -p width-test
cd width-test

# Create files with different name lengths
cat > short.txt << 'EOF'
Short filename test
Line 2
Line 3
EOF

cat > medium-length-filename.txt << 'EOF'
Medium filename test
Line 2 modified
Line 3
EOF

cat > very-long-configuration-filename-that-might-cause-width-issues.yaml << 'EOF'
# Very long filename test
config:
  setting1: value1
  setting2: value2
EOF

cat > very-long-configuration-filename-that-might-cause-width-issues-updated.yaml << 'EOF'
# Very long filename test - updated
config:
  setting1: new_value1
  setting2: value2
  setting3: added_value
EOF

echo "✅ Created test files with various name lengths:"
echo "   📄 short.txt"
echo "   📄 medium-length-filename.txt"
echo "   📄 very-long-configuration-filename-that-might-cause-width-issues.yaml"
echo "   📄 very-long-configuration-filename-that-might-cause-width-issues-updated.yaml"
echo ""

echo "Testing width fix..."
echo "==================="
echo ""

echo "What to verify:"
echo "✅ Selected file line should NOT be cut off"
echo "✅ Help text should appear on separate line"
echo "✅ Long filenames should be truncated with '...'"
echo "✅ All text should be visible regardless of terminal width"
echo ""

echo "Testing with files that have different name lengths..."
echo ""

echo "Test 1: Short filename"
echo "Press Enter to test..."
read
../filecmp short.txt medium-length-filename.txt

echo ""
echo "Test 2: Long filename comparison"
echo "Press Enter to test..."
read
../filecmp very-long-configuration-filename-that-might-cause-width-issues.yaml very-long-configuration-filename-that-might-cause-width-issues-updated.yaml

echo ""
echo "=========================================="
echo "  Test Results Verification"
echo "=========================================="
echo ""

echo "Expected behavior:"
echo "✅ '► Selected: filename' appears on its own line"
echo "✅ Help text appears on the next line below it"
echo "✅ Long filenames show '...filename.ext' truncation"
echo "✅ No text overflows off the screen edge"
echo "✅ You can navigate with ↑/↓ to change selection"
echo "✅ Selected file line updates when you change selection"
echo ""

echo "If any text was cut off or overflowed, the fix needs more work."
echo "If everything displays properly, the fix is working! ✨"
echo ""

# Cleanup
cd ..
rm -rf width-test
echo "🗑️  Cleaned up test files"
echo ""
echo "Width fix test completed! 📏"
