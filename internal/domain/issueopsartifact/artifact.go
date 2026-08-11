package issueopsartifact

import (
	"fmt"
	"strings"

	issueopsartifactcontract "agent-harness/internal/contract/issueopsartifact"
	"agent-harness/internal/domain/secretdetection"
)

func NormalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	switch name {
	case "plan", "spec", "turing-loop":
		return name, nil
	default:
		return "", fmt.Errorf("artifact name must be plan|spec|turing-loop")
	}
}

func ValidateContent(content []byte) error {
	if len(content) == 0 {
		return fmt.Errorf("artifact content is empty")
	}
	if len(content) > issueopsartifactcontract.MaxBytes {
		return fmt.Errorf("artifact exceeds %d bytes", issueopsartifactcontract.MaxBytes)
	}
	if secretdetection.Contains(string(content)) {
		return fmt.Errorf("artifact content contains secret-like values; redact them before staging")
	}
	return nil
}

func CanStage(record issueopsartifactcontract.Record, name string) bool {
	if record.Execution == nil {
		return true
	}
	execution := record.Execution
	return name == "plan" &&
		execution.Mode == issueopsartifactcontract.ExecutionModeOrca &&
		execution.Lease.Status == issueopsartifactcontract.LeaseStatusReleased &&
		execution.Lease.Holder == nil &&
		execution.Pending == nil &&
		execution.Completion == nil
}
