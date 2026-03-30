package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	stateFile    = "forgectl-state.json"
	stateBackup  = "forgectl-state.json.bak"
	stateTmp     = "forgectl-state.json.tmp"
	stateCorrupt = "forgectl-state.json.corrupt"
)

// Exists returns true if a state file exists in dir.
func Exists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, stateFile))
	return err == nil
}

// Recover performs startup recovery checks per the spec.
// It must be called before Load on every command.
func Recover(dir string) error {
	jsonPath := filepath.Join(dir, stateFile)
	bakPath := filepath.Join(dir, stateBackup)
	tmpPath := filepath.Join(dir, stateTmp)

	jsonExists := fileExists(jsonPath)
	bakExists := fileExists(bakPath)
	tmpExists := fileExists(tmpPath)

	switch {
	case jsonExists && !tmpExists:
		// Normal. Check for corruption.
		data, err := os.ReadFile(jsonPath)
		if err != nil {
			return fmt.Errorf("reading state file: %w", err)
		}
		var js json.RawMessage
		if json.Unmarshal(data, &js) != nil {
			// Corrupt JSON.
			if bakExists {
				corruptPath := filepath.Join(dir, stateCorrupt)
				if err := os.Rename(jsonPath, corruptPath); err != nil {
					return fmt.Errorf("moving corrupt state: %w", err)
				}
				if err := os.Rename(bakPath, jsonPath); err != nil {
					return fmt.Errorf("restoring backup: %w", err)
				}
				fmt.Fprintf(os.Stderr, "Warning: state file was corrupt. Restored from backup.\n")
			} else {
				return fmt.Errorf("state file is corrupt and no backup exists")
			}
		}

	case !jsonExists && bakExists && !tmpExists:
		// Crashed between step 2 and 3.
		if err := os.Rename(bakPath, jsonPath); err != nil {
			return fmt.Errorf("restoring backup: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Warning: recovered state from backup.\n")

	case !jsonExists && !bakExists && tmpExists:
		// Crashed between step 1 and 2.
		if err := os.Rename(tmpPath, jsonPath); err != nil {
			return fmt.Errorf("recovering from tmp: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Warning: recovered state from temporary file.\n")

	case jsonExists && tmpExists:
		// Crashed after step 1, before cleanup.
		if err := os.Remove(tmpPath); err != nil {
			return fmt.Errorf("removing stale tmp: %w", err)
		}
	}

	return nil
}

// Load reads the state file from dir. Calls Recover first.
func Load(dir string) (*ForgeState, error) {
	if err := Recover(dir); err != nil {
		return nil, err
	}

	path := filepath.Join(dir, stateFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no state file found. Run 'forgectl init' first")
		}
		return nil, fmt.Errorf("reading state file: %w", err)
	}

	var s ForgeState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing state file: %w", err)
	}

	return &s, nil
}

// Save writes the state file atomically using tmp→backup→rename.
func Save(dir string, s *ForgeState) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}

	jsonPath := filepath.Join(dir, stateFile)
	tmpPath := filepath.Join(dir, stateTmp)
	bakPath := filepath.Join(dir, stateBackup)

	// Step 1: Write to tmp.
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("writing tmp state: %w", err)
	}

	// Step 2: Rename existing → backup (if exists).
	if fileExists(jsonPath) {
		if err := os.Rename(jsonPath, bakPath); err != nil {
			return fmt.Errorf("creating backup: %w", err)
		}
	}

	// Step 3: Rename tmp → state.
	if err := os.Rename(tmpPath, jsonPath); err != nil {
		return fmt.Errorf("renaming state: %w", err)
	}

	return nil
}

// ArchiveSession copies the active state file to <stateDir>/sessions/<domain>-<date>.json.
// This is called at terminal state (DONE in implementing, or PHASE_SHIFT after COMPLETE in specifying).
func ArchiveSession(stateDir string, domain string, s *ForgeState) error {
	sessionsDir := filepath.Join(stateDir, "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return fmt.Errorf("creating sessions dir: %w", err)
	}

	date := time.Now().Format("2006-01-02")
	archiveName := domain + "-" + date + ".json"
	archivePath := filepath.Join(sessionsDir, archiveName)

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state for archive: %w", err)
	}

	if err := os.WriteFile(archivePath, data, 0644); err != nil {
		return fmt.Errorf("writing archive: %w", err)
	}

	return nil
}

// StateDir returns the absolute path to the state directory.
// If cfg.Paths.StateDir is absolute, it is returned as-is.
// Otherwise, it is joined with projectRoot.
func StateDir(projectRoot string, cfg Config) string {
	if filepath.IsAbs(cfg.Paths.StateDir) {
		return cfg.Paths.StateDir
	}
	return filepath.Join(projectRoot, cfg.Paths.StateDir)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// NewSpecifyingState creates initial specifying state from queue entries.
func NewSpecifyingState(specs []SpecQueueEntry) *SpecifyingState {
	queue := make([]SpecQueueEntry, len(specs))
	copy(queue, specs)
	return &SpecifyingState{
		Queue:     queue,
		Completed: []CompletedSpec{},
	}
}

// NewPlanningState creates initial planning state from queue entries.
func NewPlanningState(plans []PlanQueueEntry) *PlanningState {
	queue := make([]PlanQueueEntry, len(plans))
	copy(queue, plans)
	return &PlanningState{
		Queue:     queue,
		Completed: []interface{}{},
	}
}

// NewImplementingState creates initial implementing state.
func NewImplementingState() *ImplementingState {
	return &ImplementingState{
		LayerHistory: []LayerHistory{},
	}
}
