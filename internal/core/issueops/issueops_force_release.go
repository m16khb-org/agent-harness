package issueops

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/session"
)

type ForceReleaseCASRequest struct {
	ExpectedRawSHA256       string `json:"expected_raw_sha256"`
	ExpectedCanonicalSHA256 string `json:"expected_canonical_sha256"`
}

type ForceReleaseCASResult struct {
	OK                     bool           `json:"ok"`
	Record                 IssueOpsRecord `json:"record"`
	BeforeRawSHA256        string         `json:"before_raw_sha256"`
	BeforeCanonicalSHA256  string         `json:"before_canonical_sha256"`
	AfterRawSHA256         string         `json:"after_raw_sha256"`
	AfterCanonicalSHA256   string         `json:"after_canonical_sha256"`
	RepoBindingCountBefore int            `json:"repo_binding_count_before"`
	RepoBindingCountAfter  int            `json:"repo_binding_count_after"`
	BindingAbsenceVerified bool           `json:"binding_absence_verified"`
}

func ForceReleaseIssueOps(stateRoot, id, reason string) (IssueOpsRecord, error) {
	var rec IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		var e error
		rec, e = forceReleaseLocked(stateRoot, id, reason)
		return e
	})
	if err == nil {
		unbindIssueOpsSessionForCycle(rec)
	}
	return rec, err
}

// ForceReleaseIssueOpsCAS is the sealed-recovery variant of force-release.
// One state-root span lock covers the exact raw/canonical record comparison,
// normalized-repository binding absence check, record mutation, and proof
// readback. Unlike ordinary force-release, it deliberately skips post-span
// unbind: an unsealed binding can make the operation fail or survive, but this
// path never deletes one.
func ForceReleaseIssueOpsCAS(stateRoot, id, reason string, req ForceReleaseCASRequest) (ForceReleaseCASResult, error) {
	result := ForceReleaseCASResult{}
	if err := validateForceReleaseCASDigest("expected raw SHA-256", req.ExpectedRawSHA256); err != nil {
		return result, err
	}
	if err := validateForceReleaseCASDigest("expected canonical SHA-256", req.ExpectedCanonicalSHA256); err != nil {
		return result, err
	}
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		raw, err := readRawIssueOpsBytes(stateRoot, id)
		if err != nil {
			return err
		}
		beforeRaw, beforeCanonical, err := computeForceReleaseCASDigests(raw)
		if err != nil {
			return err
		}
		if beforeRaw != req.ExpectedRawSHA256 {
			return fmt.Errorf("force-release CAS raw SHA-256 differs from sealed expectation")
		}
		if beforeCanonical != req.ExpectedCanonicalSHA256 {
			return fmt.Errorf("force-release CAS canonical SHA-256 differs from sealed expectation")
		}
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		bindings, err := forceReleaseRepoBindings(stateRoot, record.Repo)
		if err != nil {
			return err
		}
		if len(bindings) != 0 {
			return fmt.Errorf("force-release CAS requires zero session bindings for the target repository")
		}
		released, err := forceReleaseLocked(stateRoot, id, reason)
		if err != nil {
			return err
		}
		afterRawBytes, err := readRawIssueOpsBytes(stateRoot, id)
		if err != nil {
			return err
		}
		afterRaw, afterCanonical, err := computeForceReleaseCASDigests(afterRawBytes)
		if err != nil {
			return err
		}
		afterBindings, err := forceReleaseRepoBindings(stateRoot, record.Repo)
		if err != nil {
			return err
		}
		if len(afterBindings) != 0 {
			return fmt.Errorf("force-release CAS binding absence changed inside the locked span")
		}
		result = ForceReleaseCASResult{
			OK:                     true,
			Record:                 released,
			BeforeRawSHA256:        beforeRaw,
			BeforeCanonicalSHA256:  beforeCanonical,
			AfterRawSHA256:         afterRaw,
			AfterCanonicalSHA256:   afterCanonical,
			RepoBindingCountBefore: len(bindings),
			RepoBindingCountAfter:  len(afterBindings),
			BindingAbsenceVerified: true,
		}
		return nil
	})
	return result, err
}

