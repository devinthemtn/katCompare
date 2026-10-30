# File Compare

**Compare files, directories, and git refs — then merge just the changes you want. Also I wrote it to be easier for red/green colour blind people to use because I'm tired of all diff tools using at least by default red/green.**

File Compare is a fast, color-coded diff and merge tool written in Go that is clean looking and simple to use. It ships two ways from one engine: a **native desktop app** (Wails) and a **keyboard-driven terminal UI** (Bubble Tea).

![File Compare desktop GUI comparing a config file between HEAD and origin/main in unified view](assets/snip-gui-file-cmp.png)

<sub>The desktop GUI in Git mode, diffing `HEAD` against `origin/main` — changed files on the left, a color-coded unified diff on the right, with Side-by-side and Merge views one click away.</sub>

## Why File Compare?

- 🎨 **Diffs you can read at a glance** — blue for additions, red for deletions, with old/new line numbers and live `equal / +added / -deleted` counts
- 🌱 **Git-aware** — diff any ref against another ref, or against your working tree, without leaving the app
- 📁 **Whole-directory comparison** — recursively finds every text file on both sides and flags identical (✓), different (✗), and one-side-only files
- 🔀 **Cherry-pick merging** — choose individual insertions/deletions to apply and save to `<file>.merged`; originals are never overwritten
- 📋 **Directory sync** — copy files that exist on only one side into the other, in either direction
- 👥 **Unified or side-by-side** views, switchable instantly
- ✏️ **Hand off to your own tools** — open a file in your editor at the line you're looking at, send the pair to your external diff tool (meld, kdiff3, VS Code, nvimdiff…), or resolve git merge conflicts with `git mergetool`
- 🔎 **Filter and autocomplete** — narrow the file list by name, and get live path suggestions as you type
- 🖱️⌨️ **Your choice of interface** — point-and-click desktop app or vim-style (`j`/`k`) terminal UI, same results either way
- 🔍 **60+ text file types** recognized by name, plus any other file that looks like text (no binary content)

## Quick Start

### Build and install both in one step

From the repository root (needs the GUI prerequisites below):

```bash
make install                          # builds TUI + GUI, installs to $(go env GOPATH)/bin
make install BINDIR=~/.local/bin      # or pick another directory on your PATH
```

This installs `filecmp` (TUI) and `filecmp-gui` (GUI). Use `make install-tui` / `make install-gui` for just one, and `make uninstall` to remove them.

**Under WSL**, `make install` also cross-compiles the Windows builds, installs `filecmp.exe` and `filecmp-gui.exe` to `%LOCALAPPDATA%\Programs\filecmp`, and adds a `filecmp-win` wrapper to `BINDIR`:

```bash
filecmp-win ./a ./b          # Windows GUI in a native window
filecmp-win --tui ./a ./b    # Windows TUI in this terminal
```

The wrapper converts path arguments to Windows paths. Use `make install WSL=` to skip the Windows builds, or `WIN_INSTALL_DIR=/mnt/c/...` to install them somewhere else. `make build-windows` / `make build-gui-windows` just build them (output goes to `build/`).

### Desktop GUI

