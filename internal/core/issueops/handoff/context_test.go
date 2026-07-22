package handoff

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
)

func TestIssueOpsHandoffContextOmitsEmptySourceRecipient(t *testing.T) {
	projection := ContextProjection{
		Version: 1, CycleID: "io-current-context", Branch: "16-demo",
		WorktreePath: "/repo.worktrees/16-demo", PlanPath: "/repo/plans/handoff.md", PlanSHA256: strings.Repeat("a", 64),
		Attempt: 1, OwnershipEpoch: "epoch-1", AttemptBaseHead: strings.Repeat("b", 40),
	}
	encoded, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte(`{"version":1,"cycle_id":"io-current-context","branch":"16-demo","worktree_path":"/repo.worktrees/16-demo","plan_path":"/repo/plans/handoff.md","plan_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","attempt":1,"ownership_epoch":"epoch-1","attempt_base_head":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}`)
	if !bytes.Equal(encoded, want) {
		t.Fatalf("current context projection changed\n got=%s\nwant=%s", encoded, want)
	}
	sum := sha256.Sum256(encoded)
	if got := hex.EncodeToString(sum[:]); got != "be16a043efdf7e84cf09750ff40a03573b60b21610b8c78989c80357ecf85daa" {
		t.Fatalf("current context projection hash = %s", got)
	}

	projection.CoordinatorRecipient = "term_coordinator"
	withRecipient, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	sourceWithRecipient, err := json.Marshal(contextSourceProjection(projection))
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string][]byte{"context": withRecipient, "source": sourceWithRecipient} {
		if !bytes.Contains(value, []byte(`"coordinator_recipient":"term_coordinator"`)) {
			t.Fatalf("nonempty sealed coordinator missing from %s projection: %s", name, value)
		}
		if bytes.Equal(value, want) {
			t.Fatalf("nonempty sealed coordinator did not change %s hash input", name)
		}
	}
}

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
	if first.Projection.Provider != "github" || first.Projection.IssueURL != record.IssueURL {
		t.Fatalf("provider-linked issue authority missing from context: %#v", first.Projection)
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

func TestIssueOpsOwnershipContextIncludesWorkspaceSeal(t *testing.T) {
	record := contextRecordForTest(t)
	handoff := model.CurrentExecutionHandoff(record)
	handoff.WorkspaceEpoch = "workspace-epoch-1"
	handoff.WorkspaceSHA256 = strings.Repeat("c", 64)

	packet, err := BuildContext(record, ContextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if packet.Projection.WorkspaceEpoch != "workspace-epoch-1" || packet.Projection.WorkspaceSHA256 != strings.Repeat("c", 64) {
		t.Fatalf("ownership workspace seal missing from context: %#v", packet.Projection)
	}
}

func TestIssueOpsOwnershipContextSealsHostLaunchProfile(t *testing.T) {
	record := contextRecordForTest(t)
	profile, err := ResolveAgentLaunchProfile("codex")
	if err != nil {
		t.Fatal(err)
	}
	handoff := model.CurrentExecutionHandoff(record)
	handoff.Agent = "codex"
	handoff.LaunchProfile = &profile

	packet, err := BuildContext(record, ContextOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if packet.Projection.Agent != "codex" || packet.Projection.Model != "gpt-5.6-terra" || packet.Projection.ReasoningEffort != "high" {
		t.Fatalf("host launch profile missing from context: %#v", packet.Projection)
	}
}

func TestResolveAgentLaunchProfileUsesCurrentHostDefaults(t *testing.T) {
	for _, tt := range []struct {
		agent, model, effort string
	}{
		{agent: "codex", model: "gpt-5.6-terra", effort: "high"},
		{agent: "claude", model: "opus"},
		{agent: "gjc"},
	} {
		profile, err := ResolveAgentLaunchProfile(tt.agent)
		if err != nil {
			t.Fatalf("ResolveAgentLaunchProfile(%q): %v", tt.agent, err)
		}
		if profile.Agent != tt.agent || profile.Model != tt.model || profile.ReasoningEffort != tt.effort {
			t.Fatalf("ResolveAgentLaunchProfile(%q) = %#v", tt.agent, profile)
		}
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
		{name: "provider", mutate: func(r *model.IssueOpsRecord) { r.BranchPrepare.Provider = "gitlab" }},
		{name: "intent", mutate: func(r *model.IssueOpsRecord) { r.Intent.InterpretedIntent = "changed intent" }},
		{name: "worktree", mutate: func(r *model.IssueOpsRecord) { r.WorktreePath = filepath.Join(filepath.Dir(r.WorktreePath), "other") }},
		{name: "attempt base", mutate: func(r *model.IssueOpsRecord) {
			model.CurrentExecutionHandoff(*r).AttemptBaseHead = strings.Repeat("c", 40)
		}},
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
			Verification:          []string{"current fixtures"},
			Approved:              true,
		},
		DevilsAdvocateReview: &model.IssueOpsDevilsAdvocateReview{
			Verdict:  "pass",
			Findings: []string{"prove create-at-most-once"},
		},
		BranchPrepare: &model.IssueOpsBranchPrepare{Provider: "github", IssueURL: "https://github.com/example/repo/issues/16", BaseBranch: "main", BaseSHA: strings.Repeat("b", 40)},
		CycleState:    model.IssueOpsCycleActive,
		Ownership: &model.IssueOpsOwnershipLedger{ActiveAttempt: 1, Attempts: []model.IssueOpsOwnershipAttempt{{
			Number:    1,
			Workspace: &model.IssueOpsExecutionWorkspace{State: "ready", WorkspaceEpoch: "workspace-epoch-1", Driver: "orca", Agent: "codex", CoordinatorRoot: "/repo/example", WorkerRoot: "/repo/example.worktrees/16-demo", PreparationSession: &model.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "source"}, BaseHead: strings.Repeat("b", 40)},
			Handoff:   &model.IssueOpsExecutionHandoff{State: StateOwnershipDispatching, Attempt: 1, AttemptBaseHead: strings.Repeat("b", 40), OwnershipEpoch: "epoch-1", WorkspaceEpoch: "workspace-epoch-1", WorkspaceSHA256: strings.Repeat("c", 64), CoordinatorRoot: "/repo/example", WorkerRoot: "/repo/example.worktrees/16-demo"},
		}}},
	}
}
