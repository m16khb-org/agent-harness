package issueopslease

type ResumeRequest struct {
	ID                 string `json:"id"`
	ExpectedGeneration uint64 `json:"expected_generation"`
	CWD                string `json:"cwd"`
}

type ResumeArtifacts struct {
	ClaimTokenPath      string `json:"claim_token_path"`
	IssueBodySHA256     string `json:"issue_body_sha256"`
	ContextPacketPath   string `json:"context_packet_path"`
	ContextPacketSHA256 string `json:"context_packet_sha256"`
	OwnerPromptPath     string `json:"owner_prompt_path"`
	OwnerPromptSHA256   string `json:"owner_prompt_sha256"`
}

type ResumeReceipt struct {
	Execution Execution       `json:"execution"`
	Artifacts ResumeArtifacts `json:"artifacts"`
}

type ResumeStageReceipt struct {
	TerminalPTYID string `json:"terminal_pty_id,omitempty"`
	TaskID        string `json:"task_id,omitempty"`
	DispatchID    string `json:"dispatch_id,omitempty"`
}

type ResumeStageInventory struct {
	Candidates        []ResumeStageReceipt `json:"candidates"`
	AuthoritativeZero bool                 `json:"authoritative_zero"`
}
