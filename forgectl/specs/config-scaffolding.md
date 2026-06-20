# Configuration Scaffolding

## Topic of Concern
> The scaffold guarantees a project configuration file exists, creating the `.forgectl/` directory and writing a default config from an embedded template when either is absent.

## Context

forgectl reads all session settings from `.forgectl/config` (TOML) at the project root. Before a session can be initialized, that file — and the `.forgectl/` directory that holds it — must exist. Requiring users to hand-create both before the tool would do anything created a bootstrap gap: a brand-new project could not be run without manually authoring config.

This topic closes that gap. Whenever the scaffold is initialized, it ensures the project is bootstrapped: it establishes a project root, creates `.forgectl/` if no project root is found in the directory hierarchy, and writes a default `.forgectl/config` if one does not already exist. The default content is an embedded copy of the canonical commented configuration template. The effective configuration that this default materializes is identical to the configuration the scaffold applies when config fields are omitted — writing the default changes nothing about how a session behaves; it only makes the defaults visible and editable.

Scaffolding is non-destructive: an existing config is never read, modified, or overwritten by this topic. It only ever *creates* what is missing.

This topic owns the *existence and initial content* of the configuration file. It does not own *loading, parsing, or validating* that file — those are session-initialization concerns that run after scaffolding has guaranteed the file exists.

## Depends On
- **state-persistence** — provides the atomic file-write mechanism used to write the config file without leaving partial content on failure.

## Integration Points

| Adjacent concern | Relationship |
|------------------|-------------|
| session-init | Session initialization invokes configuration scaffolding as its first step. Scaffolding guarantees `.forgectl/` and `.forgectl/config` exist and returns the established project root, which session-init uses to locate config and resolve all relative paths. Scaffolding never returns a parsed config; session-init loads, parses, and validates it afterward. The fresh-default notice is emitted by scaffolding itself, not by session-init — the caller consumes only the project root. |
| state-persistence | Reuses the atomic temp-write-then-rename mechanism for the config file. |

---

## Data Models

### Embedded Default Config Template
The default configuration content compiled into the binary. It is a verbatim copy of the canonical commented TOML template (`docs/default-config.toml`).

| Field | Type | Required | Nullable | Notes / Constraints |
|-------|------|----------|----------|---------------------|
| (content) | TOML text | yes | no | Every value present is set to its default. Parsing it yields a configuration byte-for-byte equivalent in effect to the configuration produced when no config file fields are set. It always passes config validation. |

### Scaffolding Result
What scaffolding reports to its caller.

| Field | Type | Required | Nullable | Notes / Constraints |
|-------|------|----------|----------|---------------------|
| project_root | path | yes | no | The directory containing the `.forgectl/` that the session will use. |
| created_dir | boolean | yes | no | True when `.forgectl/` was created during this invocation. |
| created_config | boolean | yes | no | True when `.forgectl/config` was written during this invocation. |

---

## Interface

### Inputs

| Input | Type | Required | Description |
|-------|------|----------|-------------|
| current working directory | path | yes | The directory the scaffold is invoked from; the starting point for project-root resolution. |
| embedded default template | TOML text | yes | Compiled into the binary; the content written when no config exists. |

Scaffolding takes no CLI flags of its own. It is triggered as a side effect of `init`.

### Outputs

- A `.forgectl/` directory at the resolved project root (created only if no `.forgectl/` was found in the hierarchy).
- A `.forgectl/config` file containing the embedded default template (written only if it did not already exist).
- A Scaffolding Result reported to the caller.
- A user-facing notice on standard output when a default config is freshly written (see Observability).

### Rejection

| Condition | Signal | Rationale |
|-----------|--------|-----------|
| `.forgectl/` directory cannot be created (e.g., permission denied, path occupied by a non-directory file) | Error with the OS failure detail. Exit code 1. | The project root cannot be established. |
| `.forgectl/config` cannot be written (e.g., permission denied, target exists as a directory) | Error with the OS failure detail. Exit code 1. | A required artifact cannot be materialized. |

Scaffolding never rejects on the grounds that a config already exists, is malformed, or is invalid — it does not read existing config. Malformed or invalid existing config is detected and rejected downstream by session initialization, not here.

---

## Behavior

### Ensuring the Configuration File Exists

#### Preconditions
- The process has a current working directory.

