# Notes: Command Infrastructure

## Registering a new command

Each command lives in its own file under `cmd/`. Add a package-level `var` for the cobra command and register it in `init()`:

```go
var generateWorkflowCmd = &cobra.Command{
    Use:   "generate-workflow <plan.json>",
    Short: "Compile a plan.json into a Claude Code workflow script",
    Args:  cobra.ExactArgs(1),
    RunE:  runGenerateWorkflow,
}

func init() {
    rootCmd.AddCommand(generateWorkflowCmd)
}
```

## Resolving config from the working tree

`resolveSession()` in `root.go` is the canonical helper:

```go
projectRoot, stateDir, cfg, err := resolveSession()
```

For `generate-workflow` (which does not read a state file), `projectRoot` and `cfg` are what matter. `stateDir` can be ignored. If no `.forgectl/config` is resolvable, `resolveSession()` returns an error — this becomes the "missing config" rejection case.

## Config fields used by generate-workflow

All live on `state.ForgeConfig`:

| Field | Type | Default |
|---|---|---|
| `cfg.Implementing.Batch` | `int` | 2 |
| `cfg.Implementing.Implement.Model` | `string` | `"sonnet"` |
| `cfg.Implementing.Implement.Type` | `string` | `"general-purpose"` |
| `cfg.Implementing.Eval.Model` | `string` | `"sonnet"` |
| `cfg.Implementing.Eval.Type` | `string` | `"general-purpose"` |
| `cfg.Implementing.Eval.MinRounds` | `int` | 1 |
| `cfg.Implementing.Eval.MaxRounds` | `int` | 3 |
| `cfg.General.EnableCommits` | `bool` | false |

`cfg.Implementing.Eval.EvalMode` / `cfg.Implementing.Eval.Count` are NOT read by the generator.

## Plan validation entry point

```go
errs := state.ValidatePlanJSON(data, baseDir)
// data: raw JSON bytes from the plan file
// baseDir: filepath.Dir(planPath) — used to resolve relative refs/notes paths
// returns []string of error messages; nil/empty = valid
```

On failure, print each error like `validate.go` does:
```go
fmt.Fprintf(cmd.OutOrStdout(), "Error: validation failed with %d error(s):\n", len(errs))
for i, e := range errs {
    fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s\n", i+1, e)
}
return fmt.Errorf("validation failed")
```

## Output patterns

Always write to `cmd.OutOrStdout()`, not `os.Stdout`. For logging levels, use plain `fmt.Fprintf` prefixed with `INFO:`, `WARN:`, `ERROR:`, `DEBUG:`.

## Atomic file write pattern

`state.Save()` uses: write to `.tmp` → rename existing to `.bak` → rename `.tmp` to final. For a new output file (no existing file to back up), a simpler pattern suffices: write to a temp path, then `os.Rename` to the final path. If the rename fails, remove the temp file. Never leave a partial file visible.

## Output directory

The `.claude/workflows/` directory must be created if it doesn't exist (`os.MkdirAll`). The path is relative to `projectRoot`.
