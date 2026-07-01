package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"forgectl/evaluators"
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

// jsString encodes s as a JavaScript string literal. JSON string encoding is a
// subset of JS string syntax, so this is a safe, single-line embedding for
// arbitrary prompt text (newlines become \n) — which keeps the emitted script's
// scaffolding on distinct lines from the baked content and lets a string-strip
// pass cleanly separate code from data. HTML escaping is disabled so '<', '>',
// and '&' (common in <domain> placeholders and spec anchors) stay readable.
func jsString(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimRight(buf.String(), "\n")
}

// buildBatchContext renders every item in the batch — id, name, description,
// steps, files, specs, and acceptance tests — as the shared context block baked
// into both the primary and the evaluator prompts (the whole-batch invariant).
func buildBatchContext(b Batch) string {
	var sb strings.Builder
	for i, it := range b.Items {
		if i > 0 {
			sb.WriteString("\n")
		}
		fmt.Fprintf(&sb, "### Item %s — %s\n", it.ID, it.Name)
		fmt.Fprintf(&sb, "Description:\n%s\n", it.Description)
		writeBullets(&sb, "Steps", it.Steps)
		writeBullets(&sb, "Files", it.Files)
		writeBullets(&sb, "Specs", it.Specs)
		sb.WriteString("Acceptance criteria (tests):\n")
		for _, t := range it.Tests {
			fmt.Fprintf(&sb, "- [%s] %s\n", t.Category, t.Description)
		}
	}
	return sb.String()
}

// writeBullets writes a labeled bullet list, noting when a section is empty so
// the agent sees the field was intentionally blank rather than dropped.
func writeBullets(sb *strings.Builder, label string, items []string) {
	fmt.Fprintf(sb, "%s:\n", label)
	if len(items) == 0 {
		sb.WriteString("- (none specified)\n")
		return
	}
	for _, it := range items {
		fmt.Fprintf(sb, "- %s\n", it)
	}
}

// buildPrimaryPrompt is the once-per-batch implementation instruction. It
// presents the whole batch and asks for a complete implementation of every
// item. A commit sentence is included only when commits are enabled.
func buildPrimaryPrompt(b Batch, enableCommits bool) string {
	var sb strings.Builder
	sb.WriteString("You are the primary implementation agent in an automated implementation gauntlet. ")
	sb.WriteString("Implement every work item in this batch completely and correctly — no placeholders, no stubs. ")
	sb.WriteString("Each item lists acceptance criteria under \"Acceptance criteria (tests)\"; your implementation must satisfy all of them.\n\n")
	sb.WriteString("# Batch items\n\n")
	sb.WriteString(buildBatchContext(b))
	if enableCommits {
		sb.WriteString("\nWhen every item is implemented and its tests pass, commit your work (git add -A && git commit).\n")
	}
	return sb.String()
}

// buildEvaluatorPrompt bakes the embedded adversarial-review instructions
// followed by the whole batch. The evaluator mutates code directly; it emits no
// verdict. A commit sentence is included only when commits are enabled.
func buildEvaluatorPrompt(b Batch, enableCommits bool) string {
	var sb strings.Builder
	sb.WriteString(evaluators.GauntletEval)
	sb.WriteString("\n\n# Batch under review\n\n")
	sb.WriteString(buildBatchContext(b))
	if enableCommits {
		sb.WriteString("\nIf your corrections leave the batch complete and correct, commit the changes (git add -A && git commit).\n")
	}
	return sb.String()
}

// buildBaselinePrompt is the once-per-batch snapshot step that runs after the
// primary and before the evaluator loop. Staging the primary's output as the
// baseline lets each round's change-detector measure only that round's evaluator
// edits — without it, round one would always look "changed" because of the
// primary's work. It never commits, so it is safe to emit regardless of the
// commit flag.
func buildBaselinePrompt() string {
	return "You establish the change-detection baseline for this batch. The workflow harness cannot run git itself. " +
		"Run `git add -A` so the current working tree — the primary agent's output — becomes the reference snapshot. " +
		"Later rounds compare against this snapshot to measure only the evaluator's per-round edits. " +
		"Reply with the single token STAGED. If git cannot be run at all, reply with the single token GIT_ERROR."
}

