# Environment Variables

## FORGECTL_PYTHON_PROJECT

**Purpose:** Override the installed Python environment with a local development checkout.

**When to use:** When actively developing the `reverse-engineer` Python code and you want forgectl to use your local changes instead of the installed copy.

**Value:** Absolute or relative path to the `reverse_engineer/` project directory (the directory containing `pyproject.toml`).

**Requirements:** The directory must have a `.venv/` with dependencies installed. Run `uv sync` in the project directory first.

**Example:**

```bash
export FORGECTL_PYTHON_PROJECT=/path/to/reverse_engineer
forgectl advance  # Uses local Python code, skips checksum verification
```

**Behavior:**
- Bypasses the versioned install directory (`~/.local/share/forgectl/<version>/python/`)
- Skips checksum verification (you're expected to be modifying files)
- Uses the `.venv/bin/python` from the specified directory
- Fails with a clear message if the venv doesn't exist in that directory
