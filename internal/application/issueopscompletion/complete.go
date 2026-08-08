package issueopscompletion

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/url"
	"slices"
	"strings"

	completioncontract "agent-harness/internal/contract/issueopscompletion"
	completiondomain "agent-harness/internal/domain/issueopscompletion"
)

type Request struct {
	ID                string
	Generation        uint64
	Actor             completioncontract.Actor
	Ancestry          []completioncontract.ProcessReceipt
	CWD               string
	FinalHead         string
	TuringReportPath  string
	Verification      []string
	RemoteArtifactURL string
	Confirm           bool
}

type Result struct {
	OK              bool
	ID              string
	Execution       completioncontract.Execution
	OrcaTaskSettled bool
	OrcaTaskError   string
}

type Service struct {
	repository  Repository
	environment Environment
	clock       Clock
	inspect     ProcessInspector
	settler     TaskSettler
}

func NewService(repository Repository, environment Environment, clock Clock, inspect ProcessInspector, settler TaskSettler) *Service {
	return &Service{repository: repository, environment: environment, clock: clock, inspect: inspect, settler: settler}
}

func (s *Service) Complete(ctx context.Context, request Request) (Result, error) {
	if s == nil || s.repository == nil || s.environment == nil || s.clock == nil || s.inspect == nil {
		return Result{ID: request.ID}, fmt.Errorf("completion dependencies are required")
	}
	actor, err := resolveActor(ctx, request.Actor, request.Ancestry, s.inspect)
	if err != nil {
		return Result{ID: request.ID}, err
	}
	if !request.Confirm {
		return Result{ID: request.ID}, fmt.Errorf("execution complete requires confirm")
	}
	verification, err := normalizeVerification(request.Verification)
	if err != nil {
		return Result{ID: request.ID}, err
	}
	if err := validateRemoteArtifactURL(request.RemoteArtifactURL); err != nil {
		return Result{ID: request.ID}, err
	}
	command := completioncontract.Command{
		Generation: request.Generation, Actor: actor, FinalHead: request.FinalHead,
		TuringReportPath: request.TuringReportPath, Verification: verification,
		RemoteArtifactURL: request.RemoteArtifactURL,
	}
	persisted, err := s.repository.Update(ctx, request.ID, func(before completioncontract.RecordSnapshot) (completioncontract.RecordSnapshot, bool, error) {
		if !before.Prepared {
			return before, false, completioncontract.ErrExecutionNotPrepared
		}
		if before.Completion != nil {
			if terminalCompletionMatches(before, command, s.environment) && s.environment.VerifyArtifact(before, request.RemoteArtifactURL) == nil {
				return before, false, nil
			}
			return before, false, fmt.Errorf("execution completion already exists with different evidence")
		}
		if before.Phase != "pr" {
			return before, false, fmt.Errorf("execution completion requires pr phase")
		}
		if err := s.environment.VerifyArtifact(before, request.RemoteArtifactURL); err != nil {
			return before, false, err
		}
		canonicalCWD := s.environment.PathsMatch(request.CWD, before.CanonicalRoot)
		if err := completiondomain.ValidateActive(toDomainSnapshot(before), command, canonicalCWD); err != nil {
			return before, false, publicDomainError(err, request.Generation)
		}
		head, err := s.environment.CurrentHead(ctx, before.CanonicalRoot)
		if err != nil {
			return before, false, err
		}
		if !validFullCommitSHA(request.FinalHead) || !strings.EqualFold(strings.TrimSpace(request.FinalHead), head) {
			return before, false, fmt.Errorf("final_head must match canonical worktree HEAD")
		}
		report, err := s.environment.VerifyReport(before.CanonicalRoot, request.TuringReportPath)
		if err != nil {
			return before, false, err
		}
		completedAt := s.clock.Now()
		transitionedAt := s.clock.Now()
		outcome := completiondomain.ApplyAt(toDomainSnapshot(before), command, report, completedAt, transitionedAt)
		return fromDomainOutcome(before, outcome), true, nil
	})
	if err != nil {
		return Result{ID: request.ID}, err
	}
	result := Result{OK: true, ID: request.ID, Execution: persisted.Execution}
	settle(persisted.Record, s.settler, &result)
	return result, nil
}

