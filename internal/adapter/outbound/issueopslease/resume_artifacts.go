package issueopslease

import (
	"context"
	"fmt"

	leasecontract "agent-harness/internal/contract/issueopslease"
)

type ResumeArtifactReader func(context.Context, leasecontract.Record) (leasecontract.ResumeArtifacts, error)

type ResumeArtifacts struct{ read ResumeArtifactReader }

func NewResumeArtifacts(read ResumeArtifactReader) *ResumeArtifacts {
	return &ResumeArtifacts{read: read}
}

func (a *ResumeArtifacts) ReadAndVerify(ctx context.Context, record leasecontract.Record) (leasecontract.ResumeArtifacts, error) {
	if a == nil || a.read == nil {
		return leasecontract.ResumeArtifacts{}, fmt.Errorf("resume artifact reader is required")
	}
	return a.read(ctx, record)
}
