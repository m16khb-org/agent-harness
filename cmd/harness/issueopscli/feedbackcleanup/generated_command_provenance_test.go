package feedbackcleanup

import (
	issueopscore "agent-harness/internal/adapter/issueops"
	"agent-harness/internal/adapter/orca"
	commandparsecontract "agent-harness/internal/contract/commandparse"
	issueopscontract "agent-harness/internal/contract/issueops"
	"agent-harness/internal/domain/commandparse"
	"agent-harness/internal/port"
	provenanceport "agent-harness/internal/port/issueopsprovenance"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type cleanupProvenanceObserverStub struct {
	evidence provenanceport.Receipt
}

func TestCleanupFinishPreviewEmitsBoundFinishCommand(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := cleanupStatusRecord(t, true, true)
	var printed any
	deps := cleanupStatusDeps(nil)
	deps.Provenance = cleanupProvenanceObserverStub{evidence: provenanceport.Receipt{
		ExecutablePath: "/repo/bin/agent-harness", ExecutableSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}
	deps.PrintJSON = func(value any) error {
		printed = value
		return nil
	}
	deps.Provider = func(string) (port.IssueProvider, error) {
		return &cleanupStatusProvider{snapshot: port.ExecutionIssueSnapshot{
			URL: record.IssueURL, Body: port.IssueBodyCompletionStartMarker, State: "closed",
		}}, nil
	}
	deps.VerifyMergedHead = func(issueopscontract.IssueOpsRemoteArtifactVerification) (issueopscore.CleanupRemoteBranchArtifactHead, error) {
		return issueopscore.CleanupRemoteBranchArtifactHead{HeadRefName: record.Branch, HeadRefOID: "abc123", BaseRefName: "main"}, nil
	}
	if err := RunCleanup([]string{"finish", "--id", record.ID, "--preview", "--json"}, deps); err != nil {
		t.Fatal(err)
	}
	result, ok := printed.(issueopscore.CleanupFinishResult)
	if !ok || !strings.Contains(result.NextCommand, "cleanup finish") || !strings.Contains(result.NextCommand, "--generated-for-generation 1") {
		t.Fatalf("cleanup finish preview result = %#v", printed)
	}
}

func (s cleanupProvenanceObserverStub) Observe(context.Context) (provenanceport.Receipt, error) {
	return s.evidence, nil
}

func TestBindCleanupNextCommandCoversPreviewAndFinish(t *testing.T) {
	observer := cleanupProvenanceObserverStub{evidence: provenanceport.Receipt{
		ExecutablePath: "/repo/bin/agent-harness", ExecutableSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}}
	commands := map[string]string{
		"preview": "agent-harness issueops cleanup finish --id io-1 --preview --json",
		"finish":  "agent-harness issueops cleanup finish --id io-1 --apply --confirm --fingerprint abc --json",
	}
	for name, command := range commands {
		t.Run(name, func(t *testing.T) {
			bound, err := bindCleanupNextCommand(command, 9, observer)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"--generated-by-executable", "--generated-by-sha256", "--generated-for-generation 9"} {
				if !strings.Contains(bound, want) {
					t.Fatalf("bound cleanup command %q does not contain %q", bound, want)
				}
			}
		})
	}
}

func TestBindCleanupNextCommandMissingObserverHasNoFallback(t *testing.T) {
	bound, err := bindCleanupNextCommand("agent-harness issueops cleanup finish --id io-1 --preview --json", 9, nil)
	if err == nil || bound != "" {
		t.Fatalf("missing observer bound=%q err=%v", bound, err)
	}
	fields, ok := err.(interface{ IssueOpsErrorFields() map[string]any })
	if !ok || fields.IssueOpsErrorFields()["code"] != "generated_command_provenance_observation_failed" {
		t.Fatalf("missing observer failure is not structured: %T %v", err, err)
	}
}

func TestCurrentRelayCleanupGeneratedCommandDogfood(t *testing.T) {
	binary := os.Getenv("AGENT_HARNESS_CURRENT_RELAY_DOGFOOD_BINARY")
	lifecycleID := os.Getenv("AGENT_HARNESS_CURRENT_RELAY_DOGFOOD_ID")
	if binary == "" || lifecycleID == "" {
		t.Skip("live current-relay dogfood requires an exact binary and lifecycle id")
	}
	binary, err := filepath.EvalSymlinks(binary)
	if err != nil {
		t.Fatal(err)
	}
	binaryBytes, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	binaryHash := sha256.Sum256(binaryBytes)
	live, err := issueopscore.ReadIssueOps(issueopscore.IssueOpsStateRoot(), lifecycleID)
	if err != nil {
		t.Fatal(err)
	}
	if live.Execution == nil || live.Execution.Orca == nil {
		t.Fatal("live dogfood lifecycle is not an Orca execution")
	}

	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := cleanupStatusRecord(t, true, true)
	binding := *live.Execution.Orca
	binding.WorktreeID = binding.RepoID + "::" + filepath.Join(t.TempDir(), "absent-cleanup-worktree")
	binding.WorktreeInstanceID = "303-cleanup-dogfood-absent"
	binding.TaskID = "task-303-cleanup-dogfood-absent"
	binding.DispatchID = "ctx-303-cleanup-dogfood-absent"
	binding.TerminalPTYID = "pty-303-cleanup-dogfood-absent"
	record.Execution.Mode = issueopscontract.ExecutionModeOrca
	record.Execution.Workspace.Driver = "orca"
	record.Execution.Orca = &binding
	if _, err := issueopscore.WriteIssueOps(issueopscore.IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}

	observer := cleanupProvenanceObserverStub{evidence: provenanceport.Receipt{
		ExecutablePath: binary, ExecutableSHA256: hex.EncodeToString(binaryHash[:]),
	}}
	provider := &liveCleanupProvider{cleanupStatusProvider: cleanupStatusProvider{snapshot: port.ExecutionIssueSnapshot{
		URL: record.IssueURL, Body: port.IssueBodyCompletionStartMarker, State: "closed",
	}}}
	var printed any
	removeCalls := 0
	client := orca.New()
	deps := cleanupStatusDeps(nil)
	deps.Provenance = observer
	deps.PrintJSON = func(value any) error { printed = value; return nil }
	deps.CleanupFinishGit = func(_ string, args ...string) (int, string) {
		switch args[0] {
		case "status", "ls-remote", "worktree", "update-ref":
			return 0, ""
		case "rev-parse":
			return 0, "abc123\n"
		default:
			return 1, "unexpected git call"
		}
	}
	deps.Provider = func(string) (port.IssueProvider, error) { return provider, nil }
	deps.VerifyMergedHead = func(issueopscontract.IssueOpsRemoteArtifactVerification) (issueopscore.CleanupRemoteBranchArtifactHead, error) {
		return issueopscore.CleanupRemoteBranchArtifactHead{HeadRefName: record.Branch, HeadRefOID: "abc123", BaseRefName: "main"}, nil
	}
	deps.RemoveOrcaWorktree = func(ctx context.Context, worktreeID string) error {
		removeCalls++
		err := client.RemoveWorktree(ctx, worktreeID, false)
		if err != nil {
			var orcaErr *port.OrcaError
			if errors.As(err, &orcaErr) && strings.Contains(strings.ToLower(orcaErr.Code), "not_found") {
				return nil
			}
			message := strings.ToLower(err.Error())
			if strings.Contains(message, "not found") || strings.Contains(message, "unknown worktree") {
				return nil
			}
		}
		return err
	}
	if err := RunCleanup([]string{"finish", "--id", record.ID, "--preview", "--json"}, deps); err != nil {
		t.Fatal(err)
	}
	preview, ok := printed.(issueopscore.CleanupFinishResult)
	if !ok || preview.NextCommand == "" {
		t.Fatalf("cleanup current-relay preview = %#v", printed)
	}
	tokens := commandparse.SplitCommandTokens(preview.NextCommand)
	if len(tokens) < 4 || tokens[0] != binary || tokens[1] != "issueops" || tokens[2] != "cleanup" {
		t.Fatalf("cleanup generated command = %q", preview.NextCommand)
	}
	clean, provenance, present, err := commandparsecontract.ConsumeGeneratedCommandProvenance(tokens[2:])
	if err != nil || !present {
		t.Fatalf("cleanup generated provenance err=%v present=%t", err, present)
	}
	observed := commandparsecontract.GeneratedCommandProvenance{
		ExecutablePath: observer.evidence.ExecutablePath, ExecutableSHA256: observer.evidence.ExecutableSHA256, LeaseGeneration: 1,
	}
	if err := commandparsecontract.ValidateGeneratedCommandInvocation(provenance, observed, 1); err != nil {
		t.Fatal(err)
	}
	if err := RunCleanup(clean[1:], deps); err != nil {
		t.Fatal(err)
	}
	if removeCalls != 1 {
		t.Fatalf("current relay remove calls = %d, want 1", removeCalls)
	}
	if _, err := issueopscore.ReadIssueOps(issueopscore.IssueOpsStateRoot(), record.ID); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cleanup dogfood record was not deleted: %v", err)
	}
}

type liveCleanupProvider struct {
	cleanupStatusProvider
}

func (p *liveCleanupProvider) UpdateIssueBodySection(req port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error) {
	return port.IssueProviderUpdateIssueBodySectionResult{OK: true, URL: req.IssueURL, Updated: true}, nil
}
