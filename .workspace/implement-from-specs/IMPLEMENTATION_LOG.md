# Implementation Log

Log of implementation updates across sessions.

---

## Entries

### 2026-06-19 — config-scaffolding + session-init scaffolding
- **Errors:** None
- **All Tests Pass:** Yes (`go test ./...`)
- **Notes:** Implemented `config-scaffolding` spec (commit 2b73d36). New `state/Scaffold(cwd)` guarantees `.forgectl/` and `.forgectl/config` exist: reuses an ancestor `.forgectl/` directory if found, else creates one at cwd; writes the embedded default template (`state/default-config.toml`, a verbatim copy of `docs/default-config.toml`, drift-guarded by a test) atomically only when no config file exists; never reads/overwrites an existing config; partial-bootstrap is resumable. Wired into `init` (`cmd/init.go`) as the first step, replacing the old `resolveSession()` path; prints the fresh-default notice before any session output. Reconciled the documented toml-vs-`DefaultForgeConfig()` mismatch (spec invariant #3, defaults-equivalence): the template/`docs/configurations.md` were already canonical (`user_guided=true`, `specifying.batch=3`, `implementing.batch=2`, plus cross_reference/reconciliation/study_code/refine sub-agent configs); `DefaultForgeConfig()` was the stale/incomplete side and was completed to match. Updated the stale `TestDefaultForgeConfigValues` pin. Added `state/scaffold_test.go` (all 9 spec testing criteria) and `cmd` integration test `TestInitScaffoldsConfigWhenAbsent`. No derived-doc changes needed — docs already matched the template.
