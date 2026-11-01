#!/bin/bash

# Demo script for the copy functionality
echo "=========================================="
echo "  File Copy Functionality Demo"
echo "=========================================="
echo ""

# Create test directories
mkdir -p copy-demo
cd copy-demo

echo "Creating test scenario for copy functionality..."
echo ""

# Create source directories with different files
mkdir -p project-old project-new

# Files that exist in both directories (common files)
cat > project-old/shared-config.yaml << 'EOF'
# Shared configuration - old version
version: "1.0"
database:
  host: "localhost"
  port: 5432
features:
  legacy_mode: true
EOF

cat > project-new/shared-config.yaml << 'EOF'
# Shared configuration - new version
version: "2.0"
database:
  host: "prod-db.example.com"
  port: 5432
features:
  legacy_mode: false
  new_features: true
EOF

# Files that only exist in OLD directory (LEFT ONLY)
cat > project-old/legacy-module.py << 'EOF'
"""
Legacy Python module that was removed in the new version.
This file should be copied to the new version if we want to keep it.
"""

def old_function():
    print("This is the old way of doing things")
    return "legacy_result"

def deprecated_feature():
    print("This feature was deprecated")
    return None
EOF

cat > project-old/old-readme.md << 'EOF'
# Old Project Documentation

This documentation was replaced in the new version.
Contains important historical information.

## Old Features
- Legacy authentication system
- Deprecated API endpoints
- Old configuration format

## Migration Notes
These features were removed in v2.0
EOF

cat > project-old/legacy-config.ini << 'EOF'
[database]
old_connection_string=mysql://localhost/olddb
legacy_auth=basic

[features]
deprecated_feature=enabled
old_ui=true
remove_in_v2=yes
EOF

# Files that only exist in NEW directory (RIGHT ONLY)
cat > project-new/new-feature.js << 'EOF'
/**
 * New JavaScript module added in version 2.0
 * This file should be copied to the old version if we want to backport features
 */

class NewFeature {
    constructor(config) {
        this.config = config;
        this.initialized = false;
    }

    initialize() {
        console.log("Initializing new feature");
        this.initialized = true;
        return this;
    }

    execute() {
        if (!this.initialized) {
            throw new Error("Feature not initialized");
        }
        console.log("Executing new feature");
        return "success";
    }
}

module.exports = NewFeature;
EOF

cat > project-new/api-v2-docs.json << 'EOF'
{
  "version": "2.0",
  "title": "Project API v2 Documentation",
  "description": "New API endpoints introduced in version 2.0",
  "endpoints": [
    {
      "path": "/api/v2/users",
      "methods": ["GET", "POST", "PUT", "DELETE"],
      "description": "Enhanced user management"
    },
    {
      "path": "/api/v2/auth/oauth",
      "methods": ["POST"],
      "description": "OAuth authentication endpoint"
    },
    {
      "path": "/api/v2/features",
      "methods": ["GET"],
      "description": "List available features"
    }
  ]
}
EOF

cat > project-new/migration-script.sql << 'EOF'
-- Migration script for version 2.0
-- This file helps migrate from old to new database schema

