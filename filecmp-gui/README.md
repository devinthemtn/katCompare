# File Compare — Desktop GUI

A native desktop rebuild of the [File Comparison TUI Tool](../README.md) using [Wails](https://wails.io) (Go backend + vanilla HTML/CSS/JS frontend, no framework). It reuses the TUI's diff/merge/file/git logic verbatim (copied into `internal/`, unchanged) behind a mouse-driven UI, so behavior matches the terminal version exactly.

This is a separate Go module from the repository root — it does not affect or depend on the TUI build.

## Features

- Compare two files or two directories (recursive, text-file-aware); the file list shows common files, and files unique to either side, with identical/different indicators
- **Git mode**: diff a ref against another ref, or a ref against the working tree
- Unified and side-by-side diff views, color-coded like the TUI (blue = added, red = deleted)
- **Merge mode**: select which insertions/deletions to apply per line, then save to `<file>.merged` — the originals are never overwritten
- **Copy mode**: copy files that exist on only one side into the other directory, in either direction
- Path autocomplete and native file/folder picker dialogs
- File-list filtering

## Running

Prerequisites: Go 1.25+, Node.js (for the Vite-built frontend), and the [Wails v2 CLI](https://wails.io/docs/gettingstarted/installation) (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`).

```bash
cd filecmp-gui
wails dev      # live development with hot reload
wails build    # production binary in build/bin/
```

### Linux: WebKitGTK version

Wails v2 links against WebKitGTK 4.0 by default. Distros that only ship WebKitGTK 4.1 (e.g. Ubuntu 24.04+) need the `webkit2_41` build tag, or the Go build fails with a `pkg-config` error for `webkit2gtk-4.0`:

```bash
wails dev -tags webkit2_41
wails build -tags webkit2_41
```

Check what you have with `pkg-config --exists webkit2gtk-4.0 && echo 4.0` / `pkg-config --exists webkit2gtk-4.1 && echo 4.1`; only pass the tag if you have 4.1 but not 4.0.

### Building for Windows

**On Windows**, with Go and the Wails CLI installed, just run `wails build` in this directory — no extra setup needed. It uses WebView2 (present by default on current Windows 10/11).

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

Output goes to `build/bin/`:
- `filecmp-gui.exe` — the app
- `filecmp-gui-amd64-installer.exe` — NSIS installer (only with `-nsis`)

By default the exe downloads WebView2 on first run if it's missing. To bundle it instead (fully offline install), add `-webview2 embed`.

## Project structure

```
app.go              # Wails-bound backend: LoadComparison, LoadGitComparison,
                     # GetDiff, SaveMerge, CopyFiles, SuggestPaths, dialogs
main.go              # wails.Run(...) entrypoint
internal/
├── differ/          # diff engine (copied from the TUI, unmodified)
├── file/            # file/directory loading & type detection
├── merge/           # merge/change-selection logic
└── git/             # git ref / working-tree diff support
frontend/
├── index.html
└── src/
    ├── main.js      # view state machine, rendering, all UI logic
    └── style.css
```

## Notes

- Copy mode is unavailable when comparing git refs, since the compared "roots" are synthetic (commit contents / working tree), not real directories to copy into.
- `internal/{differ,file,merge,git}` here are plain copies of the same-named packages at the repository root, not imports — Go's `internal/` visibility rules don't allow cross-module imports, and this module is intentionally independent of the TUI's module.
