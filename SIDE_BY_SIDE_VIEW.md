# Side-by-Side View Documentation

## Overview

The **Side-by-Side View** is a powerful visualization feature that displays file differences in a split-screen layout, making it easier to compare changes between two files. This feature complements the traditional unified diff view by providing a more intuitive way to see exactly what changed and where.

## 🚀 Key Features

- **📱 Split-Screen Layout**: View left and right files simultaneously
- **🎨 Color-Coded Differences**: Visual highlighting of insertions, deletions, and modifications
- **🔄 Seamless Toggle**: Switch between unified and side-by-side views instantly
- **⌨️ Intuitive Navigation**: Full keyboard navigation with vim-like controls
- **📏 Responsive Design**: Automatically adapts to terminal width
- **👁️ Visual Focus**: Left/right focus indicators for better orientation

## 🎮 How to Use Side-by-Side View

### Basic Workflow

1. **Start File Comparison**: `./filecmp file1.txt file2.txt`
2. **Enter Diff View**: Press `Ctrl+D` to start comparing
3. **Switch to Side-by-Side**: Press `s` to toggle view mode
4. **Navigate**: Use arrow keys or `j/k` to move through differences
5. **Switch Back**: Press `s` again to return to unified view

### Side-by-Side Controls

| Key | Action |
|-----|--------|
| `↑/↓` or `j/k` | Navigate through diff lines |
| `h/l` or `←/→` | Visual focus left/right (for reference) |
| `g` | Go to top of diff |
| `G` | Go to bottom of diff |
| `s` | Switch to unified view mode |
| `n/p` | Next/previous file |
| `m` | Enter merge mode |
| `Esc` | Return to file selection |
| `?` | Show help screen |
| `Q/Ctrl+C` | Quit application |

## 🎨 Visual Layout

### Split-Screen Format
```
┌─────────────────────────────────┬─────────────────────────────────┐
│ file1.txt [Unified]             │ file2.txt                       │
│                                 │                                 │
│ Lines: 10 equal, 3 inserted (+), 2 deleted (-)                   │
│                                 │                                 │
│ ▶   1 Line 1: Same content      │     1 Line 1: Same content      │
│     2 Line 2: Deleted line      │       (empty)                   │
│     3 Line 3: Modified (old)    │     2 Line 3: Modified (new)    │
│     4 Line 4: Common line       │     3 Line 4: Common line       │
│       (empty)                   │     4 Line 4.5: New line        │
│     5 Line 5: Final line        │     5 Line 5: Final line        │
│                                 │                                 │
│ Navigation: ↑↓/j/k:Lines h/l:L/R s:View n/p:Files m:Merge        │
└─────────────────────────────────┴─────────────────────────────────┘
```

### Color Coding

- **🟦 Blue Background**: Deleted lines (appear only on left side)
- **🟩 Green Background**: Inserted lines (appear only on right side)
- **🟨 Yellow Background**: Modified lines (different content on both sides)
- **⚪ Gray Text**: Unchanged lines (same content on both sides)
- **▶ Arrow**: Current cursor position

## 🛠️ Technical Features

### Adaptive Layout
- **Dynamic Width**: Each side gets ~50% of terminal width
- **Minimum Width**: Ensures readability even on narrow terminals
- **Line Truncation**: Long lines are truncated with ellipsis
- **Scroll Synchronization**: Both sides scroll together

### Smart Line Alignment
- **Deletion Handling**: Empty space shown on right when lines are deleted from left
- **Insertion Handling**: Empty space shown on left when lines are added to right
- **Modification Display**: Both versions shown side-by-side for changed lines
- **Line Numbering**: Original line numbers preserved from source files

### Performance Optimizations
- **Viewport Rendering**: Only visible lines are rendered
- **Efficient Scrolling**: Smooth navigation through large diffs
- **Memory Management**: Minimal memory overhead for line reconstruction

## 🎯 Use Cases & Benefits

### 1. Code Review
**Scenario**: Reviewing code changes in a pull request

**Benefits**:
- See original and modified code simultaneously
- Easily identify what was changed vs what was added/removed
- Better understanding of refactoring and logic changes
- Quick spotting of formatting and style differences

### 2. Configuration Comparison
**Scenario**: Comparing configuration files between environments

**Benefits**:
- Side-by-side comparison of settings
- Easy identification of missing or different configuration values
- Clear view of environment-specific differences
- Quick validation of configuration migrations

### 3. Documentation Updates
**Scenario**: Reviewing documentation changes and updates

**Benefits**:
- See old and new content simultaneously
- Understand the context of changes better
- Verify that important information wasn't lost
- Review structural changes to documents

### 4. Data File Analysis
**Scenario**: Comparing CSV, JSON, or other data files

**Benefits**:
- Structured view of data differences
- Easy identification of added/removed records
- Clear view of field-level changes
- Better understanding of data evolution

## 🔄 View Mode Comparison

### Unified View
- **Best for**: Quick overview of all changes
- **Advantages**: Compact display, traditional diff format
- **Use cases**: Small changes, line-by-line review

### Side-by-Side View  
- **Best for**: Detailed comparison and analysis
- **Advantages**: Context preservation, visual clarity
- **Use cases**: Large changes, code review, configuration comparison

### Toggle Strategy
- Start with **Unified** for quick assessment
- Switch to **Side-by-Side** for detailed analysis
- Use **Unified** for merge operations
- Use **Side-by-Side** for understanding complex changes

