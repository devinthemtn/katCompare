# Merge Functionality Documentation

## Overview

The File Comparison Tool now includes powerful **Interactive Merge Mode** that allows you to selectively apply changes between files. This feature transforms the tool from a read-only diff viewer into an interactive merge editor, enabling you to cherry-pick specific changes and create custom merged versions.

## 🚀 Key Features

- **🎯 Selective Change Application**: Choose exactly which insertions and deletions to apply
- **🔄 Bidirectional Merging**: Apply changes to either the left or right file as a base
- **📊 Real-time Statistics**: See counts of selected vs. total changes
- **👁️ Visual Indicators**: Clear highlighting of selected/unselected changes
- **💾 File Export**: Save merged results with one keypress
- **⚡ Interactive Preview**: See merge results before saving

## 🎮 How to Use

### Basic Workflow

1. **Start Comparison**: `./filecmp file1.txt file2.txt`
2. **View Diff**: Press `Ctrl+D` to start comparing
3. **Enter Merge Mode**: Press `m` in diff view
4. **Select Changes**: Use `Space` or `Enter` to toggle changes
5. **Save Result**: Press `s` to save merged file

### Merge Mode Controls

| Key | Action |
|-----|--------|
| `↑/↓` or `j/k` | Navigate through diff lines |
| `Space/Enter` | Toggle selection of current change |
| `t` | Switch merge target (left ↔ right) |
| `a` | Select all changes |
| `n` | Select no changes (clear all) |
| `s` | Save merged result to file |
| `Esc` | Return to diff view |
| `?` | Show help screen |

## 🎨 Visual Indicators

### In Merge Mode:
- **Yellow background** → Selected changes (will be applied)
- **Strikethrough text** → Unselected changes (will be skipped)
- **[✓]** → Selected change checkbox
- **[ ]** → Unselected change checkbox
- **▶** → Current cursor position

### Change Types:
- **Green `+`** → Insertions (additions from right file)
- **Red `-`** → Deletions (removals from left file)
- **Gray** → Unchanged lines (always included)

## 🎯 Merge Targets

### Target: LEFT
- **Base**: Start with the left file content
- **Apply**: Selected insertions are added, selected deletions are removed
- **Result**: Left file with chosen changes from the right file
- **Use Case**: Updating an original file with select improvements

### Target: RIGHT  
- **Base**: Start with the right file content
- **Apply**: Selected insertions are removed, selected deletions are restored
- **Result**: Right file with chosen changes from the left file
- **Use Case**: Reverting specific changes from a modified file

## 📁 File Output

Merged files are saved with the `.merged` extension:
- `file1.txt.merged` (when targeting left file)
- `file2.txt.merged` (when targeting right file)

The saved file contains only the final merged content, ready for use.

## 🛠️ Use Cases

### 1. Configuration Management
```bash
./filecmp config-old.yaml config-new.yaml
# Select only security updates, skip breaking changes
# Result: Incremental, safe configuration update
```

### 2. Code Review & Integration
```bash
./filecmp main-branch.js feature-branch.js  
# Cherry-pick specific functions, skip experimental code
# Result: Selective feature integration
```

### 3. Document Collaboration
```bash
./filecmp draft-v1.md draft-v2.md
# Accept some edits, reject others
# Result: Collaborative document with chosen revisions
```

### 4. Database Schema Migration
```bash
./filecmp schema-old.sql schema-new.sql
# Apply new tables, keep existing data migrations
# Result: Custom migration script
```

## 🔧 Technical Implementation

### Architecture
```
internal/
├── merge/          # Merge functionality
│   └── merge.go    # Core merge logic and change selection
├── ui/             # User interface
│   ├── model.go    # Extended with merge mode state
│   └── views.go    # Merge view rendering
└── differ/         # Diff computation (existing)
```

### Core Components

#### ChangeSelection
Tracks which changes to apply:
```go
type ChangeSelection struct {
    ApplyInsertions map[int]bool
    ApplyDeletions  map[int]bool
}
```

#### MergeResult
Contains merged content and statistics:
```go
type MergeResult struct {
    Content string
    Applied int
    Skipped int
}
```

