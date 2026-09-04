package issueopslease

import (
	"context"
	"errors"
	"testing"

	"issueops/internal/adapter/issueops"
)

func TestReseedHandlerRequiresService(t *testing.T) {
	_, err := NewReseedHandler(nil)(context.Background(), "/state", issueops.ExecutionReseedRequest{ID: "io-reseed"})
	if !errors.Is(err, issueops.ErrReseedHandlerUnavailable) {
		t.Fatalf("error=%v", err)
	}
}
