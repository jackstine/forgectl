"""Tests for the bundled prompt file loader."""

import pytest

from reverse_engineer.prompts_loader import load_prompt


def test_load_prompt_returns_content():
    """load_prompt returns a non-empty string for a known bundled file."""
    content = load_prompt("reverse-engineering-prompt.md")
    assert isinstance(content, str)
    assert len(content) > 0


def test_load_prompt_all_bundled_files():
    """All four bundled prompt files are loadable."""
    files = [
        "reverse-engineering-prompt.md",
        "spec-format-reference.md",
        "review-work-prompt.md",
        "peer-review-prompt.md",
    ]
    for filename in files:
        content = load_prompt(filename)
        assert len(content) > 0, f"{filename} must not be empty"


def test_load_prompt_missing_file_raises_file_not_found():
    """load_prompt raises FileNotFoundError for a nonexistent bundled file."""
    with pytest.raises(FileNotFoundError, match="nonexistent-prompt.md"):
        load_prompt("nonexistent-prompt.md")
