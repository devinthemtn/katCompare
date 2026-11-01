#!/bin/bash

# Create test directories with unique files to demonstrate all files functionality
echo "Creating test directories with unique files..."

# Clean up any existing test directories
rm -rf test-all-files-left test-all-files-right

# Create left directory with some files
mkdir -p test-all-files-left
cd test-all-files-left

# Files that exist in both directories (common files)
cat > common1.txt << 'EOF'
This file exists in both directories
But the content is different
Line 3 from left
EOF

cat > common2.yaml << 'EOF'
# Common config file - left version
version: 1.0
debug: false
port: 8080
EOF

# Files that only exist in LEFT directory
cat > left-only1.txt << 'EOF'
This file ONLY exists in the LEFT directory
It should show up with a blue arrow ◄
And have [LEFT ONLY] tag
EOF

cat > left-only2.md << 'EOF'
# Left Only Documentation

This markdown file only exists in the left directory.
It represents documentation that was removed in the right version.

## Features
- Legacy feature A
- Deprecated feature B
EOF

cat > legacy-config.ini << 'EOF'
[legacy]
old_setting=true
deprecated_option=enabled
remove_this=yes
EOF

cd ..

# Create right directory with some files
mkdir -p test-all-files-right
cd test-all-files-right

# Files that exist in both directories (common files) - but with different content
cat > common1.txt << 'EOF'
This file exists in both directories
But the content is different
Line 3 from right - MODIFIED
Line 4 added in right
EOF

cat > common2.yaml << 'EOF'
# Common config file - right version
version: 2.0
debug: true
port: 9000
new_feature: enabled
EOF

# Files that only exist in RIGHT directory
cat > right-only1.txt << 'EOF'
This file ONLY exists in the RIGHT directory
It should show up with an orange arrow ►
And have [RIGHT ONLY] tag
EOF

cat > right-only2.json << 'EOF'
{
  "name": "New Feature Config",
  "version": "2.0",
  "features": {
    "new_feature_1": true,
    "new_feature_2": false,
    "experimental": true
  }
}
EOF

cat > migration-guide.txt << 'EOF'
Migration Guide from v1 to v2

This file only exists in the right directory.
It explains how to migrate from the left version to the right version.

Steps:
1. Update common1.txt with new content
2. Update common2.yaml configuration
3. Remove legacy-config.ini
4. Add new feature configurations
EOF

cd ..

echo "✅ Created test directories:"
echo ""
echo "📁 test-all-files-left/ (LEFT directory):"
echo "   📄 common1.txt (exists in both, different content)"
echo "   📄 common2.yaml (exists in both, different content)"
echo "   📄 left-only1.txt [LEFT ONLY]"
echo "   📄 left-only2.md [LEFT ONLY]"
echo "   📄 legacy-config.ini [LEFT ONLY]"
echo ""
echo "📁 test-all-files-right/ (RIGHT directory):"
echo "   📄 common1.txt (exists in both, different content)"
echo "   📄 common2.yaml (exists in both, different content)"
echo "   📄 right-only1.txt [RIGHT ONLY]"
echo "   📄 right-only2.json [RIGHT ONLY]"
echo "   📄 migration-guide.txt [RIGHT ONLY]"
echo ""
echo "Expected results in the UI:"
echo "=========================="
echo "📊 Found 7 files (2 common, 6 unique)"
echo "📄 File list should show:"
echo "   ✗ common1.txt (different content in both)"
echo "   ✗ common2.yaml (different content in both)"
echo "   ◄ left-only1.txt [LEFT ONLY]"
echo "   ◄ left-only2.md [LEFT ONLY]"
echo "   ◄ legacy-config.ini [LEFT ONLY]"
echo "   ► right-only1.txt [RIGHT ONLY]"
echo "   ► right-only2.json [RIGHT ONLY]"
echo "   ► migration-guide.txt [RIGHT ONLY]"
echo ""
echo "Now run: ./katcmp test-all-files-left test-all-files-right"
echo "You should see ALL files listed, not just the common ones!"
