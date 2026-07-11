package handoff

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
)

func TestIssueOpsHandoffContextDeterministicAndBounded(t *testing.T) {
	record := contextRecordForTest(t)
	options := ContextOptions{
		CriteriaIDs:               []string{"ORCA-02", "ORCA-01", "ORCA-01"},
		RequiredDocs:              []string{".agent-harness/TESTING.md", "AGENTS.md"},
		RequiredSkills:            []string{"turing", "issueops"},
		WorkerScope:               "Implement issue #16 only.",
		VerificationCommands:      []string{"go test ./... -count=1"},
		HeartbeatCadence:          "every 5 minutes",
		StopConditions:            []string{"scope drift", "destructive action"},
		ResultFormat:              "bounded Turing evidence",
		AllowCodexHookTrustBypass: true,
	}
	first, err := BuildContext(record, options)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildContext(record, options)
	if err != nil {
		t.Fatal(err)
	}
	if first.SHA256 != second.SHA256 || first.Markdown != second.Markdown {
		t.Fatalf("context is not deterministic: first=%s second=%s", first.SHA256, second.SHA256)
	}
	if len(first.Markdown) > MaxRenderedContextBytes {
		t.Fatalf("rendered context bytes = %d, limit = %d", len(first.Markdown), MaxRenderedContextBytes)
	}
	if first.PlanSHA256 == "" || first.SHA256 == "" || first.SourceSHA256 == "" {
		t.Fatalf("missing context hashes: %#v", first)
	}
	if !first.Projection.AllowCodexHookTrustBypass || !CanonicalContextOptions(options).AllowCodexHookTrustBypass || !ContextOptionsFromModel(CanonicalContextOptions(options)).AllowCodexHookTrustBypass {
		t.Fatal("Codex hook-trust attestation was not preserved in the delivery projection and model")
	}
	withDifferentOptions, err := BuildContext(record, ContextOptions{WorkerScope: "different runtime delivery option"})
	if err != nil {
		t.Fatal(err)
	}
	if withDifferentOptions.SHA256 == first.SHA256 || withDifferentOptions.SourceSHA256 != first.SourceSHA256 {
		t.Fatalf("runtime options must change full context but not source fingerprint: first=%#v other=%#v", first, withDifferentOptions)
	}
}

func TestIssueOpsHandoffContextRedactsSecrets(t *testing.T) {
	record := contextRecordForTest(t)
	record.Intent.RawRequest = "Authorization: Bearer super-secret-token"
	record.DesignReview.Risks = []string{"api_key=super-secret-token"}
	packet, err := BuildContext(record, ContextOptions{WorkerScope: "token=super-secret-token"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(packet.Markdown, "super-secret-token") {
		t.Fatalf("rendered context leaked a secret: %s", packet.Markdown)
	}
	encoded, err := json.Marshal(packet.Projection)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "super-secret-token") {
		t.Fatalf("context projection leaked a secret: %s", encoded)
	}
}

func TestIssueOpsHandoffContextHashChangesForPlanBranchIntentAndWorktree(t *testing.T) {
	base := contextRecordForTest(t)
	first, err := BuildContext(base, ContextOptions{})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		mutate func(*model.IssueOpsRecord)
	}{
		{name: "plan", mutate: func(r *model.IssueOpsRecord) { _ = os.WriteFile(r.PlanPath, []byte("changed plan\n"), 0o644) }},
		{name: "branch", mutate: func(r *model.IssueOpsRecord) { r.Branch = "16-other" }},
		{name: "intent", mutate: func(r *model.IssueOpsRecord) { r.Intent.InterpretedIntent = "changed intent" }},
		{name: "worktree", mutate: func(r *model.IssueOpsRecord) { r.WorktreePath = filepath.Join(filepath.Dir(r.WorktreePath), "other") }},
		{name: "attempt base", mutate: func(r *model.IssueOpsRecord) { r.ExecutionHandoff.AttemptBaseHead = strings.Repeat("c", 40) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := contextRecordForTest(t)
			tt.mutate(&record)
			packet, err := BuildContext(record, ContextOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if packet.SHA256 == first.SHA256 {
				t.Fatalf("context hash did not change for %s", tt.name)
			}
			if packet.SourceSHA256 == first.SourceSHA256 {
				t.Fatalf("context source fingerprint did not change for %s", tt.name)
			}
		})
	}
}

func contextRecordForTest(t *testing.T) model.IssueOpsRecord {
	t.Helper()
	root := t.TempDir()
	plan := filepath.Join(root, "plans", "handoff.md")
	if err := os.MkdirAll(filepath.Dir(plan), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan, []byte("approved plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return model.IssueOpsRecord{
		ID:           "io-handoff",
		Repo:         root,
		Branch:       "16-demo",
		IssueURL:     "https://github.com/example/repo/issues/16",
		PlanPath:     plan,
		WorktreePath: root,
		Intent: &model.IssueOpsIntentContract{
			RawRequest:        "add supervised handoff",
			InterpretedIntent: "keep IssueOps authoritative",
			SuccessCriteria:   []string{"ORCA-01 passes", "ORCA-14 passes"},
			Constraints:       []string{"never retry create"},
			NonGoals:          []string{"generic scheduler"},
		},
		DesignReview: &model.IssueOpsDesignReview{
			ProposedDesign: "one durable execution lease",
			Alternatives:   []string{"prompt-only handoff"},
			Risks:          []string{"duplicate creates"},
		},
		CompatibilityReview: &model.IssueOpsCompatibilityReview{
			BackwardCompatibility: []string{"absent handoff remains inline"},
			SideEffects:           []string{"optional Orca mutations"},
			RollbackPlan:          "remove optional handoff wiring",
			Verification:          []string{"legacy fixtures"},
			Approved:              true,
		},
		DevilsAdvocateReview: &model.IssueOpsDevilsAdvocateReview{
			Verdict:  "pass",
			Findings: []string{"prove create-at-most-once"},
		},
		BranchPrepare: &model.IssueOpsBranchPrepare{BaseBranch: "main", BaseSHA: strings.Repeat("b", 40)},
		ExecutionHandoff: &model.IssueOpsExecutionHandoff{
			ProtocolVersion: 1,
			State:           StateCoordinatorPreparing,
			Attempt:         1,
			AttemptBaseHead: strings.Repeat("b", 40),
			OwnershipEpoch:  "epoch-1",
		},
	}
}
