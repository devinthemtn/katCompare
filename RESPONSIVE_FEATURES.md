# Responsive TUI Features

This file comparison tool automatically adapts to different terminal sizes and provides an optimal user experience across various screen dimensions.

## Key Responsive Features

### 1. Dynamic Layout Adaptation

The application automatically detects your terminal size and adjusts all UI components accordingly:

- **Title and headers**: Expand to full width with proper padding
- **Input fields**: Scale with terminal width while maintaining minimum usable size
- **File lists**: Adapt height to show maximum files without scrolling off screen
- **Diff views**: Optimize line display for current terminal dimensions

### 2. Intelligent Text Handling

#### Path Truncation
- Long file paths are intelligently truncated with "..." prefix
- Maintains readability while fitting available space
- Both input fields and status displays adapt

#### Content Wrapping
- Diff content automatically truncates long lines
- Line numbers and diff markers remain visible
- Content width adjusts to terminal size

### 3. Scrollable File Lists

When comparing directories with many files:
- Automatically calculates how many files can be displayed
- Shows scroll indicators (e.g., "Showing 1-5 of 21 files")
- Navigation keeps selected file visible
- Sorted alphabetically for consistent ordering

### 4. Adaptive Help Text

Help messages change based on available width:
- **Wide terminals (>80 chars)**: Full descriptive text
- **Medium terminals (60-80 chars)**: Abbreviated but clear
- **Narrow terminals (<60 chars)**: Compact symbols and shortcuts

Examples:
```
Wide:    "↑↓: Navigate suggestions • Tab: Next suggestion • Enter: Accept • Esc: Cancel"
Medium:  "↑↓: Navigate • Tab: Next • Enter: Accept • Esc: Cancel"  
Narrow:  "↑↓:Nav Tab:Next Enter:Accept Esc:Cancel"
```

### 5. Smart Suggestion Display

Path suggestions adapt to terminal size:
- Limits number of suggestions based on available height
- Truncates long paths with "..." prefix
- Shows "and X more" indicator when suggestions exceed display limit
- Maintains minimum of 3 suggestions even in very small terminals

### 6. File Status Indicators

File comparison results scale appropriately:
- Color-coded status indicators (✓ for identical, ✗ for different)
- File sizes in human-readable format (B, KB, MB, GB)
- Intelligent size display:
  - Same size: `filename.txt (1.2KB)`
  - Different sizes: `filename.txt (L:1.2KB R:856B)`

### 7. Diff View Optimizations

The diff comparison view adapts to terminal dimensions:
- Line numbers and content scale to available width
- Scroll indicators adjust to terminal size
- Navigation help text adapts to available space
- File names in header truncate intelligently

## Minimum Terminal Requirements

- **Minimum width**: 40 characters (gracefully handles smaller)
- **Minimum height**: 10 lines (essential UI elements remain functional)
- **Recommended**: 80x24 or larger for optimal experience

## Testing Different Sizes

You can test the responsive behavior by:

1. Resizing your terminal window while the app is running
2. Starting with different terminal dimensions
3. Using many files to test scrolling behavior
4. Testing with very long file paths

## Technical Implementation

The responsive features are implemented through:

- Dynamic width/height calculations based on `tea.WindowSizeMsg`
- Adaptive styling with `lipgloss` width/height constraints  
- Smart truncation algorithms for text content
- Scroll offset calculations for large file lists
- Conditional text rendering based on available space

The application maintains usability across all terminal sizes while providing an optimal experience on standard terminal dimensions.