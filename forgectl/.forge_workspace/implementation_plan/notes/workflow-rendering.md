# Notes: Workflow Script Rendering

## Batch computation algorithm

Input: a validated `PlanJSON` and `batchSize int`.

1. Process `plan.Layers` in their declared order (index 0, 1, ...).
2. For each layer, get the ordered list of item IDs (`layer.Items`).
3. Within the layer, topologically sort by `depends_on` using a stable sort that preserves declared order as the tiebreak. Because validation guarantees no forward-layer deps and no cycles, a simple stable topo sort suffices: repeatedly emit items whose deps are already emitted.
4. Chunk the sorted ID list into groups of at most `batchSize`, preserving order.
5. Number batches sequentially starting at 1 across ALL layers (not restarting per layer).

Result: `[]Batch` where each batch has an index and a slice of `PlanItem`.

Key invariants (enforced by the algorithm above):
- Every item appears in exactly one batch.
- For any item, all depends_on items are in the same or earlier-numbered batch.
- No batch exceeds batchSize; no batch is empty.
- Cycle/forward-dep violations cannot occur (validation rejects them upstream).

## Data structure

```go
type Batch struct {
    Index int        // 1-based
    Items []PlanItem // full item records, in run order
}
```

## Script structure

The emitted script is a JavaScript string. It must:

1. Begin with a pure-literal `meta` export (no variables, function calls, spreads):
```js
export const meta = {
  name: '<domain>-<module>-impl',
  description: 'Implement <domain>/<module>',
  phases: [
    { title: 'Batch 1' },
    { title: 'Batch 2' },
    // ...one per batch...
    { title: 'Evaluate' },
  ],
}
```

2. Body uses only harness primitives: `agent()`, `log()`, `phase()`. No `parallel()` or `pipeline()` needed — the gauntlet loop is sequential per batch.

3. Per batch:
```js
phase('Batch N')
log('Starting batch N: [item ids...]')

// Primary: runs exactly once
await agent(`<primary prompt>`, {
  label: 'primary:batch-N',
  model: '<baked implement.model>',
  agentType: '<baked implement.type>',
})

// Evaluator loop: min_rounds floor, max_rounds ceiling
let round = 0
let changed = true
while (round < MAX_ROUNDS) {
  round++
  // Evaluator (adversarial, mutating)
  await agent(`<evaluator prompt>`, {
    label: `eval:batch-N:round-${round}`,
    model: '<baked eval.model>',
    agentType: '<baked eval.type>',
  })
  // Change-detector (git, with optional commit)
  const detector = await agent(`<change-detector prompt>`, {
    label: `detect:batch-N:round-${round}`,
    model: 'haiku',
    agentType: 'claude',
  })
  changed = /* parse detector output for changed/clean */
  log(`Batch N round ${round}: ${changed ? 'changed' : 'clean'}`)
  if (round >= MIN_ROUNDS && !changed) {
    log('Batch N converged at round ' + round)
    break
  }
}
if (changed) {
  log('WARN: Batch N force-accepted after ' + MAX_ROUNDS + ' rounds')
}
```

4. Template variables baked in: `MIN_ROUNDS`, `MAX_ROUNDS`, models, agentTypes, commit flag, all item fields.

## Primary prompt structure

For each item in the batch, include:
- `name` and `description`
- `steps` (ordered implementation instructions)
- `files` (files to create or modify)
- `specs` (spec references, display only)
- `tests` (acceptance criteria — what the evaluator will check)

When `enable_commits` is false: no commit instructions.
When `enable_commits` is true: include instruction to commit when work is complete.

## Evaluator prompt structure

Include:
- The `GauntletEval` adversarial instructions (embedded)
- All batch items: name, description, steps, files, specs, tests
- Instruction: review the current code for all items in this batch against their test criteria; edit code directly to fix any deficiency; do not just report problems

When `enable_commits` is false: no commit instructions in the evaluator prompt.

## Change-detector prompt structure

The change-detector is a lightweight `haiku` agent with Bash capability:
- Run `git status` and `git diff` (or `git diff --stat`) to detect changes
- Report whether the working tree has been modified since the last detected state
- When `enable_commits` is true: also run `git add -A && git commit -m "..."` if changes were found
- The workflow harness cannot run git itself; this agent is the authoritative git observer

The change-detector's result (changed/clean) drives the convergence check. Its self-description is not relied upon — the agent must actually run git.

## Output name sanitization

Sanitize `context.domain` and `context.module` to the slash-command charset:
- Lowercase all characters
- Replace any character that is not `[a-z0-9-]` with `-`
- Collapse runs of `-` to a single `-`
- Trim leading/trailing `-`

Result: `<sanitized-domain>-<sanitized-module>-impl.js`

## Collision handling

Check `filepath.Join(projectRoot, ".claude", "workflows", name)` before writing:
- If it doesn't exist: write there.
- If it exists: prepend a short prefix (e.g. a counter or short hash) to produce a new name. Log WARN naming both.
- Never overwrite an existing file.

## Atomic write

Write content to a `.tmp` path, then `os.Rename` to the final path. If rename fails, remove the temp file. The `.claude/workflows/` directory must exist (`os.MkdirAll` before writing).
