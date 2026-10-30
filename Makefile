.PHONY: all build build-gui build-release build-all build-windows build-gui-windows build-win-installer clean test run demo help install install-tui install-gui install-windows uninstall

# Variables
BINARY_NAME=filecmp
BUILD_DIR=build
MAIN_FILE=main.go
GUI_DIR=filecmp-gui
GUI_BINARY=filecmp-gui
WAILS ?= wails
# Version stamped into both binaries (override with: make VERSION=v1.2.3)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS = -X main.version=$(VERSION)
# Install location (override with: make install BINDIR=/usr/local/bin)
BINDIR ?= $(shell go env GOPATH)/bin
# Linux launcher entry + icon location (override with: make install DATADIR=...)
DATADIR ?= $(HOME)/.local/share
GUI_ICON = $(DATADIR)/icons/hicolor/scalable/apps/$(GUI_BINARY).svg
HOST_OS := $(shell go env GOOS)
# Executable suffix for the host (".exe" when building natively on Windows)
EXE := $(shell go env GOEXE)

# Windows builds (cross-compiled; no cgo needed)
WIN_TUI = $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe
WIN_GUI = $(BUILD_DIR)/$(GUI_BINARY)-windows-amd64.exe
WIN_INSTALLER = $(BUILD_DIR)/$(GUI_BINARY)-windows-amd64-installer.exe

# Under WSL, "make install" also installs the Windows builds into
# WIN_INSTALL_DIR (default: %LOCALAPPDATA%\Programs\filecmp) and a
# "filecmp-win" wrapper into BINDIR. Disable with: make install WSL=
WSL ?= $(shell grep -qi microsoft /proc/version 2>/dev/null && echo 1)
WIN_INSTALL_DIR ?= $(shell wslpath -u "$$(cd /mnt/c 2>/dev/null; cmd.exe /c 'echo %LOCALAPPDATA%' 2>/dev/null | tr -d '\r')" 2>/dev/null)/Programs/filecmp

# Default target: build both the TUI and the GUI
all: build build-gui

# Build the application
build:
	@echo "🔨 Building $(BINARY_NAME)..."
	@go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME)$(EXE) $(MAIN_FILE)
	@echo "✅ Build complete: ./$(BINARY_NAME)$(EXE)"

# Build the Wails desktop GUI (also installs frontend npm deps)
build-gui:
	@command -v $(WAILS) >/dev/null 2>&1 || { echo "❌ wails CLI not found. Install it with: go install github.com/wailsapp/wails/v2/cmd/wails@latest"; exit 1; }
	@echo "🔨 Building $(GUI_BINARY)..."
	@cd $(GUI_DIR) && $(WAILS) build -clean -ldflags "$(LDFLAGS)"
	@echo "✅ Build complete: ./$(GUI_DIR)/build/bin/$(GUI_BINARY)$(EXE)"

# Cross-compile the TUI for Windows
build-windows:
	@echo "🔨 Building $(WIN_TUI)..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(WIN_TUI) $(MAIN_FILE)
	@echo "✅ Build complete: ./$(WIN_TUI)"

# Cross-compile the Wails GUI for Windows. The exe is copied out of the Wails
# build/bin directory so a later "build-gui -clean" can't delete it.
build-gui-windows:
	@command -v $(WAILS) >/dev/null 2>&1 || { echo "❌ wails CLI not found. Install it with: go install github.com/wailsapp/wails/v2/cmd/wails@latest"; exit 1; }
	@echo "🔨 Building $(WIN_GUI)..."
	@cd $(GUI_DIR) && CGO_ENABLED=0 $(WAILS) build -platform windows/amd64 -ldflags "$(LDFLAGS)"
	@mkdir -p $(BUILD_DIR)
	@cp $(GUI_DIR)/build/bin/$(GUI_BINARY).exe $(WIN_GUI)
	@echo "✅ Build complete: ./$(WIN_GUI)"

# Build with optimizations for release
build-release:
	@echo "🚀 Building $(BINARY_NAME) for release..."
	@go build -ldflags="-s -w $(LDFLAGS)" -o $(BINARY_NAME)$(EXE) $(MAIN_FILE)
	@echo "✅ Release build complete: ./$(BINARY_NAME)$(EXE)"

