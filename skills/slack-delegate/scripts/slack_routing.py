from __future__ import annotations

from dataclasses import dataclass

from slack_backends import BackendOutcome
from slack_models import (
    Backend,
    BackendFailure,
    CapabilityManifest,
    CapabilitySelection,
    Confirmation,
    DelegateResult,
    Risk,
    ToolName,
)


@dataclass(frozen=True, slots=True)
class SelectionError:
    message: str


def failed_result(backend: Backend, message: str) -> DelegateResult:
    return DelegateResult(
        ok=False,
        backend=backend,
        answer="",
        sources=(),
        error=message,
    )


def select_capability(
    capability_name: str | None,
    manifest: CapabilityManifest,
) -> CapabilitySelection | SelectionError:
    if capability_name is None:
        return CapabilitySelection(
            name=None,
            risk=Risk.READ,
            backends=manifest.backend_order,
        )
    capability = manifest.tools.get(capability_name)
    if capability is None:
        return SelectionError(
            message=f"Unsupported Slack capability: {capability_name}"
        )
    return CapabilitySelection(
        name=ToolName(capability_name),
        risk=capability.risk,
        backends=capability.backends,
    )


def confirmation_error(
    selection: CapabilitySelection,
    confirmation: Confirmation,
) -> str | None:
    allowed = {
        Risk.READ: (
            Confirmation.NONE,
            Confirmation.WRITE,
            Confirmation.DESTRUCTIVE,
        ),
        Risk.WRITE: (Confirmation.WRITE, Confirmation.DESTRUCTIVE),
        Risk.DESTRUCTIVE: (Confirmation.DESTRUCTIVE,),
    }[selection.risk]
    if confirmation in allowed:
        return None
    return {
        Risk.READ: "",
        Risk.WRITE: "Write capability requires explicit write confirmation",
        Risk.DESTRUCTIVE: (
            "Destructive capability requires explicit destructive confirmation"
        ),
    }[selection.risk]


def candidate_backends(
    requested: Backend,
    selection: CapabilitySelection,
) -> tuple[Backend, ...] | SelectionError:
    if requested is Backend.AUTO:
        if selection.risk is Risk.READ:
            return selection.backends
        return selection.backends[:1]
    if requested not in selection.backends:
        return SelectionError(
            message=f"Capability is unavailable on requested backend: {requested.value}"
        )
    return (requested,)


def read_tools(
    manifest: CapabilityManifest,
    backend: Backend,
) -> tuple[ToolName, ...]:
    return tuple(
        ToolName(name)
        for name, capability in manifest.tools.items()
        if capability.risk is Risk.READ and backend in capability.backends
    )


def as_failure(
    outcome: BackendOutcome,
    expected_backend: Backend,
) -> BackendFailure | None:
    if isinstance(outcome, BackendFailure):
        return outcome
    if outcome.ok and outcome.backend is expected_backend:
        return None
    if outcome.ok:
        return BackendFailure(
            backend=expected_backend,
            message=f"Backend identity mismatch: {outcome.backend.value}",
        )
    return BackendFailure(
        backend=outcome.backend,
        message=outcome.error or "Backend returned an unsuccessful result",
    )
