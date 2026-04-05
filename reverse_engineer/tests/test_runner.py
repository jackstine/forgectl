"""Tests for the mode-specific execution runner."""

import json
from pathlib import Path
from unittest.mock import AsyncMock, patch

import pytest

from reverse_engineer.runner import (
    execute,
    run_multi_pass,
    run_peer_review,
    run_self_refine,
    run_single_shot,
)
from reverse_engineer.schemas import (
    DrafterConfig,
    ExecutionConfig,
    ExecutionFile,
    MultiPassConfig,
    PeerReviewConfig,
    SelfRefineConfig,
    SpecEntry,
    SpecResult,
    SubAgentConfig,
)


# --- Fixtures ---

def make_entry(
    name: str = "Auth Handler",
    action: str = "create",
    domain: str = "myservice",
) -> SpecEntry:
    return SpecEntry(
        name=name,
        domain=domain,
        topic="handles user authentication requests",
        file="specs/auth-handler.md",
        action=action,
        code_search_roots=["src/auth/"],
        depends_on=[],
    )


def make_config(
    mode: str = "single_shot",
    rounds: int = 2,
    passes: int = 2,
    reviewers: int = 2,
    review_rounds: int = 1,
) -> ExecutionConfig:
    base = dict(
        mode=mode,
        drafter=DrafterConfig(
            model="opus",
            subagents=SubAgentConfig(model="sonnet", type="explorer", count=2),
        ),
    )
    if mode == "self_refine":
        base["self_refine"] = SelfRefineConfig(rounds=rounds)
    elif mode == "multi_pass":
        base["multi_pass"] = MultiPassConfig(passes=passes)
    elif mode == "peer_review":
        base["peer_review"] = PeerReviewConfig(
            reviewers=reviewers,
            rounds=review_rounds,
            subagents=SubAgentConfig(model="haiku", type="reviewer", count=1),
        )
    return ExecutionConfig(**base)


_SUCCESS = SpecResult(status="success", iterations_completed=1)


# --- Functional tests ---

@pytest.mark.anyio
async def test_single_shot_all_entries_get_success_results() -> None:
    """single_shot runs _run_entry once per entry and returns a result for each."""
    entries = [make_entry("Entry A"), make_entry("Entry B")]
    config = make_config("single_shot")

    with patch("reverse_engineer.runner._run_entry", new_callable=AsyncMock) as mock:
        mock.return_value = _SUCCESS
        results = await run_single_shot(entries, config, "/project")

    assert len(results) == 2
    assert all(r.status == "success" for r in results)
    assert mock.call_count == 2


@pytest.mark.anyio
async def test_self_refine_follow_ups_contain_round_context() -> None:
    """self_refine sends one follow-up per round, each annotated with round number."""
    entries = [make_entry()]
    config = make_config("self_refine", rounds=2)
    captured: list[list[str]] = []

    async def capture(_entry, _cfg, _root, follow_ups):
        captured.append(follow_ups)
        return _SUCCESS

    with patch("reverse_engineer.runner._run_entry", side_effect=capture):
        await run_self_refine(entries, config, "/project")

    assert len(captured) == 1
    follow_ups = captured[0]
    assert len(follow_ups) == 2
    assert "review round 1 of 2" in follow_ups[0].lower()
    assert "review round 2 of 2" in follow_ups[1].lower()


@pytest.mark.anyio
async def test_multi_pass_flips_create_to_update_after_first_pass() -> None:
    """multi_pass overrides 'create' to 'update' for passes after the first."""
    entries = [make_entry(action="create")]
    config = make_config("multi_pass", passes=2)
    seen_actions: list[str] = []

    async def capture(entry, _cfg, _root, _follow_ups):
        seen_actions.append(entry.action)
        return _SUCCESS

    with patch("reverse_engineer.runner._run_entry", side_effect=capture):
        await run_multi_pass(entries, config, "/project")

    assert seen_actions == ["create", "update"]


