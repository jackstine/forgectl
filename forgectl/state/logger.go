package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// LogEntry is a single structured log record written to a JSONL file.
type LogEntry struct {
	TS        string                 `json:"ts"`
	Cmd       string                 `json:"cmd"`
	Phase     string                 `json:"phase"`
	PrevState string                 `json:"prev_state,omitempty"`
	State     string                 `json:"state"`
	Detail    map[string]interface{} `json:"detail"`
}

// Logger writes log entries to a JSONL file. It is always valid; when
// disabled (or when initialisation failed), Write is a no-op.
type Logger struct {
	enabled bool
	path    string // absolute path to log file
}

// NewLogger creates a Logger for the given session. The log file lives at
// ~/.forgectl/logs/<phase>-<session_id_prefix>.jsonl where the prefix is
// the first 8 characters of the UUID. Returns a no-op logger when logging
// is disabled or the home directory cannot be resolved.
func NewLogger(cfg LogsConfig, phase PhaseName, sessionID string) *Logger {
	if !cfg.Enabled || sessionID == "" {
		return &Logger{enabled: false}
	}
	prefix := sessionID
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	filename := fmt.Sprintf("%s-%s.jsonl", phase, prefix)
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: activity logging: cannot get home dir: %v\n", err)
		return &Logger{enabled: false}
	}
	logDir := filepath.Join(home, ".forgectl", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "warning: activity logging: cannot create log dir: %v\n", err)
		return &Logger{enabled: false}
	}
	return &Logger{enabled: true, path: filepath.Join(logDir, filename)}
}

// Write appends a log entry to the JSONL file. Any failure is printed to
// stderr; the write never causes the calling command to fail.
func (l *Logger) Write(entry LogEntry) {
	if !l.enabled {
		return
	}
	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: activity logging: marshal: %v\n", err)
		return
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: activity logging: open %s: %v\n", l.path, err)
		return
	}
	defer f.Close()
	f.Write(append(data, '\n'))
}

// Enabled reports whether the logger will actually write entries.
func (l *Logger) Enabled() bool {
	return l.enabled
}

// Path returns the absolute log file path (empty when disabled).
func (l *Logger) Path() string {
	return l.path
}

// PruneLogs deletes old log files from ~/.forgectl/logs/ before creating a
// new session. Age-based deletion runs first, then count-based deletion.
func PruneLogs(cfg LogsConfig) {
	if !cfg.Enabled {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	logDir := filepath.Join(home, ".forgectl", "logs")
	files, err := filepath.Glob(filepath.Join(logDir, "*.jsonl"))
	if err != nil || len(files) == 0 {
		return
	}
	// Sort by modification time (oldest first).
	sort.Slice(files, func(i, j int) bool {
		si, _ := os.Stat(files[i])
		sj, _ := os.Stat(files[j])
		if si == nil || sj == nil {
			return false
		}
		return si.ModTime().Before(sj.ModTime())
	})
	now := time.Now()
	// Age-based deletion.
	if cfg.RetentionDays > 0 {
		cutoff := now.AddDate(0, 0, -cfg.RetentionDays)
		var remaining []string
		for _, f := range files {
			si, _ := os.Stat(f)
			if si != nil && si.ModTime().Before(cutoff) {
				os.Remove(f)
			} else {
				remaining = append(remaining, f)
			}
		}
		files = remaining
	}
	// Count-based deletion: keep at most max_files files.
	if cfg.MaxFiles > 0 {
		for len(files) > cfg.MaxFiles {
			os.Remove(files[0])
			files = files[1:]
		}
	}
}

// LogNow returns the current time in ISO 8601 UTC format for log entries.
func LogNow() string {
	return time.Now().UTC().Format(time.RFC3339)
}