func resolveActor(ctx context.Context, actor completioncontract.Actor, ancestry []completioncontract.ProcessReceipt, inspect ProcessInspector) (completioncontract.Actor, error) {
	actor.Host = strings.ToLower(strings.TrimSpace(actor.Host))
	actor.SessionID = strings.TrimSpace(actor.SessionID)
	actor.AgentID = strings.TrimSpace(actor.AgentID)
	if actor.Process != nil {
		process := *actor.Process
		process.StartedAt = strings.TrimSpace(process.StartedAt)
		process.Executable = strings.TrimSpace(process.Executable)
		actor.Process = &process
	}
	if actor.Host != "codex" && actor.Host != "claude" {
		return completioncontract.Actor{}, fmt.Errorf("native actor host must be codex or claude")
	}
	if actor.SessionID == "" {
		return completioncontract.Actor{}, fmt.Errorf("native actor session_id is required")
	}
	if actor.Process == nil || actor.Process.PID <= 0 || actor.Process.StartedAt == "" || actor.Process.Executable == "" {
		return completioncontract.Actor{}, fmt.Errorf("native actor requires a PID reuse-safe session_process receipt")
	}
	found := false
	for _, observed := range ancestry {
		if observed == *actor.Process {
			found = true
			break
		}
	}
	if !found {
		return completioncontract.Actor{}, fmt.Errorf("native session process receipt is not in the local process ancestry")
	}
	status, observed, err := inspect(ctx, *actor.Process)
	if err != nil {
		return completioncontract.Actor{}, err
	}
	if status != "live" {
		return completioncontract.Actor{}, fmt.Errorf("native process identity is not live: pid=%d status=%s", actor.Process.PID, status)
	}
	if observed != *actor.Process {
		return completioncontract.Actor{}, fmt.Errorf("native process identity does not match live PID %d", actor.Process.PID)
	}
	return actor, nil
}

func normalizeVerification(values []string) ([]string, error) {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("verification entries must be nonempty")
		}
		result = append(result, value)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("execution completion requires verification evidence")
	}
	return result, nil
}

func validateRemoteArtifactURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.Path == "" {
		return fmt.Errorf("execution completion requires an HTTPS draft PR or MR URL")
	}
	return nil
}

func validFullCommitSHA(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func terminalCompletionMatches(record completioncontract.RecordSnapshot, command completioncontract.Command, environment Environment) bool {
	completion := record.Completion
	return record.Phase == "done" && record.Lease.Generation == command.Generation && record.Lease.Status == "released" &&
		record.Lease.Holder == nil && record.Lease.ClaimTokenSHA256 == "" && strings.TrimSpace(record.Lease.ReleasedAt) != "" &&
		completion != nil && strings.TrimSpace(completion.CompletedAt) != "" &&
		(completion.Generation == 0 || completion.Generation == command.Generation) &&
		strings.EqualFold(completion.FinalHead, strings.TrimSpace(command.FinalHead)) &&
		environment.PathsMatch(completion.TuringReportPath, command.TuringReportPath) &&
		slices.Equal(completion.Verification, command.Verification) &&
		completion.RemoteArtifactURL == strings.TrimSpace(command.RemoteArtifactURL)
}

func toDomainSnapshot(record completioncontract.RecordSnapshot) completiondomain.Snapshot {
	return completiondomain.Snapshot{Phase: record.Phase, Lease: record.Lease, Completion: record.Completion, Ledger: record.Ledger}
}

func fromDomainOutcome(before completioncontract.RecordSnapshot, outcome completiondomain.Outcome) completioncontract.RecordSnapshot {
	result := before.Clone()
	result.Phase = outcome.Phase
	result.Lease = outcome.Lease
	result.Completion = outcome.Completion
	result.Ledger = outcome.Ledger
	return result
}

func publicDomainError(err error, generation uint64) error {
	switch completiondomain.CodeOf(err) {
	case completiondomain.DenyAuthority:
		return fmt.Errorf("only the current holder may complete generation %d", generation)
	case completiondomain.DenyCWD:
		return fmt.Errorf("completion cwd must be the canonical worktree")
	default:
		return err
	}
}

func settle(record completioncontract.RecordSnapshot, settler TaskSettler, result *Result) {
	if settler == nil || record.Mode != "orca" || record.Orca == nil || strings.TrimSpace(record.Orca.TaskID) == "" {
		return
	}
	if err := settler.Settle(context.Background(), strings.TrimSpace(record.Orca.RunID), strings.TrimSpace(record.Orca.TaskID)); err != nil {
		result.OrcaTaskError = err.Error()
		return
	}
	result.OrcaTaskSettled = true
}
