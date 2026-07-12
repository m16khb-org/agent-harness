package issueops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unicode"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/policy"
)

var publicationFullCommitPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
var publicationRemotePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,255}$`)

const publicationDiagnosticLimit = 4096

type IssueOpsHandoffPublishRequest struct {
	ID      string `json:"id"`
	Confirm bool   `json:"confirm,omitempty"`
}

type IssueOpsHandoffPublicationReader interface {
	LocalRefHead(context.Context, string, string) (string, error)
	RemoteRefHead(context.Context, string, string, string) (string, error)
	PushExact(context.Context, string, string, string, string) error
}

func (GitIssueOpsHandoffPublicationReader) PushExact(ctx context.Context, repo, remote, branch, finalHead string) error {
	if !publicationRemotePattern.MatchString(remote) || !safePublicationBranch(branch) || !publicationFullCommitPattern.MatchString(finalHead) {
		return fmt.Errorf("publication remote, branch, or final head is unsafe")
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	ref := "refs/heads/" + branch
	code, _, stderr := publicationGitCmd(bounded, repo, "push", "--", remote, finalHead+":"+ref)
	if code != 0 {
		return fmt.Errorf("push exact publication ref at %s: %s", finalHead, strings.TrimSpace(stderr))
	}
	return nil
}

type GitIssueOpsHandoffPublicationReader struct{}

func (GitIssueOpsHandoffPublicationReader) LocalRefHead(ctx context.Context, repo, ref string) (string, error) {
	if !safePublicationRef(ref) {
		return "", fmt.Errorf("local publication ref is unsafe")
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	code, stdout, stderr := publicationGitCmd(bounded, repo, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	if code != 0 {
		return "", fmt.Errorf("resolve local publication ref %s: %s", ref, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(stdout), nil
}

func (GitIssueOpsHandoffPublicationReader) RemoteRefHead(ctx context.Context, repo, remote, ref string) (string, error) {
	if !publicationRemotePattern.MatchString(remote) || !safePublicationRef(ref) {
		return "", fmt.Errorf("remote publication identity is unsafe")
	}
	bounded, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	code, stdout, stderr := publicationGitCmd(bounded, repo, "ls-remote", "--heads", "--", remote, ref)
	if code != 0 {
		return "", fmt.Errorf("resolve remote publication ref %s/%s: %s", remote, ref, strings.TrimSpace(stderr))
	}
	fields := strings.Fields(strings.TrimSpace(stdout))
	if len(fields) != 2 || fields[1] != ref {
		return "", fmt.Errorf("remote publication ref %s/%s did not resolve exactly once", remote, ref)
	}
	return fields[0], nil
}

func publicationGitCmd(ctx context.Context, repo string, args ...string) (int, string, string) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repo
	env := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_TERMINAL_PROMPT=") {
			env = append(env, entry)
		}
	}
	cmd.Env = append(env, "GIT_TERMINAL_PROMPT=0")
	stdout, stderr := &publicationBoundedBuffer{limit: publicationDiagnosticLimit}, &publicationBoundedBuffer{limit: publicationDiagnosticLimit}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err := cmd.Run()
	if err == nil {
		return 0, strings.TrimSpace(stdout.String()), publicationDiagnostic(stderr.String())
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), strings.TrimSpace(stdout.String()), publicationDiagnostic(stderr.String())
	}
	return 1, strings.TrimSpace(stdout.String()), publicationDiagnostic(err.Error())
}

type publicationBoundedBuffer struct {
	data      []byte
	limit     int
	truncated bool
}

func (b *publicationBoundedBuffer) Write(p []byte) (int, error) {
	original := len(p)
	remaining := b.limit - len(b.data)
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		b.data = append(b.data, p...)
	}
	if original > remaining {
		b.truncated = true
	}
	return original, nil
}

func (b *publicationBoundedBuffer) String() string {
	value := string(b.data)
	if b.truncated {
		value += "...[truncated]"
	}
	return value
}

func publicationDiagnostic(value string) string {
	value = policy.RedactFreeform(strings.TrimSpace(value))
	if len(value) > publicationDiagnosticLimit {
		value = value[:publicationDiagnosticLimit] + "...[truncated]"
	}
	return value
}

func safePublicationRef(ref string) bool {
	return strings.HasPrefix(ref, "refs/heads/") && safePublicationBranch(strings.TrimPrefix(ref, "refs/heads/"))
}

func safePublicationBranch(branch string) bool {
	return branch != "" && len(branch) <= 1024 && branch == strings.TrimSpace(branch) && strings.IndexFunc(branch, unicode.IsControl) < 0 && !strings.HasPrefix(branch, "-") && !strings.HasPrefix(branch, "/") && !strings.HasSuffix(branch, "/") && !strings.HasSuffix(branch, ".") && !strings.Contains(branch, "..") && !strings.Contains(branch, "@{") && !strings.ContainsAny(branch, " ~^:?*[\\")
}

type issueOpsPublicationIdentity struct {
	Provider  string
	Remote    string
	Branch    string
	Base      string
	LocalRef  string
	RemoteRef string
	FinalHead string
}

func RecordIssueOpsHandoffPublishReceipt(ctx context.Context, stateRoot string, req IssueOpsHandoffPublishRequest, reader IssueOpsHandoffPublicationReader, lease IssueOpsOrcaDispatchClient, clock IssueOpsHandoffPrepareClock) (IssueOpsRecord, error) {
	if !req.Confirm {
		return IssueOpsRecord{}, fmt.Errorf("publish receipt requires --confirm")
	}
	validated, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return IssueOpsRecord{}, err
	}
	identity, err := issueOpsAcceptedPublicationIdentity(validated)
	if err != nil {
		return IssueOpsRecord{}, err
	}
	if lease == nil {
		return IssueOpsRecord{}, fmt.Errorf("publication sole-writer dependency is unavailable")
	}
	validated, err = attestIssueOpsPublicationSoleWriter(ctx, stateRoot, validated, lease, issueOpsHandoffNow(clock))
	if err != nil {
		return IssueOpsRecord{}, err
	}
	if err := verifyIssueOpsLocalPublicationHead(ctx, validated.Repo, identity, reader); err != nil {
		return IssueOpsRecord{}, err
	}
	if validated.ExecutionHandoff.PublishReceipt != nil {
		if err := validateIssueOpsPublishReceipt(validated, identity); err != nil {
			return IssueOpsRecord{}, err
		}
		if err := verifyIssueOpsRemotePublicationHead(ctx, validated.Repo, identity, reader); err != nil {
			return IssueOpsRecord{}, err
		}
		return validated, nil
	}
	if err := reader.PushExact(ctx, validated.Repo, identity.Remote, identity.Branch, identity.FinalHead); err != nil {
		return IssueOpsRecord{}, err
	}
	if err := verifyIssueOpsRemotePublicationHead(ctx, validated.Repo, identity, reader); err != nil {
		return IssueOpsRecord{}, err
	}
	now := issueOpsHandoffNow(clock)
	var persisted IssueOpsRecord
	err = withIssueOpsLock(stateRoot, validated.ID, func() error {
		current, readErr := ReadIssueOps(stateRoot, validated.ID)
		if readErr != nil {
			return readErr
		}
		if !reflect.DeepEqual(current, validated) {
			return fmt.Errorf("accepted handoff changed during publication verification")
		}
		current.ExecutionHandoff.PublishReceipt = &model.IssueOpsExecutionHandoffPublishReceipt{
			Provider: identity.Provider, Remote: identity.Remote, Branch: identity.Branch,
			RemoteRef: identity.RemoteRef, FinalHead: identity.FinalHead, VerifiedAt: now,
		}
		current.ExecutionHandoff.UpdatedAt = now
		current.UpdatedAt = now
		persisted, readErr = writeIssueOps(stateRoot, current)
		return readErr
	})
	return persisted, err
}

func ValidateIssueOpsHandoffPublication(ctx context.Context, stateRoot string, record IssueOpsRecord, provider, head, base string, reader IssueOpsHandoffPublicationReader, lease IssueOpsOrcaDispatchClient) error {
	if err := handoff.ValidateEnvelope(record); err != nil {
		return err
	}
	identity, err := issueOpsAcceptedPublicationIdentity(record)
	if err != nil {
		return err
	}
	if lease == nil {
		return fmt.Errorf("publication sole-writer dependency is unavailable")
	}
	if strings.ToLower(strings.TrimSpace(provider)) != identity.Provider || strings.TrimSpace(head) != identity.Branch || strings.TrimSpace(base) != identity.Base {
		return fmt.Errorf("publication provider, head branch, or base branch differs from durable IssueOps authority")
	}
	current, err := attestIssueOpsPublicationSoleWriter(ctx, stateRoot, record, lease, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	if err := validateIssueOpsPublishReceipt(current, identity); err != nil {
		return err
	}
	return verifyIssueOpsPublicationHeads(ctx, current.Repo, identity, reader)
}

func attestIssueOpsPublicationSoleWriter(ctx context.Context, stateRoot string, expected IssueOpsRecord, lease IssueOpsOrcaDispatchClient, now string) (IssueOpsRecord, error) {
	err := attestHandoffSoleWriter(ctx, expected, lease, "")
	if err == nil && expected.ExecutionHandoff.PublicationRecovery == nil {
		return expected, nil
	}
	var recoveryErr handoffSoleWriterRecoveryError
	var conflictErr handoffSoleWriterConflictError
	recoveryCode := ""
	if errors.As(err, &recoveryErr) {
		recoveryCode = "publication_inventory_ambiguous"
	} else if errors.As(err, &conflictErr) {
		recoveryCode = "publication_writer_conflict"
	} else if err != nil {
		return expected, fmt.Errorf("publication sole-writer re-attestation failed: %w", err)
	}
	var persisted IssueOpsRecord
	persistErr := withIssueOpsLock(stateRoot, expected.ID, func() error {
		current, readErr := ReadIssueOps(stateRoot, expected.ID)
		if readErr != nil {
			return readErr
		}
		if !reflect.DeepEqual(current, expected) {
			return fmt.Errorf("accepted handoff changed during publication attestation")
		}
		if err == nil {
			current.ExecutionHandoff.PublicationRecovery = nil
		} else {
			current.ExecutionHandoff.PublicationRecovery = &model.IssueOpsExecutionHandoffFailure{Code: recoveryCode, Message: soleWriterRecoveryDiagnostic(err.Error()), At: now}
		}
		current.ExecutionHandoff.UpdatedAt = now
		current.UpdatedAt = now
		persisted, readErr = writeIssueOps(stateRoot, current)
		return readErr
	})
	if persistErr != nil {
		return expected, persistErr
	}
	if err != nil {
		return persisted, fmt.Errorf("publication sole-writer re-attestation failed: %w", err)
	}
	return persisted, nil
}

func issueOpsAcceptedPublicationIdentity(record IssueOpsRecord) (issueOpsPublicationIdentity, error) {
	h := record.ExecutionHandoff
	if h == nil || h.State != handoff.StateClosed || h.ClosedDisposition != handoff.DispositionAccepted || h.Result == nil || h.Orca == nil || record.BranchPrepare == nil {
		return issueOpsPublicationIdentity{}, fmt.Errorf("publication requires a closed accepted execution handoff")
	}
	provider := strings.ToLower(strings.TrimSpace(record.BranchPrepare.Provider))
	if provider != "github" && provider != "gitlab" {
		return issueOpsPublicationIdentity{}, fmt.Errorf("publication provider must be github or gitlab")
	}
	branch := strings.TrimSpace(record.Branch)
	base := strings.TrimSpace(record.BranchPrepare.BaseBranch)
	finalHead := strings.TrimSpace(h.Result.FinalHead)
	baseRef := strings.TrimSpace(h.Orca.BaseRef)
	prefix, suffix := "refs/remotes/", "/"+branch
	if branch == "" || base == "" || !publicationFullCommitPattern.MatchString(finalHead) || !strings.HasPrefix(baseRef, prefix) || !strings.HasSuffix(baseRef, suffix) {
		return issueOpsPublicationIdentity{}, fmt.Errorf("publication branch, base, final head, or remote authority is incomplete")
	}
	remote := strings.TrimSuffix(strings.TrimPrefix(baseRef, prefix), suffix)
	if remote == "" || strings.ContainsAny(remote, " \t\r\n") {
		return issueOpsPublicationIdentity{}, fmt.Errorf("publication remote authority is invalid")
	}
	return issueOpsPublicationIdentity{
		Provider: provider, Remote: remote, Branch: branch, Base: base,
		LocalRef: "refs/heads/" + branch, RemoteRef: "refs/heads/" + branch, FinalHead: finalHead,
	}, nil
}

func verifyIssueOpsPublicationHeads(ctx context.Context, repo string, identity issueOpsPublicationIdentity, reader IssueOpsHandoffPublicationReader) error {
	if err := verifyIssueOpsLocalPublicationHead(ctx, repo, identity, reader); err != nil {
		return err
	}
	return verifyIssueOpsRemotePublicationHead(ctx, repo, identity, reader)
}

func verifyIssueOpsLocalPublicationHead(ctx context.Context, repo string, identity issueOpsPublicationIdentity, reader IssueOpsHandoffPublicationReader) error {
	if reader == nil {
		return fmt.Errorf("publication ref verification dependency is unavailable")
	}
	localHead, err := reader.LocalRefHead(ctx, repo, identity.LocalRef)
	if err != nil {
		return err
	}
	if strings.TrimSpace(localHead) != identity.FinalHead {
		return fmt.Errorf("local publication ref head does not equal accepted final_head")
	}
	return nil
}

func verifyIssueOpsRemotePublicationHead(ctx context.Context, repo string, identity issueOpsPublicationIdentity, reader IssueOpsHandoffPublicationReader) error {
	remoteHead, err := reader.RemoteRefHead(ctx, repo, identity.Remote, identity.RemoteRef)
	if err != nil {
		return err
	}
	if strings.TrimSpace(remoteHead) != identity.FinalHead {
		return fmt.Errorf("remote publication ref head does not equal accepted final_head")
	}
	return nil
}

func validateIssueOpsPublishReceipt(record IssueOpsRecord, identity issueOpsPublicationIdentity) error {
	receipt := record.ExecutionHandoff.PublishReceipt
	if receipt == nil {
		return fmt.Errorf("durable publication receipt is required")
	}
	if receipt.Provider != identity.Provider || receipt.Remote != identity.Remote || receipt.Branch != identity.Branch || receipt.RemoteRef != identity.RemoteRef || receipt.FinalHead != identity.FinalHead || strings.TrimSpace(receipt.VerifiedAt) == "" {
		return fmt.Errorf("durable publication receipt does not match current accepted authority")
	}
	return nil
}