## 📊 Layout Examples

### Example 1: Simple Text Changes
```
Left File (Original)           │ Right File (Modified)
────────────────────────────   │ ────────────────────────────
  1 Hello, World!              │   1 Hello, Universe!
  2 This is line two           │   2 This is line two  
  3 Line three here            │   3 Line three here
  4 Final line                 │   4 Final line
                               │   5 New line at end
```

### Example 2: Code Refactoring
```
Left File (Before)             │ Right File (After)
────────────────────────────   │ ────────────────────────────
  1 function oldName() {       │   1 function newName() {
  2   var x = 10;              │   2   const x = 10;
  3   return x * 2;            │   3   return x * 2;
  4 }                          │   4 }
  5                            │   5 
  6 oldName();                 │   6 newName();
```

### Example 3: Configuration Changes  
```
Left File (Dev Config)         │ Right File (Prod Config)
────────────────────────────   │ ────────────────────────────
  1 debug=true                 │   1 debug=false
  2 port=3000                  │   2 port=8080
  3 host=localhost             │   3 host=0.0.0.0
  4                            │   4 ssl=true
  5                            │   5 cert_path=/etc/ssl/cert
```

## 🚦 Best Practices

### When to Use Side-by-Side View
- ✅ **Large file changes** with many modifications
- ✅ **Configuration file comparison** across environments
- ✅ **Code review** where context is important
- ✅ **Data file analysis** with structured content
- ✅ **Documentation review** with significant changes

### When to Use Unified View
- ✅ **Small, focused changes** with few modifications
- ✅ **Quick overview** of what changed
- ✅ **Merge operations** where you need to select changes
- ✅ **Narrow terminals** where side-by-side doesn't fit well
- ✅ **Traditional diff workflow** preference

### Navigation Tips
- **Use `j/k`** for vim-like navigation speed
- **Use `g/G`** to quickly jump to beginning/end
- **Use `h/l`** for visual reference of which side you're focusing on
- **Use `n/p`** to quickly switch between different files
- **Use `s`** frequently to toggle views as needed

## 🎭 Visual Indicators

### Cursor Position
- **▶** on left side: Currently viewing left-side context
- **Line highlighting**: Current line is visually distinct
- **Scroll position**: Shows which part of the file you're viewing

### Content Status
- **Empty spaces**: Indicate lines that don't exist on one side
- **Color backgrounds**: Show the type of change (insert/delete/modify)
- **Line numbers**: Help maintain orientation within original files
- **Truncation marks**: Show when content is cut off (...)

## 🔧 Advanced Features

### Responsive Behavior
- **Auto-width calculation**: Optimal use of available terminal space
- **Minimum width protection**: Ensures readability on narrow terminals
- **Dynamic truncation**: Adjusts content display based on available width
- **Scroll synchronization**: Both sides move together for consistency

### Integration with Other Modes
- **Merge Mode Compatibility**: Switch from side-by-side to merge seamlessly
- **File Navigation**: Navigate between multiple files while maintaining view preference
- **Help Integration**: Context-sensitive help based on current view mode
- **Error Handling**: Graceful fallback for unsupported terminal sizes

## 🐛 Troubleshooting

### Common Issues

**Issue**: Side-by-side view appears cramped or unreadable
- **Cause**: Terminal window too narrow
- **Solution**: Increase terminal width or use unified view for narrow terminals

**Issue**: Content appears truncated with "..."
- **Cause**: Long lines don't fit in allocated column width
- **Solution**: This is expected behavior; use unified view for full line content

**Issue**: Navigation feels different from unified view
- **Cause**: Side-by-side uses synchronized scrolling for both sides
- **Solution**: This is intended behavior; both sides move together for alignment

**Issue**: Empty spaces appear on one side
- **Cause**: Lines exist on one side but not the other (insertions/deletions)
- **Solution**: This is correct behavior showing the difference structure

### Performance Considerations
- **Large files**: Side-by-side view may be slower for very large files
- **Complex diffs**: Many changes may require more processing time
- **Memory usage**: Slightly higher memory usage due to line reconstruction
- **Terminal refresh**: May be slower on very old or slow terminal emulators

## 🔮 Future Enhancements

Planned improvements for side-by-side view:
- [ ] **Syntax highlighting** within side-by-side panels
- [ ] **Word-level diffing** showing character-by-character changes
- [ ] **Custom column width** adjustment
- [ ] **Line wrapping** options for long lines
- [ ] **Independent scrolling** mode option
- [ ] **Minimap overview** for large files
- [ ] **Split-screen merge** operations
- [ ] **Color customization** for different change types

## 📈 Benefits Summary

### For Developers
- **Enhanced code review** with better context understanding
- **Faster identification** of changes and their impact
- **Improved debugging** of configuration and data file differences
- **Better visualization** of refactoring and structural changes

### For System Administrators
- **Configuration comparison** across environments
- **Quick validation** of deployment changes
- **Easy identification** of environment-specific differences
- **Better understanding** of configuration evolution

### For Content Creators
- **Document comparison** with clear before/after visualization
- **Change tracking** in text files and documentation
- **Version comparison** for content updates
- **Collaborative editing** review and validation

---

**The Side-by-Side View transforms file comparison from sequential analysis to parallel visualization, enabling faster comprehension and more effective change analysis.** 🚀