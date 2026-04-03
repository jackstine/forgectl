# Sub-Agent Spawn Points

The scaffold does not spawn sub-agents. It outputs instructions telling the architect what to spawn. The architect (or the skill driving the session) is responsible for spawning them.

## Consistent Output Wording

Every spawn instruction follows the pattern:

```
Please spawn {count} {type} sub-agent(s) to {purpose}.
```

## Spawn Points

| # | Phase | State | Purpose | Default Type | Default Count | Config Key |
|---|-------|-------|---------|-------------|--------------|------------|
| 1 | Specifying | EVALUATE | Evaluate spec batch | opus | 1 | `specifying.eval` |
| 2 | Specifying | CROSS_REFERENCE | Cross-reference domain specs | haiku | 3 | `specifying.cross_reference` |
| 3 | Specifying | CROSS_REFERENCE_EVAL | Evaluate cross-reference work | opus | 1 | `specifying.cross_reference.eval` |
| 4 | Specifying | RECONCILE_EVAL | Evaluate cross-domain reconciliation | opus | 1 | `specifying.reconciliation` |
| 5 | Planning | STUDY_CODE | Explore codebase | haiku | 3 | `planning.study_code` |
| 6 | Planning | EVALUATE | Evaluate plan | opus | 1 | `planning.eval` |
| 7 | Planning | REFINE | Update plan from eval findings | opus | 1 | `planning.refine` |
| 8 | Implementing | EVALUATE | Evaluate implementation batch | opus | 1 | `implementing.eval` |

## Configuration

`model` is a string that can be a model name (e.g., `"opus"`, `"haiku"`) or a descriptive phrase (e.g., `"opus explorer"`, `"spec-eval-expert"`). The scaffold includes this value verbatim in its spawn instructions.

`type` identifies the role of the sub-agent at this spawn point: `"eval"`, `"explore"`, or `"refine"`.

Each spawn point has `type`, `model`, and `count` in `.forgectl/config`:

```toml
[specifying.eval]
type = "eval"
model = "opus"
count = 1

[specifying.cross_reference]
type = "explore"
model = "haiku"
count = 3

[specifying.cross_reference.eval]
type = "eval"
model = "opus"
count = 1

[specifying.reconciliation]
type = "eval"
model = "opus"
count = 1

[planning.study_code]
type = "explore"
model = "haiku"
count = 3

[planning.eval]
type = "eval"
model = "opus"
count = 1

[planning.refine]
type = "refine"
model = "opus"
count = 1

[implementing.eval]
type = "eval"
model = "opus"
count = 1
```
