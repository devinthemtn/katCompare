// Package config reads katCompare's settings file, shared by the GUI and the
// TUI, and creates it the first time either runs.
//
// The file belongs to the user. It's written once, when there's nothing at its
// path, and never changed after that. New settings take their default until
// the user adds them.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"katcompare/internal/differ"
	"katcompare/internal/file"

	"github.com/BurntSushi/toml"
)

// Config holds the settings.
type Config struct {
	Compare Compare `toml:"compare"`
	View    View    `toml:"view"`
	Tools   Tools   `toml:"tools"`
	GUI     GUI     `toml:"gui"`
}

// Compare holds what a comparison looks at.
type Compare struct {
	Ignore            []string `toml:"ignore"`
	ShowHidden        bool     `toml:"show_hidden"`
	HideIdentical     bool     `toml:"hide_identical"`
	IgnoreLineEndings bool     `toml:"ignore_line_endings"`
	IgnoreWhitespace  string   `toml:"ignore_whitespace"`
}

// View holds how things start out.
type View struct {
	DefaultView        string `toml:"default_view"`
	DefaultMergeTarget string `toml:"default_merge_target"`
}

// Tools holds external tools. Diff and merge tools are in git config, because
// git difftool and git mergetool read them from there.
type Tools struct {
	Editor string `toml:"editor"`
}

// GUI holds the desktop app's settings.
type GUI struct {
	// AutoCollapsePaths collapses the paths bar after the user runs Compare.
	AutoCollapsePaths bool    `toml:"auto_collapse_paths"`
	DiffFontSize      float64 `toml:"diff_font_size"`
}

// Defaults returns the settings used when the file doesn't set them.
func Defaults() Config {
	return Config{
		Compare: Compare{IgnoreWhitespace: differ.WhitespaceNone},
		View:    View{DefaultView: ViewUnified, DefaultMergeTarget: "left"},
		GUI:     GUI{DiffFontSize: 12.5},
	}
}

// The values of default_view.
const (
	ViewUnified    = "unified"
	ViewSideBySide = "sidebyside"
)

// The limits of diff_font_size.
const (
	minFontSize = 8
	maxFontSize = 32
)

// validate says what's wrong with settings TOML itself can't check.
func (c Config) validate() error {
	switch c.Compare.IgnoreWhitespace {
	case differ.WhitespaceNone, differ.WhitespaceTrailing, differ.WhitespaceAll:
	default:
		return fmt.Errorf("ignore_whitespace must be \"none\", \"trailing\" or \"all\", not %q", c.Compare.IgnoreWhitespace)
	}
	if v := c.View.DefaultView; v != ViewUnified && v != ViewSideBySide {
		return fmt.Errorf("default_view must be \"unified\" or \"sidebyside\", not %q", v)
	}
	if t := c.View.DefaultMergeTarget; t != "left" && t != "right" {
		return fmt.Errorf("default_merge_target must be \"left\" or \"right\", not %q", t)
	}
	if s := c.GUI.DiffFontSize; s < minFontSize || s > maxFontSize {
		return fmt.Errorf("diff_font_size must be from %d to %d, not %v", minFontSize, maxFontSize, s)
	}
	for _, pattern := range c.Compare.Ignore {
		trimmed := strings.Trim(pattern, "/")
		if trimmed == "" {
			return fmt.Errorf("ignore has an empty pattern (%q)", pattern)
		}
		if _, err := path.Match(trimmed, ""); err != nil {
			return fmt.Errorf("ignore pattern %q isn't valid: %w", pattern, err)
		}
	}
	return nil
}

// FileOptions returns what to leave out when loading folders.
func (c Config) FileOptions() file.Options {
	return file.Options{Ignore: c.Compare.Ignore, ShowHidden: c.Compare.ShowHidden}
}

// DifferOptions returns what a comparison ignores.
func (c Config) DifferOptions() differ.Options {
	return differ.Options{IgnoreLineEndings: c.Compare.IgnoreLineEndings, IgnoreWhitespace: c.Compare.IgnoreWhitespace}
}

// template is the file created on first run. Its values must match Defaults.
const template = `# katCompare settings.
#
# This file is yours: katCompare creates it once and never changes it after that.
# Delete it to get a fresh copy the next time the app starts.
#
# Diff and merge tools are chosen in git config, not here. See the app's help.

[compare]
# Files and folders to skip when comparing folders. A name or wildcard matches
# at any depth; one containing / matches from the compared folder, and ending in
# / matches only folders. This is simple matching, not full .gitignore syntax.
# ignore = ["node_modules", "build", "dist", "*.log", "docs/generated/"]
ignore = []

# Include dotfiles like .github. (.git is always skipped.)
show_hidden = false

# When comparing folders, list only the files that differ.
hide_identical = false

# Treat CRLF and LF line endings as the same.
ignore_line_endings = false

# Ignore whitespace: "none", "trailing" (at line ends) or "all" (anywhere).
ignore_whitespace = "none"

[view]
# The view to start in: "unified" or "sidebyside".
default_view = "unified"

# The side a merge saves to: "left" or "right".
default_merge_target = "left"

[tools]
# Your editor, ahead of $KATCMP_EDITOR, $VISUAL and $EDITOR. Empty means use
# those. Put double quotes around a path with spaces, and on Windows wrap the
# whole value in single quotes so backslashes work:
# editor = '"C:\Program Files\Notepad++\notepad++.exe" -n'
editor = ""

[gui]
# Collapse the paths bar after you run Compare.
auto_collapse_paths = false

# The size of the diff text, from 8 to 32.
diff_font_size = 12.5
`

// Path returns where the settings file lives: under the user's config
// directory, which depends on the OS (%AppData% on Windows, ~/.config on Linux).
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("can't find the config directory: %w", err)
	}
	return filepath.Join(dir, "katcompare", "config.toml"), nil
}

// Install creates the settings file if there's nothing at its path, and reports
// whether it did. Anything already there is left alone, whatever it holds.
func Install() (bool, error) {
	path, err := Path()
	if err != nil {
		return false, err
	}
	return installAt(path)
}

func installAt(path string) (bool, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return false, err
	}
	// O_EXCL makes "create only if missing" one step, so this can't overwrite a
	// file even if another copy of the app creates it at the same moment. A
	// check first and a write after would leave a gap for that.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if errors.Is(err, fs.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, err = f.WriteString(template)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(path) //nolint:errcheck // Don't leave half a file to be kept forever.
		return false, err
	}
	return true, nil
}

// Load reads the settings. A missing file gives the defaults. A file that can't
// be used also gives the defaults, with an error to show the user, and is left
// as it is so that they can fix it.
func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Defaults(), err
	}
	return loadFrom(path)
}

func loadFrom(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Defaults(), nil
	}
	if err != nil {
		return Defaults(), err
	}
	cfg := Defaults()
	// Settings it doesn't know are ignored, so a file written by a newer version
	// still works in an older one.
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Defaults(), err
	}
	if err := cfg.validate(); err != nil {
		return Defaults(), err
	}
	return cfg, nil
}
