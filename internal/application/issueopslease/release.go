package issueopslease

import (
	"context"
	"fmt"
	"strings"

	leasecontract "agent-harness/internal/contract/issueopslease"
	"agent-harness/internal/domain/issueopslease"
)

type ReleaseRequest struct {
	ID         string
	Generation uint64
	Actor      issueopslease.Actor
	Ancestry   []issueopslease.ProcessReceipt
	CWD        string
}
type ReleaseResult struct {
	OK        bool
	ID        string
	Execution leasecontract.Execution
}

type ReleaseService struct {
	repository Repository
	clock      Clock
	inspect    ProcessInspector
	paths      CanonicalPathMatcher
}

func NewReleaseService(repository Repository, clock Clock, inspect ProcessInspector, paths CanonicalPathMatcher) *ReleaseService {
	return &ReleaseService{repository: repository, clock: clock, inspect: inspect, paths: paths}
}

func (s *ReleaseService) Release(ctx context.Context, request ReleaseRequest) (ReleaseResult, error) {
	actor, err := resolveActor(ctx, request.Actor, request.Ancestry, s.inspect)
	if err != nil {
		return ReleaseResult{ID: request.ID}, err
	}
	validate := func(before Record) error {
		canonicalCWD := s.paths != nil && s.paths.Matches(request.CWD, before.CanonicalRoot)
		release := issueopslease.ReleaseRequest{Generation: request.Generation, Actor: actor, AuthorityVerified: true, CanonicalCWD: canonicalCWD}
		return issueopslease.ValidateRelease(toDomainLease(before.Lease), release)
	}
	after, err := s.repository.Update(ctx, request.ID, validate, func(before Record) (Record, error) {
		outcome := issueopslease.ApplyRelease(s.clock.Now())
		before.Lease.Status = outcome.Status
		before.Lease.Holder = nil
		before.Lease.ClaimTokenSHA256 = ""
		before.Lease.ReleasedAt = outcome.ReleasedAt
		return before, nil
	})
	if err != nil {
		return ReleaseResult{ID: request.ID}, err
	}
	return ReleaseResult{OK: true, ID: request.ID, Execution: after.Execution}, nil
}

func toDomainLease(lease leasecontract.Lease) issueopslease.Lease {
	result := issueopslease.Lease{Generation: lease.Generation, Status: lease.Status}
	if lease.Holder != nil {
		result.Holder = &issueopslease.Actor{Host: lease.Holder.Host, SessionID: lease.Holder.SessionID, AgentID: lease.Holder.AgentID}
		if lease.Holder.SessionProcess != nil {
			result.Holder.Process = &issueopslease.ProcessReceipt{PID: lease.Holder.SessionProcess.PID, StartedAt: lease.Holder.SessionProcess.StartedAt, Executable: lease.Holder.SessionProcess.Executable}
		}
	}
	return result
}

func resolveActor(ctx context.Context, actor issueopslease.Actor, ancestry []issueopslease.ProcessReceipt, inspect ProcessInspector) (issueopslease.Actor, error) {
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
		return issueopslease.Actor{}, fmt.Errorf("native actor host must be codex or claude")
	}
	if actor.SessionID == "" {
		return issueopslease.Actor{}, fmt.Errorf("native actor session_id is required")
	}
	if actor.Process == nil || actor.Process.PID <= 0 || actor.Process.StartedAt == "" || actor.Process.Executable == "" {
		return issueopslease.Actor{}, fmt.Errorf("native actor requires a PID reuse-safe session_process receipt")
	}
	found := false
	for _, observed := range ancestry {
		if observed == *actor.Process {
			found = true
			break
		}
	}
	if !found {
		return issueopslease.Actor{}, fmt.Errorf("native session process receipt is not in the local process ancestry")
	}
	if inspect == nil {
		return issueopslease.Actor{}, fmt.Errorf("native process inspector is required")
	}
	status, observed, err := inspect(ctx, *actor.Process)
	if err != nil {
		return issueopslease.Actor{}, err
	}
	if status != "live" {
		return issueopslease.Actor{}, fmt.Errorf("native process identity is not live: pid=%d status=%s", actor.Process.PID, status)
	}
	if observed != *actor.Process {
		return issueopslease.Actor{}, fmt.Errorf("native process identity does not match live PID %d", actor.Process.PID)
	}
	return actor, nil
}
