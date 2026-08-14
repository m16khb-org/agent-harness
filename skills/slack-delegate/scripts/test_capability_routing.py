# /// script
# requires-python = ">=3.13"
# dependencies = [
#   "pydantic==2.11.7",
#   "pytest==8.4.1",
#   "typer==0.27.1",
# ]
# ///
# ─── How to run ───
# Run capability routing tests:
#   uv run --with pytest==8.4.1 pytest -q test_capability_routing.py

from __future__ import annotations

import json
from pathlib import Path

from pydantic import BaseModel, Field
from slack_delegate import Backend, ProcessOutput, dispatch_query
from slack_models import load_manifest

SKILL_DIR = Path(__file__).resolve().parent.parent


class _ItemSchema(BaseModel):
    required: tuple[str, ...]


class _SourcesSchema(BaseModel):
    item_schema: _ItemSchema = Field(alias="items")


class _PropertiesSchema(BaseModel):
    sources: _SourcesSchema


class _ResultSchema(BaseModel):
    properties: _PropertiesSchema


def _result(backend: Backend) -> dict[str, bool | str | None | list[dict[str, str]]]:
    return {
        "ok": True,
        "backend": backend.value,
        "answer": "completed",
        "sources": [
            {
                "channel": "self-dm",
                "author": "self",
                "timestamp": "1786417094.295949",
                "permalink": "https://example.slack.com/archives/D0123456789/p1786417094295949",
                "excerpt": "test",
            }
        ],
        "error": None,
    }


def _claude_output(*, ok: bool = True) -> ProcessOutput:
    result = _result(Backend.CLAUDE)
    result["ok"] = ok
    result["error"] = None if ok else "failed"
    return ProcessOutput(
        returncode=0,
        stdout=json.dumps(
            {
                "is_error": False,
                "result": json.dumps(result),
                "structured_output": result,
            }
        ),
        stderr="",
    )


def _codex_output() -> ProcessOutput:
    return ProcessOutput(
        returncode=0,
        stdout=f"{json.dumps(_result(Backend.CODEX))}\n",
        stderr="",
    )


class FakeRunner:
    """Record invocations because routing order is the observable contract."""

    def __init__(self, outputs: dict[str, ProcessOutput]) -> None:
        self.outputs: dict[str, ProcessOutput] = outputs
        self.calls: list[tuple[str, ...]] = []

    def run(
        self,
        args: tuple[str, ...],
        *,
        cwd: Path,
        timeout_seconds: float,
    ) -> ProcessOutput:
        del cwd, timeout_seconds
        self.calls.append(args)
        return self.outputs[args[0]]


def test_manifest_contains_complete_union_without_slack_lists_product() -> None:
    # Given
    manifest = load_manifest(SKILL_DIR / "capabilities.json")

    # When
    tools = manifest.tools

    # Then
    assert len(tools) == 32
    assert manifest.slack_lists_product_supported is False


def test_result_schema_omits_unsupported_draft_declaration() -> None:
    # Given
    schema_text = (SKILL_DIR / "result.schema.json").read_text()

    # When / Then
    assert '"$schema"' not in schema_text


def test_result_schema_requires_every_source_field() -> None:
    # Given
    schema = _ResultSchema.model_validate_json(
        (SKILL_DIR / "result.schema.json").read_text()
    )

    # When
    required = schema.properties.sources.item_schema.required

    # Then
    assert required == ("channel", "author", "timestamp", "permalink", "excerpt")


def test_write_requires_confirmation_before_spawning_agent() -> None:
    # Given
    runner = FakeRunner(outputs={"claude": _claude_output()})

    # When
    result = dispatch_query(
        "send the approved marker to my self-DM",
        backend=Backend.AUTO,
        capability="slack_send_message",
        confirmation="none",
        runner=runner,
        timeout_seconds=30,
    )

    # Then
    assert result.ok is False
    assert runner.calls == []


def test_destructive_capability_requires_destructive_confirmation() -> None:
    # Given
    runner = FakeRunner(outputs={"claude": _claude_output()})

    # When
    result = dispatch_query(
        "replace the canvas section",
        backend=Backend.AUTO,
        capability="slack_update_canvas",
        confirmation="write",
        runner=runner,
        timeout_seconds=30,
    )

    # Then
    assert result.ok is False
    assert runner.calls == []


def test_shared_write_does_not_fallback_after_claude_dispatch() -> None:
    # Given
    runner = FakeRunner(
        outputs={"claude": _claude_output(ok=False), "codex": _codex_output()}
    )

    # When
    result = dispatch_query(
        "send one marker to my self-DM",
        backend=Backend.AUTO,
        capability="slack_send_message",
        confirmation="write",
        runner=runner,
        timeout_seconds=30,
    )

    # Then
    assert result.ok is False
    assert [call[0] for call in runner.calls] == ["claude"]


def test_codex_only_write_routes_directly_to_codex() -> None:
    # Given
    runner = FakeRunner(outputs={"codex": _codex_output()})

    # When
    result = dispatch_query(
        "create the requested reminder",
        backend=Backend.AUTO,
        capability="slack_create_reminder",
        confirmation="write",
        runner=runner,
        timeout_seconds=30,
    )

    # Then
    assert result.ok is True
    assert result.backend is Backend.CODEX
    assert [call[0] for call in runner.calls] == ["codex"]
    assert "--approve-for-me" in runner.calls[0]


def test_unsupported_forced_backend_fails_before_spawning_agent() -> None:
    # Given
    runner = FakeRunner(outputs={"claude": _claude_output()})

    # When
    result = dispatch_query(
        "create the requested reminder",
        backend=Backend.CLAUDE,
        capability="slack_create_reminder",
        confirmation="write",
        runner=runner,
        timeout_seconds=30,
    )

    # Then
    assert result.ok is False
    assert runner.calls == []