#### Steps
1. Resolve the project root by walking up from the current working directory looking for an ancestor that contains a `.forgectl/` directory.
2. If an ancestor containing `.forgectl/` is found, that ancestor is the project root. Do not create a new `.forgectl/`.
3. If no ancestor contains `.forgectl/`, the project root is the current working directory. Create `.forgectl/` there.
4. Determine the config path as `.forgectl/config` under the project root.
5. If a file already exists at the config path, leave it untouched.
6. If no file exists at the config path, atomically write the embedded default template to it.
7. Report the Scaffolding Result (project root, whether the directory was created, whether the config was written).

#### Postconditions
- A `.forgectl/` directory exists at the resolved project root.
- A `.forgectl/config` file exists at the resolved project root.
- If a config file existed before this invocation, its content is byte-for-byte unchanged.
- If the config file was freshly written, its content is exactly the embedded default template.

#### Error Handling
- Creating `.forgectl/` fails: error with the OS failure detail, exit code 1. No config is written.
- Writing the config file fails: error with the OS failure detail, exit code 1. The atomic write (temp file then rename) leaves no partial or truncated config file at the config path. If `.forgectl/` was created earlier in this same invocation, it is left in place — it is not rolled back. A subsequent invocation reuses that directory and completes the bootstrap, because config creation is independently idempotent.

---

## Configuration

This topic produces the configuration file; it has no tunable parameters of its own. The content it writes is the embedded default template, which mirrors the canonical reference in `docs/default-config.toml`. The default values that template expresses are the same defaults the scaffold applies for any omitted field; this topic does not redefine them.

---

## Observability

### User-facing notice
When step 6 writes a default config, the scaffold prints a notice to standard output stating that a default `.forgectl/config` was created and that domain-aware spec placement is not enforced until `[[domains]]` entries are added to it. This breaks the silence of an otherwise-successful run on an unconfigured project. The notice is printed before the caller's own output (session-init's initialization output), so the user sees the bootstrap happened before the session result. When step 5 leaves an existing config untouched, no notice is printed.

### Error output
| Condition | Output |
|-----------|--------|
| Step 3 fails to create `.forgectl/` | The OS failure detail on standard error; exit code 1. |
| Step 6 fails to write `.forgectl/config` | The OS failure detail on standard error; exit code 1. |

This topic does not emit its own activity-log entries or metrics; session activity logging is owned by activity-logging and begins after scaffolding returns.

---

## Invariants

