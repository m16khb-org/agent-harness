package executiondecision

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/model"
)

type Store struct {
	Read       func(string, string) (model.IssueOpsRecord, error)
	TouchWrite func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error)
}

var validSubagentPatterns = map[string]bool{
	"high-volume-exploration":           true,
	"isolated-worktree-work":            true,
	"forked-context-exploration":        true,
	"devils-advocate-review":            true,
	"cross-verification-consensus":      true,
	"parallel-independent-research":     true,
	"task-fan-out-coordination":         true,
	"background-long-running-work":      true,
	"model-specialization-cost-routing": true,
	"tool-permission-gating":            true,
	"plan-then-execute-separation":      true,
	"triage-specialist-routing":         true,
}

var validSubagentBenefits = map[string]bool{
	"context_isolation":    true,
	"parallel_speed":       true,
	"fresh_review":         true,
	"tool_gating":          true,
	"long_running":         true,
	"model_specialization": true,
	"isolated_worktree":    true,
}

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(api[_-]?key|apikey|secret|token|password|passwd|credential|private[_-]?key)\s*[:=]\s*\S+`),
	regexp.MustCompile(`-----BEGIN (RSA |EC |DSA |OPENSSH |PGP )?PRIVATE KEY-----`),
	regexp.MustCompile(`(?i)(ghp|gho|ghu|ghs|ghr|glpat|gldt|glft)_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`eyJ[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{10,}`),
}

func Record(store Store, stateRoot, id string, req model.IssueOpsExecutionDecisionRecordRequest) (model.IssueOpsRecord, error) {
	decision, err := Validate(req)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	record.ExecutionDecision = &decision
	return store.TouchWrite(stateRoot, record)
}

func Validate(req model.IssueOpsExecutionDecisionRecordRequest) (model.IssueOpsExecutionDecision, error) {
	autoProceed, err := cleanRequiredList("auto_proceed", req.AutoProceed)
	if err != nil {
		return model.IssueOpsExecutionDecision{}, err
	}
	hookBlocked, err := cleanRequiredList("hook_blocked", req.HookBlocked)
	if err != nil {
		return model.IssueOpsExecutionDecision{}, err
	}
	humanGates, err := cleanRequiredList("human_gates", req.HumanGates)
	if err != nil {
		return model.IssueOpsExecutionDecision{}, err
	}
	subagentUse := strings.TrimSpace(req.SubagentUse)
	switch subagentUse {
	case "none":
		if strings.TrimSpace(req.SubagentRationale) == "" {
			return model.IssueOpsExecutionDecision{}, fmt.Errorf("subagent_rationale is required when subagent_use=none")
		}
		if len(req.SubagentPlans) > 0 {
			return model.IssueOpsExecutionDecision{}, fmt.Errorf("subagent_plans must be empty when subagent_use=none")
		}
	case "planned":
		if len(req.SubagentPlans) == 0 {
			return model.IssueOpsExecutionDecision{}, fmt.Errorf("subagent_plans is required when subagent_use=planned")
		}
	default:
		return model.IssueOpsExecutionDecision{}, fmt.Errorf("invalid subagent_use %q; must be none or planned", subagentUse)
	}
	if containsSecretPattern(req.SubagentRationale) {
		return model.IssueOpsExecutionDecision{}, fmt.Errorf("execution decision appears to contain secrets or credentials; redact them before storing")
	}
	plans, err := validateSubagentPlans(req.SubagentPlans)
	if err != nil {
		return model.IssueOpsExecutionDecision{}, err
	}
	return model.IssueOpsExecutionDecision{
		AutoProceed:       autoProceed,
		HookBlocked:       hookBlocked,
		HumanGates:        humanGates,
		SubagentUse:       subagentUse,
		SubagentRationale: strings.TrimSpace(req.SubagentRationale),
		SubagentPlans:     plans,
		RecordedAt:        time.Now().UTC().Format(time.RFC3339Nano),
	}, nil
}

func cleanRequiredList(field string, values []string) ([]string, error) {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if containsSecretPattern(value) {
			return nil, fmt.Errorf("%s appears to contain secrets or credentials; redact it before storing", field)
		}
		out = append(out, value)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s requires at least one entry", field)
	}
	return out, nil
}

func validateSubagentPlans(plans []model.IssueOpsSubAgentPlan) ([]model.IssueOpsSubAgentPlan, error) {
	out := make([]model.IssueOpsSubAgentPlan, 0, len(plans))
	for i, plan := range plans {
		cleaned := model.IssueOpsSubAgentPlan{
			Objective:            strings.TrimSpace(plan.Objective),
			Pattern:              strings.TrimSpace(plan.Pattern),
			Benefit:              strings.TrimSpace(plan.Benefit),
			Scope:                strings.TrimSpace(plan.Scope),
			Verification:         strings.TrimSpace(plan.Verification),
			Fallback:             strings.TrimSpace(plan.Fallback),
			NetPositiveRationale: strings.TrimSpace(plan.NetPositiveRationale),
		}
		for field, value := range map[string]string{
			"objective":              cleaned.Objective,
			"pattern":                cleaned.Pattern,
			"benefit":                cleaned.Benefit,
			"scope":                  cleaned.Scope,
			"verification":           cleaned.Verification,
			"fallback":               cleaned.Fallback,
			"net_positive_rationale": cleaned.NetPositiveRationale,
		} {
			if value == "" {
				return nil, fmt.Errorf("subagent_plans[%d].%s is required", i, field)
			}
			if containsSecretPattern(value) {
				return nil, fmt.Errorf("subagent_plans[%d].%s appears to contain secrets or credentials; redact it before storing", i, field)
			}
		}
		if !validSubagentPatterns[cleaned.Pattern] {
			return nil, fmt.Errorf("invalid subagent pattern %q", cleaned.Pattern)
		}
		if !validSubagentBenefits[cleaned.Benefit] {
			return nil, fmt.Errorf("invalid subagent benefit %q", cleaned.Benefit)
		}
		tradeoffs, err := cleanRequiredList(fmt.Sprintf("subagent_plans[%d].tradeoffs", i), plan.Tradeoffs)
		if err != nil {
			return nil, err
		}
		cleaned.Tradeoffs = tradeoffs
		if delegatesMainAgentJudgement(cleaned) {
			return nil, fmt.Errorf("subagent plan must not delegate safety, reversibility, or user-intent alignment judgement")
		}
		out = append(out, cleaned)
	}
	return out, nil
}

func delegatesMainAgentJudgement(plan model.IssueOpsSubAgentPlan) bool {
	text := strings.ToLower(strings.Join([]string{
		plan.Objective,
		plan.Scope,
		plan.Verification,
		plan.Fallback,
	}, " "))
	for _, phrase := range []string{
		"delegate safety",
		"decide safety",
		"judge safety",
		"delegate reversibility",
		"decide reversibility",
		"judge reversibility",
		"user intent alignment",
		"user-intent alignment",
		"decide user intent",
		"judge user intent",
	} {
		if strings.Contains(text, phrase) {
			return true
		}
	}
	return false
}

func containsSecretPattern(s string) bool {
	for _, pat := range secretPatterns {
		if pat.MatchString(s) {
			return true
		}
	}
	return false
}
