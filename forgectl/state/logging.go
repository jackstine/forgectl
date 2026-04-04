package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// LogEntry is one line in the JSONL activity log.
type LogEntry struct {
	Ts        string         `json:"ts"`
	Cmd       string         `json:"cmd"`
	Phase     string         `json:"phase"`
	PrevState string         `json:"prev_state,omitempty"`
	State     string         `json:"state"`
	Detail    map[string]any `json:"detail,omitempty"`
}

// LogDir returns the path to the forgectl log directory (~/.forgectl/logs/).
func LogDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".forgectl", "logs")
	}
	return filepath.Join(home, ".forgectl", "logs")
}

// LogFileName returns "<phase>-<prefix>.jsonl" where prefix is the session ID
// up to (but not including) the first '-'.
func LogFileName(phase, sessionID string) string {
	prefix := sessionID
	if idx := strings.Index(sessionID, "-"); idx >= 0 {
		prefix = sessionID[:idx]
	}
	return phase + "-" + prefix + ".jsonl"
}

// WriteLogEntry opens the log file in append mode and writes entry as a JSON line.
// Best-effort: silently ignores all errors.
func WriteLogEntry(sessionID, startPhase string, entry LogEntry) {
	if sessionID == "" {
		return
	}

	dir := LogDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}

	fname := filepath.Join(dir, LogFileName(string(startPhase), sessionID))
	f, err := os.OpenFile(fname, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	line, err := json.Marshal(entry)
	if err != nil {
		return
	}
	line = append(line, '\n')
	_, _ = f.Write(line)
}

// PruneLogFiles removes .jsonl files from dir that are older than retentionDays days,
// then removes the oldest files until at most maxFiles remain.
// Best-effort: silently ignores all errors.
func PruneLogFiles(dir string, retentionDays int, maxFiles int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	type fileInfo struct {
		path    string
		modTime time.Time
	}

	var files []fileInfo
	var cutoff time.Time
	if retentionDays > 0 {
		cutoff = time.Now().AddDate(0, 0, -retentionDays)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		path := filepath.Join(dir, e.Name())
		if !cutoff.IsZero() && info.ModTime().Before(cutoff) {
			_ = os.Remove(path)
			continue
		}
		files = append(files, fileInfo{path: path, modTime: info.ModTime()})
	}

	// Sort newest first.
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.After(files[j].modTime)
	})

	// Remove oldest files beyond maxFiles limit.
	if maxFiles > 0 && len(files) > maxFiles {
		for _, f := range files[maxFiles:] {
			_ = os.Remove(f.path)
		}
	}
}

// NowTS returns the current UTC time formatted as RFC3339.
func NowTS() string {
	return time.Now().UTC().Format(time.RFC3339)
}
