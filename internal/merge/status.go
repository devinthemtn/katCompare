package merge

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// How a save's checks came out.
const (
	StateSafe  = "safe"
	StateCheck = "check"
)

// What kind of view made a save.
const (
	KindMerge     = "merge"
	KindConflicts = "conflicts"
)

// How a file on disk compares with what a comparison loaded.
const (
	DiskUnchanged = "unchanged"
	DiskChanged   = "changed"
	DiskMissing   = "missing"
)

// Record is one save made this session.
type Record struct {
	Report  *SaveReport
	Kind    string // KindMerge | KindConflicts
	Target  string // file name, set by Tracker.Record
	Applied int    // merge saves only
	Skipped int    // merge saves only
}

// State returns StateCheck if the save's checks found anything.
func (r *Record) State() string {
	if len(r.Report.Issues) > 0 {
		return StateCheck
	}
	return StateSafe
}

// Tracker remembers the saves made this session, for as long as each file
// still matches its save. It's safe for concurrent use.
type Tracker struct {
	mu      sync.Mutex
	records map[string]*Record // by the target's absolute path
}

// NewTracker returns an empty Tracker.
func NewTracker() *Tracker {
	return &Tracker{records: make(map[string]*Record)}
}

func statusKey(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

// Record notes a save to path.
func (t *Tracker) Record(path string, rec *Record) {
	rec.Target = filepath.Base(path)
	t.mu.Lock()
	defer t.mu.Unlock()
	t.records[statusKey(path)] = rec
}

// valid returns the record for key if the file still matches that save, and
// forgets the record if it doesn't. The caller holds mu.
func (t *Tracker) valid(key string) *Record {
	rec, ok := t.records[key]
	if !ok {
		return nil
	}
	data, err := os.ReadFile(key)
	if err != nil || HashContent(data) != rec.Report.Hash {
		delete(t.records, key)
		return nil
	}
	return rec
}

// StatusOf returns the record of the given paths, or nil if none has one. If
// several do, one that needs checking takes priority. Empty paths are skipped.
func (t *Tracker) StatusOf(paths ...string) *Record {
	t.mu.Lock()
	defer t.mu.Unlock()
	var status *Record
	for _, path := range paths {
		if path == "" {
			continue
		}
		rec := t.valid(statusKey(path))
		if rec != nil && (status == nil || rec.State() == StateCheck) {
			status = rec
		}
	}
	return status
}

// Backups returns the absolute paths of the backups of valid records.
func (t *Tracker) Backups() map[string]bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	backups := make(map[string]bool)
	for key := range t.records {
		if rec := t.valid(key); rec != nil {
			backups[statusKey(rec.Report.Backup)] = true
		}
	}
	return backups
}

// AllBackups reports whether paths isn't empty and every one is in backups. An
// empty path, meaning a side that isn't a file on disk, doesn't count as one.
func AllBackups(paths []string, backups map[string]bool) bool {
	for _, path := range paths {
		if path == "" || !backups[statusKey(path)] {
			return false
		}
	}
	return len(paths) > 0
}

// SavedUnder returns the slash-separated paths, relative to root, of the valid
// records inside root.
func (t *Tracker) SavedUnder(root string) []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	var saved []string
	for key := range t.records {
		if t.valid(key) == nil {
			continue
		}
		rel, err := filepath.Rel(root, key)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		saved = append(saved, filepath.ToSlash(rel))
	}
	return saved
}

// DiskState compares the file at path with the content a comparison loaded
// and returns DiskUnchanged, DiskChanged or DiskMissing.
func DiskState(path, loaded string) (string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return DiskMissing, nil
	}
	if err != nil {
		return "", err
	}
	if string(data) != loaded {
		return DiskChanged, nil
	}
	return DiskUnchanged, nil
}
