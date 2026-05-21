"""Tests for Pydantic schemas in schemas.py."""

import pytest
from pydantic import ValidationError

from reverse_engineer.schemas import (
    ExecutionConfig,
    ExecutionFile,
    SpecEntry,
    SpecResult,
    SubAgentConfig,
)


VALID_EXECUTION_FILE = {
    "project_root": "/project/",
    "config": {
        "mode": "self_refine",
        "drafter": {
            "model": "opus",
            "subagents": {
                "model": "opus",
                "type": "explorer",
                "count": 3,
            },
        },
        "self_refine": {"rounds": 2},
    },
    "specs": [
        {
            "name": "Auth Middleware Validation",
            "domain": "optimizer",
            "topic": "The optimizer validates authentication tokens",
            "file": "specs/auth-middleware-validation.md",
            "action": "create",
            "code_search_roots": ["src/middleware/", "src/auth/"],
            "depends_on": [],
            "result": None,
        }
    ],
}


def test_valid_execution_file_parses():
    """Functional: A valid execute.json round-trips through the model."""
    ef = ExecutionFile.model_validate(VALID_EXECUTION_FILE)
    assert ef.project_root == "/project/"
    assert ef.config.mode == "self_refine"
    assert ef.config.drafter.model == "opus"
    assert ef.config.drafter.subagents.count == 3
    assert ef.config.self_refine is not None
    assert ef.config.self_refine.rounds == 2
    assert len(ef.specs) == 1
    spec = ef.specs[0]
    assert spec.name == "Auth Middleware Validation"
    assert spec.action == "create"
    assert spec.result is None


def test_unknown_mode_rejected():
    """Rejection: unknown mode value produces a validation error."""
    bad = {
        **VALID_EXECUTION_FILE,
        "config": {
            "mode": "turbo",
            "drafter": {
                "model": "opus",
                "subagents": {"model": "opus", "type": "explorer", "count": 1},
            },
        },
    }
    with pytest.raises(ValidationError) as exc_info:
        ExecutionFile.model_validate(bad)
    assert "Unknown mode" in str(exc_info.value)


def test_subagent_count_below_minimum_rejected():
    """Rejection: drafter.subagents.count < 1 produces a validation error."""
    bad = {
        **VALID_EXECUTION_FILE,
        "config": {
            "mode": "single_shot",
            "drafter": {
                "model": "opus",
                "subagents": {"model": "opus", "type": "explorer", "count": 0},
            },
        },
    }
    with pytest.raises(ValidationError):
        ExecutionFile.model_validate(bad)


def test_missing_required_field_rejected():
    """Rejection: missing required field (project_root) produces a validation error."""
    bad = {
        "config": {
            "mode": "single_shot",
            "drafter": {
                "model": "opus",
                "subagents": {"model": "opus", "type": "explorer", "count": 1},
            },
        },
        "specs": [],
    }
    with pytest.raises(ValidationError):
        ExecutionFile.model_validate(bad)
