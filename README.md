# File Comparison TUI Tool

A powerful terminal-based user interface (TUI) application for visually comparing files and directories with beautiful color-coded diff highlighting. Built with Go using Bubble Tea framework.

> **Prefer a GUI?** A native desktop version of this tool, built with [Wails](https://wails.io), lives in [`filecmp-gui/`](filecmp-gui/README.md). It's a mouse-driven rebuild of everything below (file/directory/git comparison, side-by-side diff, interactive merge, directory copy) sharing the same diff/merge/git engine as this TUI.

## ✨ Features

- 🎨 **Visual Diff Highlighting**: Blue backgrounds for additions, red backgrounds for deletions
- 👥 **Side-by-Side View**: Compare files in unified or side-by-side layout modes
- 🔀 **Interactive Merge Mode**: Cherry-pick and apply specific changes between files
- 📋 **File Copy Mode**: Easily copy unique files between directories with selective control
- 📁 **Complete Directory Analysis**: Shows ALL files from both directories (common and unique)
- 🏷️ **Source Identification**: Clear indicators for files that exist in only one directory
- 🌱 **Git Mode**: Compare a ref against the working tree, or two refs against each other
- 🔎 **File List Filtering**: Narrow the file list by typing `/` followed by a substring
- ⌨️ **Path Autocomplete**: Live filesystem suggestions as you type a path
- ⌨️ **Intuitive Controls**: Vim-like navigation (j/k) with full arrow key support
- 🔍 **Intelligent File Detection**: Automatically identifies 60+ text file types for comparison
- 📊 **Real-time Diff Statistics**: Live counts of equal, added, and deleted lines
- 🖥️ **Full Screen TUI**: Clean, distraction-free interface with proper scrolling
- 🚀 **Fast Performance**: Efficient diff algorithm with semantic cleanup
- 📱 **Responsive Design**: Adapts to terminal window size changes
- 🎯 **Multi-file Navigation**: Easy switching between multiple file comparisons
- 💾 **Selective Merging**: Save merged results with only the changes you want
- 🔄 **Directory Synchronization**: Copy unique files between directories for easy sync

## Installation

```bash
# Clone the repository
git clone <repository-url>
cd golang-fileCmp

# Build the application
go build -o filecmp

# Or run directly
go run main.go
```

## Usage

### Command Line Options

```bash
# Start with interactive file selection
./filecmp

# Compare two files directly
./filecmp file1.txt file2.txt

# Compare two directories (finds common files automatically)
./filecmp ./project-v1 ./project-v2

# Load left file, enter right path in TUI
./filecmp /path/to/file1.txt

# Compare HEAD against the working tree (must be run inside a git repo)
./filecmp --git

# Compare a ref against the working tree (includes staged changes)
./filecmp --git HEAD~1

# Compare two refs against each other
./filecmp --git HEAD~3 HEAD

# Show comprehensive help
./filecmp --help
```

### Quick Start with Make

```bash
# Install dependencies and build
make deps && make build

# Try the demo with example files
make demo

# Demo the merge functionality
make demo-merge

# Quick test with sample files
make run-files

# Quick test with sample directories
make run-dirs
```

### Interactive Controls

#### File Selection Mode
- **Tab**: Switch between left/right input fields and the View Diff / Quit buttons, or cycle through path suggestions when they're showing
- **Enter**: Accept a highlighted path suggestion, load the focused input's path, or activate the focused button
- **↑/↓**: Navigate suggestions (when showing) or the list of all files (common and unique)
- **/**: Filter the file list by name (type to narrow, Backspace to edit, Esc to clear)
- **Ctrl+D**: Start comparing the selected files
- **?**: Show help screen
- **q/Ctrl+C**: Quit application

Path autocomplete kicks in automatically as you type in either input field, suggesting matching files/directories from the filesystem (press Tab or ↑/↓ to cycle, Enter to accept).

#### Diff View Mode (Unified & Side-by-Side)
- **↑/↓** or **j/k**: Navigate through diff lines
- **h/l** or **←/→**: Scroll left/right (side-by-side view only)
- **s**: Switch view mode (Unified ↔ Side-by-Side)
- **g**: Go to top of diff
- **G**: Go to bottom of diff
- **n**: Next file
- **p**: Previous file
- **m**: Enter merge mode (only available for files that exist on both sides)
- **c**: Enter copy mode (only available when the loaded directories have unique files)
- **Esc**: Return to file selection
- **?**: Show help screen
- **q/Ctrl+C**: Quit application

#### Merge Mode
- **↑/↓** or **j/k**: Navigate through diff lines
- **Space/Enter**: Toggle selection of current change
- **t**: Switch merge target (left/right file)
- **a**: Select all changes
- **n**: Select no changes
- **s**: Save merged result to a new `<file>.merged` file (the original is never overwritten)
- **Esc**: Return to diff view
- **?**: Show help screen
- **q/Ctrl+C**: Quit application

#### Copy Mode (Directory Comparison Only)
- **↑/↓** or **j/k**: Navigate through unique files
- **Space/Enter**: Toggle selection of current file to copy
- **t**: Switch copy target (to-left ↔ to-right)
- **a**: Select all unique files
- **n**: Select no files
- **s**: Copy selected files to target directory
- **Esc**: Return to diff view
- **?**: Show help screen
- **q/Ctrl+C**: Quit application

> Copy mode is entered with **c** from the diff view (not from file selection).

## Color Legend

### Diff Colors
- **Blue background**: Added lines (+)
- **Red background**: Deleted lines (-)
- **Gray text**: Unchanged lines
- **Yellow background**: Selected changes (merge mode)
- **Strikethrough text**: Unselected changes (merge mode)

### File Status Indicators
- **✓ Green checkmark**: Identical files (same content in both directories)
- **✗ Red X**: Different files (content differs between directories)
- **◄ Blue arrow**: File exists only in LEFT directory **[LEFT ONLY]**
- **► Orange arrow**: File exists only in RIGHT directory **[RIGHT ONLY]**

## 📄 Supported File Types

The tool intelligently detects and compares 60+ text file types:

### Programming Languages
`.go` `.js` `.ts` `.py` `.java` `.c` `.cpp` `.h` `.hpp` `.rs` `.swift` `.kt` `.cs` `.vb` `.fs` `.php` `.rb` `.scala` `.clj` `.hs` `.elm` `.pl` `.pm` `.r` `.R` `.m`

### Web & Data Files
`.html` `.htm` `.css` `.xml` `.json` `.yaml` `.yml` `.toml` `.csv`

### Documentation & Text
`.txt` `.md`

### Configuration & Scripts
`.sql` `.ini` `.cfg` `.conf` `.config` `.properties` `.env` `.sh` `.bash` `.zsh` `.fish` `.ps1` `.bat` `.cmd`

### Build & Project Files
`.dockerfile` `.makefile` `.cmake` `.ninja` `.gradle` `.pom` `.gitignore` `.gitattributes` `.editorconfig`

### Special Files (no extension, matched case-insensitively)
`README` `LICENSE` `CHANGELOG` `AUTHORS` `CONTRIBUTORS` `Makefile` `Dockerfile` `Gemfile` `Rakefile` `Procfile`

> Note: hidden files (names starting with `.`) are skipped during directory scans, so a bare `.gitignore` won't show up when comparing directories even though its extension is on the supported list.

## How It Works

1. **Load Paths**: Enter file or directory paths in the input fields (with live autocomplete), or use `--git` to diff against a git ref/working tree
2. **Analyze All Files**: For directories, the tool finds ALL text files from both locations
3. **Categorize Files**: Files are marked as common (both sides), left-only, or right-only
4. **Select File**: Choose any file to compare using the arrow keys, optionally filtering the list with `/`
5. **View Diff**: See the comparison with color-coded changes
6. **Navigate**: Move through all files (common and unique) seamlessly
7. **Merge Changes**: Press 'm' to enter merge mode (only for common files)
8. **Save Results**: Choose which changes to keep and save the merged file

## Examples

### Comparing Two Files
```bash
./filecmp config.old.yaml config.new.yaml
```

### Comparing Project Directories
```bash
./filecmp ./project-v1 ./project-v2
```
This will find all files from both directories and allow you to compare them, merge changes, and copy unique files.

### Interactive Mode
```bash
./filecmp
```
Then enter paths interactively using the TUI.

### Comparing Against Git
```bash
# HEAD vs working tree (includes staged and unstaged changes)
./filecmp --git

# A specific commit vs working tree
./filecmp --git HEAD~1

# Two commits/refs against each other
./filecmp --git main feature-branch
```

### Copy Unique Files Between Directories
```bash
# Compare directories, then from the diff view press 'c' to enter copy mode
./filecmp old-project/ new-project/
# Select files to copy and press 's' to copy them
```

## 🏗️ Architecture

### Core Components
- **TUI Layer**: Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) for responsive terminal interface
- **Styling**: [Lip Gloss](https://github.com/charmbracelet/lipgloss) for beautiful colors and layouts
- **Diff Engine**: LCS-based line-level diff for accurate comparisons
- **File System**: Smart file detection and recursive directory traversal
- **Git Integration**: Shells out to `git` to diff refs/working tree for `--git` mode

### Project Structure
```
internal/
├── ui/         # TUI state, key handling (model.go) and rendering (views.go)
├── differ/     # Diff computation engine
├── merge/      # Merge functionality and change selection
├── file/       # File operations and type detection
├── git/        # Git ref / working-tree comparison support (--git mode)
└── ...

filecmp-gui/    # Wails desktop GUI (separate Go module) — see filecmp-gui/README.md
examples/       # Sample files for testing
```

## 🎛️ Advanced Usage

### Keyboard Shortcuts Summary
| Key | File Selection | Diff View | Side-by-Side | Merge Mode | Copy Mode | Description |
|-----|----------------|-----------|--------------|------------|-----------|-------------|
| `Tab` | ✅ | ❌ | ❌ | ❌ | ❌ | Switch input fields / cycle suggestions |
| `Enter` | ✅ | ❌ | ❌ | ✅ | ✅ | Load entered path / Toggle change / Toggle file |
| `↑/↓` | ✅ | ✅ | ✅ | ✅ | ✅ | Navigate lists/lines |
| `j/k` | ❌ | ✅ | ✅ | ✅ | ✅ | Vim-style navigation |
| `h/l` or `←/→` | ❌ | ❌ | ✅ | ❌ | ❌ | Scroll left/right |
| `g/G` | ❌ | ✅ | ✅ | ❌ | ❌ | Jump to top/bottom |
| `/` | ✅ | ❌ | ❌ | ❌ | ❌ | Filter file list |
| `s` | ❌ | ✅ | ✅ | ✅ | ✅ | Switch view mode / Save result / Copy files |
| `n/p` | ❌ | ✅ | ✅ | ❌ | ❌ | Next/previous file |
| `m` | ❌ | ✅ | ✅ | ❌ | ❌ | Enter merge mode |
| `c` | ❌ | ✅ | ✅ | ❌ | ❌ | Enter copy mode |
| `t` | ❌ | ❌ | ❌ | ✅ | ✅ | Switch merge/copy target |
| `a` | ❌ | ❌ | ❌ | ✅ | ✅ | Select all changes/files |
| `Space` | ❌ | ❌ | ❌ | ✅ | ✅ | Toggle current change/file |
| `Ctrl+D` | ✅ | ❌ | ❌ | ❌ | ❌ | Start comparison |
| `Esc` | ❌ | ✅ | ✅ | ✅ | ✅ | Return to previous view |
| `?` | ✅ | ✅ | ✅ | ✅ | ✅ | Show help screen |
| `q`/`Ctrl+C` | ✅ | ✅ | ✅ | ✅ | ✅ | Quit application |

### Performance Tips
- Large files (>10MB) may take a moment to process
- Directory comparisons are optimized to only load text files
- The diff algorithm uses semantic cleanup for better readability

## License

MIT License - see [LICENSE](LICENSE) file for details.

## 🧪 Testing

Run the included test suite:
```bash
# Run Go unit tests
make test

# Try with examples
make demo
```

## 🚀 Building from Source

```bash
# Clone and build
git clone <repository-url>
cd golang-fileCmp
make deps
make build

# Or build for multiple platforms
make build-all

# Install system-wide (optional)
make install
```

## 🤝 Contributing

Contributions are welcome! Areas for improvement:
- Additional file type support
- Syntax highlighting within diffs
- Advanced merge conflict resolution
- Copy operation undo/rollback
- Export diff results
- Configuration file support
- Undo/redo for merge operations
- Directory structure visualization

Please feel free to submit issues, feature requests, or pull requests.

## 📋 Roadmap

- [x] Interactive merge mode with selective change application
- [x] Complete directory analysis (all files, not just common ones)
- [x] File source identification with clear indicators
- [x] Copy mode for easily copying unique files between directories
- [x] Side-by-side comparison view with unified/split toggle
- [x] File filtering by name
- [x] Path autocomplete
- [x] Integration with Git for ref/working-tree diffs
- [x] Native desktop GUI (Wails) — see [`filecmp-gui/`](filecmp-gui/README.md)
- [ ] Syntax highlighting for code diffs
- [ ] Three-way merge support
- [ ] Merge conflict resolution
- [ ] Export diffs to HTML/PDF
- [ ] Configuration file support
- [ ] Plugin system for custom file types
- [ ] Undo/redo functionality in merge mode
- [ ] Directory tree visualization
