# Creating the Plan Queue

> Step-by-step guide for building a `plan-queue.json` and starting the forgectl planning phase.

---

## Objective

The `plan-queue.json` is the entry point to forgectl's planning phase. It defines **which implementation plans to produce** by mapping staged spec files to domains, grouping them into logical plan entries, and pointing forgectl at the right source code to study.

The goal: take a set of spec files and produce a structured queue that forgectl can process through its planning state machine (ORIENT → STUDY_SPECS → STUDY_CODE → STUDY_PACKAGES → REVIEW → DRAFT → EVALUATE → REFINE → ACCEPT).

See [plan-queue-format.json](plan-queue-format.json) for the file template and [plan-queue-format.md](plan-queue-format.md) for the full schema reference.

---

## Step 1 — Identify the Staged Specs

Run `git diff --cached --name-only` to see all staged spec files. These are the specs that the plan queue must cover.

---

## Step 2 — Group Specs by Domain

Organize specs into plan entries. The most common grouping is **one plan per domain** (e.g., `api`, `launcher`, `optimizer`, `portal`). Each domain's specs go into a single plan entry.

Consider the branch name and initiative context to inform grouping. All specs in a plan should be related enough that a single implementation plan can address them coherently.

---

## Step 3 — Research Each Domain's Specs

For each domain, spawn sub-agents to read the spec files and summarize:

- **Purpose** — what each spec defines
- **Logging/observability requirements** — relevant if the initiative involves logging
- **Code directories touched** — informs the `code_search_roots` field

Split specs across agents (2–3 specs per agent) for parallel processing.

---

## Step 4 — Draft Each Plan Entry

For each domain, fill in the 6 required fields:

| Field | How to determine |
|---|---|
| `name` | `<Domain> <Initiative Name>` (e.g., "API Centralized Logging") |
| `domain` | The domain directory name (e.g., `api`) |
| `file` | `<domain>/.forge_workspace/implementation_plan/plan.json` |
| `specs` | All staged spec paths for this domain |
| `spec_commits` | Git commit hashes associated with the specs (for viewing diffs); may be empty |
| `code_search_roots` | The domain's source directory (e.g., `["api/"]`); add cross-domain roots if specs reference shared code |

---

## Step 5 — Write the File

Place the file at `.workspace/plan-queue.json`. Validate with:

```bash
python3 -c "import json; d = json.load(open('.workspace/plan-queue.json')); print(f'{len(d[\"plans\"])} plans')"
```

Forgectl also validates strictly on `init` — if any field is missing or extra fields are present, it exits with an error and prints the expected schema.

---

## Step 6 — Initialize Forgectl

```bash
forgectl init --phase planning --from .workspace/plan-queue.json
```

| Flag | Purpose |
|---|---|
| `--from` | Path to the plan queue file |
| `--phase` | Starting phase (use `planning`) |

All other settings (batch size, round limits, guided mode) are configured in `.forgectl/config` (TOML). See `docs/configurations.md` for the full reference.

After init, use `forgectl status` to see the session overview and `forgectl advance` to begin processing.

---

## Tips

- **New vs modified specs**: Identify which specs are new (core initiative work) vs modified (integration changes). This helps write a better `name`.
- **Code search roots**: Typically just the domain directory. Add additional roots only if specs explicitly reference cross-domain code (e.g., a shared `lib/` directory).
- **Review before init**: Present each plan entry to the user for approval before writing. The plan queue is hard to change after `forgectl init`.
