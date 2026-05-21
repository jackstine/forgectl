"""Pydantic models for validating execute.json."""

from __future__ import annotations

from typing import Literal, Optional

from pydantic import BaseModel, Field, model_validator


class SubAgentConfig(BaseModel):
    model: str
    type: str
    count: int = Field(ge=1)


class DrafterConfig(BaseModel):
    model: str
    subagents: SubAgentConfig


class SelfRefineConfig(BaseModel):
    rounds: int = Field(ge=1)


class MultiPassConfig(BaseModel):
    passes: int = Field(ge=1)


class PeerReviewConfig(BaseModel):
    reviewers: int = Field(ge=1)
    rounds: int = Field(ge=1)
    subagents: SubAgentConfig


VALID_MODES = frozenset({"single_shot", "self_refine", "multi_pass", "peer_review"})


class ExecutionConfig(BaseModel):
    mode: str
    drafter: DrafterConfig
    self_refine: Optional[SelfRefineConfig] = None
    multi_pass: Optional[MultiPassConfig] = None
    peer_review: Optional[PeerReviewConfig] = None

    @model_validator(mode="after")
    def validate_mode_and_config(self) -> "ExecutionConfig":
        if self.mode not in VALID_MODES:
            raise ValueError(
                f"Unknown mode {self.mode!r}. Valid modes: {sorted(VALID_MODES)}"
            )
        if self.mode == "self_refine" and self.self_refine is None:
            raise ValueError("self_refine config block is required when mode is 'self_refine'")
        if self.mode == "multi_pass" and self.multi_pass is None:
            raise ValueError("multi_pass config block is required when mode is 'multi_pass'")
        if self.mode == "peer_review" and self.peer_review is None:
            raise ValueError("peer_review config block is required when mode is 'peer_review'")
        return self


class SpecResult(BaseModel):
    status: Literal["success", "failure"]
    iterations_completed: Optional[int] = None
    error: Optional[str] = None


class SpecEntry(BaseModel):
    name: str
    domain: str
    topic: str
    file: str
    action: Literal["create", "update"]
    code_search_roots: list[str]
    depends_on: list[str]
    result: Optional[SpecResult] = None


class ExecutionFile(BaseModel):
    project_root: str
    config: ExecutionConfig
    specs: list[SpecEntry]
