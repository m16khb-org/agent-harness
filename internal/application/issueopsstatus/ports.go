package issueopsstatus

import (
	"context"

	issueopsstatuscontract "agent-harness/internal/contract/issueopsstatus"
)

type Repository interface {
	Read(context.Context, string, string) (issueopsstatuscontract.Record, error)
}

type Projector interface {
	Project(issueopsstatuscontract.Record) issueopsstatuscontract.Record
}
