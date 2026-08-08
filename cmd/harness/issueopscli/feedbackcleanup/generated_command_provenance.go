package feedbackcleanup

import (
	"context"

	provenanceapp "agent-harness/internal/application/issueopsprovenance"
	provenanceport "agent-harness/internal/port/issueopsprovenance"
)

func bindCleanupNextCommand(command string, generation uint64, observer provenanceport.Observer) (string, error) {
	return provenanceapp.Bind(context.Background(), command, generation, observer)
}
