// Package issueopspreparation owns the pure execution-preparation decision matrix.
package issueopspreparation

import (
	"errors"
	"fmt"
	"strings"

	preparationcontract "agent-harness/internal/contract/issueopspreparation"
)

type Code string

const (
	CodePreviewDirect    Code = "preview_direct"
	CodePreviewOrca      Code = "preview_orca"
	CodeApplyDirect      Code = "apply_direct"
	CodeApplyOrca        Code = "apply_orca"
	CodeExisting         Code = "existing"
	CodePendingReconcile Code = "pending_reconcile"
	CodeModeMismatch     Code = "mode_mismatch"
	CodeWriterless       Code = "writerless"
	CodeRootConflict     Code = "root_conflict"
)

type OrcaReadiness struct {
	Ready bool
	Code  string
}

type DecisionInput struct {
	Command  preparationcontract.Command
	Snapshot preparationcontract.Snapshot
	Orca     OrcaReadiness
}

type Decision struct {
	Code          Code
	RequestedMode string
	ResolvedMode  string
	FallbackCode  string
	RootConflict  *preparationcontract.RootClaim
}

type DenialReason string

const (
	DenialInvalidMode     DenialReason = "invalid_mode"
	DenialOrcaUnavailable DenialReason = "orca_unavailable"
)

type Denial struct {
	Reason DenialReason
	Code   string
	Cause  error
}

func (denial *Denial) Error() string { return denial.Cause.Error() }
func (denial *Denial) Unwrap() error { return denial.Cause }

func DenialReasonOf(err error) DenialReason {
	var denial *Denial
	if errors.As(err, &denial) {
		return denial.Reason
	}
	return ""
}

func Decide(input DecisionInput) (Decision, error) {
	requested, err := NormalizeMode(input.Command.Mode)
	if err != nil {
		return Decision{}, &Denial{Reason: DenialInvalidMode, Cause: err}
	}
	decision := Decision{RequestedMode: requested}
	if execution := input.Snapshot.Record.Execution; execution != nil {
		decision.ResolvedMode = execution.Mode
		switch {
		case execution.Pending != nil:
			decision.Code = CodePendingReconcile
		case requested != preparationcontract.ModeAuto && requested != execution.Mode:
			decision.Code = CodeModeMismatch
		case input.Command.Confirm && writerless(execution.Lease.Status):
			decision.Code = CodeWriterless
		default:
			decision.Code = CodeExisting
		}
		return decision, nil
	}
	if input.Snapshot.RootConflict != nil {
		claim := *input.Snapshot.RootConflict
		decision.Code = CodeRootConflict
		decision.RootConflict = &claim
		return decision, nil
	}
	if requested == preparationcontract.ModeDirect {
		decision.ResolvedMode = preparationcontract.ModeDirect
		decision.Code = preparationCode(input.Command.Confirm, false)
		return decision, nil
	}
	if input.Orca.Ready {
		decision.ResolvedMode = preparationcontract.ModeOrca
		decision.Code = preparationCode(input.Command.Confirm, true)
		return decision, nil
	}
	code := strings.TrimSpace(input.Orca.Code)
	if code == "" {
		code = "orca_probe_failed"
	}
	if requested == preparationcontract.ModeAuto {
		decision.ResolvedMode = preparationcontract.ModeDirect
		decision.FallbackCode = code
		decision.Code = preparationCode(input.Command.Confirm, false)
		return decision, nil
	}
	message := "Orca probe failed: " + code
	if code == "orca_adapter_unavailable" {
		message = "Orca provisioner is unavailable"
	}
	return decision, &Denial{Reason: DenialOrcaUnavailable, Code: code, Cause: errors.New(message)}
}

func NormalizeMode(mode string) (string, error) {
	switch normalized := strings.ToLower(strings.TrimSpace(mode)); normalized {
	case "", preparationcontract.ModeAuto:
		return preparationcontract.ModeAuto, nil
	case preparationcontract.ModeDirect, preparationcontract.ModeOrca:
		return normalized, nil
	default:
		return "", fmt.Errorf("execution mode must be auto, direct, or orca")
	}
}

func writerless(status string) bool {
	switch status {
	case "claimable", "released", "revoking":
		return true
	default:
		return false
	}
}

func preparationCode(confirm, orca bool) Code {
	if confirm {
		if orca {
			return CodeApplyOrca
		}
		return CodeApplyDirect
	}
	if orca {
		return CodePreviewOrca
	}
	return CodePreviewDirect
}