Requires Go 1.25+, Node.js, and the [Wails v2 CLI](https://wails.io):

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
cd filecmp-gui
wails build            # Linux: needs GTK/WebKit -dev packages (see below)
./build/bin/filecmp-gui
./build/bin/filecmp-gui old.yaml new.yaml   # pre-fill both paths and compare right away
```

Pick the **Files / Directories** tab to compare two paths (type them, use the native file/folder picker, or pass them on the command line), or the **Git** tab to compare refs — leave the right ref blank to compare against the working tree.

### Terminal UI

```bash
go build -o filecmp
./filecmp                         # interactive path entry
./filecmp old.yaml new.yaml       # two files
./filecmp ./project-v1 ./project-v2   # two directories
./filecmp --git HEAD~1            # a git ref vs the working tree
```

---

## Desktop GUI (Wails)

The desktop app lives in [`filecmp-gui/`](filecmp-gui/). It imports the same diff/merge/file/git/external packages from the repository's `internal/` directory as the TUI, behind a mouse-driven interface built with [Wails](https://wails.io) — a Go backend with a vanilla HTML/CSS/JS frontend, no framework. Both share a single Go module; the TUI binary never imports Wails, so it doesn't pull in any GUI dependencies.

### GUI Features

- Compare two files or two directories (recursive, text-file-aware); the file list shows common files and files unique to either side, with identical/different indicators
- **Git mode**: diff a ref against another ref, or a ref against the working tree
- **Unified** and **side-by-side** diff views, color-coded like the TUI (blue = added, red = deleted)
- **Merge view**: select which insertions/deletions to apply per line, then save to `<file>.merged` — the originals are never overwritten
- **Copy**: copy files that exist on only one side into the other directory, in either direction
- Path autocomplete and native file/folder picker dialogs
- File-list filtering
- **External tools**: **Open left/right** (or **Open working copy** in git mode) opens the file in your editor; click any line number to open it at that line. **External diff** opens the pair in your diff tool, **Folders in diff tool** opens two compared directories, and **Resolve conflict** runs `git mergetool` on conflicted files (marked ⚠). Shortcuts: `e` = open, `d` = external diff. See [Editor & external diff tools](#editor--external-diff-tools)

### Building the GUI

Prerequisites: Go 1.25+, Node.js (for the Vite-built frontend), and the Wails v2 CLI:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

**Linux only:** Wails also needs GTK/WebKit *development* headers to compile against (having the runtime libraries installed isn't enough — `pkg-config` needs the `-dev` packages, or the Go build fails with a cgo/`pkg-config` error). On Debian/Ubuntu:

```bash
sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev
```

Run `wails doctor` to check what's missing on your system.

Then, from the repo root:

```bash
cd filecmp-gui
wails dev      # live development with hot reload
wails build    # production binary in build/bin/
```
then you can just copy the bin from the 'build/bin' directory into your personal bin directory.

#### Linux: WebKitGTK version

Wails v2 links against WebKitGTK 4.0 unless built with the `webkit2_41` tag. Most current distros (e.g. Ubuntu 24.04+) only ship WebKitGTK 4.1, so this project sets that tag by default in `wails.json` (`"build:tags": "webkit2_41"`) — plain `wails dev` / `wails build` just work. The tag has no effect on macOS or Windows builds.

If your system only has WebKitGTK 4.0 (older distros), remove the `build:tags` line from `wails.json`, or the Go build fails with a `pkg-config` error for `webkit2gtk-4.1`. Check what you have with `pkg-config --exists webkit2gtk-4.0 && echo 4.0` / `pkg-config --exists webkit2gtk-4.1 && echo 4.1`.

#### Building for Windows

**On Windows**, with Go and the Wails CLI installed, just run `wails build` in `filecmp-gui/` — no extra setup needed. It uses WebView2 (present by default on current Windows 10/11).

**Cross-compiling from Linux/WSL** works too. Install the mingw-w64 cross-compiler (and NSIS if you want an installer):

```bash
sudo apt install gcc-mingw-w64-x86-64 nsis
```

Then build with `CC` pointed at the cross-compiler:

```bash
# Just the .exe
CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc wails build -platform windows/amd64

# .exe + NSIS installer
CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc wails build -platform windows/amd64 -nsis
```

Output goes to `filecmp-gui/build/bin/`:
- `filecmp-gui.exe` — the app
- `filecmp-gui-amd64-installer.exe` — NSIS installer (only with `-nsis`)

**Installer for Windows users:** `make build-win-installer` (needs `sudo apt install nsis`) creates `build/filecmp-gui-windows-amd64-installer.exe`. Double-clicking it installs File Compare for the current user (no admin prompt) into `%LOCALAPPDATA%\Programs\File Compare`. It adds Start menu and desktop shortcuts and an uninstall entry in *Apps & features*. The installer is unsigned, so Windows SmartScreen asks users to click *More info → Run anyway*. The product name and version shown come from the `info` section of `filecmp-gui/wails.json`. Bump `productVersion` there when you release.

By default the exe downloads WebView2 on first run if it's missing. To bundle it instead (fully offline install), add `-webview2 embed`.

#### Building for macOS

Wails does not support cross-compiling to macOS from Linux or Windows (it needs Apple's Cocoa/WebKit frameworks and `clang` with the macOS SDK). Build directly on a Mac with `wails build`.

### GUI project structure

```
filecmp-gui/
├── app.go              # Wails-bound backend: LoadComparison, LoadGitComparison,
│                        # GetDiff, SaveMerge, CopyFiles, SuggestPaths, dialogs
├── main.go              # wails.Run(...) entrypoint
│                        # (logic packages are imported from the root internal/)
└── frontend/
    ├── index.html
    └── src/
        ├── main.js      # view state machine, rendering, all UI logic
        └── style.css
```

### GUI notes

- Copy mode is unavailable when comparing git refs, since the compared "roots" are synthetic (commit contents / working tree), not real directories to copy into.

---

## Terminal UI

The TUI is a full-screen, keyboard-driven interface built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss). It adapts to terminal resizes and supports vim-style navigation.

### Installation

```bash
git clone <repository-url>
cd fileCmp-golang-wails
go build -o filecmp     # or: make deps && make build
```

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
- **e**: Edit the right file (the working tree file in git mode) at the cursor line
- **E**: Edit the left file at the cursor line
- **D**: Open the file pair in your external diff tool
- **R**: Resolve a git merge conflict with `git mergetool` (conflicted files show ⚠ **[CONFLICT]**)
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

## Editor & external diff tools

Both the GUI and the TUI can hand the current file off to other apps.

**Editor.** The first one found is used:
1. `$FILECMP_EDITOR`
2. `$VISUAL`
3. `$EDITOR`
4. the OS default app (`xdg-open` / `open` / `start`)

The value can include arguments, for example `FILECMP_EDITOR="code --wait"`. The GUI has no terminal, so it skips terminal-only editors (`nvim`, `emacsclient -t`, `nano`, …) and moves on to the next choice. The TUI prefers terminal editors: if `$VISUAL` is a GUI editor (e.g. `emacsclient -c`) and `$EDITOR` runs in a terminal (e.g. `emacsclient -t`), the TUI uses `$EDITOR`. `$FILECMP_EDITOR` always wins. The TUI suspends itself while a terminal editor runs. Jumping to a line works for vi/vim/nvim, emacs/emacsclient, nano, micro, VS Code, Zed, Sublime and Helix.

**Diff and merge tools** come from your git config, so set them once:

```bash
git config --global diff.tool meld       # or kdiff3, vscode, nvimdiff, bc, ...
git config --global merge.tool meld
git config --global diff.guitool meld    # optional: GUI-only choice (e.g. diff.tool = nvimdiff for the TUI)
```

The **TUI prefers terminal tools** (vimdiff, nvimdiff, or a custom `difftool.<name>.cmd` that runs a terminal program such as `emacsclient -nw`). It checks `diff.tool`, then `diff.guitool`, then any installed nvim/vim. It only uses a GUI tool when it can't find a terminal one. The **GUI** checks `diff.guitool`, then `diff.tool`, and always skips terminal tools. Merge tools follow the same rules with `merge.tool` / `merge.guitool`.

- **Git mode** runs `git difftool <ref> [<ref>] -- <file>`.
- **Plain files and folders** don't need to be in a git repository. Files are passed to `git difftool --no-index`, which works anywhere. If git isn't installed or no tool is configured, the app launches the tool directly: a `difftool.<name>.cmd` you've defined, or the first of meld / kdiff3 / VS Code it finds (nvim/vim `-d` too, but only in the TUI). A file that exists on only one side is compared against an empty file.
- **Whole folders** open directly in tools that support directories (meld, kdiff3, Beyond Compare).
- **Merge conflicts**: in git mode (ref vs working tree), files with unresolved conflicts during a merge or rebase are marked ⚠. **Resolve conflict** / `R` runs `git mergetool -- <file>`, then reloads the list.

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
- **⚠ Yellow warning**: File has unresolved git merge conflicts **[CONFLICT]**

## 📄 Supported File Types

Files with any of the 60+ extensions or names below are always treated as text. Any other file is still compared if its content looks like text — the same heuristic git uses: it's non-empty and has no NUL bytes in the first 8000 bytes. So `.lock`, `.tf`, `.vue`, extensionless scripts, etc. work fine, while images, archives, and compiled binaries are skipped.

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
└── external/   # Launching editors and external diff/merge tools

filecmp-gui/    # Wails desktop GUI (same Go module, imports internal/) — see "Desktop GUI (Wails)" above
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
| `e`/`E` | ❌ | ✅ | ✅ | ❌ | ❌ | Edit right/left file in your editor |
| `D` | ❌ | ✅ | ✅ | ❌ | ❌ | Open in external diff tool |
| `R` | ❌ | ✅ | ✅ | ❌ | ❌ | Resolve git conflict with mergetool |
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
cd fileCmp-golang-wails
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
- [x] Native desktop GUI (Wails) — see [Desktop GUI (Wails)](#desktop-gui-wails)
- [ ] Syntax highlighting for code diffs
- [ ] Three-way merge support
- [ ] Merge conflict resolution
- [ ] Export diffs to HTML/PDF
- [ ] Configuration file support
- [ ] Plugin system for custom file types
- [ ] Undo/redo functionality in merge mode
- [ ] Directory tree visualization
