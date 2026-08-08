# Copy Functionality Documentation

## Overview

The **Copy Functionality** is a powerful feature that allows you to easily copy unique files between directories during comparison. This feature is perfect for project synchronization, feature backporting, documentation sharing, and directory migration tasks.

## 🚀 Key Features

- **🔄 Bidirectional Copying**: Copy files from left-to-right or right-to-left
- **📋 Selective Copying**: Choose exactly which unique files to copy
- **🎯 Smart Filtering**: Only shows files that exist in one directory (unique files)
- **💾 Safe Operations**: Creates directories as needed and preserves file permissions
- **⚡ Batch Operations**: Copy multiple files at once
- **🔍 Visual Indicators**: Clear selection and direction indicators

## 🎮 How to Use Copy Mode

### Basic Workflow

1. **Start Directory Comparison**: `./filecmp left-dir right-dir`
2. **Enter Copy Mode**: Press `c` from file selection view
3. **Select Files**: Use `Space` or `Enter` to toggle file selection
4. **Choose Direction**: Press `t` to switch between to-left/to-right
5. **Execute Copy**: Press `s` to copy selected files

### Copy Mode Controls

| Key | Action |
|-----|--------|
| `↑/↓` or `j/k` | Navigate through unique files |
| `Space/Enter` | Toggle selection of current file |
| `t` | Switch copy target (to-left ↔ to-right) |
| `a` | Select all unique files |
| `n` | Select no files (clear all) |
| `s` | Copy selected files to target directory |
| `Esc` | Return to file selection |
| `?` | Show help screen |

## 🎨 Visual Indicators

### Selection Status
- **[✓]** - Selected for copying (highlighted in yellow)
- **[ ]** - Not selected (normal appearance)
- **[-]** - Cannot copy (grayed out - wrong direction)

### File Source
- **◄** - File exists only in LEFT directory
- **►** - File exists only in RIGHT directory

### Copy Direction
- **TO-RIGHT** - Copy [LEFT ONLY] files → RIGHT directory
- **TO-LEFT** - Copy [RIGHT ONLY] files → LEFT directory

## 🎯 Copy Directions Explained

### TO-RIGHT Direction
- **Source**: Files that exist only in the LEFT directory
- **Target**: RIGHT directory
- **Use Cases**:
  - Adding legacy features to new version
  - Preserving historical documentation
  - Keeping old configuration files

### TO-LEFT Direction
- **Source**: Files that exist only in the RIGHT directory
- **Target**: LEFT directory
- **Use Cases**:
  - Backporting new features to old version
  - Adding new documentation to old branch
  - Sharing new configuration templates

## 🛠️ Use Cases & Examples

### 1. Project Migration
**Scenario**: Migrating from v1.0 to v2.0, want to keep some legacy files

```bash
./filecmp project-v1/ project-v2/
# Press 'c' to enter copy mode
# Set direction to TO-RIGHT
# Select legacy files you want to preserve
# Press 's' to copy them to v2.0
```

**Benefits**:
- Preserve important legacy code
- Keep historical documentation
- Maintain compatibility files

### 2. Feature Backporting
**Scenario**: New feature in main branch needs to be added to stable branch

```bash
./filecmp stable-branch/ main-branch/
# Press 'c' to enter copy mode
# Set direction to TO-LEFT
# Select new feature files
# Press 's' to copy them to stable branch
```

**Benefits**:
- Safely backport specific features
- Add new functionality to older versions
- Share improvements across branches

### 3. Environment Synchronization
**Scenario**: Development environment missing some production config files

```bash
./filecmp dev-config/ prod-config/
# Press 'c' to enter copy mode
# Set direction to TO-LEFT
# Select missing production configs
# Press 's' to copy them to dev environment
```

**Benefits**:
- Sync configurations across environments
- Ensure dev/prod parity
- Share environment-specific files

### 4. Documentation Management
**Scenario**: New documentation created in one branch needs to be shared

```bash
./filecmp docs-old/ docs-new/
# Press 'c' to enter copy mode
# Choose appropriate direction
# Select documentation files to share
# Press 's' to copy them
```

**Benefits**:
- Share documentation across versions
- Preserve historical information
- Maintain documentation consistency

## 📊 Copy Operation Results

After copying files, you'll see a summary like:
```
Copy operation completed: 3 copied, 1 skipped

Files copied:
- legacy-module.py → project-new/legacy-module.py
- old-docs.md → project-new/old-docs.md  
- config-template.ini → project-new/config-template.ini
```

### Result Statistics
- **Copied**: Number of files successfully copied
- **Skipped**: Number of files not selected for copying
- **Errors**: Any files that failed to copy (with error details)

## 🔧 Technical Details

