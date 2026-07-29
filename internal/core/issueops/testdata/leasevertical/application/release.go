package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/testdata/leasevertical/domain"
)

type Repository interface {
	Update(context.Context, string, func(domain.Record) (domain.Record, error)) (domain.Record, error)
}

type Clock interface {
	Now() time.Time
}

type ProcessInspector func(context.Context, domain.ProcessReceipt) (string, domain.ProcessReceipt, error)

// CanonicalPathMatcher는 application이 필요한 경로 동치 판단만 표현한다.
// 실제 파일시스템 관찰은 outbound adapter가 소유한다.
type CanonicalPathMatcher interface {
	Matches(string, string) bool
}

type ReleaseRequest struct {
	ID         string
	Generation uint64
	Actor      domain.Actor
	CWD        string
}

type ReleaseResult struct {
	OK        bool
	ID        string
	Execution domain.Execution
}

type ReleaseService struct {
	repository Repository
	clock      Clock
	inspect    ProcessInspector
	paths      CanonicalPathMatcher
}

func NewReleaseService(
	repository Repository,
	clock Clock,
	inspect ProcessInspector,
	paths CanonicalPathMatcher,
) *ReleaseService {
	return &ReleaseService{repository: repository, clock: clock, inspect: inspect, paths: paths}
}

func (s *ReleaseService) Release(ctx context.Context, request ReleaseRequest) (ReleaseResult, error) {
	actor, err := resolveActor(ctx, request.Actor, s.inspect)
	if err != nil {
		return ReleaseResult{ID: request.ID}, domain.Deny(domain.DenyLeaseAuthority, err)
	}
	after, err := s.repository.Update(ctx, request.ID, func(before domain.Record) (domain.Record, error) {
		canonicalCWD := s.paths != nil && s.paths.Matches(request.CWD, before.Execution.Workspace.Root)
		releaseRequest := domain.ReleaseRequest{
			Generation:        request.Generation,
			Actor:             actor,
			AuthorityVerified: true,
			CanonicalCWD:      canonicalCWD,
		}
		if err := domain.ValidateRelease(before, releaseRequest); err != nil {
			return domain.Record{}, err
		}
		return domain.ApplyRelease(before, s.clock.Now()), nil
	})
	if err != nil {
		return ReleaseResult{ID: request.ID}, err
	}
	return ReleaseResult{OK: true, ID: request.ID, Execution: after.Execution}, nil
}

func resolveActor(ctx context.Context, actor domain.Actor, inspect ProcessInspector) (domain.Actor, error) {
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
		return domain.Actor{}, fmt.Errorf("native actor host must be codex or claude")
	}
	if actor.SessionID == "" || actor.Process == nil || actor.Process.PID <= 0 ||
		actor.Process.StartedAt == "" || actor.Process.Executable == "" {
		return domain.Actor{}, fmt.Errorf("native actor receipt is incomplete")
	}
	locallyObserved := false
	for _, observed := range actor.Ancestry {
		if observed == *actor.Process {
			locallyObserved = true
			break
		}
	}
	if !locallyObserved {
		return domain.Actor{}, fmt.Errorf("native session process receipt is not in the local process ancestry")
	}
	if inspect == nil {
		return domain.Actor{}, fmt.Errorf("native process inspector is required")
	}
	status, observed, err := inspect(ctx, *actor.Process)
	if err != nil {
		return domain.Actor{}, err
	}
	if status != "live" {
		return domain.Actor{}, fmt.Errorf("native process identity is not live: pid=%d status=%s", actor.Process.PID, status)
	}
	if observed != *actor.Process {
		return domain.Actor{}, fmt.Errorf("native process identity does not match live PID %d", actor.Process.PID)
	}
	actor.Ancestry = nil
	return actor, nil
}
