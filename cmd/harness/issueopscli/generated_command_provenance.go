package issueopscli

import (
	"context"
	"strings"

	issueopscontract "agent-harness/internal/contract/issueops"
	"agent-harness/internal/core"
	issueopscore "agent-harness/internal/core/issueops"
)

func prepareGeneratedCommandInvocation(args []string, deps Dependencies) ([]string, error) {
	clean, expected, present, err := issueopscontract.ConsumeGeneratedCommandProvenance(args)
	if err != nil || !present {
		return clean, err
	}
	id := issueOpsCommandID(clean)
	if id == "" {
		return nil, &issueopscontract.GeneratedCommandProvenanceError{
			Code: "generated_command_provenance_invalid", Message: "generated command provenance requires an IssueOps id",
		}
	}
	record, err := issueopscore.ReadIssueOps(core.IssueOpsStateRoot(), id)
	if err != nil {
		return nil, err
	}
	if record.Execution == nil {
		return nil, &issueopscontract.GeneratedCommandProvenanceError{
			Code: "generated_command_provenance_invalid", Message: "generated command provenance requires IssueOps execution state",
		}
	}
	if deps.Provenance == nil {
		return nil, issueopscontract.NewGeneratedCommandProvenanceObservationError(nil)
	}
	receipt, err := deps.Provenance.Observe(context.Background())
	if err != nil {
		return nil, issueopscontract.NewGeneratedCommandProvenanceObservationError(err)
	}
	observed := issueopscontract.GeneratedCommandProvenance{
		ExecutablePath:   receipt.ExecutablePath,
		ExecutableSHA256: receipt.ExecutableSHA256,
		LeaseGeneration:  record.Execution.Lease.Generation,
	}
	if err := issueopscontract.ValidateGeneratedCommandInvocation(expected, observed, record.Execution.Lease.Generation); err != nil {
		return nil, err
	}
	return clean, nil
}

func issueOpsCommandID(args []string) string {
	for index := 0; index < len(args); index++ {
		if args[index] == "--id" && index+1 < len(args) {
			return strings.TrimSpace(args[index+1])
		}
		if strings.HasPrefix(args[index], "--id=") {
			return strings.TrimSpace(strings.TrimPrefix(args[index], "--id="))
		}
	}
	return ""
}

func issueOpsJSONRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--json" {
			return true
		}
	}
	return false
}
