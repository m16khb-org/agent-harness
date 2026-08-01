package issueopspublication

type InvocationState string

const (
	InvocationUnknown          InvocationState = "unknown"
	InvocationNotInvokedProven InvocationState = "not_invoked_proven"
)

type ProcessReceipt struct {
	PID        int
	StartedAt  string
	Executable string
}

type Actor struct {
	Host            string
	SessionID       string
	AgentID         string
	SessionProcess  *ProcessReceipt
	ProcessAncestry []ProcessReceipt
}

func (a Actor) Clone() Actor {
	cloned := a
	if a.SessionProcess != nil {
		process := *a.SessionProcess
		cloned.SessionProcess = &process
	}
	if a.ProcessAncestry != nil {
		cloned.ProcessAncestry = append([]ProcessReceipt{}, a.ProcessAncestry...)
	}
	return cloned
}

type CreateCommand struct {
	ID                 string
	Provider           string
	Title              string
	Body               string
	Head               string
	Base               string
	Labels             []string
	Assignees          []string
	ExpectedGeneration uint64
	Actor              Actor
	CWD                string
	Confirm            bool
}

func (c CreateCommand) Clone() CreateCommand {
	cloned := c
	cloned.Labels = cloneStrings(c.Labels)
	cloned.Assignees = cloneStrings(c.Assignees)
	cloned.Actor = c.Actor.Clone()
	return cloned
}

type ProviderCreateRequest struct {
	Repo            string   `json:"repo"`
	ProjectKey      string   `json:"project_key,omitempty"`
	Title           string   `json:"title"`
	Body            string   `json:"body"`
	HeadBranch      string   `json:"head_branch"`
	BaseBranch      string   `json:"base_branch"`
	Labels          []string `json:"labels"`
	Assignees       []string `json:"assignees"`
	Draft           bool     `json:"draft"`
	ExpectedHeadSHA string   `json:"expected_head_sha,omitempty"`
	Confirm         bool     `json:"confirm"`
	Host            string   `json:"host,omitempty"`
	SessionID       string   `json:"session_id,omitempty"`
	AgentID         string   `json:"agent_id,omitempty"`
	CWD             string   `json:"cwd,omitempty"`
}

func (r ProviderCreateRequest) Clone() ProviderCreateRequest {
	cloned := r
	cloned.Labels = cloneStrings(r.Labels)
	cloned.Assignees = cloneStrings(r.Assignees)
	return cloned
}

type ProviderCreateResult struct {
	OK      bool   `json:"ok"`
	URL     string `json:"url"`
	Number  string `json:"number"`
	Preview string `json:"preview,omitempty"`
}

type CreateEligibility struct {
	Provider                 string
	Kind                     string
	Confirm                  bool
	PhasePR                  bool
	ExecutionActive          bool
	NoPending                bool
	NoArtifact               bool
	BranchAuthority          bool
	CanonicalLabelsAssignees bool
}

type PreparedCreate struct {
	Request     ProviderCreateRequest
	Eligibility CreateEligibility
}

func (p PreparedCreate) Clone() PreparedCreate {
	cloned := p
	cloned.Request = p.Request.Clone()
	return cloned
}

type Candidate struct {
	URL              string
	ProjectKey       string
	SourceProjectKey string
	HeadBranch       string
	BaseBranch       string
	HeadSHA          string
	Title            string
	BodySHA256       string
	Labels           []string
	Assignees        []string
	Draft            bool
	State            string
}

func (c Candidate) Clone() Candidate {
	cloned := c
	cloned.Labels = cloneStrings(c.Labels)
	cloned.Assignees = cloneStrings(c.Assignees)
	return cloned
}

type Inventory struct {
	Candidates        []Candidate
	AuthoritativeZero bool
}

func (i Inventory) Clone() Inventory {
	cloned := i
	if i.Candidates != nil {
		cloned.Candidates = make([]Candidate, len(i.Candidates))
		for index, candidate := range i.Candidates {
			cloned.Candidates[index] = candidate.Clone()
		}
	}
	return cloned
}

type RecordSnapshot struct {
	ID  string
	Raw []byte
}

func (r RecordSnapshot) Clone() RecordSnapshot {
	cloned := r
	cloned.Raw = cloneBytes(r.Raw)
	return cloned
}

type Intent struct {
	Record          RecordSnapshot
	OperationID     string
	Generation      uint64
	Provider        string
	Kind            string
	Request         ProviderCreateRequest
	InvocationState InvocationState
	RetryCount      int
	KnownURL        string
	Eligibility     CreateEligibility
	Raw             []byte
}

func (i Intent) Clone() Intent {
	cloned := i
	cloned.Record = i.Record.Clone()
	cloned.Request = i.Request.Clone()
	cloned.Raw = cloneBytes(i.Raw)
	return cloned
}

type ReconcileResult struct {
	Record                 RecordSnapshot
	Reconciled             bool
	Code                   string
	ExternalStateInspected bool
}

func (r ReconcileResult) Clone() ReconcileResult {
	cloned := r
	cloned.Record = r.Record.Clone()
	return cloned
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string{}, values...)
}

func cloneBytes(value []byte) []byte {
	if value == nil {
		return nil
	}
	return append([]byte{}, value...)
}
