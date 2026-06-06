package guard

type GuardCheckRequest struct {
	RepoRoot string   `json:"repo_root"`
	Staged   bool     `json:"staged"`
	All      bool     `json:"all"`
	Files    []string `json:"files,omitempty"`
}

type GuardCheckResult struct {
	OK           bool           `json:"ok"`
	RepoRoot     string         `json:"repo_root"`
	Mode         string         `json:"mode"`
	CheckedFiles []string       `json:"checked_files"`
	Findings     []GuardFinding `json:"findings"`
	Summary      GuardSummary   `json:"summary"`
	Warnings     []string       `json:"warnings,omitempty"`
}

type GuardFinding struct {
	Severity    string   `json:"severity"`
	Rule        string   `json:"rule"`
	File        string   `json:"file,omitempty"`
	Line        int      `json:"line,omitempty"`
	Message     string   `json:"message"`
	Evidence    string   `json:"evidence,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
}

type GuardSummary struct {
	Block  int `json:"block"`
	Warn   int `json:"warn"`
	Review int `json:"review"`
	Info   int `json:"info"`
}

type GuardBlockedError struct {
	Findings []GuardFinding
}

func (e GuardBlockedError) Error() string {
	if len(e.Findings) == 0 {
		return "guard check blocked"
	}
	return "guard check blocked: " + e.Findings[0].Rule
}

func IsGuardBlocked(err error) bool {
	_, ok := err.(GuardBlockedError)
	return ok
}
