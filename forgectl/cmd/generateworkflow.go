package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"forgectl/state"
)

// Batch is an ordered, non-empty group of plan items the generator compiles
// into the emitted workflow script. Batches are an interface commitment of the
// generated artifact, not a field on plan.json — the generator derives them
// deterministically from the plan's layers and each item's depends_on.
type Batch struct {
	Index int              // 1-based position in run order across all layers
	Items []state.PlanItem // full item records, in run order
}

// computeBatches turns a validated plan and a batch size into an ordered list
// of batches: layers are processed in declared order, items within a layer are
// topologically sorted by depends_on (stable, declared order as the tiebreak),
// the ordered items are chunked into groups of at most batchSize, and batches
// are numbered sequentially 1-based across all layers (the count does not reset
// at a layer boundary).
//
// The plan is assumed to have passed validation, which guarantees full layer
// coverage, an acyclic depends_on graph, and no forward-layer dependencies —
// so batching never has to reject its input.
func computeBatches(plan state.PlanJSON, batchSize int) []Batch {
	if batchSize < 1 {
		batchSize = 1
	}
	byID := make(map[string]state.PlanItem, len(plan.Items))
	for _, it := range plan.Items {
		byID[it.ID] = it
	}

	var batches []Batch
	index := 0
	for _, layer := range plan.Layers {
		ordered := topoSortLayer(layer.Items, byID)
		for start := 0; start < len(ordered); start += batchSize {
			end := min(start+batchSize, len(ordered))
			index++
			items := make([]state.PlanItem, 0, end-start)
			for _, id := range ordered[start:end] {
				items = append(items, byID[id])
			}
			batches = append(batches, Batch{Index: index, Items: items})
		}
	}
	return batches
}

// topoSortLayer returns a layer's item ids in a stable topological order: an
// item is emitted only after every depends_on that also belongs to this layer
// has been emitted, and among ready items the earliest in declared order wins.
// Dependencies in earlier layers are already satisfied (earlier layers were
// fully emitted) and are ignored here. Validation guarantees no cycles and no
// forward-layer dependencies, so every item is eventually emitted; the
// no-progress fallback exists only to keep a malformed input from looping.
func topoSortLayer(ids []string, byID map[string]state.PlanItem) []string {
	inLayer := make(map[string]bool, len(ids))
	for _, id := range ids {
		inLayer[id] = true
	}
	emitted := make(map[string]bool, len(ids))
	result := make([]string, 0, len(ids))

	for len(result) < len(ids) {
		progressed := false
		for _, id := range ids {
			if emitted[id] {
				continue
			}
			ready := true
			for _, dep := range byID[id].DependsOn {
				if inLayer[dep] && !emitted[dep] {
					ready = false
					break
				}
			}
			if ready {
				emitted[id] = true
				result = append(result, id)
				progressed = true
				break // restart the scan so declared order stays the tiebreak
			}
		}
		if !progressed {
			// Unreachable for a validated plan (cycle or dangling dep). Emit the
			// remaining items in declared order rather than spin forever.
			for _, id := range ids {
				if !emitted[id] {
					emitted[id] = true
					result = append(result, id)
				}
			}
		}
	}
	return result
}

// nonSlashCommandChars matches any run of characters outside the slash-command
// charset; dashRuns matches runs of hyphens to collapse.
var (
	nonSlashCommandChars = regexp.MustCompile(`[^a-z0-9-]+`)
	dashRuns             = regexp.MustCompile(`-+`)
)

// sanitizeSegment reduces a string to the slash-command charset: lowercased,
// every character outside [a-z0-9-] replaced with '-', runs of '-' collapsed to
// one, and leading/trailing '-' trimmed.
func sanitizeSegment(s string) string {
	s = strings.ToLower(s)
	s = nonSlashCommandChars.ReplaceAllString(s, "-")
	s = dashRuns.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// workflowBaseName is the slash-command name (no extension) derived from the
// plan context: "<domain>-<module>-impl", each segment sanitized. A final
// collapse+trim guards against empty segments producing stray leading hyphens.
func workflowBaseName(ctx state.PlanContext) string {
	base := sanitizeSegment(ctx.Domain) + "-" + sanitizeSegment(ctx.Module) + "-impl"
	base = dashRuns.ReplaceAllString(base, "-")
	return strings.Trim(base, "-")
}

// deriveOutputName returns the derived workflow filename
// "<domain>-<module>-impl.js", sanitized to the slash-command charset.
func deriveOutputName(ctx state.PlanContext) string {
	return workflowBaseName(ctx) + ".js"
}

// resolveWorkflowPath returns the path under workflowsDir to write the workflow
// to. It never overwrites: if a file named intendedName already exists, it
// prepends a numeric disambiguating prefix to form a fresh, non-colliding name,
// writes a WARN naming both the intended and chosen filenames to warnOut, and
// returns the fresh path. When there is no collision it returns the intended
// path and logs nothing.
func resolveWorkflowPath(workflowsDir, intendedName string, warnOut io.Writer) (string, error) {
	exists, err := fileExists(filepath.Join(workflowsDir, intendedName))
	if err != nil {
		return "", err
	}
	if !exists {
		return filepath.Join(workflowsDir, intendedName), nil
	}
	for i := 1; ; i++ {
		chosenName := fmt.Sprintf("%d-%s", i, intendedName)
		chosenPath := filepath.Join(workflowsDir, chosenName)
		exists, err := fileExists(chosenPath)
		if err != nil {
			return "", err
		}
		if !exists {
			fmt.Fprintf(warnOut, "WARN: output name %q already exists under .claude/workflows/; writing to %q instead\n", intendedName, chosenName)
			return chosenPath, nil
		}
	}
}

// fileExists reports whether path exists, distinguishing a genuine stat error
// (e.g. a permission problem) from simple absence.
func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
