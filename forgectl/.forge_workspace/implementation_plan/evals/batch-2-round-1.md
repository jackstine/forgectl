# Evaluation Report

**Round:** 1
**Batch:** 2
**Layer:** L1 Core Logic

VERDICT: PASS

## Items Evaluated

### [workflow.batch-compute] Implement batch computation

**Files reviewed:** forgectl/cmd/generateworkflow.go (computeBatches, topoSortLayer), forgectl/cmd/generateworkflow_test.go, forgectl/specs/workflow-generation.md (Batch Computation section), forgectl/.forge_workspace/implementation_plan/notes/workflow-rendering.md

#### Test Results

- [PASS] Every plan item appears in exactly one batch across all layers — `computeBatches` iterates `plan.Layers` in declared order, topo-sorts each layer's full item list via `topoSortLayer`, and chunks the whole ordered slice without dropping or duplicating ids. `TestComputeBatches_EveryItemExactlyOnce` verifies this directly and passes.
- [PASS] For any item, all its depends_on items appear in the same batch or an earlier-numbered batch — `topoSortLayer` only emits an item once all of its in-layer deps are emitted, guaranteeing deps precede dependents in the layer's ordered sequence; since batches are chunked in that order and numbered monotonically, and cross-layer deps are already fully flushed into earlier, lower-numbered batches (layers processed sequentially, index never resets), the invariant holds. `TestComputeBatches_DependenciesNotLater` exercises intra-layer chained deps (a→d→e) and passes.
- [PASS] No batch exceeds the configured batch size; every batch has at least one item — chunking loop uses `min(start+batchSize, len(ordered))` and only advances by `batchSize`, so every chunk has 1..batchSize items (never zero, since `start < len(ordered)` is the loop guard). `TestComputeBatches_SizeBounds` passes.
- [PASS] Batches numbered sequentially from 1, continuing across layer boundaries without resetting — `index` is declared once outside the layer loop and only incremented, never reset per layer. `TestComputeBatches_SequentialNumberingAcrossLayers` confirms indices 1,2,3 across a two-layer plan and passes.
- [PASS] edge: batch size exceeds a layer's item count — chunking loop naturally produces one chunk of `len(ordered)` items when `batchSize >= len(ordered)`. `TestComputeBatches_BatchLargerThanLayer` (batchSize=5, layer of 2) passes.
- [PASS] edge: single-item plan — a one-item, one-layer plan yields one batch of one item with index 1. `TestComputeBatches_SingleItem` passes.

#### Notes
- `topoSortLayer`'s stable-tiebreak approach (rescan from the start of declared order each time an item is emitted, take the first ready item) is a correct, if O(n^2), stable topological sort; layer sizes in this domain (implementation plan batches) are small so this is not a practical concern. `TestTopoSortLayer_DependencyBeforeDependent` confirms a dependency declared later in the list is still emitted before its dependent.
- The no-progress fallback (emit remaining items in declared order) is dead code for validated plans per the spec's precondition, and is documented as such — acceptable defensive code, not a correctness issue.
- Matches spec's Batch Computation section (`forgectl/specs/workflow-generation.md`) steps 1-4 and postconditions exactly, and matches the notes file's algorithm description.

### [workflow.output-name] Derive output filename and handle collisions

**Files reviewed:** forgectl/cmd/generateworkflow.go (sanitizeSegment, workflowBaseName, deriveOutputName, resolveWorkflowPath, fileExists), forgectl/cmd/generateworkflow_test.go, forgectl/specs/workflow-generation.md (Outputs / Collision handling sections), forgectl/.forge_workspace/implementation_plan/notes/workflow-rendering.md

#### Test Results

- [PASS] domain/module values containing uppercase letters and non-alphanumeric characters are sanitized to lowercase hyphen-separated output — `sanitizeSegment` lowercases, replaces any run of non-`[a-z0-9-]` with `-`, collapses hyphen runs, and trims leading/trailing hyphens; `deriveOutputName` composes `<sanitized-domain>-<sanitized-module>-impl.js`. `TestDeriveOutputName_Sanitized` covers underscores, dots, spaces, repeated punctuation, and pre-existing leading/trailing hyphens, and additionally asserts the output charset is pure `[a-z0-9-]` — all pass.
- [PASS] edge: when the derived name already exists, a new prefixed name is chosen and the original file is left byte-for-byte unchanged — `resolveWorkflowPath` stats the intended path; on collision it loops `1-<name>`, `2-<name>`, ... until it finds a name that does not exist, never writing or touching the original file itself (only stats it). `TestResolveWorkflowPath_CollisionChoosesFreshName` verifies the chosen path differs from the intended name, is itself absent (so the later write step creates it fresh), and that the original file's bytes are unchanged after the call. Passes.
- [PASS] A WARN line is printed on collision resolution, naming both the intended and the chosen filename — the WARN is emitted only in the collision branch via `fmt.Fprintf(warnOut, "WARN: output name %q already exists ...; writing to %q instead\n", intendedName, chosenName)`, naming both. `TestResolveWorkflowPath_CollisionWarns` checks the string contains "WARN", the intended name, and the chosen base name; `TestResolveWorkflowPath_NoCollision` confirms no WARN is emitted when there's no collision. Both pass.

#### Notes
- Collision handling never overwrites: it only ever reads (`os.Stat`) the intended/candidate paths inside `resolveWorkflowPath`; the actual write happens elsewhere (not in scope of these two items) at the path this function returns, consistent with the spec's "never overwrite" invariant.
- `fileExists` correctly distinguishes "does not exist" from a genuine stat error (e.g. permission issues), propagating the latter as an error rather than silently treating it as "no collision."
- Matches the spec's Outputs/Collision section and the notes file's "Collision handling" guidance (prefix-based disambiguation, WARN naming both names, never overwrite).

## Summary

Ran `cd forgectl && go build ./...`, `go vet ./...`, and `go test ./cmd/ -run 'ComputeBatches|TopoSort|DeriveOutputName|ResolveWorkflowPath' -v`: build and vet are clean, and all 11 relevant tests pass. Both items' code was read line-by-line against the spec (`workflow-generation.md`, Batch Computation and Outputs/Collision sections) and the implementation-guidance notes (`workflow-rendering.md`); the algorithms are correct, not just superficially tested — the tests exercise the actual dependency-ordering, sequential-numbering, and byte-for-byte-preservation semantics the spec requires. No deficiencies found.
