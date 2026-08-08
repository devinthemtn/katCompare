#!/bin/bash

# Comprehensive Merge Example for golang-fileCmp
# This script demonstrates the new merge functionality

echo "=========================================="
echo "  File Comparison Tool - Merge Example"
echo "=========================================="
echo

# Create test directory
mkdir -p merge-demo
cd merge-demo

# Create test files with meaningful differences
echo "Creating test files..."

# Original configuration file
cat > config-old.yaml << 'EOF'
# Application Configuration
app:
  name: "MyApp"
  version: "1.0.0"
  debug: false
  port: 8080

database:
  host: "localhost"
  port: 5432
  username: "admin"
  password: "secret123"
  ssl_mode: "disable"

features:
  analytics: true
  logging: true
  monitoring: false
  cache: false

# Legacy settings
old_feature: true
deprecated_option: "remove_me"
EOF

# Updated configuration file
cat > config-new.yaml << 'EOF'
# Application Configuration
app:
  name: "MyApp"
  version: "2.0.0"
  debug: true
  port: 9000
  environment: "production"

database:
  host: "db.example.com"
  port: 5432
  username: "admin"
  password: "secret123"
  ssl_mode: "require"
  pool_size: 10

features:
  analytics: true
  logging: true
  monitoring: true
  cache: true
  metrics: true

# New settings
api_rate_limit: 1000
backup_enabled: true
EOF

echo "✅ Created test files:"
echo "   - config-old.yaml (original version)"
echo "   - config-new.yaml (updated version)"
echo

echo "Differences between files:"
echo "=========================="
echo "📝 Changes in config-new.yaml:"
echo "   • Version updated: 1.0.0 → 2.0.0"
echo "   • Debug enabled: false → true"
echo "   • Port changed: 8080 → 9000"
echo "   • Added: environment, pool_size"
echo "   • Database host changed to production"
echo "   • SSL mode: disable → require"
echo "   • Monitoring and cache enabled"
echo "   • Added new metrics feature"
echo "   • Removed legacy settings"
echo "   • Added API rate limiting and backup"
echo

echo "How to use Merge Mode:"
echo "====================="
echo "1. Run: ../filecmp config-old.yaml config-new.yaml"
echo "2. Press Ctrl+D to start comparing"
echo "3. Navigate through differences with ↑/↓ or j/k"
echo "4. Press 'm' to enter MERGE MODE"
echo "5. In merge mode:"
echo "   • Yellow background = Selected changes (will be applied)"
echo "   • Strikethrough = Unselected changes (will be skipped)"
echo "   • Space/Enter = Toggle current change selection"
echo "   • 't' = Switch merge target (LEFT ↔ RIGHT)"
echo "   • 'a' = Select all changes"
echo "   • 'n' = Select no changes"
echo "   • 's' = Save merged result"
echo

echo "Merge Strategies:"
echo "================"
echo "🎯 TARGET: LEFT (start with config-old.yaml)"
echo "   • Select version update only → Incremental upgrade"
echo "   • Select all production settings → Full migration"
echo "   • Select only security changes → Security-focused update"
echo

echo "🎯 TARGET: RIGHT (start with config-new.yaml)"
echo "   • Select old password → Keep existing credentials"
echo "   • Select old debug setting → Maintain dev environment"
echo "   • Mix old and new features → Custom configuration"
echo

echo "Example Workflows:"
echo "=================="
echo "🔄 Scenario 1: Gradual Migration"
echo "   1. Target: LEFT (old config)"
echo "   2. Select: version update, new features"
echo "   3. Skip: production settings, breaking changes"
echo "   4. Result: Updated features with safe settings"
echo

echo "🔄 Scenario 2: Production Deployment"
echo "   1. Target: RIGHT (new config)"
echo "   2. Select: all production optimizations"
echo "   3. Skip: debug settings"
echo "   4. Result: Production-ready configuration"
echo

echo "🔄 Scenario 3: Cherry-Pick Features"
echo "   1. Target: LEFT (old config)"
echo "   2. Select: specific features (monitoring, cache)"
echo "   3. Skip: version changes, breaking updates"
echo "   4. Result: Selective feature adoption"
echo

echo "Starting the comparison tool..."
echo "Press any key to continue..."
read -n 1 -s
echo

# Launch the comparison tool
../filecmp config-old.yaml config-new.yaml

echo
echo "=========================================="
echo "  Merge Demo Complete!"
echo "=========================================="
echo

# Check if any merged files were created
if ls *.merged 2>/dev/null; then
    echo "✅ Merged files created:"
    for file in *.merged; do
        echo "   📄 $file"
        echo "      Preview (first 10 lines):"
        head -10 "$file" | sed 's/^/         /'
        echo "      ..."
        echo
    done
else
    echo "ℹ️  No merged files found. To create merged files:"
    echo "   1. Enter merge mode with 'm'"
    echo "   2. Select desired changes with Space/Enter"
    echo "   3. Save with 's'"
fi

echo
echo "Key Features Demonstrated:"
echo "========================="
echo "✨ Interactive diff visualization"
echo "✨ Selective change application"
echo "✨ Bidirectional merge targets"
echo "✨ Real-time merge preview"
echo "✨ File export functionality"
echo
echo "This merge capability allows you to:"
echo "• Merge configuration files selectively"
echo "• Apply only specific code changes"
echo "• Create custom combinations of file versions"
echo "• Safely migrate between file versions"
echo "• Resolve conflicts by choosing specific changes"
echo

# Clean up option
echo "Clean up demo files? (y/N)"
read -n 1 response
echo
if [[ $response =~ ^[Yy]$ ]]; then
    cd ..
    rm -rf merge-demo
    echo "🗑️  Demo files cleaned up"
else
    echo "📁 Demo files kept in ./merge-demo/"
fi

echo "Thank you for trying the merge functionality! 🚀"
