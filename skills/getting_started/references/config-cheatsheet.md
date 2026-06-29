# Config Cheat-Sheet

`.forgectl/config` is TOML, phase-scoped, and **locked into the state file at
`init`** — edit it *before* you initialize a session; mid-session edits do nothing
until the next `init`. You can run entirely on defaults; this sheet is just the
handful of knobs that matter while you learn. The fully-commented reference is
[docs/default-config.toml](../../../docs/default-config.toml) and
[docs/configurations.md](../../../docs/configurations.md).

A partial config is valid — anything you omit falls back to the documented default.

---

## Day-one knobs (set these consciously)

```toml
[general]
# Pause at human checkpoints (SELECT / REVIEW / ORIENT) to discuss before proceeding.
# Leave true while learning — it keeps you in the loop. Toggle per-call with
# --guided / --no-guided on any `advance`.
user_guided = true

# false: forgectl does NO git operations; you commit by hand. --message not required.
# true:  --message is required at commit points; forgectl stages and commits for you.
# Leave false until you trust the loop.
enable_commits = false
```

```toml
[specifying]
batch = 3          # specs processed per cycle (grouped by domain; a batch never mixes domains)

[implementing]
batch = 2          # max unblocked plan items per implementation batch

[ui_implementing]
batch = 1          # UI batches are small on purpose — three verification loops run per batch
```

Smaller batches = tighter feedback, more checkpoints. Start small; widen once
comfortable.

---

## The evaluation loop (every phase has one)

```toml
[implementing.eval]
min_rounds = 1     # a PASS below this still forces another refine round
max_rounds = 3     # a FAIL at this threshold forces acceptance (prevents infinite loops)
model = "sonnet"   # model for the evaluator sub-agent
count = 1          # how many evaluator sub-agents to spawn
eval_mode = "report"   # "report": sub-agent writes a file, you pass --eval-report
                       # "direct": sub-agent edits files in place
                       # "conversational": verdict relayed verbally, no file
```

The same `[<phase>.eval]` shape exists for `specifying`, `planning`,
`implementing`, and `ui_implementing`. `min_rounds`/`max_rounds` bound the
quality loop so it always terminates. **Constraint: `min_rounds <= max_rounds`**,
or `init` rejects the config.

---

## Required for ui_implementing

`init --phase ui_implementing` validates these are **non-empty** — fill them or init
fails, naming each missing key:

```toml
[ui_implementing.app]
launch_command = "npm run dev"      # how the QA loop launches your app
url = "http://localhost:3000"       # where to reach it
ready_timeout_seconds = 30

[ui_implementing.e2e]
test_command = "npx playwright test"   # runs the authored e2e suite
test_dir = "e2e/"                      # where the authored tests live
```

---

## Optional: declare domains

Domains are optional. Declared, they constrain where specs may live and how the
plan queue groups work. No domain path may be a prefix of another.

```toml
[[domains]]
name = "optimizer"
path = "optimizer"

[[domains]]
name = "portal"
path = "portal"
```

Without a `[[domains]]` section, forgectl derives domains from spec file paths.

After editing config, confirm your domains with:

```bash
forgectl domains
```

This lists every declared domain (name + `<path>/specs/` location) so you can
verify setup before running `init`. Run it any time — it is read-only and safe.

### Opt-in: prompt for domains inside the session

Both `specifying` and `reverse_engineering` support an optional domain prompt
that gates the workflow until domains are confirmed. **Disabled by default.**

```toml
[specifying]
prompt_domains = false   # true: scaffold prompts at ORIENT before spec work starts

[reverse_engineering]
prompt_domains = false   # true: scaffold prompts at QUEUE; `forgectl add-domain` available
```

Leave both `false` — declare domains in `[[domains]]` before `init` and confirm
with `forgectl domains`. Enable only if your domain list cannot be known at init
time (e.g. a survey pass is expected to reveal new subsystems).

---

## Paths and logs (rarely changed)

```toml
[paths]
state_dir = ".forgectl/state"     # where the state file lives
workspace_dir = ".forge_workspace" # per-domain artifacts: <domain>/<workspace_dir>/

[logs]
enabled = true
retention_days = 90    # 0 = keep forever
max_files = 50         # 0 = unlimited; logs live in ~/.forgectl/logs/
```

---

## Quick rules

- Omit what you don't need — defaults fill the gaps.
- `min_rounds <= max_rounds` for every eval block, or init rejects.
- `commit_strategy` per phase must be one of `strict`, `all-specs`, `scoped`,
  `tracked`, `all`.
- Edit before `init`. The session uses the snapshot taken at init time.
</content>