# Build a per-user Windows installer for the GUI (no admin prompt; installs to
# %LOCALAPPDATA%\Programs\File Compare with Start menu + desktop shortcuts).
# Needs NSIS: sudo apt install nsis. The installer's product name/version come
# from the "info" section of filecmp-gui/wails.json.
build-win-installer:
	@command -v $(WAILS) >/dev/null 2>&1 || { echo "❌ wails CLI not found. Install it with: go install github.com/wailsapp/wails/v2/cmd/wails@latest"; exit 1; }
	@command -v makensis >/dev/null 2>&1 || { echo "❌ makensis not found. Install it with: sudo apt install nsis"; exit 1; }
	@echo "🔨 Building Windows installer..."
	@cd $(GUI_DIR) && CGO_ENABLED=0 $(WAILS) build -platform windows/amd64 -nsis -installscope user -ldflags "$(LDFLAGS)"
	@mkdir -p $(BUILD_DIR)
	@cp $(GUI_DIR)/build/bin/$(GUI_BINARY)-amd64-installer.exe $(WIN_INSTALLER)
	@echo "✅ Installer complete: ./$(WIN_INSTALLER)"

# Build for multiple platforms (TUI for all, plus the Windows GUI)
build-all: build-gui-windows
	@echo "🌍 Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_FILE)
	@GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_FILE)
	@GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_FILE)
	@GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(WIN_TUI) $(MAIN_FILE)
	@echo "✅ Multi-platform builds complete in $(BUILD_DIR)/"
	@ls -la $(BUILD_DIR)/

# Install dependencies
deps:
	@echo "📦 Installing dependencies..."
	@go mod tidy
	@go mod download
	@echo "✅ Dependencies installed"

# Run tests
test:
	@echo "🧪 Running tests..."
	@go test ./...
	@echo "✅ Tests complete"

# Run the application with example files
run-files:
	@echo "🚀 Running with example files..."
	@./$(BINARY_NAME) examples/file1.txt examples/file2.txt

# Run the application with example directories
run-dirs:
	@echo "🚀 Running with example directories..."
	@./$(BINARY_NAME) examples/project-v1 examples/project-v2

# Run the application in interactive mode
run:
	@echo "🚀 Running in interactive mode..."
	@./$(BINARY_NAME)

# Show demo script
demo:
	@echo "🎬 Running demo script..."
	@./test-demo.sh

# Show merge functionality demo
demo-merge:
	@echo "🔀 Running merge functionality demo..."
	@./merge-example.sh

# Demo the new merge feature with simple files
demo-merge-simple:
	@echo "🔀 Running simple merge demo..."
	@./demo-merge.sh

# Demo the all files functionality (common + unique files)
demo-all-files:
	@echo "📁 Running all files demo..."
	@./demo-all-files.sh

# Demo the copy functionality for unique files
demo-copy:
	@echo "📋 Running copy functionality demo..."
	@./demo-copy-files.sh

# Demo the side-by-side view functionality
demo-side-by-side:
	@echo "👥 Running side-by-side view demo..."
	@./demo-side-by-side.sh

# Test responsive design at different terminal widths
test-responsive:
	@echo "📱 Running responsive design test..."
	@./test-responsive.sh

# Test width fix for selected file status
test-width-fix:
	@echo "📏 Testing width fix for selected file status..."
	@./test-width-fix.sh

# Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	@rm -f $(BINARY_NAME) $(BINARY_NAME).exe
	@rm -rf $(BUILD_DIR)
	@rm -rf $(GUI_DIR)/build/bin
	@echo "✅ Clean complete"

# Build and install both the TUI and the GUI to $(BINDIR)
# (under WSL, also the Windows builds plus the filecmp-win wrapper)
install: install-tui install-gui $(if $(WSL),install-windows)
	@echo "✅ Installation complete. Run '$(BINARY_NAME)' (TUI) or '$(GUI_BINARY)' (GUI) from anywhere"
	@$(if $(WSL),echo "   Windows builds: 'filecmp-win' (GUI) or 'filecmp-win --tui' (TUI)")

install-tui: build
	@echo "📲 Installing $(BINARY_NAME) to $(BINDIR)..."
	@install -Dm755 $(BINARY_NAME)$(EXE) $(BINDIR)/$(BINARY_NAME)$(EXE)

install-gui: build-gui
	@echo "📲 Installing $(GUI_BINARY) to $(BINDIR)..."
	@install -Dm755 $(GUI_DIR)/build/bin/$(GUI_BINARY)$(EXE) $(BINDIR)/$(GUI_BINARY)$(EXE)
ifeq ($(HOST_OS),linux)
	@echo "📲 Adding launcher entry to $(DATADIR)/applications..."
	@install -Dm644 $(GUI_DIR)/build/appicon.svg $(GUI_ICON)
	@sed -e "s|@BIN@|$(BINDIR)/$(GUI_BINARY)|" -e "s|@ICON@|$(GUI_ICON)|" \
		$(GUI_DIR)/build/linux/$(GUI_BINARY).desktop > $(GUI_DIR)/build/bin/$(GUI_BINARY).desktop
	@install -Dm644 $(GUI_DIR)/build/bin/$(GUI_BINARY).desktop $(DATADIR)/applications/$(GUI_BINARY).desktop
	@if command -v update-desktop-database >/dev/null 2>&1; then update-desktop-database -q $(DATADIR)/applications; fi
