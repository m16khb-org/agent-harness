# /// script
# requires-python = ">=3.13"
# dependencies = [
#   "pydantic==2.11.7",
#   "typer==0.27.1",
# ]
# ///
# ─── How to run ───
# Read Slack through Claude first, with Codex fallback:
#   uv run slack_delegate.py "Find the latest deployment discussion"
# Perform one confirmed Slack mutation:
#   uv run slack_delegate.py --capability slack_send_message --confirm-write "Send ..."

from __future__ import annotations

from pathlib import Path
from typing import Annotated, Final

import typer
from pydantic import ValidationError
from slack_backends import Runner, SubprocessRunner, run_backend
from slack_models import (
    Backend,
    BackendFailure,
    CapabilityManifest,
    CapabilitySelection,
    Confirmation,
    DelegateResult,
    DispatchInput,
    ProcessOutput,
    QueryInput,
    load_manifest,
)
from slack_routing import (
    SelectionError,
    as_failure,
    candidate_backends,
    confirmation_error,
    failed_result,
    read_tools,
    select_capability,
)

__all__ = ["Backend", "ProcessOutput", "dispatch_query"]

SCRIPT_DIR: Final = Path(__file__).resolve().parent
SKILL_DIR: Final = SCRIPT_DIR.parent
SCHEMA_PATH: Final = SKILL_DIR / "result.schema.json"
MANIFEST_PATH: Final = SKILL_DIR / "capabilities.json"
WORKDIR: Final = Path("/tmp")


def _run_candidates(
    parsed: DispatchInput,
    *,
    manifest: CapabilityManifest,
    selection: CapabilitySelection,
    candidates: tuple[Backend, ...],
    runner: Runner,
) -> DelegateResult:
    failures: list[BackendFailure] = []
    query = QueryInput(text=parsed.request)
    for candidate in candidates:
        outcome = run_backend(
            query,
            backend=candidate,
            selection=selection,
            read_tools=read_tools(manifest, candidate),
            schema_path=SCHEMA_PATH,
            workdir=WORKDIR,
            runner=runner,
            timeout_seconds=parsed.timeout_seconds,
        )
        failure = as_failure(outcome, candidate)
        if failure is None:
            assert isinstance(outcome, DelegateResult)
            return outcome
        failures.append(failure)
    return failed_result(
        parsed.backend,
        " | ".join(
            f"{failure.backend.value}: {failure.message}" for failure in failures
        ),
    )


def _dispatch_selected(
    parsed: DispatchInput,
    *,
    manifest: CapabilityManifest,
    selection: CapabilitySelection,
    runner: Runner,
) -> DelegateResult:
    confirmation_failure = confirmation_error(selection, parsed.confirmation)
    if confirmation_failure is not None:
        return failed_result(parsed.backend, confirmation_failure)
    candidate_result = candidate_backends(parsed.backend, selection)
    if isinstance(candidate_result, SelectionError):
        return failed_result(parsed.backend, candidate_result.message)
    return _run_candidates(
        parsed,
        manifest=manifest,
        selection=selection,
        candidates=candidate_result,
        runner=runner,
    )


def _dispatch(
    parsed: DispatchInput,
    *,
    manifest: CapabilityManifest,
    runner: Runner,
) -> DelegateResult:
    selection_result = select_capability(parsed.capability, manifest)
    if isinstance(selection_result, SelectionError):
        return failed_result(parsed.backend, selection_result.message)
    return _dispatch_selected(
        parsed,
        manifest=manifest,
        selection=selection_result,
        runner=runner,
    )


def dispatch_query(
    request: str,
    *,
    backend: Backend,
    runner: Runner,
    timeout_seconds: float,
    capability: str | None = None,
    confirmation: Confirmation | str = Confirmation.NONE,
) -> DelegateResult:
    try:
        parsed = DispatchInput.model_validate(
            {
                "request": request,
                "backend": backend,
                "capability": capability,
                "confirmation": confirmation,
                "timeout_seconds": timeout_seconds,
            }
        )
    except ValidationError as error:
        return failed_result(backend, str(error))
    return _dispatch(parsed, manifest=load_manifest(MANIFEST_PATH), runner=runner)


app = typer.Typer(add_completion=False, pretty_exceptions_enable=False)


@app.command()
def main(
    request: Annotated[str, typer.Argument(help="Slack request")],
    backend: Annotated[
        Backend,
        typer.Option(help="Backend; auto prefers Claude"),
    ] = Backend.AUTO,
    capability: Annotated[
        str | None,
        typer.Option(help="Exact capability from capabilities.json"),
    ] = None,
    confirm_write: Annotated[
        bool,
        typer.Option(help="Confirm one additive Slack mutation"),
    ] = False,
    confirm_destructive: Annotated[
        bool,
        typer.Option(help="Confirm one destructive Slack mutation"),
    ] = False,
    timeout_seconds: Annotated[
        float,
        typer.Option(min=1, help="Per-backend timeout"),
    ] = 180,
) -> None:
    if confirm_write and confirm_destructive:
        raise typer.BadParameter("Choose only one confirmation flag")
    confirmation = {
        (False, False): Confirmation.NONE,
        (True, False): Confirmation.WRITE,
        (False, True): Confirmation.DESTRUCTIVE,
    }[(confirm_write, confirm_destructive)]
    result = dispatch_query(
        request,
        backend=backend,
        capability=capability,
        confirmation=confirmation,
        runner=SubprocessRunner(),
        timeout_seconds=timeout_seconds,
    )
    typer.echo(result.model_dump_json())
    if not result.ok:
        raise typer.Exit(code=1)


if __name__ == "__main__":
    app()
