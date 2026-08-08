// Package issueopspreparation owns the pure execution-preparation decision matrix.
package issueopspreparation

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

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
	Available bool
	Ready     bool
	Code      string
	Provider  string
	Issue     int
}

type DecisionInput struct {
	Command  preparationcontract.Command
	Snapshot preparationcontract.Snapshot
	Orca     OrcaReadiness
}

type Decision struct {
	Code                 Code
	RequestedMode        string
	ResolvedMode         string
	FallbackCode         string
	ProbeAttempted       bool
	ProbeAvailable       bool
	ProbeReady           bool
	ProbeCode            string
	ProbeProvider        string
	ProbeIssue           int
	ReadinessFingerprint string
	ExplicitDirectReason string
	RootConflict         *preparationcontract.RootClaim
}

type DenialReason string

const (
	DenialInvalidMode                 DenialReason = "invalid_mode"
	DenialOrcaUnavailable             DenialReason = "orca_unavailable"
	DenialDirectReasonRequired        DenialReason = "direct_reason_required"
	DenialInvalidDirectReason         DenialReason = "invalid_direct_reason"
	DenialReadinessFingerprintChanged DenialReason = "readiness_fingerprint_changed"
)

type Denial struct {
	Reason DenialReason
	Code   string
	Cause  error
}

func (denial *Denial) Error() string { return denial.Cause.Error() }
func (denial *Denial) Unwrap() error { return denial.Cause }

func (denial *Denial) IssueOpsErrorFields() map[string]any {
	fields := map[string]any{"code": string(denial.Reason)}
	if denial.Code != "" {
		fields["probe_code"] = denial.Code
	}
	return fields
}

func DenialReasonOf(err error) DenialReason {
	if denial, ok := errors.AsType[*Denial](err); ok {
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
		reason, reasonErr := NormalizeDirectReason(input.Command.DirectReason)
		if reasonErr != nil {
			reasonKind := DenialInvalidDirectReason
			if strings.TrimSpace(input.Command.DirectReason) == "" {
				reasonKind = DenialDirectReasonRequired
			}
			return Decision{}, &Denial{Reason: reasonKind, Cause: reasonErr}
		}
		decision.ResolvedMode = preparationcontract.ModeDirect
		decision.ExplicitDirectReason = reason
		decision.Code = preparationCode(input.Command.Confirm, false)
		decision.ReadinessFingerprint = Fingerprint(decision, input.Command)
		return decision, nil
	}
	decision.ProbeAttempted = true
	decision.ProbeAvailable = input.Orca.Available
	decision.ProbeReady = input.Orca.Available && input.Orca.Ready
	decision.ProbeCode = strings.TrimSpace(input.Orca.Code)
	decision.ProbeProvider = strings.ToLower(strings.TrimSpace(input.Orca.Provider))
	decision.ProbeIssue = input.Orca.Issue
	if decision.ProbeReady && decision.ProbeCode == "" {
		decision.ProbeCode = "ready"
	}
	if decision.ProbeReady {
		decision.ResolvedMode = preparationcontract.ModeOrca
		decision.Code = preparationCode(input.Command.Confirm, true)
		decision.ReadinessFingerprint = Fingerprint(decision, input.Command)
		return decision, nil
	}
	code := strings.TrimSpace(input.Orca.Code)
	if code == "" {
		code = "orca_probe_failed"
	}
	if requested == preparationcontract.ModeAuto {
		decision.ResolvedMode = preparationcontract.ModeDirect
		decision.FallbackCode = code
		decision.ProbeCode = code
		decision.Code = preparationCode(input.Command.Confirm, false)
		decision.ReadinessFingerprint = Fingerprint(decision, input.Command)
		return decision, nil
	}
	message := "Orca probe failed: " + code
	if code == "orca_adapter_unavailable" {
		message = "Orca provisioner is unavailable"
	}
	return decision, &Denial{Reason: DenialOrcaUnavailable, Code: code, Cause: errors.New(message)}
}

func NormalizeDirectReason(reason string) (string, error) {
	normalized := strings.TrimSpace(reason)
	if normalized == "" {
		return "", fmt.Errorf("explicit direct mode requires a reason")
	}
	if !utf8.ValidString(normalized) || len([]byte(normalized)) > 512 {
		return "", fmt.Errorf("explicit direct reason must be valid UTF-8 and at most 512 bytes")
	}
	for _, value := range []byte(normalized) {
		if value < 0x20 || value == 0x7f {
			return "", fmt.Errorf("explicit direct reason must not contain ASCII control bytes")
		}
	}
	return normalized, nil
}

func Fingerprint(decision Decision, command preparationcontract.Command) string {
	values := []string{decision.RequestedMode, decision.ResolvedMode, strconv.FormatBool(decision.ProbeAttempted), strconv.FormatBool(decision.ProbeAvailable), strconv.FormatBool(decision.ProbeReady), decision.ProbeCode, decision.FallbackCode, decision.ExplicitDirectReason}
	if decision.ProbeAttempted {
		values = append(values, strings.ToLower(strings.TrimSpace(command.OwnerHost)), strings.TrimSpace(command.OwnerModel), strings.TrimSpace(command.OwnerEffort), decision.ProbeProvider, strconv.Itoa(decision.ProbeIssue))
	}
	var projection strings.Builder
	for _, value := range values {
		projection.WriteString(strconv.Itoa(len(value)))
		projection.WriteByte(':')
		projection.WriteString(value)
		projection.WriteByte('|')
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(projection.String())))
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
