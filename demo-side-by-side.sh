#!/bin/bash

# Demo script for side-by-side view functionality

echo "🎬 Side-by-Side View Demo"
echo "========================="
echo
echo "This demo shows the new side-by-side view functionality."
echo "Use 's' key to toggle between unified and side-by-side views."
echo
echo "Controls in side-by-side mode:"
echo "  ↑/↓ or j/k: Navigate lines"
echo "  h/l or ←/→: Visual left/right focus"
echo "  s: Switch to unified view"
echo "  g/G: Top/bottom"
echo "  n/p: Next/previous file"
echo "  m: Merge mode"
echo "  ?: Help"
echo
echo "Press any key to start demo..."
read -n 1 -s

# Create test files if they don't exist
mkdir -p test-side-by-side/{left,right}

cat > test-side-by-side/left/demo.txt << 'EOFTEST'
Line 1: This line is the same
Line 2: This line will be deleted
Line 3: This line is modified (old version)
Line 4: Another common line
Line 5: This will be removed
Line 6: Final common line
EOFTEST

cat > test-side-by-side/right/demo.txt << 'EOFTEST'
Line 1: This line is the same
Line 3: This line is modified (new version)
Line 4: Another common line
Line 4.5: This is a new line
Line 6: Final common line
Line 7: Another new line at the end
EOFTEST

echo "Created test files in test-side-by-side/"
echo "Starting file comparison..."
echo
echo "Try these steps:"
echo "1. Press Ctrl+D to start comparing"
echo "2. Press 's' to switch to side-by-side view"
echo "3. Press 's' again to switch back to unified view"
echo "4. Use ↑/↓ to navigate"
echo "5. Press 'Q' to quit"
echo

./filecmp test-side-by-side/left test-side-by-side/right

# Cleanup
echo "Demo finished. Cleaning up test files..."
rm -rf test-side-by-side/
echo "Done!"