### File Filtering
Copy mode only shows **unique files** - files that exist in only one directory:
- Files with `Source: SourceLeft` (LEFT ONLY)
- Files with `Source: SourceRight` (RIGHT ONLY)
- Common files are excluded (can't be copied since they exist in both places)

### Directory Creation
- Target directories are created automatically if they don't exist
- Preserves the original directory structure
- Uses appropriate permissions (0755 for directories, 0644 for files)

### Error Handling
- Graceful handling of permission errors
- Clear error messages for failed operations
- Continues copying other files even if some fail
- Detailed error reporting in results

## 🚦 Best Practices

### Before Copying
- ✅ Review all unique files first
- ✅ Understand the impact of each file
- ✅ Check target directory has sufficient space
- ✅ Backup important directories before major copy operations

### During Copy Selection
- ✅ Start with 'a' to select all, then deselect unwanted files
- ✅ Use 't' to preview both copy directions
- ✅ Check the file count statistics before copying
- ✅ Verify copy direction matches your intention

### After Copying
- ✅ Verify copied files in target directory
- ✅ Test functionality if copying code files
- ✅ Update any configuration files that reference new files
- ✅ Commit changes to version control

## 🎭 Example Scenarios

### Scenario 1: Legacy Cleanup with Preservation
```
Goal: Migrate to new codebase but keep important legacy files
Steps:
1. Compare old-code/ vs new-code/
2. Enter copy mode
3. Direction: TO-RIGHT (copy legacy to new)
4. Select: important legacy modules, documentation
5. Skip: deprecated files, old configs
6. Copy selected files to new codebase
```

### Scenario 2: Feature Development Sync
```
Goal: Share new features between development branches
Steps:
1. Compare feature-branch/ vs main-branch/
2. Enter copy mode  
3. Direction: TO-LEFT (copy new features to main)
4. Select: stable new features, tests, documentation
5. Skip: experimental files, work-in-progress
6. Copy selected files to main branch
```

### Scenario 3: Environment Configuration Sync
```
Goal: Ensure all environments have necessary config files
Steps:
1. Compare local-config/ vs server-config/
2. Enter copy mode
3. Direction: TO-LEFT (copy server configs locally)
4. Select: missing configuration templates
5. Skip: environment-specific secrets
6. Copy configuration files for local development
```

## 🐛 Troubleshooting

### Common Issues

**Issue**: "No unique files to copy" message
- **Cause**: Both directories have identical file structures
- **Solution**: This is normal! No unique files means directories are in sync

**Issue**: Copy operation shows "0 copied, X skipped"  
- **Cause**: No files were selected for copying
- **Solution**: Use Space/Enter to select files before pressing 's'

**Issue**: Files not appearing in copy mode
- **Cause**: Files might not be detected as text files
- **Solution**: Check that files have recognized text extensions

**Issue**: Permission denied errors during copy
- **Cause**: Insufficient permissions in target directory
- **Solution**: Ensure write permissions in target directory

### Copy Direction Confusion

**Remember**:
- **TO-RIGHT**: Takes [LEFT ONLY] files and puts them in RIGHT directory
- **TO-LEFT**: Takes [RIGHT ONLY] files and puts them in LEFT directory
- Use 't' to switch directions and see which files become available

## 🔮 Advanced Tips

### Keyboard Efficiency
- Use 'a' then deselect unwanted files (faster than selecting individually)
- Use 'n' to clear all selections and start over
- Navigate with 'j/k' for vim-like speed
- Use 't' to compare both directions before deciding

### Workflow Integration
- **Version Control**: Use copy mode to sync files between branches
- **Deployment**: Copy environment-specific files between deployments
- **Backup**: Copy unique files to backup directories
- **Migration**: Gradually move files during system migrations

### Batch Operations
- Select multiple files and copy them all at once
- Use copy mode repeatedly for different file categories
- Combine with merge mode for comprehensive directory synchronization

## 🎓 Learning Path

### Beginner
1. Try `make demo-copy` for hands-on tutorial
2. Practice with test directories
3. Focus on one copy direction at a time

### Intermediate  
4. Use in real project scenarios
5. Combine with merge mode for full synchronization
6. Learn to identify which files should/shouldn't be copied

### Advanced
7. Integrate into development workflows
8. Create scripts that use copy operations
9. Use for complex multi-environment synchronization

## 🔗 Integration with Other Features

### With Merge Mode
1. Use copy mode to sync unique files between directories
2. Use merge mode to handle files that exist in both directories
3. Complete directory synchronization workflow

### With All Files View
- Copy mode only shows unique files (subset of all files view)
- All files view shows complete picture: common + unique files
- Use all files view to understand full directory differences

### With Directory Comparison
- Copy mode requires directory comparison (not individual files)
- Works with any directory structure
- Preserves relative paths and directory structure

## 📈 Future Enhancements

Planned improvements for copy functionality:
- [ ] Preview mode showing exact copy operations before execution
- [ ] Undo/rollback capability for copy operations
- [ ] Copy templates and presets for common scenarios
- [ ] Integration with version control systems
- [ ] Dry-run mode to simulate copy operations
- [ ] Copy operation logging and history
- [ ] Conflict resolution for existing files

## 🎉 Benefits Summary

### For Developers
- **Easy synchronization** of files between project versions
- **Safe backporting** of features to older branches
- **Streamlined migration** workflows
- **Reduced manual file copying** errors

### for DevOps
- **Configuration management** across environments
- **Deployment synchronization** capabilities
- **Environment parity** maintenance
- **Infrastructure evolution** support

### For Project Managers
- **Visible file synchronization** process
- **Controlled feature migration** between versions
- **Risk reduction** through selective copying
- **Project evolution** tracking

---

**The Copy Functionality transforms directory comparison from passive viewing to active synchronization, enabling precise control over which files are shared between directories.** 🚀