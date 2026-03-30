# Count Lines of Code

## Topic of Concern
> Measure the size of the forgectl codebase across two categories — Go source files and specification documents — and report counts by file, line, and character.

## Context
The `count-loc` script provides a snapshot of project size across the two artifact types that matter for this repo: Go source code and specification documents. It exists to give developers a quick, consistent view of how much code and how much spec content exists without manual counting or relying on IDE tools.

Token counting is supported as an optional mode for estimating AI context consumption.

---

## Interface

### Inputs

| Input | Type | Required | Description |
|-------|------|----------|-------------|
| `--tokens` | flag | No | When present, each language category's content is sent to the Anthropic token-counting API and the result is appended as a column |

### Outputs

A table printed to stdout with the following columns:

| Column | Present without `--tokens` | Present with `--tokens` |
|--------|---------------------------|-------------------------|
| Language | Yes | Yes |
| Files | Yes | Yes |
| Blank | Yes | Yes |
| Comment | Yes | Yes |
| Code | Yes | Yes |
| Characters | Yes | Yes |
| Tokens | No | Yes |

Exactly two rows appear in the body: one for Go, one for Specs. A TOTAL row follows the separator.

The header line shows the project root path and total file/line counts from cloc before the table.

### Rejection

| Condition | Response |
|-----------|----------|
| `cloc` is not installed | Exits with error: `cloc is not installed. Install it with: brew install cloc` |
| `--tokens` provided without `ANTHROPIC_API_KEY` set | Exits with error naming the missing variable and how to set it |
| Unknown flag passed | Exits with error naming the flag and printing usage |

---

## Behavior

### Go Counting

#### Preconditions
- `cloc` is installed and on PATH.

#### Steps
1. Run `cloc` with `--json` against the project root, excluding `node_modules`, `.next`, `dist`, `build`, `.venv`, `__pycache__`, `.egg-info`, `vendor`, `.git`, `.github`, `.vscode`.
2. Extract the `Go` entry from the cloc JSON output: `nFiles`, `blank`, `comment`, `code`.
3. Count Go characters: find all `*.go` files under the project root (same exclusions) and sum their byte counts via `wc -c`.
4. If `--tokens` is set, concatenate all Go file contents and POST to the Anthropic token-counting API.

#### Postconditions
- The Go row in the table reflects `nFiles`, `blank`, `comment`, and `code` from cloc, plus the character count from `wc -c`.

#### Error Handling
- If cloc returns no `Go` entry, all Go fields default to `0`.
- If `wc -c` produces no output, character count is `0`.
- If the token API call fails or returns no `input_tokens` field, token count is `0`.

---

### Specs Counting

#### Preconditions
- `cloc` is installed and on PATH (for the project-level header).

#### Steps
1. Find all `*.md` files whose path matches `*/specs/*.md` under the project root, applying the same exclusions as Go counting.
2. Count files: pipe the find output to `wc -l`.
3. Count lines: run `wc -l` across all matched files and take the grand total from the last line of output.
4. Count characters: run `wc -c` across all matched files and take the grand total.
5. Report blank and comment as `0` — cloc's blank/comment distinction is not applied to spec files.
6. If `--tokens` is set, concatenate all matched spec file contents and POST to the Anthropic token-counting API.

#### Postconditions
- The Specs row in the table shows the count of `*.md` files found in all `specs/` directories, the total line count of those files, and their total byte count.
- The label in the Language column reads `Specs`, not `Markdown`.

#### Error Handling
- If no spec files are found, all Specs fields are `0`.
- If `wc -c` produces no output, character count is `0`.
- If the token API call fails, token count is `0`.

---

### Token Counting (optional mode)

#### Preconditions
- `--tokens` flag is provided.
- `ANTHROPIC_API_KEY` is set in the environment (or in `.env` at the project root).

#### Steps
1. Load `.env` from the project root if it exists.
2. Validate `ANTHROPIC_API_KEY` is set; exit with an error if not.
3. For each language category, concatenate all matched file contents into a single string.
4. POST the string to `https://api.anthropic.com/v1/messages/count_tokens` using model `claude-sonnet-4-6`.
5. Extract `input_tokens` from the response.
6. Accumulate into `TOTAL_TOKENS` and display as an additional column.

#### Postconditions
- A `Tokens` column appears in the table.
- The TOTAL row includes the sum of token counts across all categories.

#### Error Handling
- If `curl` returns an empty response or the response has no `input_tokens` field, the count for that category is `0`.
- Partial failure (one category fails, another succeeds) is silently handled — the failed category shows `0`.

---

### Totals Row