@pytest.mark.anyio
async def test_peer_review_follow_up_contains_reviewer_config() -> None:
    """peer_review embeds reviewers, subagent_model, and subagent_type in follow-up."""
    entries = [make_entry()]
    config = make_config("peer_review", reviewers=3, review_rounds=1)
    captured: list[list[str]] = []

    async def capture(_entry, _cfg, _root, follow_ups):
        captured.append(follow_ups)
        return _SUCCESS

    with patch("reverse_engineer.runner._run_entry", side_effect=capture):
        await run_peer_review(entries, config, "/project")

    assert len(captured) == 1
    follow_ups = captured[0]
    assert len(follow_ups) == 1
    # Template uses {peer_review.reviewers}, {peer_review.subagents.model}, etc.
    assert "3" in follow_ups[0]       # reviewers count
    assert "haiku" in follow_ups[0]   # subagents.model
    assert "reviewer" in follow_ups[0]  # subagents.type


@pytest.mark.anyio
async def test_execute_reads_routes_mode_and_writes_results(tmp_path: Path) -> None:
    """execute() reads execute.json, routes to the correct mode runner, writes results."""
    domain_root = tmp_path / "myservice"
    domain_root.mkdir()

    payload = {
        "project_root": str(tmp_path),
        "config": {
            "mode": "single_shot",
            "drafter": {
                "model": "opus",
                "subagents": {"model": "sonnet", "type": "explorer", "count": 2},
            },
        },
        "specs": [
            {
                "name": "Auth Handler",
                "domain": "myservice",
                "topic": "handles auth",
                "file": "specs/auth.md",
                "action": "create",
                "code_search_roots": ["src/"],
                "depends_on": [],
                "result": None,
            }
        ],
    }
    exec_file = tmp_path / "execute.json"
    exec_file.write_text(json.dumps(payload), encoding="utf-8")

    with patch("reverse_engineer.runner.run_single_shot", new_callable=AsyncMock) as mock:
        mock.return_value = [SpecResult(status="success", iterations_completed=1)]
        await execute(str(exec_file))

    assert mock.call_count == 1
    written = ExecutionFile.model_validate_json(exec_file.read_text(encoding="utf-8"))
    assert written.specs[0].result is not None
    assert written.specs[0].result.status == "success"


# --- Edge case tests ---

@pytest.mark.anyio
async def test_session_failure_writes_failure_result_and_continues() -> None:
    """_run_entry catches SDK exceptions and returns a failure result; others continue."""
    entries = [make_entry("Failing"), make_entry("Succeeding")]
    config = make_config("single_shot")
    _FAILURE = SpecResult(status="failure", error="session timed out")

    async def per_entry(entry, _cfg, _root, _follow_ups):
        # _run_entry itself catches exceptions — return the failure result directly.
        if entry.name == "Failing":
            return _FAILURE
        return _SUCCESS

    with patch("reverse_engineer.runner._run_entry", side_effect=per_entry):
        results = await run_single_shot(entries, config, "/project")

    assert len(results) == 2
    failing_result = next(r for r, e in zip(results, entries) if e.name == "Failing")
    succeeding_result = next(r for r, e in zip(results, entries) if e.name == "Succeeding")
    assert failing_result.status == "failure"
    assert "session timed out" in (failing_result.error or "")
    assert succeeding_result.status == "success"


@pytest.mark.anyio
async def test_multi_pass_single_pass_does_not_flip_action() -> None:
    """multi_pass with passes=1 does not flip 'create' entries to 'update'."""
    entries = [make_entry(action="create")]
    config = make_config("multi_pass", passes=1)
    seen_actions: list[str] = []

    async def capture(entry, _cfg, _root, _follow_ups):
        seen_actions.append(entry.action)
        return _SUCCESS

    with patch("reverse_engineer.runner._run_entry", side_effect=capture):
        await run_multi_pass(entries, config, "/project")

    assert seen_actions == ["create"]
