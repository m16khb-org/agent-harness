package workpool

const WorkPoolCurrentSchemaVersion = 1

type WorkPool struct {
	OK            bool   `json:"ok"`
	SchemaVersion int    `json:"schema_version"`
	ID            string `json:"id"`
	Repo          string `json:"repo"`
	Name          string `json:"name"`
	ParentCycleID string `json:"parent_cycle_id,omitempty"`
	Size          int    `json:"size"`
	LeaseTTL      string `json:"lease_ttl"`
	MaxAttempts   int    `json:"max_attempts"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type WorkTask struct {
	OK                 bool     `json:"ok"`
	SchemaVersion      int      `json:"schema_version"`
	ID                 string   `json:"id"`
	PoolID             string   `json:"pool_id"`
	Title              string   `json:"title"`
	Instructions       string   `json:"instructions"`
	Scope              []string `json:"scope,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
	Status             string   `json:"status"`
	WorkerID           string   `json:"worker_id,omitempty"`
	LeaseExpiresAt     string   `json:"lease_expires_at,omitempty"`
	LastHeartbeatAt    string   `json:"last_heartbeat_at,omitempty"`
	Attempts           int      `json:"attempts"`
	Branch             string   `json:"branch,omitempty"`
	WorktreePath       string   `json:"worktree_path,omitempty"`
	Evidence           []string `json:"evidence,omitempty"`
	SubmittedAt        string   `json:"submitted_at,omitempty"`
	RejectReason       string   `json:"reject_reason,omitempty"`
	CreatedAt          string   `json:"created_at"`
	UpdatedAt          string   `json:"updated_at"`
}

type CreatePoolRequest struct {
	Repo          string
	Name          string
	ParentCycleID string
	Size          int
	LeaseTTL      string
	MaxAttempts   int
}

type AddTaskRequest struct {
	Title              string
	Instructions       string
	Scope              []string
	AcceptanceCriteria []string
}

type StatusResult struct {
	OK     bool           `json:"ok"`
	Pool   WorkPool       `json:"pool"`
	Tasks  []WorkTask     `json:"tasks"`
	Counts map[string]int `json:"counts"`
	Reaped []WorkTask     `json:"reaped,omitempty"`
}
