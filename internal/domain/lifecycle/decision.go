package lifecycle

import (
	"strings"

	lifecyclecontract "agent-harness/internal/contract/lifecycle"
)

type Action string

const (
	ActionAllow Action = "allow"
	ActionBlock Action = "block"
)

type Decision struct {
	Action Action
	Reason string
}

func DecidePreToolUse(deny *lifecyclecontract.IssueOpsDenyReason) Decision {
	if deny == nil {
		return Decision{Action: ActionAllow}
	}
	reason := strings.TrimSpace(deny.Reason)
	if reason == "" {
		reason = strings.TrimSpace(deny.Code)
	}
	return Decision{Action: ActionBlock, Reason: reason}
}
