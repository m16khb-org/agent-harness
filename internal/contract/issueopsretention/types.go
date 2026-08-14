package issueopsretention

import issueopscontract "agent-harness/internal/contract/issueops"

type Record = issueopscontract.IssueOpsRecord

type Result struct {
	OK         bool     `json:"ok"`
	StateRoot  string   `json:"state_root"`
	MaxAge     string   `json:"max_age"`
	Cutoff     string   `json:"cutoff"`
	DryRun     bool     `json:"dry_run"`
	Pruned     []string `json:"pruned"`
	Kept       []string `json:"kept"`
	Unreadable []string `json:"unreadable"`
}

const (
	LeaseStatusReleased = issueopscontract.LeaseStatusReleased
	PhaseDone           = issueopscontract.IssueOpsPhaseDone
)
