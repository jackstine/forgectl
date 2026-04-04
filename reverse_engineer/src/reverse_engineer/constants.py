"""Package constants for the reverse engineer agent.

These values are hardcoded in the package and never supplied externally.
"""

ALLOWED_TOOLS: list[str] = ["Read", "Glob", "Grep", "Edit", "Write", "Agent"]

PERMISSION_MODE: str = "acceptEdits"

READ_CLAUDE_MD: bool = False