#### Merger
Handles merge operations:
- `ApplyToLeft()` - Apply changes to left file base
- `ApplyToRight()` - Apply changes to right file base
- `CreateMergePreview()` - Generate preview text

## 📊 Statistics & Feedback

The tool provides real-time feedback:
- **Selection counts**: "Selected: 3/5 insertions, 2/4 deletions"
- **Merge results**: "Applied 5 changes, skipped 2 changes"
- **File output**: "Saved merged result to config.yaml.merged"

## 🎭 Example Scenarios

### Scenario 1: Safe Production Update
```
Target: LEFT (production config)
Select: Performance optimizations
Skip: Experimental features, debug settings
Result: Production-safe updates only
```

### Scenario 2: Development Rollback
```
Target: RIGHT (latest changes)  
Select: Bug fixes from old version
Skip: New features causing issues
Result: Stable version with essential fixes
```

### Scenario 3: Custom Feature Mix
```
Target: LEFT (base functionality)
Select: Specific new features
Skip: Breaking changes, dependencies
Result: Incremental feature adoption
```

## 🚦 Best Practices

### Before Merging
- ✅ Review all changes in diff mode first
- ✅ Understand the impact of each change
- ✅ Test with non-critical files initially

### During Merge
- ✅ Start with "Select All" then deselect unwanted changes
- ✅ Use 't' to preview different merge targets
- ✅ Check statistics to verify selection count

### After Merge
- ✅ Review the generated `.merged` file
- ✅ Test the merged result before replacing originals
- ✅ Keep backups of original files

## 🔮 Advanced Tips

### Keyboard Efficiency
- Use `a` then deselect unwanted changes (faster than selecting individually)
- Use `n` to start fresh if you change your mind
- Navigate with `j/k` for vim-like speed

### Merge Strategy Planning
1. **Additive Merge**: Select all insertions, few deletions
2. **Subtractive Merge**: Select all deletions, few insertions  
3. **Selective Merge**: Carefully choose both types
4. **Conservative Merge**: Select minimal changes

### File Organization
- Merged files get `.merged` extension automatically
- Use descriptive names: `config-production.yaml.merged`
- Version control your merge decisions

## 🐛 Troubleshooting

### Common Issues

**Issue**: Changes not appearing as expected
- **Solution**: Check merge target (left vs right)
- **Tip**: Use 't' to switch and compare results

**Issue**: Merge result looks wrong  
- **Solution**: Review selection in merge mode
- **Tip**: Yellow = applied, strikethrough = skipped

**Issue**: File not saved
- **Solution**: Ensure write permissions in directory
- **Tip**: Check terminal output for error messages

## 🎓 Learning Path

### Beginner
1. Try `make demo-merge-simple` 
2. Practice with example files
3. Focus on single-type changes (just insertions OR deletions)

### Intermediate  
4. Use `make demo-merge` for complex scenarios
5. Practice with configuration files
6. Learn both merge targets (left/right)

### Advanced
7. Integrate into development workflow
8. Create merge scripts for common patterns
9. Combine with version control workflows

## 🔗 Integration

### With Git
```bash
# Compare branches
git show branch1:file.txt > /tmp/file1.txt
git show branch2:file.txt > /tmp/file2.txt
./filecmp /tmp/file1.txt /tmp/file2.txt
```

### With Development Workflow
```bash
# Configuration updates
./filecmp config-prod.yaml config-staging.yaml
# Select production-safe changes
# Save as config-updated.yaml.merged
```

### With Automation
```bash
# Scripted merges (future enhancement)
./filecmp --merge --target=left --select=insertions file1 file2
```

## 📈 Future Enhancements

Planned features for merge functionality:
- [ ] Three-way merge support
- [ ] Merge conflict detection and resolution
- [ ] Batch merge operations
- [ ] Merge templates and presets  
- [ ] Integration with external diff tools
- [ ] Undo/redo functionality
- [ ] Command-line merge operations
- [ ] Custom merge rules and filters

---

**🎉 The merge functionality transforms file comparison from passive viewing to active editing, enabling precise control over how changes are integrated between file versions.**