func validateForceReleaseCASDigest(label, value string) error {
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != sha256.Size || value != strings.ToLower(value) {
		return fmt.Errorf("%s must be 64 lowercase hexadecimal characters", label)
	}
	return nil
}

func computeForceReleaseCASDigests(raw []byte) (string, string, error) {
	rawSum := sha256.Sum256(raw)
	var payload any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return "", "", fmt.Errorf("decode force-release CAS source: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return "", "", fmt.Errorf("decode force-release CAS source: trailing JSON value")
	}
	// Match the recovery manifest's compact, key-sorted UTF-8 JSON contract.
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(payload); err != nil {
		return "", "", fmt.Errorf("encode force-release CAS canonical source: %w", err)
	}
	canonical := bytes.TrimSuffix(encoded.Bytes(), []byte("\n"))
	canonicalSum := sha256.Sum256(canonical)
	return hex.EncodeToString(rawSum[:]), hex.EncodeToString(canonicalSum[:]), nil
}

func forceReleaseRepoBindings(stateRoot, repo string) ([]session.Binding, error) {
	store := session.Store{StateRoot: func() string { return stateRoot }}
	bindings, err := session.ListAllExisting(store)
	if err != nil {
		return nil, err
	}
	normalized := filepath.Clean(strings.TrimSpace(repo))
	matching := make([]session.Binding, 0)
	for _, binding := range bindings {
		if filepath.Clean(strings.TrimSpace(binding.Repo)) == normalized {
			matching = append(matching, binding)
		}
	}
	return matching, nil
}

// forceReleaseLocked performs the force-release mutation without acquiring the
// per-id lock. Callers must hold withIssueOpsLock for stateRoot+id before
// calling this function (e.g. ScanStaleIssueOpsCycles which holds the lock
// across re-read+classify+release to close the TOCTOU window).
func forceReleaseLocked(stateRoot, id, reason string) (IssueOpsRecord, error) {
	reason = strings.TrimSpace(reason)
	if len(reason) < 10 {
		return IssueOpsRecord{OK: false}, fmt.Errorf("force-release reason must be at least 10 characters")
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	if h := record.ExecutionHandoff; h != nil && h.ProtocolVersion == handoff.OwnershipTransferProtocolVersion && h.State != handoff.StateClosed {
		return IssueOpsRecord{OK: false}, fmt.Errorf("force-release cannot change a non-closed ownership-transfer handoff; use the explicit human cleanup flow")
	}
	if w := record.ExecutionWorkspace; w != nil && w.State != handoff.StateRecoveryRequired {
		return IssueOpsRecord{OK: false}, fmt.Errorf("force-release cannot change a live execution workspace; use explicit workspace recovery")
	}
	if record.Phase == IssueOpsPhaseDone {
		return record, nil
	}
	activeChildren, err := issueOpsActiveChildIDs(stateRoot, record)
	if err != nil {
		return IssueOpsRecord{OK: false}, err
	}
	record.Phase = IssueOpsPhaseDone
	record.ForceReleasedAt = time.Now().UTC().Format(time.RFC3339Nano)
	record.ForceReleaseReason = issueOpsAppendActiveChildrenAudit(reason, activeChildren)
	// Preserve the existing worktree path as an orphan audit marker so the
	// off-hot-path stale-scan reaper can later run git worktree prune/remove.
	// Do NOT synchronously delete the directory — it may contain uncommitted work.
	if strings.TrimSpace(record.WorktreePath) != "" && strings.TrimSpace(record.OrphanWorktreePath) == "" {
		record.OrphanWorktreePath = record.WorktreePath
	}
	return touchAndWriteIssueOps(stateRoot, record)
}
