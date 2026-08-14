package doctor

import (
	lifecyclecontract "agent-harness/internal/contract/lifecycle"
	operationalhealthcontract "agent-harness/internal/contract/operationalhealth"
)

type ProjectLifecycleStatePlan = lifecyclecontract.ProjectLifecycleStatePlan

type HarnessDoctorRequest struct {
	RepoRoot            string                              `json:"repo_root,omitempty"`
	HarnessRoot         string                              `json:"harness_root,omitempty"`
	Home                string                              `json:"home,omitempty"`
	Version             string                              `json:"version,omitempty"`
	StaticOnly          bool                                `json:"-"`
	DaemonAdmission     HarnessDoctorDaemonAdmission        `json:"daemon_admission,omitempty"`
	OperationalSnapshot *operationalhealthcontract.Snapshot `json:"-"`
	OperationalOptions  operationalhealthcontract.Options   `json:"-"`
}

type HarnessDoctorDaemonAdmission struct {
	Observed          bool `json:"observed"`
	ActiveConnections int  `json:"active_connections"`
	MaxConnections    int  `json:"max_connections"`
	Accepting         bool `json:"accepting"`
	Draining          bool `json:"draining"`
}

type HarnessDoctorResult struct {
	OK                bool                      `json:"ok"`
	Healthy           bool                      `json:"healthy"`
	Kind              string                    `json:"kind"`
	Version           string                    `json:"version,omitempty"`
	HarnessRoot       string                    `json:"harness_root,omitempty"`
	RepoRoot          string                    `json:"repo_root"`
	StateDir          string                    `json:"state_dir"`
	LifecycleState    ProjectLifecycleStatePlan `json:"lifecycle_state"`
	PipeCapacityBytes int                       `json:"pipe_capacity_bytes"`
	ActiveConnections int                       `json:"active_connections"`
	MaxConnections    int                       `json:"max_connections"`
	Accepting         bool                      `json:"accepting"`
	Draining          bool                      `json:"draining"`
	Checks            []HarnessDoctorCheck      `json:"checks"`
	Issues            []HarnessDoctorIssue      `json:"issues"`
	GeneratedAt       string                    `json:"generated_at"`
}

type HarnessDoctorCheck struct {
	Name    string `json:"name"`
	Healthy bool   `json:"healthy"`
	Summary string `json:"summary"`
}

type HarnessDoctorIssue struct {
	Code     string            `json:"code"`
	Severity string            `json:"severity"`
	Summary  string            `json:"summary"`
	Path     string            `json:"path,omitempty"`
	Fix      *HarnessDoctorFix `json:"fix,omitempty"`
}

type HarnessDoctorFix struct {
	Command     string `json:"command,omitempty"`
	Destructive bool   `json:"destructive"`
	Description string `json:"description"`
}
