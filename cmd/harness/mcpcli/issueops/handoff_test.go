package issueops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/cmd/harness/mcpcli"
	"agent-harness/internal/core"
	"agent-harness/internal/core/issueops/handoff"
	issueopsmodel "agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/lifecycle"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/port"
)

func TestMCPIssueOpsHandoffLifecycleParity(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	record := mcpHandoffRecord(t)
	fake := &handoffStartOrcaFake{workerRoot: record.WorktreePath}
	previousClient := mcpcli.IssueOpsHandoffOrcaClient
	mcpcli.IssueOpsHandoffOrcaClient = func() core.IssueOpsOrcaDispatchClient { return fake }
	previousProjector := mcpcli.IssueOpsWorkerDoneProjectionClient
	mcpcli.IssueOpsWorkerDoneProjectionClient = func() core.IssueOpsWorkerDoneProjectionClient { return fake }
	publication := &mcpPublicationFake{}
	previousPublication := mcpcli.IssueOpsPublicationReader
	mcpcli.IssueOpsPublicationReader = func() core.IssueOpsHandoffPublicationReader { return publication }
	t.Cleanup(func() {
		mcpcli.IssueOpsHandoffOrcaClient = previousClient
		mcpcli.IssueOpsWorkerDoneProjectionClient = previousProjector
		mcpcli.IssueOpsPublicationReader = previousPublication
	})
	unattestedPreview := callMCPToolForIssueOpsTest(t, "issueops_handoff", map[string]any{
		"action":                 "start",
		"id":                     record.ID,
		"coordinator_recipient":  "term_coordinator",
		"coordinator_host":       "codex",
		"coordinator_session_id": "coordinator-session",
		"coordinator_agent_id":   "coordinator-agent",
		"source_cwd":             record.Repo,
	})
	if unattestedPreview["preview"] != true || len(unattestedPreview["context_sha256"].(string)) != 64 {
		t.Fatalf("unattested start preview parity failed: %#v", unattestedPreview)
	}
	if unattestedPreview["codex_hook_trust_bypass_required"] != true || unattestedPreview["codex_hook_trust_bypass_attested"] != false {
		t.Fatalf("unattested Codex preview must require the hooks/list attestation: %#v", unattestedPreview)
	}
	finalPreview := callMCPToolForIssueOpsTest(t, "issueops_handoff", map[string]any{
		"action":                        "start",
		"id":                            record.ID,
		"coordinator_recipient":         "term_coordinator",
		"coordinator_host":              "codex",
		"coordinator_session_id":        "coordinator-session",
		"coordinator_agent_id":          "coordinator-agent",
		"source_cwd":                    record.Repo,
		"allow_codex_hook_trust_bypass": true,
		"codex_model":                   "gpt-5.6-terra",
		"codex_reasoning_effort":        "high",
	})
	if finalPreview["preview"] != true || finalPreview["codex_hook_trust_bypass_attested"] != true {
		t.Fatalf("attested final start preview parity failed: %#v", finalPreview)
	}
	reviewedContextSHA256, _ := finalPreview["context_sha256"].(string)
	if len(reviewedContextSHA256) != 64 || reviewedContextSHA256 == unattestedPreview["context_sha256"].(string) {
		t.Fatalf("attested final preview must reseal the reviewed context hash: %#v", finalPreview)
	}
	startConfirm := callMCPToolForIssueOpsTest(t, "issueops_handoff", map[string]any{
		"action":                        "start",
		"id":                            record.ID,
		"coordinator_recipient":         "term_coordinator",
		"coordinator_host":              "codex",
		"coordinator_session_id":        "coordinator-session",
		"coordinator_agent_id":          "coordinator-agent",
		"source_cwd":                    record.Repo,
		"confirm":                       true,
		"allow_codex_hook_trust_bypass": true,
		"codex_model":                   "gpt-5.6-terra",
		"codex_reasoning_effort":        "high",
		"expected_context_sha256":       reviewedContextSHA256,
	})
	if startConfirm["state"] != handoff.StateDispatched || startConfirm["context_sha256"] != reviewedContextSHA256 {
		t.Fatalf("start parity failed: %#v", startConfirm)
	}
	if fake.terminalRequest.CodexModel != "gpt-5.6-terra" || fake.terminalRequest.CodexReasoningEffort != "high" {
		t.Fatalf("MCP start did not preserve Codex launch options: %#v", fake.terminalRequest)
	}
	common := map[string]any{
		"id": record.ID, "attempt": 1, "ownership_epoch": "epoch-1", "context_sha256": startConfirm["context_sha256"],
	}
	claim := cloneHandoffArgs(common)
	claim["action"], claim["host"], claim["session_id"], claim["agent_id"] = "claim", "codex", "session-1", "worker-1"
	claim["cwd"], claim["orca_worktree_id"] = record.WorktreePath, "wt-1"
	claimed := callMCPToolForIssueOpsTest(t, "issueops_handoff", claim)
	if nestedMap(claimed, "execution_handoff")["state"] != handoff.StateClaimed {
		t.Fatalf("claim parity failed: %#v", claimed)
	}
	finalHead := commitMCPHandoffResult(t, record.WorktreePath)
	finish := cloneHandoffArgs(common)
	finish["action"], finish["host"], finish["session_id"], finish["agent_id"] = "finish", "codex", "session-1", "worker-1"
	finish["outcome"], finish["final_head"] = "completed", finalHead
	finish["changed_files"] = []string{"internal/x.go", ".agent-harness/research/report.md"}
	finish["turing_report_path"] = ".agent-harness/research/report.md"
	finish["verification"] = []string{"go test: pass"}
	finish["cleanup_receipts"] = []string{"temp removed"}
	finish["task_id"], finish["dispatch_id"] = "task-1", "dispatch-1"
	submitted := callMCPToolForIssueOpsTest(t, "issueops_handoff", finish)
	if nestedMap(submitted, "execution_handoff")["state"] != handoff.StateSubmitted {
		t.Fatalf("finish parity failed: %#v", submitted)
	}
	if nestedMap(nestedMap(submitted, "execution_handoff"), "worker_done_projection")["state"] != "sent" || fake.workerDoneCalls != 1 {
		t.Fatalf("automatic worker_done projection parity failed: %#v calls=%d", submitted, fake.workerDoneCalls)
	}
	accept := cloneHandoffArgs(common)
	accept["action"], accept["final_head"] = "accept", finalHead
	accept["host"], accept["session_id"], accept["agent_id"], accept["source_cwd"] = "codex", "coordinator-session", "coordinator-agent", record.Repo
	closed := callMCPToolForIssueOpsTest(t, "issueops_handoff", accept)
	if nestedMap(closed, "execution_handoff")["closed_disposition"] != handoff.DispositionAccepted {
		t.Fatalf("accept parity failed: %#v", closed)
	}
	fake.terminals = nil
	publication.localHead, publication.remoteHead = finalHead, finalHead
	published := callMCPToolForIssueOpsTest(t, "issueops_handoff", map[string]any{"action": "publish", "id": record.ID, "host": "codex", "session_id": "coordinator-session", "agent_id": "coordinator-agent", "source_cwd": record.Repo, "confirm": true})
	if nestedMap(nestedMap(published, "execution_handoff"), "publish_receipt")["final_head"] != finalHead {
		t.Fatalf("publish receipt parity failed: %#v", published)
	}
	current, err := core.ReadIssueOps(core.IssueOpsStateRoot(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	current.Phase = core.IssueOpsPhasePR
	if _, err := core.WriteIssueOps(core.IssueOpsStateRoot(), current); err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	gh := filepath.Join(binDir, "gh")
	ghBody := `#!/bin/sh
if [ "$1 $2" = "pr create" ]; then printf '%s\n' "$@" > gh.create.argv; printf 'https://github.com/acme/repo/pull/16\n'; exit 0; fi
if [ "$1 $2" = "pr view" ]; then printf '{"url":"https://github.com/acme/repo/pull/16","title":"PR","body":"literal body","headRefName":"1-handoff","baseRefName":"main","isDraft":true,"headRefOid":"` + finalHead + `","labels":[{"name":"bug"}],"assignees":[{"login":"octocat"}],"headRepository":{"nameWithOwner":"acme/repo"}}'; exit 0; fi
exit 2
`
	if err := os.WriteFile(gh, []byte(ghBody), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	payload := map[string]any{
		"id": record.ID, "provider": "github", "title": "PR", "body": "literal body", "head": "1-handoff", "base": "main",
		"labels": []any{"bug"}, "assignees": []any{"octocat"}, "confirm": true,
	}
	identity := current.ExecutionHandoff.CoordinatorSession
	decision := lifecycle.BuildLifecyclePreToolUseDecision(lifecycle.HookToolUseLifecycleRequest{
		Repo: record.Repo, CWD: record.Repo, Host: identity.Host, SessionID: identity.SessionID, AgentID: identity.AgentID,
		Tool: "mcp__agent_harness__issueops_remote_create_pr", ToolInput: payload,
	})
	if decision.Decision != "allow" {
		t.Fatalf("JSON-shaped MCP create payload failed lifecycle parity: %#v", decision)
	}
	created := callMCPToolForIssueOpsTest(t, "issueops_remote_create_pr", payload)
	if created["url"] != "https://github.com/acme/repo/pull/16" {
		t.Fatalf("MCP supervised create did not use shared claim wrapper: %#v", created)
	}
	argv, err := os.ReadFile(filepath.Join(record.Repo, "gh.create.argv"))
	if err != nil || !strings.Contains(string(argv), "\n--repo\ngithub.com/acme/repo\n") || !strings.Contains(string(argv), "\n--draft\n") {
		t.Fatalf("MCP supervised create argv = %q, err=%v", argv, err)
	}
}

type mcpPublicationFake struct {
	localHead  string
	remoteHead string
}

func (f *mcpPublicationFake) LocalRefHead(context.Context, string, string) (string, error) {
	return f.localHead, nil
}

func (f *mcpPublicationFake) RemoteRefHead(context.Context, string, string, string, string) (string, error) {
	return f.remoteHead, nil
}

func (f *mcpPublicationFake) PushTarget(context.Context, string, string) (core.IssueOpsPublicationPushTarget, error) {
	target := "https://github.com/acme/repo.git"
	sum := sha256.Sum256([]byte(target))
	return core.IssueOpsPublicationPushTarget{URL: target, Fingerprint: hex.EncodeToString(sum[:])}, nil
}

func (f *mcpPublicationFake) PushExact(context.Context, string, string, string, string, string) error {
	return nil
}

func TestMCPIssueOpsHandoffUsesOneActionTool(t *testing.T) {
	for _, forbidden := range []string{"issueops_handoff_start", "issueops_handoff_claim", "issueops_handoff_finish", "issueops_handoff_accept", "issueops_handoff_recover"} {
		if msg := callMCPUnknownTool(t, forbidden); !strings.Contains(msg, "Unknown tool") {
			t.Fatalf("duplicate tool %s was advertised or handled: %s", forbidden, msg)
		}
	}
}

func TestOwnershipTransferCLIAndMCPActionParity(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	for _, action := range []string{"start", "claim", "acknowledge-context", "publish", "complete", "cleanup-preview", "cleanup-approve", "cleanup-record", "recover"} {
		t.Run(action, func(t *testing.T) {
			err := callMCPToolForIssueOpsTestError(t, "issueops_handoff", map[string]any{"action": action, "id": "io-missing"})
			if strings.Contains(err, "handoff action must be") {
				t.Fatalf("MCP handler did not dispatch ownership-transfer action %q: %s", action, err)
			}
		})
	}
}

func mcpHandoffRecord(t *testing.T) core.IssueOpsRecord {
	t.Helper()
	repo := makeIssueOpsCLIRepoForTest(t, "handoff")
	worktree := filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".worktrees", "1-handoff")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q", "-b", "1-handoff"}, {"config", "user.name", "MCP Test"}, {"config", "user.email", "mcp@example.test"}} {
		if code, _, stderr := preflight.GitCmd(worktree, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	writeMCPHandoffFile(t, worktree, "plans/handoff.md", "# plan\n")
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", "test: prepare handoff"}} {
		if code, _, stderr := preflight.GitCmd(worktree, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	record, err := core.StartIssueOps(core.IssueOpsStateRoot(), core.IssueOpsStartRequest{Repo: repo, Branch: "1-handoff"})
	if err != nil {
		t.Fatal(err)
	}
	record.WorktreePath = worktree
	record.PlanPath = filepath.Join(worktree, "plans", "handoff.md")
	record.Phase = core.IssueOpsPhaseImplement
	baseHead := strings.TrimSpace(preflight.GitOut(worktree, "rev-parse", "HEAD"))
	record.IssueURL = "https://github.com/acme/repo/issues/16"
	record.BranchPrepare = &issueopsmodel.IssueOpsBranchPrepare{
		Provider:     "github",
		IssueURL:     record.IssueURL,
		Branch:       "1-handoff",
		BaseBranch:   "main",
		BaseSHA:      baseHead,
		LinkVerified: true,
		CreatedAt:    "2026-07-11T00:00:00Z",
	}
	record.Intent = &issueopsmodel.IssueOpsIntentContract{
		RawRequest:        "Write the supervised handoff start contract",
		InterpretedIntent: "Preserve preview/confirm CAS semantics for supervised startup",
		SuccessCriteria:   []string{"preview returns reviewed context", "confirm requires expected_context_sha256"},
		Constraints:       []string{"no extra mutations", "support github and gitlab"},
		Ambiguities:       []string{"none"},
		NonGoals:          []string{"no additional handoff phases"},
		IntentClass:       "standard",
		RecordedAt:        "2026-07-11T00:00:00Z",
	}
	record.DesignReview = &issueopsmodel.IssueOpsDesignReview{
		ProblemSummary: "Need a preview-to-confirm CAS for supervised handoff start",
		ProposedDesign: "Require the reviewed context SHA before confirm and recompute it inside the lock",
		RefactorPlan:   "Keep the change limited to request plumbing and start-path validation",
		Alternatives:   []string{"accept preview hash as advisory only"},
		Risks:          []string{"drift between preview and confirm"},
		Verification:   []string{"design review checked alternatives and risks", "go test ./internal/core/issueops ./cmd/harness/mcpcli/issueops"},
		Approved:       true,
		ReviewedAt:     "2026-07-11T00:00:00Z",
	}
	record.ExecutionDecision = &issueopsmodel.IssueOpsExecutionDecision{
		AutoProceed: []string{"preview only"},
		HookBlocked: []string{"direct mutation without confirm"},
		HumanGates:  []string{"review expected_context_sha256"},
		SubagentUse: "none",
		RecordedAt:  "2026-07-11T00:00:00Z",
	}
	record.CompatibilityReview = &issueopsmodel.IssueOpsCompatibilityReview{
		BackwardCompatibility: []string{"preview output remains unchanged"},
		SideEffects:           []string{"none before confirm"},
		RollbackPlan:          "remove expected_context_sha256 enforcement",
		Verification:          []string{"contract tests"},
		Approved:              true,
		ReviewedAt:            "2026-07-11T00:00:00Z",
	}
	record.DevilsAdvocateReview = &issueopsmodel.IssueOpsDevilsAdvocateReview{Verdict: "pass", RecordedAt: "2026-07-11T00:00:00Z"}
	record.WorktreeTools = &issueopsmodel.IssueOpsWorktreeToolPreparation{OK: true, WorktreePath: worktree, PreparedAt: "2026-07-11T00:00:00Z"}
	record.ExecutionHandoff = &issueopsmodel.IssueOpsExecutionHandoff{
		ProtocolVersion: handoff.ProtocolVersion, State: handoff.StateCoordinatorPreparing, Attempt: 1, OwnershipEpoch: "epoch-1", AttemptBaseHead: baseHead,
		ContextVersion: 0, ContextOptions: nil, Driver: "orca", Agent: "codex",
		CoordinatorRoot: repo, WorkerRoot: worktree,
		Orca: &issueopsmodel.IssueOpsOrcaIdentity{
			RuntimeID: "runtime-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/1-handoff", WorktreeID: "wt-1", WorktreeInstanceID: "instance-1", WorktreePath: worktree,
		},
	}
	record, err = core.WriteIssueOps(core.IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func commitMCPHandoffResult(t *testing.T, worktree string) string {
	t.Helper()
	writeMCPHandoffFile(t, worktree, "internal/x.go", "package internal\n")
	writeMCPHandoffFile(t, worktree, ".agent-harness/research/report.md", "# evidence\n")
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", "test: finish handoff"}} {
		if code, _, stderr := preflight.GitCmd(worktree, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	return strings.TrimSpace(preflight.GitOut(worktree, "rev-parse", "HEAD"))
}

func writeMCPHandoffFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func cloneHandoffArgs(input map[string]any) map[string]any {
	result := make(map[string]any, len(input)+1)
	for key, value := range input {
		result[key] = value
	}
	return result
}

func nestedMap(value map[string]any, key string) map[string]any {
	result, _ := value[key].(map[string]any)
	return result
}

func callMCPUnknownTool(t *testing.T, name string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"name": name, "arguments": map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	_, rpcErr := mcpcli.HandleToolCall(raw)
	if rpcErr == nil {
		return "handled"
	}
	return rpcErr.Message
}

type handoffStartOrcaFake struct {
	workerRoot      string
	terminals       []port.OrcaTerminal
	terminalRequest port.OrcaCreateTerminalRequest
	workerDoneCalls int
}

func (f *handoffStartOrcaFake) ListWorktrees(context.Context, string) ([]port.OrcaWorktree, error) {
	return nil, fmt.Errorf("unexpected worktree list during supervised start")
}

func (f *handoffStartOrcaFake) ListTerminals(context.Context, string) ([]port.OrcaTerminal, error) {
	return append([]port.OrcaTerminal(nil), f.terminals...), nil
}

func (f *handoffStartOrcaFake) CreateTerminal(_ context.Context, req port.OrcaCreateTerminalRequest) (port.OrcaTerminal, error) {
	f.terminalRequest = req
	terminal := port.OrcaTerminal{Handle: "term_worker", PTYID: "pty-1", WorktreeID: req.WorktreeID, WorktreePath: f.workerRoot, Connected: true, Writable: true}
	f.terminals = []port.OrcaTerminal{terminal}
	return terminal, nil
}

func (f *handoffStartOrcaFake) RefreshTerminal(context.Context, string, string) (port.OrcaTerminal, error) {
	return port.OrcaTerminal{}, fmt.Errorf("unexpected terminal refresh during supervised start")
}

func (f *handoffStartOrcaFake) ListTasks(context.Context) ([]port.OrcaTask, error) {
	return nil, nil
}

func (f *handoffStartOrcaFake) ListDispatchedTasks(context.Context) ([]port.OrcaTask, error) {
	return nil, nil
}

func (f *handoffStartOrcaFake) CreateTask(_ context.Context, req port.OrcaCreateTaskRequest) (port.OrcaTask, error) {
	return port.OrcaTask{ID: "task-1", Title: req.Title, DisplayName: req.DisplayName, Status: "ready"}, nil
}

func (f *handoffStartOrcaFake) Dispatch(_ context.Context, req port.OrcaDispatchRequest) (port.OrcaDispatch, error) {
	return port.OrcaDispatch{ID: "dispatch-1", TaskID: req.TaskID, AssigneeHandle: req.ToHandle, Status: "dispatched", Injected: req.Inject, Preamble: fmt.Sprintf("Your coordinator's terminal handle is: %s\nYour task ID is: %s\n  --task-id %s --dispatch-id dispatch-1", req.FromHandle, req.TaskID, req.TaskID)}, nil
}

func (f *handoffStartOrcaFake) ShowDispatch(context.Context, string) (port.OrcaDispatch, error) {
	return port.OrcaDispatch{}, fmt.Errorf("unexpected dispatch show during supervised start")
}

func (f *handoffStartOrcaFake) ShowDispatchFrom(context.Context, string, string) (port.OrcaDispatch, error) {
	return port.OrcaDispatch{}, fmt.Errorf("unexpected dispatch show during supervised start")
}

func (f *handoffStartOrcaFake) SendWorkerDone(context.Context, port.OrcaWorkerDoneRequest) (port.OrcaWorkerDoneResult, error) {
	f.workerDoneCalls++
	return port.OrcaWorkerDoneResult{MessageID: "msg-mcp", Sequence: 12}, nil
}
