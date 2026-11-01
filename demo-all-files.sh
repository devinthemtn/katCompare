#!/bin/bash

# Demo script for the enhanced all files functionality
echo "=========================================="
echo "  Enhanced File Comparison - All Files Demo"
echo "=========================================="
echo ""

# Create test directories
mkdir -p all-files-demo
cd all-files-demo

echo "Creating test scenario with mixed files..."
echo ""

# Create left directory with some files
mkdir -p project-old project-new

# Files that exist in both directories
cat > project-old/config.yaml << 'EOF'
# Old Configuration
version: "1.0"
debug: false
port: 8080
database:
  host: localhost
  name: myapp_old
EOF

cat > project-new/config.yaml << 'EOF'
# New Configuration
version: "2.0"
debug: true
port: 9000
database:
  host: db.example.com
  name: myapp_new
EOF

cat > project-old/main.go << 'EOF'
package main

import "fmt"

func main() {
    fmt.Println("Hello from v1")
    oldFunction()
}

func oldFunction() {
    fmt.Println("This is the old way")
}
EOF

cat > project-new/main.go << 'EOF'
package main

import "fmt"

func main() {
    fmt.Println("Hello from v2")
    newFunction()
}

func newFunction() {
    fmt.Println("This is the new way")
}
EOF

# Files that only exist in LEFT (old project)
cat > project-old/legacy.txt << 'EOF'
This is a legacy file that was removed in the new version.
It contains old functionality that is no longer needed.
This file should be deleted in the migration.
EOF

cat > project-old/deprecated-config.ini << 'EOF'
[old_settings]
legacy_option=true
deprecated_feature=enabled
remove_this_section=yes
EOF

cat > project-old/old-readme.md << 'EOF'
# Old Documentation

This documentation is outdated and was replaced
with better docs in the new version.

## Old Features
- Feature A (removed)
- Feature B (deprecated)
- Feature C (replaced)
EOF

# Files that only exist in RIGHT (new project)
cat > project-new/features.md << 'EOF'
# New Features

This file documents new features added in version 2.0:

## New Additions
- Enhanced security
- Better performance
- Modern UI components
- Cloud integration
EOF

cat > project-new/api-docs.json << 'EOF'
{
  "version": "2.0",
  "endpoints": [
    {
      "path": "/api/v2/users",
      "method": "GET",
      "description": "New user management API"
    },
    {
      "path": "/api/v2/auth",
      "method": "POST",
      "description": "Enhanced authentication"
    }
  ]
}
EOF

cat > project-new/migration-guide.txt << 'EOF'
Migration Guide from v1.0 to v2.0

1. Update configuration files
2. Remove legacy components
3. Update API calls to v2 endpoints
4. Test new authentication system

Breaking Changes:
- Old auth system removed
- Legacy config format deprecated
- API v1 endpoints removed
EOF

cat > project-new/docker-compose.yml << 'EOF'
version: '3.8'
services:
  app:
    image: myapp:2.0
    ports:
      - "9000:9000"
    environment:
      - ENV=production
      - VERSION=2.0

  database:
    image: postgres:13
    environment:
      - POSTGRES_DB=myapp_new
EOF

echo "✅ Created test scenario:"
echo ""
echo "📁 project-old/ (LEFT directory):"
echo "   📄 config.yaml (modified in new version)"
echo "   📄 main.go (updated in new version)"
echo "   📄 legacy.txt [LEFT ONLY - removed file]"
echo "   📄 deprecated-config.ini [LEFT ONLY - removed config]"
echo "   📄 old-readme.md [LEFT ONLY - old documentation]"
echo ""
echo "📁 project-new/ (RIGHT directory):"
echo "   📄 config.yaml (updated from old version)"
echo "   📄 main.go (updated from old version)"
echo "   📄 features.md [RIGHT ONLY - new documentation]"
echo "   📄 api-docs.json [RIGHT ONLY - new API docs]"
echo "   📄 migration-guide.txt [RIGHT ONLY - migration info]"
echo "   📄 docker-compose.yml [RIGHT ONLY - new deployment]"
echo ""

echo "File Status Indicators You'll See:"
echo "=================================="
echo "✓ Green checkmark = Files exist in both directories with identical content"
echo "✗ Red X = Files exist in both directories but content differs"
echo "◄ Blue arrow = File exists ONLY in LEFT directory [LEFT ONLY]"
echo "► Orange arrow = File exists ONLY in RIGHT directory [RIGHT ONLY]"
echo ""

echo "New Capabilities:"
echo "================"
echo "🔍 View ALL files from both directories (not just common ones)"
echo "📂 See which directory each unique file comes from"
echo "📊 Get counts: total files, common files, unique files"
echo "🔄 Compare any file (even unique ones show as all additions/deletions)"
echo "📝 Understand project evolution (what was added/removed)"
echo ""

echo "Use Cases for All Files View:"
echo "============================"
echo "🔄 Migration Planning: See what files will be added/removed"
echo "🧹 Cleanup Tasks: Identify legacy files to remove"
echo "📈 Project Evolution: Understand how codebase changed"
echo "🔍 Missing Files: Find files that should exist in both versions"
echo "📋 Documentation: Track what new files were added"
echo ""

echo "How to Use:"
echo "==========="
echo "1. Navigate through ALL files with ↑/↓ arrows"
echo "2. Look for [LEFT ONLY] and [RIGHT ONLY] tags"
echo "3. Select unique files to see them as pure additions/deletions"
echo "4. Use merge mode (m) only on files that exist in both directories"
echo "5. Plan your migration/update strategy based on file status"
echo ""

echo "Starting the enhanced file comparison tool..."
echo "Press any key to continue..."
read -n 1 -s

# Launch the comparison tool
../katcmp project-old project-new

echo ""
echo "=========================================="
echo "  Enhanced All Files Demo Complete!"
echo "=========================================="
echo ""

# Show results summary
echo "Summary of what you should have seen:"
echo "===================================="
echo "📊 Total files: 8 (2 common, 6 unique)"
echo "📄 Common files: config.yaml, main.go"
echo "📄 Unique to LEFT: legacy.txt, deprecated-config.ini, old-readme.md"
echo "📄 Unique to RIGHT: features.md, api-docs.json, migration-guide.txt, docker-compose.yml"
echo ""

echo "Key Benefits Demonstrated:"
echo "========================="
echo "✨ Complete project visibility (all files shown)"
echo "✨ Clear source identification (LEFT/RIGHT tags)"
echo "✨ Migration planning capability"
echo "✨ Legacy file identification"
echo "✨ New feature discovery"
echo ""

echo "Real-World Applications:"
echo "======================="
echo "🔄 Code migrations between versions"
echo "🧹 Cleanup of deprecated files"
echo "📋 Documentation audits"
echo "🔍 Missing file detection"
echo "📈 Project evolution tracking"
echo ""

# Cleanup option
echo "Clean up demo files? (y/N)"
read -n 1 response
echo
if [[ $response =~ ^[Yy]$ ]]; then
    cd ..
    rm -rf all-files-demo
    echo "🗑️  Demo files cleaned up"
else
    echo "📁 Demo files kept in ./all-files-demo/"
    echo "   You can re-run: ./katcmp all-files-demo/project-old all-files-demo/project-new"
fi

echo ""
echo "🎉 Now you can see ALL files, not just common ones!"
echo "🎯 Perfect for project migrations, cleanups, and audits!"