// buildDetectorPrompt is the per-round change-detector instruction. It is a
// lightweight Bash-capable agent (the harness cannot run git) and is the sole
// authority on convergence: its git result — not any agent's self-report —
// decides whether the round changed the code. When commits are enabled it also
// commits an accepted (changed) round.
func buildDetectorPrompt(enableCommits bool) string {
	var sb strings.Builder
	sb.WriteString("You are the change-detection agent and the sole authority on whether the evaluator changed the code this round. ")
	sb.WriteString("The workflow harness cannot run git itself, so you must run it.\n\n")
	sb.WriteString("Steps:\n")
	sb.WriteString("1. Run `git diff` to compare the working tree against the staged baseline snapshot from the previous step or round.\n")
	sb.WriteString("2. If git cannot be run at all (not a repository, git missing, or the command errors), reply with the single token GIT_ERROR.\n")
	sb.WriteString("3. If `git diff` shows any change, the evaluator modified the code this round — reply with the single token CHANGED.\n")
	sb.WriteString("4. If `git diff` shows no change, reply with the single token CLEAN.\n")
	if enableCommits {
		sb.WriteString("5. If the result is CHANGED, stage and commit the accepted state (git add -A && git commit) — this also refreshes the baseline. If the result is CLEAN, do nothing.\n")
	} else {
		sb.WriteString("5. Then run `git add -A` to refresh the baseline snapshot for the next round.\n")
	}
	sb.WriteString("\nReport only the single token (CHANGED, CLEAN, or GIT_ERROR) as the final line of your reply.")
	return sb.String()
}

