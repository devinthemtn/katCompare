# Changelog

All notable changes to katCompare (`katcmp`, `katcmp-gui`, `katcmp-win`).
Versions follow [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [2.0.0] - 2026-11-21

### Added
- GUI: `q` and `Ctrl+Q` quit, and ask first when there are unsaved merge picks
  or conflict choices.
- GUI: `/` focuses the file filter (`Esc` clears it), `1`–`4` switch views and
  `r` reloads the comparison from disk.
- `katCmp` alias installed alongside `katcmp`, `katcmp-gui` and `katcmp-win`.
- `task release` and `task release-publish` for building and publishing
  releases, with safety checks: the build must match the tag on `HEAD`, the
  working tree must be clean and `main` must be checked out.
- `task test-race` runs the tests with the race detector.

### Changed
- Built with Go 1.26.11 (pinned with a `toolchain` line in `go.mod`), which
  picks up the standard library security fixes since 1.26.8.
- Published TUI binaries are stripped and built with `-trimpath`.
- `.orig` merge backups and old `filecmp` build leftovers are ignored by git.

### Removed
- Old demo and test shell scripts in the repo root, and the `demo*`,
  `test-responsive` and `test-width-fix` tasks that ran them.
- Unused `merge.SaveWithBackup` (replaced by `SaveVerified`).

## [1.4.0] - 2026-11-08

### Added
- GUI focus mode, and a config file with settings, including fonts.
- GUI font zoom keys.
- TUI two-pane layout, matching the GUI.
- TUI Git button and `F2` key to open the git picker.
- `--wait` for `katcmp-win`, for use as a git diff tool.
- GUI shows merged, safe and saved status, and the Merge toolbar says what
  saving will do.

### Changed
- Faster, lighter comparisons.
- Merge and Conflicts toolbars stay visible while scrolling.
- Unticked lines are easier to read in the GUI's Merge view.
- Binary files are left out of git mode, as in a folder comparison.

### Fixed
- CRLF line endings are kept when saving a merge.
- Merge and copy bugs.

## [1.3.0] - 2026-10-31

### Added
- Resolve git conflicts in the app and save merges over the target file.
- Next/previous change navigation in the GUI and TUI.
- Compare is highlighted when the inputs don't match what's displayed.
- Windows git can use repos inside WSL.

### Changed
- Renamed to katCompare (`katcmp`, `katcmp-gui`, `katcmp-win`), with a new
  KC app icon.
- GUI upgraded to Wails v3.
- Makefile replaced with a go-task Taskfile.
- GUI file dialogs open in the current directory.

### Fixed
- Child processes no longer open console windows in the Windows GUI.

## [1.2.0] - 2026-10-30

### Added
- Git dropdown picker.
- Per-user Windows installer (`task build-win-installer`).
- `task install` under WSL also installs the Windows builds.

### Changed
- Less duplicated code between the TUI and GUI.

## [1.1.0] - 2026-10-29

### Added
- `--version` flag, and the version shown in TUI and GUI help.

## [1.0.0] - 2026-10-29

First release: a TUI and a GUI for comparing files and folders, with external
editor and diff tool support. Unknown file types are treated as text when
they look like text.

[Unreleased]: https://github.com/devinthemtn/katCompare/compare/v2.0.0...HEAD
[2.0.0]: https://github.com/devinthemtn/katCompare/compare/v1.4.0...v2.0.0
[1.4.0]: https://github.com/devinthemtn/katCompare/compare/v1.3.0...v1.4.0
[1.3.0]: https://github.com/devinthemtn/katCompare/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/devinthemtn/katCompare/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/devinthemtn/katCompare/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/devinthemtn/katCompare/releases/tag/v1.0.0
