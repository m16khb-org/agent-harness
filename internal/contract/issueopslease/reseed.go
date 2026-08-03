package issueopslease

type ReseedRequest struct {
	ID                   string `json:"id"`
	ExpectedGeneration   uint64 `json:"expected_generation"`
	CompletionGeneration uint64 `json:"completion_generation,omitempty"`
	InventoryFingerprint string `json:"inventory_fingerprint,omitempty"`
	Reason               string `json:"reason,omitempty"`
	CWD                  string `json:"cwd"`
}

type ReseedReceipt struct {
	Execution           Execution `json:"execution"`
	ClaimTokenPath      string    `json:"claim_token_path,omitempty"`
	IssueBodySHA256     string    `json:"issue_body_sha256,omitempty"`
	ContextPacketPath   string    `json:"context_packet_path,omitempty"`
	ContextPacketSHA256 string    `json:"context_packet_sha256,omitempty"`
	OwnerPromptPath     string    `json:"owner_prompt_path,omitempty"`
	OwnerPromptSHA256   string    `json:"owner_prompt_sha256,omitempty"`
}
