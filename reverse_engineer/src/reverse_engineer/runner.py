"""Mode-specific async execution runner for reverse engineering agent sessions."""

import asyncio
import logging
from pathlib import Path

from claude_agent_sdk import ClaudeAgentOptions, ClaudeSDKClient

from .factory import build_session
from .prompts_loader import load_prompt
from .schemas import ExecutionConfig, ExecutionFile, SpecEntry, SpecResult

logger = logging.getLogger(__name__)


async def _run_session(
    options: ClaudeAgentOptions,
    initial_prompt: str,
    follow_up_prompts: list[str],
) -> int:
    """Run a single agent session with optional follow-up prompts.

    Args:
        options: ClaudeAgentOptions for the session.
        initial_prompt: The initial prompt to send.
        follow_up_prompts: Follow-up prompts sent sequentially after the initial.

    Returns:
        Number of iterations completed (1 + len(follow_up_prompts)).

    Raises:
        Any exception raised by the SDK during the session.
    """
    async with ClaudeSDKClient(options) as client:
        await client.query(initial_prompt)
        async for _ in client.receive_response():
            pass

        for follow_up in follow_up_prompts:
            await client.query(follow_up)
            async for _ in client.receive_response():
                pass

    return 1 + len(follow_up_prompts)


async def _run_entry(
    entry: SpecEntry,
    config: ExecutionConfig,
    project_root: str,
    follow_up_prompts: list[str],
) -> SpecResult:
    """Build and run a session for one spec entry, returning a SpecResult.

    Errors are caught and converted to failure results so other entries can continue.
    """
    try:
        options, initial_prompt = build_session(entry, config, project_root)
        logger.info("Agent session constructed for spec '%s'", entry.name)
        iterations = await _run_session(options, initial_prompt, follow_up_prompts)
        logger.info("Agent completed for spec '%s': success", entry.name)
        return SpecResult(status="success", iterations_completed=iterations)
    except Exception as exc:  # noqa: BLE001
        logger.error("Agent session failure for '%s': %s", entry.name, exc)
        return SpecResult(status="failure", error=str(exc))


async def run_single_shot(
    entries: list[SpecEntry],
    config: ExecutionConfig,
    project_root: str,
) -> list[SpecResult]:
    """Run all entries in parallel with a single prompt each."""
    logger.info("single_shot: running %d sessions in parallel", len(entries))
    return list(
        await asyncio.gather(
            *[_run_entry(entry, config, project_root, []) for entry in entries]
        )
    )


async def run_self_refine(
    entries: list[SpecEntry],
    config: ExecutionConfig,
    project_root: str,
) -> list[SpecResult]:
    """Run all entries with initial prompt + N self-review follow-ups in parallel."""
    rounds = config.self_refine.rounds  # type: ignore[union-attr]
    review_template = load_prompt("review-work-prompt.md")

    follow_ups = [
        f"{review_template}\n\nThis is review round {n} of {rounds}."
        for n in range(1, rounds + 1)
    ]

    logger.info(
        "self_refine: running %d sessions with %d review round(s) each",
        len(entries),
        rounds,
    )
    return list(
        await asyncio.gather(
            *[_run_entry(entry, config, project_root, follow_ups) for entry in entries]
        )
    )


async def run_multi_pass(
    entries: list[SpecEntry],
    config: ExecutionConfig,
    project_root: str,
) -> list[SpecResult]:
    """Run all entries P times; after pass 1 all 'create' actions become 'update'."""
    passes = config.multi_pass.passes  # type: ignore[union-attr]
    logger.info("multi_pass: running %d pass(es) over %d entries", passes, len(entries))

    results: list[SpecResult] = []
    current_entries = list(entries)

    for pass_num in range(passes):
        if pass_num > 0:
            current_entries = [
                entry.model_copy(update={"action": "update"})
                if entry.action == "create"
                else entry
                for entry in current_entries
            ]

        results = list(
            await asyncio.gather(
                *[_run_entry(entry, config, project_root, []) for entry in current_entries]
            )
        )
        logger.info("multi_pass: pass %d/%d complete", pass_num + 1, passes)

    return results


async def run_peer_review(
    entries: list[SpecEntry],
    config: ExecutionConfig,
    project_root: str,
) -> list[SpecResult]:
    """Run all entries with initial prompt + N peer review follow-ups in parallel."""
    peer_cfg = config.peer_review  # type: ignore[union-attr]
    review_template = load_prompt("peer-review-prompt.md")

    follow_up_base = (
        review_template
        .replace("{peer_review.reviewers}", str(peer_cfg.reviewers))
        .replace("{peer_review.subagents.model}", peer_cfg.subagents.model)
        .replace("{peer_review.subagents.type}", peer_cfg.subagents.type)
    )
    follow_ups = [follow_up_base] * peer_cfg.rounds

    logger.info(
        "peer_review: running %d sessions with %d review round(s), %d reviewer(s) each",
        len(entries),
        peer_cfg.rounds,
        peer_cfg.reviewers,
    )
    return list(
        await asyncio.gather(
            *[_run_entry(entry, config, project_root, follow_ups) for entry in entries]
        )
    )


async def execute(execute_file_path: str) -> None:
    """Read execute.json, run agent sessions per mode, write results back.

    Args:
        execute_file_path: Absolute or relative path to the execute.json file.

    Raises:
        FileNotFoundError: If the execute file does not exist.
        pydantic.ValidationError: If the execute file fails schema validation.
        OSError: If the execute file cannot be written after execution.
    """
    path = Path(execute_file_path)
    if not path.exists():
        raise FileNotFoundError(f"Execution file not found: {path}")

    execution = ExecutionFile.model_validate_json(path.read_text(encoding="utf-8"))
    logger.info(
        "Execution file loaded: %d spec(s), mode=%s",
        len(execution.specs),
        execution.config.mode,
    )

    # Build the dispatch table inside execute() so that module-level patches in
    # tests replace the correct function references at call time.
    mode_runners = {
        "single_shot": run_single_shot,
        "self_refine": run_self_refine,
        "multi_pass": run_multi_pass,
        "peer_review": run_peer_review,
    }
    runner = mode_runners[execution.config.mode]
    results = await runner(execution.specs, execution.config, execution.project_root)

    for entry, result in zip(execution.specs, results, strict=True):
        entry.result = result

    try:
        path.write_text(execution.model_dump_json(indent=2) + "\n", encoding="utf-8")
        logger.info("Execution file written with all results: %s", path)
    except OSError as exc:
        logger.error("Failed to write execution file '%s': %s", path, exc)
        raise

    logger.info("All sessions complete.")
