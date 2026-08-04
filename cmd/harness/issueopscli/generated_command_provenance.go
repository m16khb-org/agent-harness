package issueopscli

import (
	"context"
	"strings"

	issueopscontract "agent-harness/internal/contract/issueops"
	"agent-harness/internal/core"
	issueopscore "agent-harness/internal/core/issueops"
)

func prepareGeneratedCommandInvocation(args []string, deps Dependencies) ([]string, bool, error) {
	clean, expected, present, err := issueopscontract.ConsumeGeneratedCommandProvenance(args)
	if err != nil || !present {
		return clean, false, err
	}
	id := issueOpsCommandID(clean)
	if id == "" {
		return nil, true, &issueopscontract.GeneratedCommandProvenanceError{
			Code: "generated_command_provenance_invalid", Message: "generated command provenance requires an IssueOps id",
		}
	}
	record, err := issueopscore.ReadIssueOps(core.IssueOpsStateRoot(), id)
	if err != nil {
		return nil, true, err
	}
	if record.Execution == nil {
		return nil, true, &issueopscontract.GeneratedCommandProvenanceError{
			Code: "generated_command_provenance_invalid", Message: "generated command provenance requires IssueOps execution state",
		}
	}
	if deps.Provenance == nil {
		return nil, true, issueopscontract.NewGeneratedCommandProvenanceObservationError(nil)
	}
	receipt, err := deps.Provenance.Observe(context.Background())
	if err != nil {
		return nil, true, issueopscontract.NewGeneratedCommandProvenanceObservationError(err)
	}
	observed := issueopscontract.GeneratedCommandProvenance{
		ExecutablePath:   receipt.ExecutablePath,
		ExecutableSHA256: receipt.ExecutableSHA256,
		LeaseGeneration:  record.Execution.Lease.Generation,
	}
	if err := issueopscontract.ValidateGeneratedCommandInvocation(expected, observed, record.Execution.Lease.Generation); err != nil {
		return nil, true, err
	}
	return clean, true, nil
}

func issueOpsCommandID(args []string) string {
	idFlag := "--id"
	if len(args) >= 2 && args[0] == "child" {
		idFlag = "--parent"
	}
	for index := 0; index < len(args); index++ {
		if args[index] == idFlag && index+1 < len(args) {
			return strings.TrimSpace(args[index+1])
		}
		if strings.HasPrefix(args[index], idFlag+"=") {
			return strings.TrimSpace(strings.TrimPrefix(args[index], idFlag+"="))
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
