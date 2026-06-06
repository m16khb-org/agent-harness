package core

const StateCurrentSchemaVersion = 1

type StateRecord struct {
	SchemaVersion int    `json:"schema_version,omitempty"`
	Key           string `json:"key"`
	Content       string `json:"content"`
	UpdatedAt     string `json:"updated_at"`
	Bytes         int    `json:"bytes"`
}

type StateResult struct {
	OK       bool        `json:"ok"`
	StateDir string      `json:"state_dir"`
	Path     string      `json:"path,omitempty"`
	Record   StateRecord `json:"record"`
}

type StateListEntry struct {
	Key           string `json:"key"`
	UpdatedAt     string `json:"updated_at"`
	Bytes         int    `json:"bytes"`
	SchemaVersion int    `json:"schema_version"`
}

type StateListResult struct {
	OK       bool             `json:"ok"`
	StateDir string           `json:"state_dir"`
	Keys     []string         `json:"keys"`
	Records  []StateListEntry `json:"records"`
}

type StatePruneResult struct {
	OK          bool             `json:"ok"`
	StateDir    string           `json:"state_dir"`
	MaxAge      string           `json:"max_age"`
	Cutoff      string           `json:"cutoff"`
	Confirm     bool             `json:"confirm"`
	DryRun      bool             `json:"dry_run"`
	DeletedKeys []string         `json:"deleted_keys"`
	Pruned      []StateListEntry `json:"pruned"`
	KeptKeys    []string         `json:"kept_keys"`
	Kept        []StateListEntry `json:"kept"`
}

type StateDoctorIssue struct {
	Path     string `json:"path"`
	Key      string `json:"key,omitempty"`
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

type StateDoctorResult struct {
	OK        bool               `json:"ok"`
	Healthy   bool               `json:"healthy"`
	StateDir  string             `json:"state_dir"`
	Checked   int                `json:"checked"`
	ValidKeys []string           `json:"valid_keys"`
	Valid     []StateListEntry   `json:"valid"`
	Issues    []StateDoctorIssue `json:"issues"`
}

type StateMigrateResult struct {
	OK            bool               `json:"ok"`
	StateDir      string             `json:"state_dir"`
	FromSchema    int                `json:"from_schema"`
	ToSchema      int                `json:"to_schema"`
	Confirm       bool               `json:"confirm"`
	DryRun        bool               `json:"dry_run"`
	CandidateKeys []string           `json:"candidate_keys"`
	Candidates    []StateListEntry   `json:"candidates"`
	MigratedKeys  []string           `json:"migrated_keys"`
	SkippedKeys   []string           `json:"skipped_keys"`
	Skipped       []StateListEntry   `json:"skipped"`
	Issues        []StateDoctorIssue `json:"issues"`
}
