package issueopslease

import (
	"context"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
)

func TestResumeArtifactsCallsReaderOnce(t *testing.T) {
	called := 0
	adapter := NewResumeArtifacts(func(context.Context, leasecontract.Record) (leasecontract.ResumeArtifacts, error) {
		called++
		return leasecontract.ResumeArtifacts{ContextPacketSHA256: "packet"}, nil
	})
	artifacts, err := adapter.ReadAndVerify(context.Background(), resumeRepositoryRecord(t, 1))
	if err != nil || artifacts.ContextPacketSHA256 != "packet" || called != 1 {
		t.Fatalf("artifacts=%+v called=%d err=%v", artifacts, called, err)
	}
}