// renderWorkflowScript produces the self-contained Claude Code workflow script
// from the plan context, the computed batches, and the config values (all baked
// as constants — the script re-reads nothing at run time). The emitted body
// conforms to the adversarial-evaluation-gauntlet contract: one primary pass per
// batch, then a bounded evaluator loop whose convergence is decided by a
// git-running change-detector.
func renderWorkflowScript(ctx state.PlanContext, batches []Batch, cfg state.ForgeConfig) string {
	name := workflowBaseName(ctx)
	im := cfg.Implementing.Implement.Model
	it := cfg.Implementing.Implement.Type
	em := cfg.Implementing.Eval.Model
	et := cfg.Implementing.Eval.Type
	minR := cfg.Implementing.Eval.MinRounds
	maxR := cfg.Implementing.Eval.MaxRounds
	commits := cfg.General.EnableCommits

	var sb strings.Builder

	// meta — a pure literal, as the harness requires.
	sb.WriteString("export const meta = {\n")
	fmt.Fprintf(&sb, "  name: %s,\n", jsString(name))
	fmt.Fprintf(&sb, "  description: %s,\n", jsString(fmt.Sprintf("Implement %s/%s via the adversarial evaluation gauntlet.", ctx.Domain, ctx.Module)))
	sb.WriteString("  phases: [\n")
	for _, b := range batches {
		fmt.Fprintf(&sb, "    { title: %s },\n", jsString(fmt.Sprintf("Batch %d", b.Index)))
	}
	sb.WriteString("    { title: \"Evaluate\" },\n")
	sb.WriteString("  ],\n")
	sb.WriteString("}\n\n")

	// classifyDetector maps the change-detector's reply to a control-flow token.
	// An unrecognized reply is treated conservatively as "changed" to avoid a
	// false convergence on the authoritative signal.
	sb.WriteString("function classifyDetector(out) {\n")
	sb.WriteString("  const s = out == null ? \"\" : String(out)\n")
	sb.WriteString("  if (s.indexOf(\"GIT_ERROR\") !== -1) return \"error\"\n")
	sb.WriteString("  if (s.indexOf(\"CLEAN\") !== -1) return \"clean\"\n")
	sb.WriteString("  if (s.indexOf(\"CHANGED\") !== -1) return \"changed\"\n")
	sb.WriteString("  return \"changed\"\n")
	sb.WriteString("}\n\n")

	for _, b := range batches {
		n := b.Index
		ids := make([]string, len(b.Items))
		for i, item := range b.Items {
			ids[i] = item.ID
		}
		idList := strings.Join(ids, ", ")

		fmt.Fprintf(&sb, "phase(%s)\n", jsString(fmt.Sprintf("Batch %d", n)))
		fmt.Fprintf(&sb, "log(%s)\n", jsString(fmt.Sprintf(
			"DEBUG: batch %d agents — primary model=%s type=%s; evaluator model=%s type=%s; change-detector model=haiku type=claude; bounds min_rounds=%d max_rounds=%d",
			n, im, it, em, et, minR, maxR)))
		fmt.Fprintf(&sb, "log(%s)\n", jsString(fmt.Sprintf("INFO: batch %d started: [%s]", n, idList)))

		// Primary pass — exactly once per batch.
		fmt.Fprintf(&sb, "const primary_%d = await agent(%s, { label: %s, model: %s, agentType: %s })\n",
			n, jsString(buildPrimaryPrompt(b, commits)),
			jsString(fmt.Sprintf("primary:batch-%d", n)), jsString(im), jsString(it))
		fmt.Fprintf(&sb, "if (primary_%d == null) {\n", n)
		fmt.Fprintf(&sb, "  log(%s)\n", jsString(fmt.Sprintf("ERROR: batch %d primary agent returned null; the evaluator will operate on whatever it produced", n)))
		sb.WriteString("}\n")

		// The evaluator loop only exists when there is at least one round to run.
		if maxR >= 1 {
			// Baseline snapshot so round one measures only the evaluator's delta.
			fmt.Fprintf(&sb, "await agent(%s, { label: %s, model: \"haiku\", agentType: \"claude\" })\n",
				jsString(buildBaselinePrompt()), jsString(fmt.Sprintf("snapshot:batch-%d", n)))

			fmt.Fprintf(&sb, "let round_%d = 0\n", n)
			fmt.Fprintf(&sb, "let changed_%d = true\n", n)
			fmt.Fprintf(&sb, "while (round_%d < %d) {\n", n, maxR)
			fmt.Fprintf(&sb, "  round_%d++\n", n)
			// Evaluator — exactly one per round, independent of eval.count.
			fmt.Fprintf(&sb, "  const evalRes_%d = await agent(%s, { label: \"eval:batch-%d:round-\" + round_%d, model: %s, agentType: %s, phase: \"Evaluate\" })\n",
				n, jsString(buildEvaluatorPrompt(b, commits)), n, n, jsString(em), jsString(et))
			fmt.Fprintf(&sb, "  if (evalRes_%d == null) {\n", n)
			fmt.Fprintf(&sb, "    log(%s + round_%d + %s)\n",
				jsString(fmt.Sprintf("ERROR: batch %d evaluator agent returned null on round ", n)), n, jsString("; recording no change for this round"))
			sb.WriteString("  }\n")
			// Change-detector — the authoritative git observer.
			fmt.Fprintf(&sb, "  const detect_%d = await agent(%s, { label: \"detect:batch-%d:round-\" + round_%d, model: \"haiku\", agentType: \"claude\", phase: \"Evaluate\" })\n",
				n, jsString(buildDetectorPrompt(commits)), n, n)
			fmt.Fprintf(&sb, "  const cls_%d = classifyDetector(detect_%d)\n", n, n)
			fmt.Fprintf(&sb, "  if (cls_%d === \"error\") {\n", n)
			fmt.Fprintf(&sb, "    log(%s + round_%d + %s)\n",
				jsString(fmt.Sprintf("ERROR: batch %d change-detector could not run git on round ", n)), n, jsString("; treating the round as changed"))
			fmt.Fprintf(&sb, "    changed_%d = true\n", n)
			sb.WriteString("  } else {\n")
			fmt.Fprintf(&sb, "    changed_%d = (cls_%d === \"changed\")\n", n, n)
			sb.WriteString("  }\n")
			fmt.Fprintf(&sb, "  log(%s + round_%d + %s + (changed_%d ? \"changed\" : \"clean\"))\n",
				jsString(fmt.Sprintf("INFO: batch %d round ", n)), n, jsString(": "), n)
			fmt.Fprintf(&sb, "  if (round_%d >= %d && !changed_%d) {\n", n, minR, n)
			fmt.Fprintf(&sb, "    log(%s + round_%d)\n", jsString(fmt.Sprintf("INFO: batch %d converged at round ", n)), n)
			sb.WriteString("    break\n")
			sb.WriteString("  }\n")
			sb.WriteString("}\n")
			fmt.Fprintf(&sb, "if (changed_%d) {\n", n)
			fmt.Fprintf(&sb, "  log(%s)\n", jsString(fmt.Sprintf("WARN: batch %d force-accepted (reached max_rounds=%d without a clean round)", n, maxR)))
			sb.WriteString("}\n")
		} else {
			// max_rounds == 0: no evaluator review; the primary's output stands,
			// surfaced as a force-accept so the absence of review is visible.
			fmt.Fprintf(&sb, "log(%s)\n", jsString(fmt.Sprintf("WARN: batch %d force-accepted (max_rounds=0, no evaluator round ran)", n)))
		}

		if commits {
			fmt.Fprintf(&sb, "log(%s)\n", jsString(fmt.Sprintf("INFO: batch %d committed", n)))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
