"""Tests for the agent session factory."""

import os
from pathlib import Path
from unittest.mock import patch

import pytest
from claude_agent_sdk import ClaudeAgentOptions

from reverse_engineer.constants import ALLOWED_TOOLS, PERMISSION_MODE
from reverse_engineer.factory import build_session
from reverse_engineer.schemas import (
    DrafterConfig,
    ExecutionConfig,
    SelfRefineConfig,
    SpecEntry,
    SubAgentConfig,
)


# --- Fixtures ---

def make_entry(
    action: str = "create",
    domain: str = "myservice",
    code_search_roots: list[str] | None = None,
) -> SpecEntry:
    return SpecEntry(
        name="Auth Handler",
        domain=domain,
        topic="handles user authentication requests",
        file="specs/auth-handler.md",
        action=action,
        code_search_roots=code_search_roots or ["src/auth/"],
        depends_on=[],
    )


def make_config(model: str = "opus") -> ExecutionConfig:
    return ExecutionConfig(
        mode="self_refine",
        drafter=DrafterConfig(
            model=model,
            subagents=SubAgentConfig(model="sonnet", type="explorer", count=2),
        ),
        self_refine=SelfRefineConfig(rounds=1),
    )


def make_domain_root(tmp_path: Path, domain: str = "myservice") -> Path:
    domain_root = tmp_path / domain
    domain_root.mkdir()
    return domain_root


# --- Functional tests ---

def test_build_session_options_have_correct_constants(tmp_path: Path) -> None:
    """build_session sets model, constants (tools, permission_mode), and disables CLAUDE.md."""
    make_domain_root(tmp_path)
    config = make_config(model="sonnet")
    entry = make_entry()

    options, _ = build_session(entry, config, str(tmp_path))

    assert options.model == "sonnet"
    assert options.allowed_tools == list(ALLOWED_TOOLS)
    assert options.permission_mode == PERMISSION_MODE
    # READ_CLAUDE_MD=False is enforced by restricting setting_sources to user-level only.
    assert options.setting_sources == ["user"]


def test_build_session_sets_cwd_to_domain_root(tmp_path: Path) -> None:
    """build_session sets cwd to the domain root directory."""
    make_domain_root(tmp_path)
    entry = make_entry()

    options, _ = build_session(entry, make_config(), str(tmp_path))

    assert options.cwd == str(tmp_path / "myservice")


def test_build_session_prompt_contains_interpolated_fields(tmp_path: Path) -> None:
    """build_session interpolates entry fields into the assembled prompt."""
    make_domain_root(tmp_path)
    entry = make_entry()

    _, prompt = build_session(entry, make_config(), str(tmp_path))

    assert "Auth Handler" in prompt
    assert "handles user authentication requests" in prompt
    assert "specs/auth-handler.md" in prompt
    assert "create" in prompt
    assert "src/auth/" in prompt


def test_build_session_update_appends_existing_spec(tmp_path: Path) -> None:
    """For update action, existing spec content is appended to the prompt."""
    domain_root = make_domain_root(tmp_path)
    spec_dir = domain_root / "specs"
    spec_dir.mkdir()
    existing = "# Existing spec content\nSome details."
    (spec_dir / "auth-handler.md").write_text(existing, encoding="utf-8")

    entry = make_entry(action="update")
    _, prompt = build_session(entry, make_config(), str(tmp_path))

    assert existing in prompt


def test_build_session_embeds_subagent_config(tmp_path: Path) -> None:
    """build_session embeds sub-agent model, type, and count into the prompt."""
    make_domain_root(tmp_path)
    entry = make_entry()
    config = make_config()  # subagents: model=sonnet, type=explorer, count=2

    _, prompt = build_session(entry, config, str(tmp_path))

    assert "sonnet" in prompt
    assert "explorer" in prompt
    assert "2" in prompt


# --- Rejection tests ---

def test_build_session_domain_root_missing(tmp_path: Path) -> None:
    """build_session raises FileNotFoundError if domain root does not exist."""
    entry = make_entry(domain="nonexistent")  # directory not created

    with pytest.raises(FileNotFoundError, match="Domain root not found"):
        build_session(entry, make_config(), str(tmp_path))


def test_build_session_update_missing_spec_raises(tmp_path: Path) -> None:
    """build_session raises FileNotFoundError if the existing spec is missing for update."""
    make_domain_root(tmp_path)
    entry = make_entry(action="update")  # spec file not created

    with pytest.raises(FileNotFoundError, match="Existing spec file not found"):
        build_session(entry, make_config(), str(tmp_path))


def test_build_session_unresolvable_placeholder_raises(tmp_path: Path) -> None:
    """build_session raises ValueError if the prompt template has unresolvable placeholders."""
    make_domain_root(tmp_path)
    entry = make_entry()

    # Mock load_prompt to return a template with an unknown placeholder.
    def mock_load_prompt(filename: str) -> str:
        if filename == "reverse-engineering-prompt.md":
            return "Placeholder: {unknown_field}"
        return ""

    with patch("reverse_engineer.factory.load_prompt", side_effect=mock_load_prompt):
        with pytest.raises(ValueError, match="unknown_field"):
            build_session(entry, make_config(), str(tmp_path))
