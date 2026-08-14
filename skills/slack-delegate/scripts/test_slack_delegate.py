# /// script
# requires-python = ">=3.13"
# dependencies = [
#   "pydantic==2.11.7",
#   "pytest==8.4.1",
#   "typer==0.27.1",
# ]
# ///
# ─── How to run ───
# Run the delegate unit tests:
#   uv run pytest -q test_slack_delegate.py

from __future__ import annotations

import json
from pathlib import Path

from slack_delegate import Backend, ProcessOutput, dispatch_query


def _result(backend: Backend) -> dict[str, bool | str | None | list[dict[str, str]]]:
    return {
        "ok": True,
        "backend": backend.value,
        "answer": "found",
        "sources": [
            {
                "channel": "self-dm",
                "author": "self",
                "timestamp": "1786415520.430339",
                "permalink": "https://example.slack.com/archives/D0123456789/p1786415520430339",
                "excerpt": "marker",
            }
        ],
        "error": None,
    }


def _claude_output(backend: Backend = Backend.CLAUDE) -> ProcessOutput:
    result = _result(backend)
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
        stdout=f"startup\n{json.dumps(_result(Backend.CODEX))}\ntokens used\n",
        stderr="",
    )


def _claude_error_output(message: str) -> ProcessOutput:
    return ProcessOutput(
        returncode=1,
        stdout=json.dumps(
            {
                "is_error": True,
                "result": message,
            }
        ),
        stderr="",
    )


class FakeRunner:
    """Record invocations because backend order is the observable contract."""

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


def test_auto_prefers_claude_when_claude_succeeds() -> None:
    # Given
    runner = FakeRunner(outputs={"claude": _claude_output(), "codex": _codex_output()})

    # When
    result = dispatch_query(
        "find marker",
        backend=Backend.AUTO,
        runner=runner,
        timeout_seconds=30,
    )

    # Then
    assert result.ok is True
    assert result.backend is Backend.CLAUDE
    assert [call[0] for call in runner.calls] == ["claude"]


def test_auto_falls_back_to_codex_when_claude_fails() -> None:
    # Given
    runner = FakeRunner(
        outputs={
            "claude": ProcessOutput(returncode=1, stdout="", stderr="failed"),
            "codex": _codex_output(),
        }
    )

    # When
    result = dispatch_query(
        "find marker",
        backend=Backend.AUTO,
        runner=runner,
        timeout_seconds=30,
    )

    # Then
    assert result.ok is True
    assert result.backend is Backend.CODEX
    assert [call[0] for call in runner.calls] == ["claude", "codex"]


def test_claude_failure_preserves_stdout_error_message() -> None:
    # Given
    runner = FakeRunner(outputs={"claude": _claude_error_output("session limit")})

    # When
    result = dispatch_query(
        "find marker",
        backend=Backend.CLAUDE,
        runner=runner,
        timeout_seconds=30,
    )

    # Then
    assert result.ok is False
    assert result.error == "claude: session limit"


def test_auto_reports_failure_when_both_backends_fail() -> None:
    # Given
    runner = FakeRunner(
        outputs={
            "claude": ProcessOutput(returncode=1, stdout="", stderr="claude failed"),
            "codex": ProcessOutput(returncode=1, stdout="", stderr="codex failed"),
        }
    )

    # When
    result = dispatch_query(
        "find marker",
        backend=Backend.AUTO,
        runner=runner,
        timeout_seconds=30,
    )

    # Then
    assert result.ok is False
    assert result.backend is Backend.AUTO
    assert result.error is not None
    assert [call[0] for call in runner.calls] == ["claude", "codex"]


def test_forced_backend_uses_only_requested_agent() -> None:
    # Given
    runner = FakeRunner(outputs={"claude": _claude_output(), "codex": _codex_output()})

    # When
    result = dispatch_query(
        "find marker",
        backend=Backend.CODEX,
        runner=runner,
        timeout_seconds=30,
    )

    # Then
    assert result.backend is Backend.CODEX
    assert [call[0] for call in runner.calls] == ["codex"]
