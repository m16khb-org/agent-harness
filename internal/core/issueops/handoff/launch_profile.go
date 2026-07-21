package handoff

import (
	"fmt"
	"strings"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/port"
)

func ResolveAgentLaunchProfile(agent string) (model.IssueOpsAgentLaunchProfile, error) {
	agent = strings.ToLower(strings.TrimSpace(agent))
	switch agent {
	case "codex":
		return model.IssueOpsAgentLaunchProfile{Agent: agent, Model: port.IssueOpsCodexModel, ReasoningEffort: port.IssueOpsCodexReasoningEffort}, nil
	case "claude":
		return model.IssueOpsAgentLaunchProfile{Agent: agent, Model: port.IssueOpsClaudeModel}, nil
	case "gjc":
		return model.IssueOpsAgentLaunchProfile{Agent: agent}, nil
	default:
		return model.IssueOpsAgentLaunchProfile{}, fmt.Errorf("unsupported handoff agent %q", agent)
	}
}
