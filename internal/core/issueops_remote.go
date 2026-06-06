package core

import (
	"encoding/json"
	"fmt"
	"time"
)

const defaultIssueOpsRemoteThreshold = 0.70

type IssueOpsRemoteArtifact struct {
	Provider string `json:"provider,omitempty"`
	Title    string `json:"title"`
	Body     string `json:"body"`
}

type IssueOpsRemoteIssueCandidate struct {
	ID     string   `json:"id"`
	Title  string   `json:"title"`
	Body   string   `json:"body,omitempty"`
	URL    string   `json:"url,omitempty"`
	State  string   `json:"state,omitempty"`
	Labels []string `json:"labels,omitempty"`
	Score  *float64 `json:"score,omitempty"`
}

type IssueOpsRemoteLabelCandidate struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Aliases     []string `json:"aliases,omitempty"`
	Score       *float64 `json:"score,omitempty"`
}

type IssueOpsRemoteScoringRequest struct {
	Provider        string                         `json:"provider,omitempty"`
	Threshold       float64                        `json:"threshold,omitempty"`
	Issue           IssueOpsRemoteArtifact         `json:"issue"`
	IssueCandidates []IssueOpsRemoteIssueCandidate `json:"issue_candidates,omitempty"`
	LabelCandidates []IssueOpsRemoteLabelCandidate `json:"label_candidates,omitempty"`
}

func DecodeIssueOpsRemoteScoringRequest(data []byte) (IssueOpsRemoteScoringRequest, error) {
	var req IssueOpsRemoteScoringRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return req, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return req, err
	}
	if _, canonical := raw["issue_candidates"]; canonical {
		if _, alias := raw["related_issues"]; alias {
			return req, fmt.Errorf("use either issue_candidates or related_issues, not both")
		}
	} else if alias, ok := raw["related_issues"]; ok {
		if err := json.Unmarshal(alias, &req.IssueCandidates); err != nil {
			return req, fmt.Errorf("parse related_issues: %w", err)
		}
	}
	if _, canonical := raw["label_candidates"]; canonical {
		if _, alias := raw["labels"]; alias {
			return req, fmt.Errorf("use either label_candidates or labels, not both")
		}
	} else if alias, ok := raw["labels"]; ok {
		if err := json.Unmarshal(alias, &req.LabelCandidates); err != nil {
			return req, fmt.Errorf("parse labels: %w", err)
		}
	}
	return req, nil
}

type IssueOpsRemoteScoredItem struct {
	ID           string   `json:"id,omitempty"`
	Name         string   `json:"name,omitempty"`
	Title        string   `json:"title,omitempty"`
	URL          string   `json:"url,omitempty"`
	Score        float64  `json:"score"`
	Threshold    float64  `json:"threshold"`
	Selected     bool     `json:"selected"`
	Evidence     []string `json:"evidence"`
	ApplyHint    string   `json:"apply_hint,omitempty"`
	RejectReason string   `json:"reject_reason,omitempty"`
}

type IssueOpsRemoteScoringResult struct {
	OK                    bool                       `json:"ok"`
	Provider              string                     `json:"provider"`
	Threshold             float64                    `json:"threshold"`
	ExecutionClass        string                     `json:"execution_class,omitempty"`
	ReadOnly              bool                       `json:"read_only,omitempty"`
	JoinBefore            string                     `json:"join_before,omitempty"`
	SelectedRelatedIssues []IssueOpsRemoteScoredItem `json:"selected_related_issues"`
	RejectedRelatedIssues []IssueOpsRemoteScoredItem `json:"rejected_related_issues"`
	SelectedLabels        []IssueOpsRemoteScoredItem `json:"selected_labels"`
	RejectedLabels        []IssueOpsRemoteScoredItem `json:"rejected_labels"`
	ApplyInstructions     []string                   `json:"apply_instructions"`
	Warnings              []string                   `json:"warnings"`
}

type IssueOpsRemoteAgyJudgeRequest struct {
	RepoRoot   string
	AgyCommand string
	Timeout    time.Duration
	Attempts   int
	Request    IssueOpsRemoteScoringRequest
}