endif

# WSL only: install the Windows TUI + GUI into WIN_INSTALL_DIR and the
# filecmp-win wrapper into BINDIR.
install-windows: build-windows build-gui-windows
	@case "$(WIN_INSTALL_DIR)" in /mnt/*) ;; *) echo "❌ Could not locate %LOCALAPPDATA% (set WIN_INSTALL_DIR=/mnt/c/...)"; exit 1;; esac
	@echo "📲 Installing Windows builds to $(WIN_INSTALL_DIR)..."
	@install -Dm755 $(WIN_TUI) "$(WIN_INSTALL_DIR)/$(BINARY_NAME).exe"
	@install -Dm755 $(WIN_GUI) "$(WIN_INSTALL_DIR)/$(GUI_BINARY).exe"
	@echo "📲 Installing filecmp-win wrapper to $(BINDIR)..."
	@mkdir -p $(BINDIR)
	@sed 's|@WIN_DIR@|$(WIN_INSTALL_DIR)|' scripts/filecmp-win > $(BINDIR)/filecmp-win
	@chmod 755 $(BINDIR)/filecmp-win

# Remove installed binaries
uninstall:
	@rm -f $(BINDIR)/$(BINARY_NAME)$(EXE) $(BINDIR)/$(GUI_BINARY)$(EXE)
	@rm -f $(DATADIR)/applications/$(GUI_BINARY).desktop $(GUI_ICON)
	@echo "🗑️  Removed $(BINARY_NAME) and $(GUI_BINARY) from $(BINDIR)"
	@$(if $(WSL),rm -f $(BINDIR)/filecmp-win "$(WIN_INSTALL_DIR)/$(BINARY_NAME).exe" "$(WIN_INSTALL_DIR)/$(GUI_BINARY).exe" && echo "🗑️  Removed filecmp-win and the Windows builds from $(WIN_INSTALL_DIR)")

# Show help
help:
	@echo "📚 File Comparison TUI Tool - Makefile Help"
	@echo "==========================================="
	@echo ""
	@echo "Available targets:"
	@echo "  all           Build both TUI and GUI (default)"
	@echo "  build         Build the TUI"
	@echo "  build-gui     Build the Wails GUI (needs wails CLI)"
	@echo "  build-release Build optimized release version"
	@echo "  build-all     Build for multiple platforms (+ Windows GUI)"
	@echo "  build-windows Cross-compile the TUI for Windows"
	@echo "  build-gui-windows Cross-compile the GUI for Windows"
	@echo "  build-win-installer Build a per-user Windows installer (needs NSIS)"
	@echo "  deps          Install Go dependencies"
	@echo "  test          Run tests"
	@echo "  run           Run in interactive mode"
	@echo "  run-files     Run with example files"
	@echo "  run-dirs      Run with example directories"
	@echo "  demo          Run demo script"
	@echo "  demo-merge    Run comprehensive merge demo"
	@echo "  demo-merge-simple Run simple merge demo"
	@echo "  demo-all-files Run all files demo (common + unique)"
	@echo "  demo-copy     Run copy functionality demo"
	@echo "  demo-side-by-side Run side-by-side view demo"
	@echo "  test-responsive Test responsive design at different widths"
	@echo "  test-width-fix Test selected file status width fix"
	@echo "  clean         Clean build artifacts"
	@echo "  install       Build and install TUI + GUI to BINDIR"
	@echo "                (default: \$$(go env GOPATH)/bin)"
	@echo "  install-tui   Build and install only the TUI"
	@echo "  install-gui   Build and install only the GUI"
	@echo "  install-windows (WSL) Install Windows TUI + GUI and filecmp-win"
	@echo "                (part of 'install' under WSL; skip with WSL=)"
	@echo "  uninstall     Remove installed binaries from BINDIR"
	@echo "  help          Show this help message"
	@echo ""
	@echo "Examples:"
	@echo "  make build              # Build the application"
	@echo "  make demo               # Show demo with examples"
	@echo "  make demo-merge         # Show merge functionality demo"
	@echo "  make demo-all-files     # Show all files functionality demo"
	@echo "  make demo-copy          # Show copy functionality demo"
	@echo "  make demo-side-by-side  # Show side-by-side view demo"
	@echo "  make test-responsive    # Test UI at different terminal widths"
	@echo "  make test-width-fix     # Test selected file status width fix"
	@echo "  make run-files          # Quick test with sample files"
	@echo "  make install            # Build + install TUI and GUI"
	@echo "  make install BINDIR=~/.local/bin"
	@echo ""
	@echo "Quick Start:"
	@echo "  1. make deps            # Install dependencies"
	@echo "  2. make build           # Build the application"
	@echo "  3. make demo            # See usage examples"
	@echo "  4. make run-files       # Try it out!"
