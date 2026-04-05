"""Tests for the CLI entry point (cli.py)."""

from unittest.mock import AsyncMock, patch

import pytest
from pydantic import ValidationError

from reverse_engineer.cli import main
from reverse_engineer.schemas import ExecutionFile


# --- Helpers ---

def _make_validation_error(data: dict) -> ValidationError:
    """Produce a real ValidationError from bad ExecutionFile data."""
    try:
        ExecutionFile.model_validate(data)
    except ValidationError as exc:
        return exc
    raise AssertionError("Expected ValidationError was not raised")  # pragma: no cover


_INVALID_CONFIG_DATA = {
    "project_root": "/project",
    "config": {
        "mode": "single_shot",
        "drafter": {
            "model": "opus",
            "subagents": {"model": "sonnet", "type": "explorer", "count": 0},  # count < 1
        },
    },
    "specs": [],
}

_UNKNOWN_MODE_DATA = {
    "project_root": "/project",
    "config": {
        "mode": "turbo_mode",
        "drafter": {
            "model": "opus",
            "subagents": {"model": "sonnet", "type": "explorer", "count": 2},
        },
    },
    "specs": [],
}


# --- Functional test ---

def test_cli_reads_execute_json_and_invokes_runner(capsys) -> None:
    """CLI passes the --execute path to the runner and exits successfully."""
    with patch("reverse_engineer.cli.execute", new_callable=AsyncMock) as mock_execute:
        with patch("sys.argv", ["reverse-engineer", "--execute", "/path/to/execute.json"]):
            main()

    mock_execute.assert_called_once_with("/path/to/execute.json")
    assert capsys.readouterr().err == ""


# --- Rejection tests ---

def test_missing_execute_flag_prints_usage_and_exits_nonzero(capsys) -> None:
    """Missing --execute flag causes argparse to print usage to stderr and exit non-zero."""
    with patch("sys.argv", ["reverse-engineer"]):
        with pytest.raises(SystemExit) as exc_info:
            main()

    assert exc_info.value.code != 0
    # argparse writes the error message to stderr
    assert len(capsys.readouterr().err) > 0


def test_invalid_execute_json_exits_nonzero_with_validation_errors(capsys) -> None:
    """Invalid execute.json (schema violation) exits non-zero with validation errors on stderr."""
    validation_error = _make_validation_error(_INVALID_CONFIG_DATA)

    with patch("reverse_engineer.cli.execute", new_callable=AsyncMock) as mock_execute:
        mock_execute.side_effect = validation_error
        with patch("sys.argv", ["reverse-engineer", "--execute", "execute.json"]):
            with pytest.raises(SystemExit) as exc_info:
                main()

    assert exc_info.value.code != 0
    assert len(capsys.readouterr().err) > 0


def test_nonexistent_execute_json_exits_nonzero_with_file_not_found(capsys) -> None:
    """Non-existent execute.json path exits non-zero with file-not-found message on stderr."""
    with patch("reverse_engineer.cli.execute", new_callable=AsyncMock) as mock_execute:
        mock_execute.side_effect = FileNotFoundError("Execution file not found: /nonexistent.json")
        with patch("sys.argv", ["reverse-engineer", "--execute", "/nonexistent.json"]):
            with pytest.raises(SystemExit) as exc_info:
                main()

    assert exc_info.value.code != 0
    stderr = capsys.readouterr().err
    assert "/nonexistent.json" in stderr


def test_unknown_mode_exits_nonzero_with_valid_modes_listed(capsys) -> None:
    """execute.json with unknown mode exits non-zero with valid modes listed on stderr."""
    validation_error = _make_validation_error(_UNKNOWN_MODE_DATA)

    with patch("reverse_engineer.cli.execute", new_callable=AsyncMock) as mock_execute:
        mock_execute.side_effect = validation_error
        with patch("sys.argv", ["reverse-engineer", "--execute", "execute.json"]):
            with pytest.raises(SystemExit) as exc_info:
                main()

    assert exc_info.value.code != 0
    stderr = capsys.readouterr().err
    # The ValidationError from schemas.py lists valid modes
    assert any(
        mode in stderr
        for mode in ("single_shot", "multi_pass", "self_refine", "peer_review")
    )
