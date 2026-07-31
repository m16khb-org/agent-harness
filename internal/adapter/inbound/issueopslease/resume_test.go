package issueopslease

import (
	"context"
	"errors"
	"testing"

	"agent-harness/internal/core/issueops"
)

func TestResumeHandlerFailsClosedWithoutService(t *testing.T) {
	result, err := NewResumeHandler(nil)(context.Background(), "state", issueops.ExecutionResumeRequest{ID: "io-resume"})
	if !errors.Is(err, issueops.ErrResumeHandlerUnavailable) || result.ID != "io-resume" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
