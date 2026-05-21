"""Agent session factory.

Builds ClaudeAgentOptions and an assembled prompt from an execution file entry.
"""

import re
from pathlib import Path

from claude_agent_sdk import ClaudeAgentOptions

from .constants import ALLOWED_TOOLS, PERMISSION_MODE, READ_CLAUDE_MD
from .prompts_loader import load_prompt
from .schemas import ExecutionConfig, SpecEntry


def build_session(
    entry: SpecEntry,
    config: ExecutionConfig,
    project_root: str,
) -> tuple[ClaudeAgentOptions, str]:
    """Construct a ClaudeAgentOptions and assembled prompt for a spec entry.

    Args:
        entry: The spec entry from the execution file.
        config: The execution configuration.
        project_root: The project root directory.

    Returns:
        A tuple of (ClaudeAgentOptions, assembled_prompt).

    Raises:
        FileNotFoundError: If the domain root does not exist or the existing
            spec file is missing for an update action.
        ValueError: If the prompt template contains unresolvable placeholders.
    """
    # Resolve domain root.
    domain_root = Path(project_root) / entry.domain
    if not domain_root.is_dir():
        raise FileNotFoundError(f"Domain root not found: {domain_root}")

    # Load and concatenate bundled prompts.
    initial_prompt = load_prompt("reverse-engineering-prompt.md")
    format_ref = load_prompt("spec-format-reference.md")
    prompt = initial_prompt + "\n\n" + format_ref

    # For update action: read existing spec file.
    existing_spec_content = ""
    if entry.action == "update":
        spec_path = domain_root / entry.file
        if not spec_path.exists():
            raise FileNotFoundError(
                f"Existing spec file not found for update: {spec_path}"
            )
        existing_spec_content = spec_path.read_text(encoding="utf-8")

    # Build substitution map.
    subagents = config.drafter.subagents
    code_search_roots_str = ", ".join(entry.code_search_roots)
    substitutions = {
        "name": entry.name,
        "topic": entry.topic,
        "file": entry.file,
        "action": entry.action,
        "code_search_roots": code_search_roots_str,
        "existing_spec_content": existing_spec_content,
        "subagent_type": subagents.type,
        "subagent_model": subagents.model,
        "subagent_count": str(subagents.count),
    }

    # Replace all known placeholders.
    for key, value in substitutions.items():
        prompt = prompt.replace("{" + key + "}", value)

    # Check for any remaining unresolved placeholders.
    remaining = re.findall(r"\{([a-z_]+)\}", prompt)
    if remaining:
        raise ValueError(
            f"Unresolvable interpolation placeholders in prompt: {remaining}"
        )

    # Build ClaudeAgentOptions.
    # READ_CLAUDE_MD=False is enforced via setting_sources=["user"], which prevents
    # Claude Code from loading project-level config (CLAUDE.md files). The
    # claude-agent-sdk does not yet expose read_claude_md directly on ClaudeAgentOptions.
    setting_sources: list[str] | None = None if READ_CLAUDE_MD else ["user"]
    options = ClaudeAgentOptions(
        model=config.drafter.model,
        cwd=str(domain_root),
        allowed_tools=list(ALLOWED_TOOLS),
        permission_mode=PERMISSION_MODE,
        setting_sources=setting_sources,
    )

    return options, prompt