1. **Non-destructive.** An existing `.forgectl/config` is never read, modified, truncated, or overwritten by this topic.
2. **Idempotent.** Running scaffolding when `.forgectl/` and `.forgectl/config` already exist performs no writes and changes no file content; the Scaffolding Result reports both `created_*` flags false.
3. **Defaults equivalence.** Loading a freshly written default config yields an effective configuration identical to the one produced when the config file is absent or empty (the scaffold's built-in defaults).
4. **Written default is valid.** A freshly written default config always passes config validation.
5. **Create at most one root.** When an ancestor `.forgectl/` exists, no new `.forgectl/` is created; a new `.forgectl/` is created only when none is found in the entire ancestor chain, and only at the current working directory.

---

## Edge Cases

- **Scenario:** No `.forgectl/` exists in the current directory or any ancestor.
  - **Expected behavior:** `.forgectl/` is created at the current working directory, which becomes the project root, and the default config is written into it.
  - **Rationale:** A brand-new project bootstraps itself from wherever the user runs the command.

- **Scenario:** A `.forgectl/` directory exists in an ancestor, but it contains no `config` file.
  - **Expected behavior:** The ancestor is reused as the project root (no new `.forgectl/` is created), and the default config is written into the existing `.forgectl/`.
  - **Rationale:** Directory presence and config presence are independent conditions; each missing artifact is materialized where the project root already is.

- **Scenario:** A `.forgectl/config` already exists at the project root.
  - **Expected behavior:** It is left exactly as-is. No notice is printed and no write occurs.
  - **Rationale:** User-authored configuration is authoritative and must never be clobbered by a bootstrap step.

- **Scenario:** A `.forgectl/config` exists but contains malformed TOML or values that fail validation.
  - **Expected behavior:** Scaffolding still does nothing to it. The malformed or invalid config is surfaced and rejected by session initialization when it loads and validates the file.
  - **Rationale:** Scaffolding only creates what is missing; it is not responsible for repairing existing content, and silently overwriting a broken config would destroy the user's work and hide the problem.

- **Scenario:** A freshly written default config has no `[[domains]]` section.
  - **Expected behavior:** Scaffolding succeeds and prints the notice that domain-aware spec placement is unenforced until domains are added. The session proceeds.
  - **Rationale:** The default is intentionally domain-agnostic; domain configuration is optional and added by the user after bootstrap.

- **Scenario:** A path component named `.forgectl` exists at the current working directory but is a regular file, not a directory, and no ancestor contains a `.forgectl/` directory.
  - **Expected behavior:** Directory creation fails; error with the OS failure detail, exit code 1.
  - **Rationale:** The project root cannot be established when the expected directory name is occupied by a file at the location where the directory would be created.

- **Scenario:** During the upward walk, an ancestor contains an entry named `.forgectl` that is a regular file, not a directory.
  - **Expected behavior:** That entry is not treated as a project root; the walk continues to the parent. If no directory-form `.forgectl/` is found anywhere, `.forgectl/` is created at the current working directory.
  - **Rationale:** Only a `.forgectl/` *directory* marks a project root; a same-named file in an ancestor is irrelevant to resolution.

- **Scenario:** `.forgectl/` is created, then writing the default config fails (e.g., permission denied on the config path).
  - **Expected behavior:** Error with the OS failure detail, exit code 1. The created `.forgectl/` is left in place; no partial config file exists. Re-running after the cause is fixed reuses the directory and writes the config.
  - **Rationale:** Directory and config creation are independent idempotent steps; a half-completed bootstrap is safely resumable rather than requiring cleanup.

---

## Testing Criteria

### Creates directory and config on a bare project
- **Verifies:** Bootstrap behavior when nothing exists.
- **Given:** A working directory with no `.forgectl/` in it or any ancestor.
- **When:** Scaffolding runs.
- **Then:** `.forgectl/` exists at the working directory, `.forgectl/config` exists with content equal to the embedded default template, and the Scaffolding Result reports `created_dir` and `created_config` both true.

### Reuses an ancestor project root
- **Verifies:** Project-root discovery and create-at-most-one-root.
- **Given:** `.forgectl/` exists two levels up from the working directory.
- **When:** Scaffolding runs.
- **Then:** No new `.forgectl/` is created in the working directory; the project root is the ancestor.

### Writes config into an existing directory missing config
- **Verifies:** Independent directory/config conditions.
- **Given:** An ancestor `.forgectl/` exists with no `config` file inside it.
- **When:** Scaffolding runs.
- **Then:** No new `.forgectl/` is created; `.forgectl/config` is written into the existing `.forgectl/` with the embedded default template; Scaffolding Result reports `created_dir` false, `created_config` true.

### Never overwrites an existing config
- **Verifies:** Non-destructive invariant.
- **Given:** `.forgectl/config` exists with custom content (including content that is malformed or fails validation).
- **When:** Scaffolding runs.
- **Then:** The file content is byte-for-byte unchanged, no notice is printed, and Scaffolding Result reports `created_config` false.

### Is idempotent
- **Verifies:** Idempotency invariant.
- **Given:** A project where `.forgectl/` and `.forgectl/config` already exist.
- **When:** Scaffolding runs twice.
- **Then:** No file is written on either run; content is unchanged; both runs report `created_dir` and `created_config` false.

### Written default equals built-in defaults and validates
- **Verifies:** Defaults-equivalence and written-default-is-valid invariants.
- **Given:** A bare project.
- **When:** Scaffolding writes the default config, which is then loaded and validated.
- **Then:** The resulting effective configuration is identical to the configuration produced from an empty/absent config file, and config validation passes.

### Notice printed only on fresh write
- **Verifies:** User-facing notice behavior.
- **Given:** A bare project (run 1) followed by an immediate re-run (run 2).
- **When:** Scaffolding runs each time.
- **Then:** Run 1 prints the default-created notice; run 2 prints no notice.

### Reports an error when the directory cannot be created
- **Verifies:** Directory-creation failure handling.
- **Given:** A working directory where a regular file named `.forgectl` already occupies the path and no ancestor `.forgectl/` exists.
- **When:** Scaffolding runs.
- **Then:** It errors with the OS failure detail and exits non-zero; no config is written.

### Leaves a resumable state when config write fails after directory creation
- **Verifies:** Partial-bootstrap error handling.
- **Given:** A bare project where `.forgectl/` does not exist and the config path is unwritable (e.g., a directory occupies the config path).
- **When:** Scaffolding runs, then is re-run after the cause is removed.
- **Then:** The first run errors non-zero with `.forgectl/` left in place and no partial config file; the second run reuses `.forgectl/` and writes the default config, reporting `created_dir` false, `created_config` true.

---

## Implements
- Automatic creation of `.forgectl/` and a default `.forgectl/config` during init
- Embedded default configuration template as the materialized default
- Non-destructive, idempotent bootstrap of project configuration
- Notice that domain-aware spec placement is unenforced until domains are configured