#### Steps
1. Sum `files`, `blank`, `comment`, `code`, and `characters` independently for Go and Specs using the custom-counted values (not cloc's SUM).
2. If `--tokens` is set, sum accumulated `TOTAL_TOKENS`.
3. Print the TOTAL row after the separator line.

#### Postconditions
- TOTAL reflects only Go + Specs. No other languages contribute to the total.

---

## Configuration

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `--tokens` | flag | off | Enables token counting via Anthropic API |
| `ANTHROPIC_API_KEY` | env var | — | Required when `--tokens` is set. Loaded from `.env` if present |

---

## Invariants

- Exactly two language rows appear in the output body: Go and Specs. No other language categories are ever displayed.
- The Specs row counts only `*.md` files located inside a directory named `specs/` at any depth under the project root.
- Markdown files outside of `specs/` directories are never counted.
- The TOTAL row file count equals the sum of the Go file count and the Specs file count.
- The `--tokens` flag is the only accepted argument. Any other argument causes an immediate exit with a usage error.

---

## Edge Cases

- **Scenario:** The project has no specs directories.
  **Expected behavior:** The Specs row shows `0` for all columns.
  **Rationale:** The script must not error out if specs directories don't exist yet; zero is the correct answer.

- **Scenario:** A `specs/` directory exists but contains no `*.md` files.
  **Expected behavior:** The Specs row shows `0` for all columns.
  **Rationale:** Same as above — absence of content is not an error condition.

- **Scenario:** The project has no Go files.
  **Expected behavior:** The Go row shows `0` for all columns.
  **Rationale:** cloc returns an empty entry; defaults handle the zero case.

- **Scenario:** `--tokens` is provided but `.env` does not exist.
  **Expected behavior:** The script proceeds without loading `.env` and validates `ANTHROPIC_API_KEY` from the environment directly.
  **Rationale:** `.env` is optional; its absence is not an error.

- **Scenario:** Token API returns an unexpected JSON structure (no `input_tokens` key).
  **Expected behavior:** Token count for that category defaults to `0`; the script continues.
  **Rationale:** A transient API issue should not abort the entire count operation.

- **Scenario:** A markdown file lives at `some/specs/sub/dir/file.md` (nested deeper than one level inside `specs/`).
  **Expected behavior:** The file is not counted.
  **Rationale:** The pattern `*/specs/*.md` matches only direct children of a `specs/` directory. Subdirectory nesting inside `specs/` is out of scope.

---

## Testing Criteria

### Go row reflects cloc output
- **Verifies:** Go Counting behavior
- **Given:** A project with Go source files present
- **When:** The script runs without `--tokens`
- **Then:** The Files, Blank, Comment, and Code columns for Go match the values in cloc's JSON output for the `Go` key

### Specs row counts only specs-directory markdown
- **Verifies:** Specs Counting behavior and the invariant that markdown outside specs/ is excluded
- **Given:** A project with markdown files both inside `specs/` directories and elsewhere (e.g., README.md at root)
- **When:** The script runs
- **Then:** The Specs file count equals the number of `*.md` files found only inside `specs/` directories; files outside are not counted

### Specs row label reads "Specs"
- **Verifies:** Specs Counting postcondition
- **Given:** Any project
- **When:** The script runs
- **Then:** The Language column for the markdown row reads `Specs`, not `Markdown`

### TOTAL equals sum of Go and Specs
- **Verifies:** Totals Row behavior and the TOTAL invariant
- **Given:** Known Go and Specs file counts
- **When:** The script runs
- **Then:** TOTAL files = Go files + Specs files; TOTAL code = Go code + Specs lines

### Unknown flag exits with usage error
- **Verifies:** Rejection condition for unknown flags
- **Given:** Any project
- **When:** The script is invoked with an unrecognized argument (e.g., `--foo`)
- **Then:** The script exits non-zero and prints the unknown flag name and usage

### Missing cloc exits with install hint
- **Verifies:** Rejection condition for missing cloc
- **Given:** `cloc` is not on PATH
- **When:** The script runs
- **Then:** The script exits non-zero and prints an install hint

### Token mode requires API key
- **Verifies:** Rejection condition for missing ANTHROPIC_API_KEY
- **Given:** `--tokens` is passed and `ANTHROPIC_API_KEY` is not set
- **When:** The script runs
- **Then:** The script exits non-zero with an error naming the missing variable

### Token column appears only with --tokens
- **Verifies:** Token Counting postcondition
- **Given:** The same project
- **When:** Run once without `--tokens` and once with `--tokens`
- **Then:** The Tokens column is absent in the first run and present in the second
