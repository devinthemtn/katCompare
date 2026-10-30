.PHONY: all build build-gui clean test run demo help install install-tui install-gui uninstall

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

# Default target: build both the TUI and the GUI
all: build build-gui

# Build the application
build:
	@echo "🔨 Building $(BINARY_NAME)..."
	@go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) $(MAIN_FILE)
	@echo "✅ Build complete: ./$(BINARY_NAME)"

# Build the Wails desktop GUI (also installs frontend npm deps)
build-gui:
	@command -v $(WAILS) >/dev/null 2>&1 || { echo "❌ wails CLI not found. Install it with: go install github.com/wailsapp/wails/v2/cmd/wails@latest"; exit 1; }
	@echo "🔨 Building $(GUI_BINARY)..."
	@cd $(GUI_DIR) && $(WAILS) build -clean -ldflags "$(LDFLAGS)"
	@echo "✅ Build complete: ./$(GUI_DIR)/build/bin/$(GUI_BINARY)"

# Build with optimizations for release
build-release:
	@echo "🚀 Building $(BINARY_NAME) for release..."
	@go build -ldflags="-s -w $(LDFLAGS)" -o $(BINARY_NAME) $(MAIN_FILE)
	@echo "✅ Release build complete: ./$(BINARY_NAME)"

# Build for multiple platforms
build-all:
	@echo "🌍 Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_FILE)
	@GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_FILE)
	@GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_FILE)
	@GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_FILE)
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

# Run the test diff program
test-diff:
	@echo "🧪 Running diff engine test..."
	@go run test-diff.go

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
	@rm -f $(BINARY_NAME)
	@rm -rf $(BUILD_DIR)
	@rm -rf $(GUI_DIR)/build/bin
	@echo "✅ Clean complete"

# Build and install both the TUI and the GUI to $(BINDIR)
install: install-tui install-gui
	@echo "✅ Installation complete. Run '$(BINARY_NAME)' (TUI) or '$(GUI_BINARY)' (GUI) from anywhere"

install-tui: build
	@echo "📲 Installing $(BINARY_NAME) to $(BINDIR)..."
	@install -Dm755 $(BINARY_NAME) $(BINDIR)/$(BINARY_NAME)

install-gui: build-gui
	@echo "📲 Installing $(GUI_BINARY) to $(BINDIR)..."
	@install -Dm755 $(GUI_DIR)/build/bin/$(GUI_BINARY) $(BINDIR)/$(GUI_BINARY)

# Remove installed binaries
uninstall:
	@rm -f $(BINDIR)/$(BINARY_NAME) $(BINDIR)/$(GUI_BINARY)
	@echo "🗑️  Removed $(BINARY_NAME) and $(GUI_BINARY) from $(BINDIR)"

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
	@echo "  build-all     Build for multiple platforms"
	@echo "  deps          Install Go dependencies"
	@echo "  test          Run tests"
	@echo "  test-diff     Run diff engine test"
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