-- Create new tables
CREATE TABLE IF NOT EXISTS new_features (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    enabled BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Migrate old data
INSERT INTO new_features (name, enabled)
SELECT 'legacy_feature', true
WHERE EXISTS (SELECT 1 FROM old_legacy_table);

-- Drop old tables (commented for safety)
-- DROP TABLE IF EXISTS old_legacy_table;

-- Add indexes for performance
CREATE INDEX idx_features_name ON new_features(name);
CREATE INDEX idx_features_enabled ON new_features(enabled);
EOF

cat > project-new/deployment-guide.md << 'EOF'
# Deployment Guide v2.0

This guide explains how to deploy the new version.
Should be copied to the old version for reference.

## Prerequisites
- Docker 20.0+
- Kubernetes 1.18+
- Node.js 16+

## Deployment Steps
1. Build the application
   ```bash
   docker build -t myapp:2.0 .
   ```

2. Deploy to staging
   ```bash
   kubectl apply -f k8s/staging/
   ```

3. Run migrations
   ```bash
   kubectl exec -it deployment/myapp -- npm run migrate
   ```

4. Deploy to production
   ```bash
   kubectl apply -f k8s/production/
   ```

## Rollback Plan
If issues occur, rollback to previous version:
```bash
kubectl rollout undo deployment/myapp
```
EOF

echo "✅ Created test scenario:"
echo ""
echo "📁 project-old/ (LEFT directory):"
echo "   📄 shared-config.yaml (exists in both, different content)"
echo "   📄 legacy-module.py [LEFT ONLY - old code to potentially keep]"
echo "   📄 old-readme.md [LEFT ONLY - historical documentation]"
echo "   📄 legacy-config.ini [LEFT ONLY - old configuration]"
echo ""
echo "📁 project-new/ (RIGHT directory):"
echo "   📄 shared-config.yaml (exists in both, different content)"
echo "   📄 new-feature.js [RIGHT ONLY - new functionality to backport]"
echo "   📄 api-v2-docs.json [RIGHT ONLY - new API documentation]"
echo "   📄 migration-script.sql [RIGHT ONLY - database migration]"
echo "   📄 deployment-guide.md [RIGHT ONLY - deployment instructions]"
echo ""

echo "Copy Functionality Overview:"
echo "==========================="
echo "🔄 The copy mode lets you easily sync files between directories"
echo "📋 Select which unique files to copy and in which direction"
echo "⚡ Perfect for project migrations, backports, and synchronization"
echo ""

echo "How Copy Mode Works:"
echo "==================="
echo "1️⃣  Press 'c' from file selection to enter Copy Mode"
echo "2️⃣  Navigate through unique files with ↑/↓ or j/k"
echo "3️⃣  Toggle files to copy with Space/Enter"
echo "4️⃣  Switch copy direction with 't' (to-left ↔ to-right)"
echo "5️⃣  Press 's' to copy selected files"
echo ""

echo "Copy Direction Options:"
echo "======================="
echo "🔵 TO-RIGHT: Copy [LEFT ONLY] files → RIGHT directory"
echo "   Use case: Add old features/docs to new version"
echo ""
echo "🔶 TO-LEFT: Copy [RIGHT ONLY] files → LEFT directory"
echo "   Use case: Backport new features to old version"
echo ""

echo "Common Copy Scenarios:"
echo "====================="
echo "📚 Preserve Documentation:"
echo "   Copy old-readme.md from old → new (historical reference)"
echo ""
echo "🔧 Keep Legacy Features:"
echo "   Copy legacy-module.py from old → new (maintain compatibility)"
echo ""
echo "⚡ Backport New Features:"
echo "   Copy new-feature.js from new → old (add new functionality)"
echo ""
echo "📋 Share Documentation:"
echo "   Copy deployment-guide.md from new → old (share procedures)"
echo ""

echo "Visual Indicators in Copy Mode:"
echo "==============================="
echo "[✓] = Selected file (will be copied)"
echo "[ ] = Unselected file (will be skipped)"
echo "[-] = Cannot copy (wrong direction for current target)"
echo "◄   = File exists only in LEFT directory"
echo "►   = File exists only in RIGHT directory"
echo ""

echo "Starting the file comparison tool..."
echo "Instructions:"
echo "============"
echo "1. Look at file list - you should see 7 files (1 common, 6 unique)"
echo "2. Press 'c' to enter Copy Mode"
echo "3. Try both copy directions:"
echo "   - 'TO-RIGHT': Copy legacy files to new project"
echo "   - 'TO-LEFT': Copy new features to old project"
echo "4. Select files with Space/Enter"
echo "5. Press 's' to copy selected files"
echo "6. Check the target directory to see copied files"
echo ""

echo "Press any key to start the demo..."
read -n 1 -s

# Launch the comparison tool
../katcmp project-old project-new

echo ""
echo "=========================================="
echo "  Copy Demo Complete!"
echo "=========================================="
echo ""

# Show results
echo "Check the results:"
echo "=================="
echo "📁 Files copied TO-RIGHT should appear in project-new/"
echo "📁 Files copied TO-LEFT should appear in project-old/"
echo ""

echo "Verify copied files:"
if [ -f "project-new/legacy-module.py" ]; then
    echo "✅ legacy-module.py was copied to project-new/"
else
    echo "ℹ️  legacy-module.py was not copied (or copy to-right not performed)"
fi

if [ -f "project-old/new-feature.js" ]; then
    echo "✅ new-feature.js was copied to project-old/"
else
    echo "ℹ️  new-feature.js was not copied (or copy to-left not performed)"
fi

echo ""
echo "Real-World Use Cases:"
echo "===================="
echo "🔄 Project Migrations: Sync files between version branches"
echo "📚 Documentation: Share docs between environments"
echo "🔧 Feature Backports: Add new features to older versions"
echo "🧹 Cleanup Planning: Identify files to remove/add"
echo "⚙️  Configuration Sync: Sync configs between dev/prod"
echo ""

# Cleanup option
echo "Clean up demo files? (y/N)"
read -n 1 response
echo
if [[ $response =~ ^[Yy]$ ]]; then
    cd ..
    rm -rf copy-demo
    echo "🗑️  Demo files cleaned up"
else
    echo "📁 Demo files kept in ./copy-demo/"
    echo "   You can examine copied files and re-run the demo"
fi

echo ""
echo "🎉 Copy functionality makes directory synchronization easy!"
echo "🎯 Perfect for managing file differences across project versions!"
