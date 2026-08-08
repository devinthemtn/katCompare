#!/bin/bash

# Test script to verify responsive text display at different terminal widths
echo "=========================================="
echo "  Responsive Text Display Test"
echo "=========================================="
echo ""

# Create simple test files
mkdir -p responsive-test
cd responsive-test

cat > file1.txt << 'EOF'
Line 1
Line 2
Line 3
EOF

cat > file2.txt << 'EOF'
Line 1 modified
Line 2
Line 3
Line 4 added
EOF

echo "Created test files for responsive testing"
echo ""

# Test different terminal widths
widths=(50 70 90 120)

echo "Testing different terminal widths..."
echo "This will resize your terminal and show how the UI adapts"
echo ""

for width in "${widths[@]}"; do
    echo "=========================================="
    echo "Testing width: ${width} characters"
    echo "=========================================="
    echo ""
    echo "Resizing terminal to ${width}x24..."

    # Resize terminal (works in most terminals)
    printf '\e[8;24;%dt' "$width"
    sleep 1

    echo "Status messages should fit properly:"
    echo "- Selected file line should not be cut off"
    echo "- Help text should be appropriately shortened"
    echo "- Error messages should wrap correctly"
    echo ""
    echo "Press Enter to start test for ${width} columns..."
    read

    # Run the tool
    timeout 10s ../filecmp file1.txt file2.txt || true

    echo ""
    echo "Press Enter to continue to next width..."
    read
done

echo ""
echo "=========================================="
echo "  Test Complete"
echo "=========================================="
echo ""

# Reset to reasonable size
printf '\e[8;30;100t'
echo "Reset terminal to normal size (100x30)"

echo "What to look for in the test:"
echo "✅ Selected file status fits on one line"
echo "✅ Help text adapts to available width"
echo "✅ No text gets cut off at screen edges"
echo "✅ File list displays properly at all widths"
echo "✅ Error messages wrap appropriately"
echo ""

# Cleanup
cd ..
rm -rf responsive-test
echo "Cleaned up test files"

echo ""
echo "Responsive design features:"
echo "- Dynamic help text shortening"
echo "- Selected file path truncation"
echo "- Adaptive status messages"
echo "- Width-aware text rendering"
echo ""
echo "Test completed! 📱"
