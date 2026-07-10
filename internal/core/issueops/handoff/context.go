package handoff

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/policy"
)

const (
	ContextVersion          = 1
	MaxRenderedContextBytes = 64 * 1024
)

type ContextOptions struct {
	CriteriaIDs          []string
	RequiredDocs         []string
	RequiredSkills       []string
	WorkerScope          string
	VerificationCommands []string
	HeartbeatCadence     string
	StopConditions       []string
	ResultFormat         string
}

type ContextProjection struct {
	Version               int      `json:"version"`
	CycleID               string   `json:"cycle_id"`
	IssueURL              string   `json:"issue_url,omitempty"`
	Branch                string   `json:"branch"`
	BaseBranch            string   `json:"base_branch,omitempty"`
	BaseSHA               string   `json:"base_sha,omitempty"`
	WorktreePath          string   `json:"worktree_path"`
	PlanPath              string   `json:"plan_path"`
	PlanSHA256            string   `json:"plan_sha256"`
	Attempt               int      `json:"attempt"`
	OwnershipEpoch        string   `json:"ownership_epoch"`
	Problem               string   `json:"problem,omitempty"`
	Intent                string   `json:"intent,omitempty"`
	SuccessCriteria       []string `json:"success_criteria,omitempty"`
	Constraints           []string `json:"constraints,omitempty"`
	NonGoals              []string `json:"non_goals,omitempty"`
	Design                string   `json:"design,omitempty"`
	Alternatives          []string `json:"alternatives,omitempty"`
	Risks                 []string `json:"risks,omitempty"`
	BackwardCompatibility []string `json:"backward_compatibility,omitempty"`
	SideEffects           []string `json:"side_effects,omitempty"`
	RollbackPlan          string   `json:"rollback_plan,omitempty"`
	BrooksFindings        []string `json:"brooks_findings,omitempty"`
	CriteriaIDs           []string `json:"criteria_ids,omitempty"`
	RequiredDocs          []string `json:"required_docs,omitempty"`
	RequiredSkills        []string `json:"required_skills,omitempty"`
	WorkerScope           string   `json:"worker_scope,omitempty"`
	VerificationCommands  []string `json:"verification_commands,omitempty"`
	HeartbeatCadence      string   `json:"heartbeat_cadence,omitempty"`
	StopConditions        []string `json:"stop_conditions,omitempty"`
	ResultFormat          string   `json:"result_format,omitempty"`
}

type ContextPacket struct {
	Version    int               `json:"version"`
	SHA256     string            `json:"sha256"`
	PlanSHA256 string            `json:"plan_sha256"`
	Projection ContextProjection `json:"projection"`
	Markdown   string            `json:"markdown"`
}

func BuildContext(record model.IssueOpsRecord, options ContextOptions) (ContextPacket, error) {
	if record.ExecutionHandoff == nil {
		return ContextPacket{}, fmt.Errorf("execution handoff is required")
	}
	planPath := strings.TrimSpace(record.PlanPath)
	plan, err := os.ReadFile(planPath)
	if err != nil {
		return ContextPacket{}, fmt.Errorf("read handoff plan: %w", err)
	}
	planHash := sha256.Sum256(plan)
	projection := ContextProjection{
		Version:              ContextVersion,
		CycleID:              strings.TrimSpace(record.ID),
		IssueURL:             strings.TrimSpace(record.IssueURL),
		Branch:               strings.TrimSpace(record.Branch),
		WorktreePath:         strings.TrimSpace(record.WorktreePath),
		PlanPath:             planPath,
		PlanSHA256:           hex.EncodeToString(planHash[:]),
		Attempt:              record.ExecutionHandoff.Attempt,
		OwnershipEpoch:       strings.TrimSpace(record.ExecutionHandoff.OwnershipEpoch),
		CriteriaIDs:          cleanList(options.CriteriaIDs),
		RequiredDocs:         cleanList(options.RequiredDocs),
		RequiredSkills:       cleanList(options.RequiredSkills),
		WorkerScope:          redact(options.WorkerScope),
		VerificationCommands: cleanList(options.VerificationCommands),
		HeartbeatCadence:     redact(options.HeartbeatCadence),
		StopConditions:       cleanList(options.StopConditions),
		ResultFormat:         redact(options.ResultFormat),
	}
	if record.BranchPrepare != nil {
		projection.BaseBranch = strings.TrimSpace(record.BranchPrepare.BaseBranch)
		projection.BaseSHA = strings.TrimSpace(record.BranchPrepare.BaseSHA)
	}
	if record.Intent != nil {
		projection.Problem = redact(record.Intent.RawRequest)
		projection.Intent = redact(record.Intent.InterpretedIntent)
		projection.SuccessCriteria = cleanList(record.Intent.SuccessCriteria)
		projection.Constraints = cleanList(record.Intent.Constraints)
		projection.NonGoals = cleanList(record.Intent.NonGoals)
	}
	if record.DesignReview != nil {
		projection.Design = redact(record.DesignReview.ProposedDesign)
		projection.Alternatives = cleanList(record.DesignReview.Alternatives)
		projection.Risks = cleanList(record.DesignReview.Risks)
	}
	if record.CompatibilityReview != nil {
		projection.BackwardCompatibility = cleanList(record.CompatibilityReview.BackwardCompatibility)
		projection.SideEffects = cleanList(record.CompatibilityReview.SideEffects)
		projection.RollbackPlan = redact(record.CompatibilityReview.RollbackPlan)
	}
	if record.DevilsAdvocateReview != nil {
		projection.BrooksFindings = cleanList(record.DevilsAdvocateReview.Findings)
	}
	canonical, err := json.Marshal(projection)
	if err != nil {
		return ContextPacket{}, err
	}
	sum := sha256.Sum256(canonical)
	pretty, err := json.MarshalIndent(projection, "", "  ")
	if err != nil {
		return ContextPacket{}, err
	}
	markdown := "# IssueOps supervised execution handoff\n\n```json\n" + string(pretty) + "\n```\n"
	if len(markdown) > MaxRenderedContextBytes {
		return ContextPacket{}, fmt.Errorf("handoff context exceeds %d bytes", MaxRenderedContextBytes)
	}
	return ContextPacket{
		Version:    ContextVersion,
		SHA256:     hex.EncodeToString(sum[:]),
		PlanSHA256: projection.PlanSHA256,
		Projection: projection,
		Markdown:   markdown,
	}, nil
}

func cleanList(values []string) []string {
	seen := map[string]struct{}{}
	clean := make([]string, 0, len(values))
	for _, value := range values {
		value = redact(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		clean = append(clean, value)
	}
	sort.Strings(clean)
	return clean
}

func redact(value string) string {
	return strings.TrimSpace(policy.RedactFreeform(strings.TrimSpace(value)))
}
