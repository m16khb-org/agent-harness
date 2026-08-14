from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass
from enum import StrEnum
from pathlib import Path
from typing import Annotated, ClassVar, NewType

from pydantic import BaseModel, ConfigDict, Field, StringConstraints

ToolName = NewType("ToolName", str)
ReadRequest = Annotated[str, StringConstraints(strip_whitespace=True, min_length=1)]


class Backend(StrEnum):
    AUTO = "auto"
    CLAUDE = "claude"
    CODEX = "codex"


class Risk(StrEnum):
    READ = "read"
    WRITE = "write"
    DESTRUCTIVE = "destructive"


class Confirmation(StrEnum):
    NONE = "none"
    WRITE = "write"
    DESTRUCTIVE = "destructive"


class BackendResolutionError(ValueError):
    """Raised when AUTO reaches a concrete backend boundary."""


class SlackSource(BaseModel):
    model_config: ClassVar[ConfigDict] = ConfigDict(frozen=True, extra="forbid")

    channel: str | None = None
    author: str | None = None
    timestamp: str | None = None
    permalink: str | None = None
    excerpt: str | None = None


class DelegateResult(BaseModel):
    model_config: ClassVar[ConfigDict] = ConfigDict(frozen=True, extra="forbid")

    ok: bool
    backend: Backend
    answer: str
    sources: tuple[SlackSource, ...]
    error: str | None


class QueryInput(BaseModel):
    model_config: ClassVar[ConfigDict] = ConfigDict(frozen=True, extra="forbid")

    text: ReadRequest


class DispatchInput(BaseModel):
    model_config: ClassVar[ConfigDict] = ConfigDict(frozen=True, extra="forbid")

    request: ReadRequest
    backend: Backend
    capability: str | None = None
    confirmation: Confirmation = Confirmation.NONE
    timeout_seconds: Annotated[float, Field(gt=0)]


class Capability(BaseModel):
    model_config: ClassVar[ConfigDict] = ConfigDict(frozen=True, extra="forbid")

    risk: Risk
    backends: tuple[Backend, ...]
    summary: str


class CapabilityManifest(BaseModel):
    model_config: ClassVar[ConfigDict] = ConfigDict(frozen=True, extra="forbid")

    version: int
    inventoried_at: str
    backend_order: tuple[Backend, ...]
    slack_lists_product_supported: bool
    tools: Mapping[str, Capability]


class ClaudeEnvelope(BaseModel):
    model_config: ClassVar[ConfigDict] = ConfigDict(frozen=True, extra="ignore")

    is_error: bool
    result: str
    structured_output: DelegateResult | None = None


@dataclass(frozen=True, slots=True)
class ProcessOutput:
    returncode: int
    stdout: str
    stderr: str


@dataclass(frozen=True, slots=True)
class BackendFailure:
    backend: Backend
    message: str


@dataclass(frozen=True, slots=True)
class CapabilitySelection:
    name: ToolName | None
    risk: Risk
    backends: tuple[Backend, ...]


def load_manifest(path: Path) -> CapabilityManifest:
    return CapabilityManifest.model_validate_json(path.read_text(encoding="utf-8"))
