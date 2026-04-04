"""Tests for package constants."""

from reverse_engineer.constants import ALLOWED_TOOLS, PERMISSION_MODE, READ_CLAUDE_MD


def test_allowed_tools():
    assert ALLOWED_TOOLS == ["Read", "Glob", "Grep", "Edit", "Write", "Agent"]


def test_permission_mode():
    assert PERMISSION_MODE == "acceptEdits"


def test_read_claude_md_is_false():
    assert READ_CLAUDE_MD is False
