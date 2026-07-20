package issueops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/core/issueops/handoff"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/port"
)

func TestWorktreePrepareAutoUnavailableIsByteExactLegacyInlineAndStateNeutral(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	want, err := frozenLegacyWorktreePrepareResult(record)
	if err != nil {
		t.Fatal(err)
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	beforeRecord := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	beforeEntries, err := os.ReadDir(stateRoot)
	if err != nil {
		t.Fatal(err)
	}

	got, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
		ID: record.ID, Orchestrator: IssueOpsOrchestratorAuto, Agent: "codex", Confirm: true,
	}, &prepareOrcaFake{probeErr: errors.New("orca unavailable")}, handoffPrepareTestClock())
	if err != nil {
		t.Fatal(err)
	}
	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotJSON, wantJSON) {
		t.Fatalf("auto fallback JSON changed legacy bytes:\n got: %s\nwant: %s", gotJSON, wantJSON)
	}
	if after := rawIssueOpsBytesForTest(t, stateRoot, record.ID); !reflect.DeepEqual(after, beforeRecord) {
		t.Fatal("auto fallback changed the durable IssueOps record")
	}
	afterEntries, err := os.ReadDir(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(entryNames(beforeEntries), entryNames(afterEntries)) {
		t.Fatalf("auto fallback created state artifacts: before=%v after=%v", entryNames(beforeEntries), entryNames(afterEntries))
	}
}

func frozenLegacyWorktreePrepareResult(record IssueOpsRecord) (IssueOpsHandoffPrepareResult, error) {
	repo, branch := strings.TrimSpace(record.Repo), strings.TrimSpace(record.Branch)
	if repo == "" || branch == "" {
		return IssueOpsHandoffPrepareResult{}, fmt.Errorf("repo and branch must be set on the IssueOps record")
	}
	baseBranch := "main"
	if record.BranchPrepare != nil && strings.TrimSpace(record.BranchPrepare.BaseBranch) != "" {
		baseBranch = strings.TrimSpace(record.BranchPrepare.BaseBranch)
	}
	path := repo + ".worktrees/" + strings.ReplaceAll(branch, "/", "-")
	return IssueOpsHandoffPrepareResult{
		OK: true, ID: record.ID, Repo: repo, Branch: branch, BaseBranch: baseBranch, WorktreePath: path,
		Command:  []string{"git", "worktree", "add", path, branch},
		NextStep: "execute the command above, then run issueops link-worktree --id " + record.ID + " --worktree-path " + path,
	}, nil
}

func entryNames(entries []os.DirEntry) []string {
	names := make([]string, len(entries))
	for i := range entries {
		names[i] = entries[i].Name()
	}
	return names
}

func TestWorktreePrepareAutoProbeFailurePreservesLegacyInlineResult(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	client := &prepareOrcaFake{probeErr: errors.New("orca unavailable")}

	got, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
		ID: record.ID, Orchestrator: "auto", Agent: "codex", Confirm: true,
	}, client, handoffPrepareTestClock())
	if err != nil {
		t.Fatalf("PrepareIssueOpsHandoffWorktree: %v", err)
	}
	if got.ResolvedMode != "" || got.FallbackCode != "" || got.RequestedMode != "" {
		t.Fatalf("expected byte-exact legacy inline fallback, got %#v", got)
	}
	if got.ID != record.ID || got.Repo != record.Repo || got.Branch != record.Branch || len(got.Command) == 0 {
		t.Fatalf("legacy inline fields changed: %#v", got)
	}
	if !reflect.DeepEqual(client.trace, []string{"probe"}) {
		t.Fatalf("expected probe-only trace, got %v", client.trace)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExecutionHandoff != nil {
		t.Fatalf("fallback must not persist handoff: %#v", persisted.ExecutionHandoff)
	}
}

func TestWorktreePrepareAutoAndOmittedReadyOrcaResolveToOrca(t *testing.T) {
	for _, mode := range []string{"", IssueOpsOrchestratorAuto} {
		name := mode
		if name == "" {
			name = "omitted"
		}
		t.Run(name, func(t *testing.T) {
			stateRoot, record := handoffPrepareRecord(t)
			client := &prepareOrcaFake{probe: port.OrcaProbeResult{
				Available: true, Ready: true, RuntimeID: "runtime-1", RepoID: "repo-1", RepoRemoteName: "origin",
			}}

			got, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
				ID: record.ID, Orchestrator: mode, Agent: "codex",
			}, client, handoffPrepareTestClock())
			if err != nil {
				t.Fatal(err)
			}
			if got.RequestedMode != IssueOpsOrchestratorAuto || got.ResolvedMode != IssueOpsOrchestratorOrca || !got.Preview {
				t.Fatalf("ready Orca resolution = %#v", got)
			}
			if !reflect.DeepEqual(client.trace, []string{"probe"}) {
				t.Fatalf("ready Orca trace = %v, want probe", client.trace)
			}
		})
	}
}

func TestWorktreePrepareExplicitInlineRequiresAuthorizationBeforeProbeOrMutation(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	client := &prepareOrcaFake{probe: port.OrcaProbeResult{Available: true, Ready: true}}

	_, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
		ID: record.ID, Orchestrator: IssueOpsOrchestratorInline,
	}, client, handoffPrepareTestClock())
	if err == nil || err.Error() != "explicit inline requires --inline-reason user-requested|recovery" {
		t.Fatalf("missing inline authorization error = %v", err)
	}
	if len(client.trace) != 0 {
		t.Fatalf("missing authorization reached Orca: %v", client.trace)
	}
	if after := rawIssueOpsBytesForTest(t, stateRoot, record.ID); !reflect.DeepEqual(after, before) {
		t.Fatal("missing authorization mutated IssueOps state")
	}
}

func TestWorktreePrepareExplicitInlineAuthorizationIsBoundedAndAuditable(t *testing.T) {
	for _, reason := range []string{IssueOpsInlineReasonUserRequested, IssueOpsInlineReasonRecovery} {
		t.Run(reason, func(t *testing.T) {
			stateRoot, record := handoffPrepareRecord(t)
			client := &prepareOrcaFake{probe: port.OrcaProbeResult{Available: true, Ready: true}}

			got, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
				ID: record.ID, Orchestrator: IssueOpsOrchestratorInline, InlineReason: reason, Confirm: true,
			}, client, handoffPrepareTestClock())
			if err != nil {
				t.Fatal(err)
			}
			if got.ResolvedMode != IssueOpsOrchestratorInline || got.InlineReason != reason || got.Preview || len(got.Command) == 0 {
				t.Fatalf("authorized inline result = %#v", got)
			}
			if len(client.trace) != 0 {
				t.Fatalf("authorized inline probed Orca: %v", client.trace)
			}
			persisted, readErr := ReadIssueOps(stateRoot, record.ID)
			if readErr != nil || persisted.ExecutionHandoff != nil {
				t.Fatalf("authorized inline persisted supervised state: %#v err=%v", persisted.ExecutionHandoff, readErr)
			}
		})
	}
}

func TestWorktreePrepareExplicitInlineRejectsUnknownReasonBeforeProbeOrMutation(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	client := &prepareOrcaFake{probe: port.OrcaProbeResult{Available: true, Ready: true}}

	_, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
		ID: record.ID, Orchestrator: IssueOpsOrchestratorInline, InlineReason: "simpler",
	}, client, handoffPrepareTestClock())
	if err == nil || err.Error() != "inline reason must be user-requested or recovery" {
		t.Fatalf("invalid inline authorization error = %v", err)
	}
	if len(client.trace) != 0 {
		t.Fatalf("invalid authorization reached Orca: %v", client.trace)
	}
	if after := rawIssueOpsBytesForTest(t, stateRoot, record.ID); !reflect.DeepEqual(after, before) {
		t.Fatal("invalid authorization mutated IssueOps state")
	}
}

func TestWorktreePrepareInlineAuthorizationValidationPrecedesStateRead(t *testing.T) {
	for _, tt := range []struct {
		name, reason, want string
	}{
		{name: "missing", want: "explicit inline requires --inline-reason user-requested|recovery"},
		{name: "unknown", reason: "simpler", want: "inline reason must be user-requested or recovery"},
		{name: "uppercase", reason: "USER-REQUESTED", want: "inline reason must be user-requested or recovery"},
		{name: "whitespace padded", reason: " recovery ", want: "inline reason must be user-requested or recovery"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot := filepath.Join(t.TempDir(), "missing-state-root")
			client := &prepareOrcaFake{probe: port.OrcaProbeResult{Available: true, Ready: true}}
			_, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
				ID: "io-missing", Orchestrator: IssueOpsOrchestratorInline, InlineReason: tt.reason,
			}, client, handoffPrepareTestClock())
			if err == nil || err.Error() != tt.want {
				t.Fatalf("authorization error = %v, want %q", err, tt.want)
			}
			if len(client.trace) != 0 {
				t.Fatalf("authorization validation reached Orca: %v", client.trace)
			}
			if _, statErr := os.Stat(stateRoot); !os.IsNotExist(statErr) {
				t.Fatalf("authorization validation touched state root: %v", statErr)
			}
		})
	}
}

func TestWorktreePrepareRejectsInlineReasonOutsideExplicitInlineBeforeStateRead(t *testing.T) {
	for _, mode := range []string{"", IssueOpsOrchestratorAuto, IssueOpsOrchestratorOrca} {
		name := mode
		if name == "" {
			name = "omitted"
		}
		t.Run(name, func(t *testing.T) {
			stateRoot := filepath.Join(t.TempDir(), "missing-state-root")
			client := &prepareOrcaFake{probe: port.OrcaProbeResult{Available: true, Ready: true}}
			_, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
				ID: "io-missing", Orchestrator: mode, InlineReason: IssueOpsInlineReasonRecovery,
			}, client, handoffPrepareTestClock())
			if err == nil || err.Error() != "--inline-reason is valid only with --orchestrator inline" {
				t.Fatalf("non-inline authorization error = %v", err)
			}
			if len(client.trace) != 0 {
				t.Fatalf("non-inline authorization reached Orca: %v", client.trace)
			}
		})
	}
}

func TestWorktreePrepareAutoUnavailablePreservesNilBranchPrepareLegacyResult(t *testing.T) {
	for _, tt := range []struct {
		name   string
		client IssueOpsOrcaWorktreeClient
		trace  []string
	}{
		{name: "adapter unavailable"},
		{name: "probe unavailable", client: &prepareOrcaFake{probeErr: errors.New("orca unavailable")}, trace: []string{"probe"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffPrepareRecord(t)
			record.BranchPrepare = nil
			if _, err := WriteIssueOps(stateRoot, record); err != nil {
				t.Fatal(err)
			}

			got, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
				ID: record.ID, Orchestrator: IssueOpsOrchestratorAuto, Agent: "codex", Confirm: true,
			}, tt.client, handoffPrepareTestClock())
			if err != nil {
				t.Fatal(err)
			}
			if got.ResolvedMode != "" || got.FallbackCode != "" || got.RequestedMode != "" || len(got.Warnings) != 0 {
				t.Fatalf("nil BranchPrepare legacy fallback = %#v", got)
			}
			if got.BaseBranch != "main" || len(got.Command) == 0 || got.Command[0] != "git" {
				t.Fatalf("nil BranchPrepare changed legacy projection: %#v", got)
			}
			if fake, ok := tt.client.(*prepareOrcaFake); ok && !reflect.DeepEqual(fake.trace, tt.trace) {
				t.Fatalf("probe trace = %v, want %v", fake.trace, tt.trace)
			}
		})
	}
}

func TestWorktreePrepareExplicitOrcaProbeFailureHasProbeOnlyTrace(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	client := &prepareOrcaFake{probeErr: errors.New("not ready")}

	_, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
		ID: record.ID, Orchestrator: "orca", Agent: "codex", Confirm: true,
	}, client, handoffPrepareTestClock())
	if err == nil || !strings.Contains(err.Error(), "probe") {
		t.Fatalf("expected explicit probe error, got %v", err)
	}
	if !reflect.DeepEqual(client.trace, []string{"probe"}) {
		t.Fatalf("expected probe-only trace, got %v", client.trace)
	}
	persisted, _ := ReadIssueOps(stateRoot, record.ID)
	if persisted.ExecutionHandoff != nil {
		t.Fatal("probe failure mutated the record")
	}
}

func TestWorktreePrepareOrchestrationUnreadyNeverCreatesArtifact(t *testing.T) {
	for _, mode := range []string{IssueOpsOrchestratorAuto, IssueOpsOrchestratorOrca} {
		t.Run(mode, func(t *testing.T) {
			stateRoot, record := handoffPrepareRecord(t)
			client := &prepareOrcaFake{probe: port.OrcaProbeResult{Available: true, Ready: false, Code: "orchestration_unready"}}
			got, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
				ID: record.ID, Orchestrator: mode, Agent: "codex", Confirm: true,
			}, client, handoffPrepareTestClock())
			if mode == IssueOpsOrchestratorAuto {
				if err != nil || got.ResolvedMode != "" || got.FallbackCode != "" || got.RequestedMode != "" {
					t.Fatalf("auto readiness fallback = %#v err=%v", got, err)
				}
			} else if err == nil {
				t.Fatal("explicit Orca readiness failure must error")
			}
			persisted, readErr := ReadIssueOps(stateRoot, record.ID)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if persisted.ExecutionHandoff != nil || client.createCalls != 0 || !reflect.DeepEqual(client.trace, []string{"probe"}) {
				t.Fatalf("orchestration readiness failure mutated state: handoff=%#v calls=%d trace=%v", persisted.ExecutionHandoff, client.createCalls, client.trace)
			}
		})
	}
}

func TestWorktreePrepareInitialInventoryFailureFallsBackOnlyInAuto(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code string
	}{
		{name: "timeout", err: context.DeadlineExceeded, code: "orca_worktree_inventory_timeout"},
		{name: "command failure", err: &port.OrcaError{Code: "command_failed", Invoked: true}, code: "orca_worktree_inventory_failed"},
		{name: "incomplete list", err: &port.OrcaError{Code: "incomplete_list", Detail: "totalCount mismatch"}, code: "orca_worktree_inventory_incomplete"},
	}
	for _, tt := range tests {
		for _, mode := range []string{IssueOpsOrchestratorAuto, IssueOpsOrchestratorOrca} {
			t.Run(tt.name+"/"+mode, func(t *testing.T) {
				stateRoot, record := handoffPrepareRecord(t)
				client := &prepareOrcaFake{
					probe:       port.OrcaProbeResult{Available: true, Ready: true, RuntimeID: "runtime-1", RepoID: "repo-1", RepoRemoteName: "origin"},
					worktreeErr: tt.err,
				}
				got, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
					ID: record.ID, Orchestrator: mode, Agent: "codex", Confirm: true,
				}, client, handoffPrepareTestClock())
				if mode == IssueOpsOrchestratorAuto {
					if err != nil || got.ResolvedMode != "" || got.FallbackCode != "" || got.RequestedMode != "" {
						t.Fatalf("auto initial inventory fallback = %#v err=%v", got, err)
					}
				} else if err == nil || !strings.Contains(err.Error(), "list Orca worktrees") {
					t.Fatalf("explicit Orca inventory failure must remain an error: %v", err)
				}
				persisted, readErr := ReadIssueOps(stateRoot, record.ID)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if persisted.ExecutionHandoff != nil || client.createCalls != 0 || !reflect.DeepEqual(client.trace, []string{"probe", "worktree-list"}) {
					t.Fatalf("inventory failure crossed mutation boundary: handoff=%#v creates=%d trace=%v", persisted.ExecutionHandoff, client.createCalls, client.trace)
				}
			})
		}
	}
}

func TestWorktreePrepareCanonicalizesAgentBeforeProbeAndPersistence(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "omitted defaults codex", input: "", want: "codex"},
		{name: "uppercase codex", input: "CODEX", want: "codex"},
		{name: "spaced claude", input: "  claude  ", want: "claude"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffPrepareRecord(t)
			worktree := handoffPrepareWorktreePath(record)
			client := &prepareOrcaFake{
				probe: port.OrcaProbeResult{Available: true, Ready: true, RuntimeID: "runtime-1", RepoID: "repo-1", RepoRemoteName: "origin"},
				create: port.OrcaWorktree{
					ID: "wt-1", InstanceID: "inst-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/16-demo", Path: worktree,
					Branch: "refs/heads/" + record.Branch, Head: record.BranchPrepare.BaseSHA, Issue: 16, Comment: issueOpsHandoffMarker(record.ID, "epoch-1", 1),
				},
			}
			materializePrepareWorktreeOnCreate(t, client, worktree)
			if _, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
				ID: record.ID, Orchestrator: IssueOpsOrchestratorOrca, Agent: tt.input, Host: tt.want, SessionID: "preparation-session", SourceCWD: record.Repo, Confirm: true,
			}, client, handoffPrepareTestClock()); err != nil {
				t.Fatal(err)
			}
			persisted, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(client.probeRequests) != 1 || client.probeRequests[0].Agent != tt.want || persisted.ExecutionWorkspace == nil || persisted.ExecutionWorkspace.Agent != tt.want {
				t.Fatalf("canonical agent was not shared by probe and workspace: probes=%#v workspace=%#v", client.probeRequests, persisted.ExecutionWorkspace)
			}
		})
	}
}

func TestWorktreePrepareRejectsUnsupportedAgentBeforeProbeOrFallback(t *testing.T) {
	tests := []struct {
		name, agent, secret string
	}{
		{name: "unsupported", agent: "reasonix"},
		{name: "bearer secret", agent: "Authorization: Bearer super-secret-token", secret: "super-secret-token"},
		{name: "api key oversize", agent: "api_key=super-secret-value" + strings.Repeat("x", 16*1024), secret: "super-secret-value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffPrepareRecord(t)
			client := &prepareOrcaFake{probe: port.OrcaProbeResult{Available: true, Ready: true}}
			got, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
				ID: record.ID, Orchestrator: IssueOpsOrchestratorAuto, Agent: tt.agent, Confirm: true,
			}, client, handoffPrepareTestClock())
			if err == nil || got.ResolvedMode == IssueOpsOrchestratorInline || got.FallbackCode != "" {
				t.Fatalf("unsupported agent must be a request error, not Orca availability fallback: got=%#v err=%v", got, err)
			}
			if tt.secret != "" && strings.Contains(err.Error(), tt.secret) || len(err.Error()) > 256 {
				t.Fatalf("unsupported agent diagnostic leaked or exceeded bound: len=%d", len(err.Error()))
			}
			if len(client.trace) != 0 || client.createCalls != 0 {
				t.Fatalf("unsupported agent crossed the mutation boundary: trace=%v creates=%d", client.trace, client.createCalls)
			}
			persisted, readErr := ReadIssueOps(stateRoot, record.ID)
			if readErr != nil || persisted.ExecutionHandoff != nil {
				t.Fatalf("unsupported agent persisted handoff: %#v err=%v", persisted.ExecutionHandoff, readErr)
			}
		})
	}
}

func TestWorktreePrepareExplicitInlineIgnoresOrcaOnlyAgentIdentity(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	client := &prepareOrcaFake{probe: port.OrcaProbeResult{Available: true, Ready: true}}
	got, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
		ID: record.ID, Orchestrator: IssueOpsOrchestratorInline, InlineReason: IssueOpsInlineReasonRecovery, Agent: "reasonix",
	}, client, handoffPrepareTestClock())
	if err != nil {
		t.Fatalf("legacy inline mode must not interpret an Orca-only agent: %v", err)
	}
	if got.ResolvedMode != IssueOpsOrchestratorInline || !got.Preview || len(client.trace) != 0 {
		t.Fatalf("inline compatibility result changed: got=%#v trace=%v", got, client.trace)
	}
	persisted, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil || persisted.ExecutionHandoff != nil {
		t.Fatalf("inline preview persisted supervised state: %#v err=%v", persisted.ExecutionHandoff, readErr)
	}
}

func TestWorktreePrepareExistingHandoffNeverFallsBackInline(t *testing.T) {
	for _, mode := range []string{"auto", "inline"} {
		t.Run(mode, func(t *testing.T) {
			stateRoot, record := handoffPrepareRecord(t)
			worktree := handoffPrepareWorktreePath(record)
			client := &prepareOrcaFake{
				probe: port.OrcaProbeResult{Available: true, Ready: true, RepoID: "repo-1", RepoRemoteName: "origin"},
				create: port.OrcaWorktree{
					ID: "wt-1", InstanceID: "inst-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/16-demo", Path: worktree,
					Branch: "refs/heads/" + record.Branch, Head: record.BranchPrepare.BaseSHA, Issue: 16, Comment: issueOpsHandoffMarker(record.ID, "epoch-1", 1),
				},
			}
			materializePrepareWorktreeOnCreate(t, client, worktree)
			if _, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
				ID: record.ID, Orchestrator: "orca", Agent: "codex", Host: "codex", SessionID: "preparation-session", SourceCWD: record.Repo, Confirm: true,
			}, client, handoffPrepareTestClock()); err != nil {
				t.Fatal(err)
			}

			client.trace = nil
			client.probeErr = errors.New("orca unavailable after mutation")
			req := IssueOpsHandoffPrepareRequest{ID: record.ID, Orchestrator: mode, Agent: "codex", Host: "codex", SessionID: "preparation-session", SourceCWD: record.Repo, Confirm: true}
			if mode == IssueOpsOrchestratorInline {
				req.InlineReason = IssueOpsInlineReasonRecovery
			}
			got, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, req, client, handoffPrepareTestClock())
			if err != nil {
				t.Fatal(err)
			}
			if got.ResolvedMode != IssueOpsOrchestratorOrca || got.WorkspaceState != "ready" || got.HandoffState != "" {
				t.Fatalf("existing workspace must remain unfenced Orca preparation: %#v", got)
			}
			if len(client.trace) != 0 || client.createCalls != 1 {
				t.Fatalf("existing handoff must not probe or create again: trace=%v creates=%d", client.trace, client.createCalls)
			}
			if len(got.Command) != 0 || strings.Contains(got.NextStep, "git worktree add") {
				t.Fatalf("existing handoff must not offer legacy inline mutation: %#v", got)
			}
		})
	}
}

func TestWorktreePreparePreviewNeverMutates(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	client := &prepareOrcaFake{probe: port.OrcaProbeResult{Available: true, Ready: true}}

	got, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
		ID: record.ID, Orchestrator: "orca", Agent: "codex",
	}, client, handoffPrepareTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if !got.Preview || got.ResolvedMode != "orca" {
		t.Fatalf("unexpected preview: %#v", got)
	}
	if !reflect.DeepEqual(client.trace, []string{"probe"}) {
		t.Fatalf("preview invoked mutation: %v", client.trace)
	}
	persisted, _ := ReadIssueOps(stateRoot, record.ID)
	if persisted.ExecutionHandoff != nil {
		t.Fatal("preview mutated record")
	}
}

func TestOrcaWorktreePrepareKeepsPreparationUnfenced(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	worktree := handoffPrepareWorktreePath(record)
	client := &prepareOrcaFake{
		probe:  port.OrcaProbeResult{Available: true, Ready: true, RepoID: "repo-1", RepoRemoteName: "origin"},
		create: port.OrcaWorktree{ID: "wt-1", InstanceID: "inst-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/16-demo", Path: worktree, Branch: "refs/heads/" + record.Branch, Head: record.BranchPrepare.BaseSHA, Issue: 16, Comment: issueOpsHandoffMarker(record.ID, "epoch-1", 1)},
	}
	materializePrepareWorktreeOnCreate(t, client, worktree)
	req := IssueOpsHandoffPrepareRequest{ID: record.ID, Orchestrator: "orca", Agent: "codex", Host: "codex", SessionID: "preparation-session", AgentID: "preparation-agent", SourceCWD: record.Repo, Confirm: true}

	first, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, req, client, handoffPrepareTestClock())
	if err != nil {
		t.Fatalf("first prepare: %v", err)
	}
	second, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, req, client, handoffPrepareTestClock())
	if err != nil {
		t.Fatalf("second prepare: %v", err)
	}
	if client.createCalls != 1 {
		t.Fatalf("expected one create call, got %d (%v)", client.createCalls, client.trace)
	}
	if first.State != "ready" || second.State != first.State || first.Orca == nil || first.Orca.WorktreeID != "wt-1" {
		t.Fatalf("unexpected results: first=%#v second=%#v", first, second)
	}
	persisted, _ := ReadIssueOps(stateRoot, record.ID)
	if persisted.WorktreePath != worktree || persisted.ExecutionWorkspace == nil || persisted.ExecutionWorkspace.State != "ready" || persisted.ExecutionHandoff != nil {
		t.Fatalf("unexpected persisted workspace/handoff: workspace=%#v handoff=%#v", persisted.ExecutionWorkspace, persisted.ExecutionHandoff)
	}
	if persisted.ExecutionWorkspace.BaseHead != record.BranchPrepare.BaseSHA || persisted.ExecutionWorkspace.Orca == nil || persisted.ExecutionWorkspace.Orca.WorkerTerminalHandle != "" || persisted.ExecutionWorkspace.Orca.TaskID != "" || persisted.ExecutionWorkspace.Orca.DispatchID != "" {
		t.Fatalf("workspace must retain only preparation identity: %#v", persisted.ExecutionWorkspace)
	}
}

func TestExecutionWorkspaceJournalsBeforeOrcaCreate(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	worktree := handoffPrepareWorktreePath(record)
	client := &prepareOrcaFake{
		probe:  port.OrcaProbeResult{Available: true, Ready: true, RepoID: "repo-1", RepoRemoteName: "origin"},
		create: port.OrcaWorktree{ID: "wt-1", InstanceID: "inst-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/16-demo", Path: worktree, Branch: "refs/heads/" + record.Branch, Head: record.BranchPrepare.BaseSHA, Issue: 16, Comment: issueOpsHandoffMarker(record.ID, "epoch-1", 1)},
	}
	client.beforeCreate = func() {
		persisted, err := ReadIssueOps(stateRoot, record.ID)
		if err != nil {
			t.Errorf("read journal before Orca create: %v", err)
			return
		}
		workspace := persisted.ExecutionWorkspace
		if workspace == nil || workspace.State != "provisioning" || workspace.PendingOperation == nil || workspace.PendingOperation.Kind != handoff.OperationWorktreeCreate || workspace.PreparationSession == nil || workspace.PreparationSession.SessionID != "preparation-session" || persisted.ExecutionHandoff != nil {
			t.Errorf("unexpected pre-create journal: workspace=%#v handoff=%#v", workspace, persisted.ExecutionHandoff)
		}
		makeGitWorktreeMarker(t, worktree)
	}
	_, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{ID: record.ID, Orchestrator: "orca", Agent: "codex", Host: "codex", SessionID: "preparation-session", SourceCWD: record.Repo, Confirm: true}, client, handoffPrepareTestClock())
	if err != nil {
		t.Fatal(err)
	}
}

func TestExecutionWorkspaceRejectsMissingOrDifferentPreparationActor(t *testing.T) {
	for _, request := range []IssueOpsHandoffPrepareRequest{
		{Orchestrator: "orca", Agent: "codex", Confirm: true},
		{Orchestrator: "orca", Agent: "codex", Host: "claude", SessionID: "other", SourceCWD: "/wrong", Confirm: true},
	} {
		stateRoot, record := handoffPrepareRecord(t)
		request.ID = record.ID
		before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
		client := &prepareOrcaFake{probe: port.OrcaProbeResult{Available: true, Ready: true, RepoID: "repo-1", RepoRemoteName: "origin"}}
		if _, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, request, client, handoffPrepareTestClock()); err == nil {
			t.Fatalf("actorless or mismatched preparation unexpectedly succeeded: %#v", request)
		}
		if after := rawIssueOpsBytesForTest(t, stateRoot, record.ID); !reflect.DeepEqual(after, before) {
			t.Fatalf("invalid preparation actor mutated IssueOps state: %#v", request)
		}
		if client.createCalls != 0 || client.adoptCalls != 0 {
			t.Fatalf("invalid preparation actor reached external mutation: %#v", client.trace)
		}
	}
}

func TestExecutionWorkspaceTimeoutRequiresExplicitReconcile(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	worktree := handoffPrepareWorktreePath(record)
	client := &prepareOrcaFake{
		probe:     port.OrcaProbeResult{Available: true, Ready: true, RepoID: "repo-1", RepoRemoteName: "origin"},
		create:    port.OrcaWorktree{ID: "wt-1", InstanceID: "inst-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/16-demo", Path: worktree, Branch: "refs/heads/" + record.Branch, Head: record.BranchPrepare.BaseSHA, Issue: 16, Comment: issueOpsHandoffMarker(record.ID, "epoch-1", 1)},
		createErr: &port.OrcaError{Code: "timeout", Invoked: true, Timeout: true},
	}
	_, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{ID: record.ID, Orchestrator: "orca", Agent: "codex", Host: "codex", SessionID: "preparation-session", SourceCWD: record.Repo, Confirm: true}, client, handoffPrepareTestClock())
	if err == nil {
		t.Fatal("invoked timeout unexpectedly succeeded")
	}
	persisted, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if persisted.ExecutionWorkspace == nil || persisted.ExecutionWorkspace.State != handoff.StateRecoveryRequired || persisted.ExecutionWorkspace.PendingOperation == nil || persisted.ExecutionHandoff != nil {
		t.Fatalf("timeout must require workspace-only reconciliation: workspace=%#v handoff=%#v", persisted.ExecutionWorkspace, persisted.ExecutionHandoff)
	}
}

func TestExecutionWorkspaceReconcileAdoptsExactlyOneCandidate(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	worktree := handoffPrepareWorktreePath(record)
	client := &prepareOrcaFake{
		probe:     port.OrcaProbeResult{Available: true, Ready: true, RepoID: "repo-1", RepoRemoteName: "origin"},
		create:    port.OrcaWorktree{ID: "wt-1", InstanceID: "inst-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/16-demo", Path: worktree, Branch: "refs/heads/" + record.Branch, Head: record.BranchPrepare.BaseSHA, Issue: 16, Comment: issueOpsHandoffMarker(record.ID, "epoch-1", 1)},
		createErr: &port.OrcaError{Code: "timeout", Invoked: true, Timeout: true},
	}
	if _, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{ID: record.ID, Orchestrator: "orca", Agent: "codex", Host: "codex", SessionID: "preparation-session", SourceCWD: record.Repo, Confirm: true}, client, handoffPrepareTestClock()); err == nil {
		t.Fatal("expected recovery-required workspace")
	}
	makeGitWorktreeMarker(t, worktree)
	client.worktrees = []port.OrcaWorktree{client.create}
	persisted, err := ReconcileIssueOpsExecutionWorkspace(context.Background(), stateRoot, IssueOpsExecutionWorkspaceReconcileRequest{ID: record.ID, WorkspaceEpoch: "epoch-1", Actor: IssueOpsActor{Host: "codex", SessionID: "preparation-session", CWD: record.Repo}}, client, "2026-07-11T01:02:04Z")
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExecutionWorkspace == nil || persisted.ExecutionWorkspace.State != "ready" || persisted.ExecutionWorkspace.Orca == nil || persisted.ExecutionWorkspace.Orca.WorktreeID != "wt-1" || persisted.ExecutionHandoff != nil {
		t.Fatalf("workspace reconcile result = %#v", persisted)
	}
}

func TestWorktreePrepareGitLabAutoAndExplicitUseVerifiedBranchWithoutGitHubIssueMetadata(t *testing.T) {
	for _, mode := range []string{IssueOpsOrchestratorAuto, IssueOpsOrchestratorOrca} {
		for _, confirm := range []bool{false, true} {
			name := mode + "/preview"
			if confirm {
				name = mode + "/confirm"
			}
			t.Run(name, func(t *testing.T) {
				stateRoot, record := gitLabHandoffPrepareRecord(t)
				worktree := handoffPrepareWorktreePath(record)
				client := &prepareOrcaFake{
					probe: port.OrcaProbeResult{Available: true, Ready: true, RuntimeID: "runtime-1", RepoID: "repo-1", RepoRemoteName: "origin", Provider: "gitlab"},
					create: port.OrcaWorktree{
						ID: "wt-1", InstanceID: "inst-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/16-demo", Path: worktree,
						Branch: "refs/heads/" + record.Branch, Head: record.BranchPrepare.BaseSHA, Comment: issueOpsHandoffMarker(record.ID, "epoch-1", 1),
					},
				}
				if confirm {
					materializePrepareWorktreeOnCreate(t, client, worktree)
				}
				got, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
					ID: record.ID, Orchestrator: mode, Agent: "codex", Host: "codex", SessionID: "preparation-session", SourceCWD: record.Repo, Confirm: confirm,
				}, client, handoffPrepareTestClock())
				if err != nil {
					t.Fatal(err)
				}
				if got.ResolvedMode != IssueOpsOrchestratorOrca || !containsString(got.Warnings, IssueOpsGitLabNativeMetadataUnavailableWarning) {
					t.Fatalf("GitLab supervised result = %#v", got)
				}
				if len(client.probeRequests) != 1 || client.probeRequests[0].Provider != "gitlab" {
					t.Fatalf("GitLab provider was not passed to probe: %#v", client.probeRequests)
				}
				if !confirm {
					if !got.Preview || client.createCalls != 0 || !reflect.DeepEqual(client.trace, []string{"probe"}) {
						t.Fatalf("GitLab preview mutated: result=%#v trace=%v", got, client.trace)
					}
					return
				}
				if got.Preview || client.createCalls != 1 || len(client.createRequests) != 1 {
					t.Fatalf("GitLab confirm did not create exactly once: result=%#v calls=%#v", got, client.createRequests)
				}
				created := client.createRequests[0]
				if created.Provider != "gitlab" || created.Issue != 16 || created.BaseBranch != "refs/remotes/origin/16-demo" {
					t.Fatalf("GitLab create request lost provider branch authority: %#v", created)
				}
			})
		}
	}
}

func TestWorktreePrepareGitLabAutoProbeFailurePreservesInlineContract(t *testing.T) {
	for _, tt := range []struct {
		name  string
		probe port.OrcaProbeResult
		err   error
	}{
		{name: "Orca missing", probe: port.OrcaProbeResult{Code: "orca_not_found"}},
		{name: "Orca unready", probe: port.OrcaProbeResult{Available: true, Code: "runtime_unready"}},
		{name: "capability failed", probe: port.OrcaProbeResult{Available: true, Code: "capability_missing"}},
		{name: "probe error", err: errors.New("orca unavailable")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := gitLabHandoffPrepareRecord(t)
			client := &prepareOrcaFake{probe: tt.probe, probeErr: tt.err}
			got, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
				ID: record.ID, Orchestrator: IssueOpsOrchestratorAuto, Agent: "codex", Confirm: true,
			}, client, handoffPrepareTestClock())
			if err != nil {
				t.Fatal(err)
			}
			if got.ResolvedMode != "" || got.FallbackCode != "" || got.RequestedMode != "" || len(got.Warnings) != 0 {
				t.Fatalf("GitLab pre-mutation fallback changed the inline contract: %#v", got)
			}
			if !reflect.DeepEqual(client.trace, []string{"probe"}) {
				t.Fatalf("GitLab fallback crossed the probe boundary: %v", client.trace)
			}
			persisted, readErr := ReadIssueOps(stateRoot, record.ID)
			if readErr != nil || persisted.ExecutionHandoff != nil {
				t.Fatalf("GitLab fallback persisted supervised state: %#v err=%v", persisted.ExecutionHandoff, readErr)
			}
		})
	}
}

func TestWorktreePrepareGitLabAutoPostProbeFallbackClearsNativeMetadataWarning(t *testing.T) {
	stateRoot, record := gitLabHandoffPrepareRecord(t)
	client := &prepareOrcaFake{
		probe:       port.OrcaProbeResult{Available: true, Ready: true, RuntimeID: "runtime-1", RepoID: "repo-1", RepoRemoteName: "origin", Provider: "gitlab"},
		worktreeErr: errors.New("inventory unavailable"),
	}
	got, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
		ID: record.ID, Orchestrator: IssueOpsOrchestratorAuto, Agent: "codex", Confirm: true,
	}, client, handoffPrepareTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if got.ResolvedMode != "" || got.FallbackCode != "" || got.RequestedMode != "" || len(got.Warnings) != 0 {
		t.Fatalf("post-probe GitLab inline fallback = %#v", got)
	}
	if !reflect.DeepEqual(client.trace, []string{"probe", "worktree-list"}) {
		t.Fatalf("post-probe fallback trace = %v", client.trace)
	}
}

func TestWorktreePrepareGitLabValidatesProviderSpecificReturnedMetadata(t *testing.T) {
	zero, exact, mismatch := 0, 16, 17
	for _, tt := range []struct {
		name            string
		linkedIssue     int
		linkedGitLab    *int
		wantErr         string
		wantUnavailable bool
		wantLinkStatus  string
	}{
		{name: "null unavailable", wantUnavailable: true, wantLinkStatus: handoff.ProviderIssueLinkGitLabUnavailable},
		{name: "zero unavailable", linkedGitLab: &zero, wantUnavailable: true, wantLinkStatus: handoff.ProviderIssueLinkGitLabUnavailable},
		{name: "exact native metadata", linkedGitLab: &exact, wantLinkStatus: handoff.ProviderIssueLinkGitLabExact},
		{name: "conflicting GitHub metadata", linkedIssue: 16, wantErr: "GitHub linked issue metadata"},
		{name: "mismatched GitLab metadata", linkedGitLab: &mismatch, wantErr: "linked GitLab issue"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := gitLabHandoffPrepareRecord(t)
			worktree := handoffPrepareWorktreePath(record)
			client := &prepareOrcaFake{
				probe: port.OrcaProbeResult{Available: true, Ready: true, RuntimeID: "runtime-1", RepoID: "repo-1", RepoRemoteName: "origin", Provider: "gitlab"},
				create: port.OrcaWorktree{
					ID: "wt-1", InstanceID: "inst-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/16-demo", Path: worktree,
					Branch: "refs/heads/" + record.Branch, Head: record.BranchPrepare.BaseSHA, Issue: tt.linkedIssue, GitLabIssue: tt.linkedGitLab,
					Comment: issueOpsHandoffMarker(record.ID, "epoch-1", 1),
				},
			}
			materializePrepareWorktreeOnCreate(t, client, worktree)
			got, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
				ID: record.ID, Orchestrator: IssueOpsOrchestratorOrca, Agent: "codex", Host: "codex", SessionID: "preparation-session", SourceCWD: record.Repo, Confirm: true,
			}, client, handoffPrepareTestClock())
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("GitLab metadata error = %v, want %q", err, tt.wantErr)
				}
				persisted, readErr := ReadIssueOps(stateRoot, record.ID)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if persisted.ExecutionWorkspace == nil || persisted.ExecutionWorkspace.State != handoff.StateRecoveryRequired || persisted.ExecutionWorkspace.Orca == nil || persisted.ExecutionWorkspace.Orca.WorktreeID != "" || persisted.ExecutionHandoff != nil {
					t.Fatalf("conflicting GitLab metadata did not retain workspace-only recovery: %#v", persisted)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.wantUnavailable != containsString(got.Warnings, IssueOpsGitLabNativeMetadataUnavailableWarning) {
				t.Fatalf("GitLab metadata warning = %#v", got.Warnings)
			}
			persisted, readErr := ReadIssueOps(stateRoot, record.ID)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if persisted.ExecutionWorkspace == nil || persisted.ExecutionWorkspace.Orca == nil || persisted.ExecutionWorkspace.Orca.ProviderIssueLinkStatus != tt.wantLinkStatus || persisted.ExecutionHandoff != nil {
				t.Fatalf("durable GitLab metadata observation = %#v, want %q", persisted.ExecutionWorkspace, tt.wantLinkStatus)
			}
			reprojected, projectErr := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
				ID: record.ID, Orchestrator: IssueOpsOrchestratorAuto, Agent: "codex", Confirm: true,
			}, nil, handoffPrepareTestClock())
			if projectErr != nil {
				t.Fatal(projectErr)
			}
			if tt.wantUnavailable != containsString(reprojected.Warnings, IssueOpsGitLabNativeMetadataUnavailableWarning) {
				t.Fatalf("reprojected GitLab metadata warning = %#v", reprojected.Warnings)
			}
		})
	}
}

func TestWorktreePrepareRejectsProviderIssueURLMismatchBeforeOrca(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	record.BranchPrepare.Provider = "gitlab"
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	client := &prepareOrcaFake{probe: port.OrcaProbeResult{Available: true, Ready: true}}
	_, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
		ID: record.ID, Orchestrator: IssueOpsOrchestratorOrca, Agent: "codex", Confirm: true,
	}, client, handoffPrepareTestClock())
	if err == nil || !strings.Contains(err.Error(), "provider does not match IssueOps issue URL") {
		t.Fatalf("provider/URL mismatch error = %v", err)
	}
	if len(client.trace) != 0 {
		t.Fatalf("provider/URL mismatch called Orca: %v", client.trace)
	}
}

func TestWorktreePrepareUsesVerifiedProviderTrackingRef(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	worktree := handoffPrepareWorktreePath(record)
	client := &prepareOrcaFake{
		probe: port.OrcaProbeResult{Available: true, Ready: true, RepoID: "repo-1", RepoRemoteName: "upstream"},
		create: port.OrcaWorktree{
			ID: "wt-1", InstanceID: "inst-1", RepoID: "repo-1", BaseRef: "refs/remotes/upstream/16-demo", Path: worktree,
			Branch: "refs/heads/" + record.Branch, Head: record.BranchPrepare.BaseSHA, Issue: 16, Comment: issueOpsHandoffMarker(record.ID, "epoch-1", 1),
		},
	}
	materializePrepareWorktreeOnCreate(t, client, worktree)

	if _, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
		ID: record.ID, Orchestrator: "orca", Agent: "codex", Host: "codex", SessionID: "preparation-session", SourceCWD: record.Repo, Confirm: true,
	}, client, handoffPrepareTestClock()); err != nil {
		t.Fatal(err)
	}
	if len(client.createRequests) != 1 || client.createRequests[0].BaseBranch != "refs/remotes/upstream/16-demo" {
		t.Fatalf("create requests = %#v", client.createRequests)
	}
}

func TestWorktreePrepareCrashAfterInvocationNeverCreatesTwice(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	client := &prepareOrcaFake{
		probe:     port.OrcaProbeResult{Available: true, Ready: true, RepoID: "repo-1", RepoRemoteName: "origin"},
		createErr: &port.OrcaError{Code: "timeout", Invoked: true, Timeout: true},
	}
	req := IssueOpsHandoffPrepareRequest{ID: record.ID, Orchestrator: "orca", Agent: "codex", Host: "codex", SessionID: "preparation-session", SourceCWD: record.Repo, Confirm: true}

	if _, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, req, client, handoffPrepareTestClock()); err == nil {
		t.Fatal("expected ambiguous invocation error")
	}
	second, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, req, client, handoffPrepareTestClock())
	if err != nil {
		t.Fatalf("repeat should return recovery status: %v", err)
	}
	if client.createCalls != 1 || second.State != "recovery_required" || second.RecoveryCode == "" {
		t.Fatalf("automatic retry occurred or recovery missing: calls=%d result=%#v", client.createCalls, second)
	}
}

func TestWorktreePrepareAmbiguousRecoveryPreservesRuntimeWithoutDispatch(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	writeIssueOpsFile(t, record.Repo, "plans/plan.md", "# recovered plan\n")
	for _, args := range [][]string{{"add", "plans/plan.md"}, {"commit", "-q", "-m", "test: add recovered plan"}} {
		if code, _, stderr := preflight.GitCmd(record.Repo, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	baseHead := strings.TrimSpace(preflight.GitOut(record.Repo, "rev-parse", "HEAD"))
	for _, ref := range []string{"refs/remotes/origin/16-demo", "refs/remotes/upstream/16-demo"} {
		if code, _, stderr := preflight.GitCmd(record.Repo, "update-ref", ref, baseHead); code != 0 {
			t.Fatalf("update %s: %s", ref, stderr)
		}
	}
	record.BranchPrepare.BaseSHA = baseHead
	record.Phase = IssueOpsPhaseCompatibilityReview
	record.PlanPath = filepath.Join(handoffPrepareWorktreePath(record), "plans", "plan.md")
	record.Intent = issueOpsIntentContractForTest()
	record.DesignReview = issueOpsDesignReviewForTest()
	record.ExecutionDecision = issueOpsExecutionDecisionForTest()
	record.CompatibilityReview = issueOpsCompatibilityReviewForTest()
	record.DevilsAdvocateReview = &IssueOpsDevilsAdvocateReview{Verdict: "pass", RecordedAt: "2026-07-11T00:00:00Z"}
	record.WorktreeTools = &IssueOpsWorktreeToolPreparation{OK: true, WorktreePath: handoffPrepareWorktreePath(record), PreparedAt: "2026-07-11T00:00:00Z"}
	if _, err := WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}

	worktree := handoffPrepareWorktreePath(record)
	client := &prepareOrcaFake{
		probe:     port.OrcaProbeResult{Available: true, Ready: true, RuntimeID: "runtime-1", RepoID: "repo-1", RepoRemoteName: "origin"},
		createErr: &port.OrcaError{Code: "timeout", Invoked: true, Timeout: true},
	}
	materializePrepareWorktreeOnCreate(t, client, worktree)
	req := IssueOpsHandoffPrepareRequest{ID: record.ID, Orchestrator: IssueOpsOrchestratorOrca, Agent: "codex", Host: "codex", SessionID: "preparation-session", SourceCWD: record.Repo, Confirm: true}
	if _, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, req, client, handoffPrepareTestClock()); err == nil {
		t.Fatal("expected ambiguous create")
	}
	pending, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pending.ExecutionWorkspace == nil || pending.ExecutionWorkspace.Orca == nil || pending.ExecutionWorkspace.Orca.RuntimeID != "runtime-1" || pending.ExecutionHandoff != nil {
		t.Fatalf("pre-mutation journal lost workspace runtime: %#v", pending)
	}
	client.worktrees = []port.OrcaWorktree{{
		ID: "wt-1", InstanceID: "inst-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/16-demo", Path: worktree,
		Branch: "refs/heads/" + record.Branch, Head: baseHead, Issue: 16, Comment: issueOpsHandoffMarker(record.ID, "epoch-1", 1),
	}}
	if _, err := ReconcileIssueOpsExecutionWorkspace(context.Background(), stateRoot, IssueOpsExecutionWorkspaceReconcileRequest{ID: record.ID, WorkspaceEpoch: "epoch-1", Actor: IssueOpsActor{Host: "codex", SessionID: "preparation-session", CWD: record.Repo}}, client, "2026-07-11T01:02:04Z"); err != nil {
		t.Fatal(err)
	}
	recovered, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.ExecutionWorkspace == nil || recovered.ExecutionWorkspace.State != "ready" || recovered.ExecutionHandoff != nil || client.createCalls != 1 {
		t.Fatalf("workspace reconciliation created ownership or retried create: record=%#v create_calls=%d", recovered, client.createCalls)
	}
}

func TestWorktreePrepareSuccessRejectsChangedAuthorizedJournal(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	worktree := handoffPrepareWorktreePath(record)
	client := &prepareOrcaFake{
		probe: port.OrcaProbeResult{Available: true, Ready: true, RuntimeID: "runtime-1", RepoID: "repo-1", RepoRemoteName: "origin"},
		create: port.OrcaWorktree{
			ID: "wt-1", InstanceID: "inst-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/16-demo", Path: worktree,
			Branch: "refs/heads/" + record.Branch, Head: record.BranchPrepare.BaseSHA, Issue: 16, Comment: issueOpsHandoffMarker(record.ID, "epoch-1", 1),
		},
	}
	materializePrepareWorktreeOnCreate(t, client, worktree)
	previous := client.beforeCreate
	client.beforeCreate = func() {
		previous()
		current, err := ReadIssueOps(stateRoot, record.ID)
		if err != nil {
			t.Fatal(err)
		}
		current.PlanPath = "concurrent-plan.md"
		if _, err := WriteIssueOps(stateRoot, current); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
		ID: record.ID, Orchestrator: IssueOpsOrchestratorOrca, Agent: "codex", Host: "codex", SessionID: "preparation-session", SourceCWD: record.Repo, Confirm: true,
	}, client, handoffPrepareTestClock()); err == nil || !strings.Contains(err.Error(), "journal") {
		t.Fatalf("stale successful result must not be adopted: %v", err)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.PlanPath != "concurrent-plan.md" || persisted.ExecutionWorkspace == nil || persisted.ExecutionWorkspace.State != handoff.StateRecoveryRequired || persisted.ExecutionWorkspace.PendingOperation == nil || persisted.ExecutionWorkspace.Orca.WorktreeID != "" || persisted.ExecutionHandoff != nil {
		t.Fatalf("stale success overwrote authority or lost pending recovery: %#v", persisted)
	}
}

func TestWorktreePrepareRequiresExactAttemptMarkerOnSuccess(t *testing.T) {
	for _, tt := range []struct {
		name, comment string
	}{
		{name: "missing"},
		{name: "substring", comment: "prefix " + issueOpsHandoffMarker("io-placeholder", "epoch-1", 1) + " suffix"},
		{name: "wrong attempt", comment: issueOpsHandoffMarker("io-placeholder", "epoch-1", 2)},
		{name: "wrong epoch", comment: issueOpsHandoffMarker("io-placeholder", "epoch-other", 1)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffPrepareRecord(t)
			comment := strings.ReplaceAll(tt.comment, "io-placeholder", record.ID)
			worktree := handoffPrepareWorktreePath(record)
			client := &prepareOrcaFake{
				probe: port.OrcaProbeResult{Available: true, Ready: true, RuntimeID: "runtime-1", RepoID: "repo-1", RepoRemoteName: "origin"},
				create: port.OrcaWorktree{
					ID: "wt-1", InstanceID: "inst-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/16-demo", Path: worktree,
					Branch: "refs/heads/" + record.Branch, Head: record.BranchPrepare.BaseSHA, Issue: 16, Comment: comment,
				},
			}
			materializePrepareWorktreeOnCreate(t, client, worktree)
			if _, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
				ID: record.ID, Orchestrator: IssueOpsOrchestratorOrca, Agent: "codex", Host: "codex", SessionID: "preparation-session", SourceCWD: record.Repo, Confirm: true,
			}, client, handoffPrepareTestClock()); err == nil || !strings.Contains(err.Error(), "attempt marker") {
				t.Fatalf("non-exact worktree marker was accepted: %v", err)
			}
			persisted, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if persisted.ExecutionWorkspace == nil || persisted.ExecutionWorkspace.State != handoff.StateRecoveryRequired || persisted.ExecutionWorkspace.PendingOperation == nil || persisted.ExecutionHandoff != nil {
				t.Fatalf("marker mismatch did not retain workspace recovery: %#v", persisted)
			}
		})
	}
}

func TestWorktreePrepareDefinitiveStartFailureClearsJournalAndAutoFallsBack(t *testing.T) {
	for _, mode := range []string{IssueOpsOrchestratorAuto, IssueOpsOrchestratorOrca} {
		t.Run(mode, func(t *testing.T) {
			stateRoot, record := handoffPrepareRecord(t)
			before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
			client := &prepareOrcaFake{
				probe:     port.OrcaProbeResult{Available: true, Ready: true, RuntimeID: "runtime-1", RepoID: "repo-1", RepoRemoteName: "origin"},
				createErr: &port.OrcaError{Code: "command_start_failed", Invoked: false},
			}
			got, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
				ID: record.ID, Orchestrator: mode, Agent: "codex", Host: "codex", SessionID: "preparation-session", SourceCWD: record.Repo, Confirm: true,
			}, client, handoffPrepareTestClock())
			if mode == IssueOpsOrchestratorAuto {
				if err != nil || got.ResolvedMode != "" || got.FallbackCode != "" || got.RequestedMode != "" {
					t.Fatalf("safe auto fallback = %#v err=%v", got, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "safe to retry") {
				t.Fatalf("explicit definitive failure must be retryable: %v", err)
			}
			persisted, readErr := ReadIssueOps(stateRoot, record.ID)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if persisted.ExecutionHandoff != nil || client.createCalls != 1 {
				t.Fatalf("non-invoked worktree create left a journal/artifact: handoff=%#v calls=%d", persisted.ExecutionHandoff, client.createCalls)
			}
			if after := rawIssueOpsBytesForTest(t, stateRoot, record.ID); !reflect.DeepEqual(after, before) {
				t.Fatalf("definitive pre-invocation failure changed durable row bytes\nbefore=%s\n after=%s", before, after)
			}
		})
	}
}

func TestWorktreePrepareDefinitiveFailureRollbackRejectsConcurrentRecordChange(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	client := &prepareOrcaFake{
		probe:     port.OrcaProbeResult{Available: true, Ready: true, RuntimeID: "runtime-1", RepoID: "repo-1", RepoRemoteName: "origin"},
		createErr: &port.OrcaError{Code: "command_start_failed", Invoked: false},
	}
	client.beforeCreate = func() {
		current, err := ReadIssueOps(stateRoot, record.ID)
		if err != nil {
			t.Fatal(err)
		}
		current.PlanPath = "concurrent-plan.md"
		if _, err := WriteIssueOps(stateRoot, current); err != nil {
			t.Fatal(err)
		}
	}
	_, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
		ID: record.ID, Orchestrator: IssueOpsOrchestratorAuto, Agent: "codex", Host: "codex", SessionID: "preparation-session", SourceCWD: record.Repo, Confirm: true,
	}, client, handoffPrepareTestClock())
	if err == nil || !strings.Contains(err.Error(), "journal changed") {
		t.Fatalf("concurrent record drift must make rollback fail closed: %v", err)
	}
	persisted, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if persisted.PlanPath != "concurrent-plan.md" || persisted.ExecutionWorkspace == nil || persisted.ExecutionWorkspace.PendingOperation == nil || persisted.ExecutionHandoff != nil {
		t.Fatalf("rollback overwrote concurrent state or erased its journal: %#v", persisted)
	}
}

func TestWorktreePrepareRejectsBaselineWithoutDeltaHeadroom(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	rows := make([]port.OrcaWorktree, 0, handoff.MaxBaselineIDs)
	for i := 0; i < handoff.MaxBaselineIDs; i++ {
		rows = append(rows, port.OrcaWorktree{ID: fmt.Sprintf("wt-%03d", i)})
	}
	client := &prepareOrcaFake{
		probe:     port.OrcaProbeResult{Available: true, Ready: true, RuntimeID: "runtime-1", RepoID: "repo-1", RepoRemoteName: "origin"},
		worktrees: rows,
	}
	if _, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
		ID: record.ID, Orchestrator: "orca", Agent: "codex", Confirm: true,
	}, client, handoffPrepareTestClock()); err == nil || !strings.Contains(err.Error(), "headroom") {
		t.Fatalf("full baseline must fail before create with headroom guidance: %v", err)
	}
	if client.createCalls != 0 {
		t.Fatalf("headroom failure invoked create %d times", client.createCalls)
	}
}

func TestWorktreePreparePreCreateCollisionNeverInvokesOrca(t *testing.T) {
	tests := []struct {
		name       string
		arrange    func(*testing.T, IssueOpsRecord, *prepareOrcaFake)
		autoInline bool
		autoExists bool
	}{
		{name: "normal legacy worktree", autoInline: true, autoExists: true, arrange: func(t *testing.T, record IssueOpsRecord, _ *prepareOrcaFake) {
			makeGitWorktreeMarker(t, handoffPrepareWorktreePath(record))
		}},
		{name: "existing local branch", autoInline: true, arrange: func(t *testing.T, record IssueOpsRecord, _ *prepareOrcaFake) {
			if code, _, stderr := preflight.GitCmd(record.Repo, "branch", record.Branch, record.BranchPrepare.BaseSHA); code != 0 {
				t.Fatalf("create local branch collision: %s", stderr)
			}
		}},
		{name: "symlink leaf", arrange: func(t *testing.T, record IssueOpsRecord, _ *prepareOrcaFake) {
			if err := os.Symlink(t.TempDir(), handoffPrepareWorktreePath(record)); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "file leaf", arrange: func(t *testing.T, record IssueOpsRecord, _ *prepareOrcaFake) {
			if err := os.WriteFile(handoffPrepareWorktreePath(record), []byte("collision"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "Orca path collision", arrange: func(_ *testing.T, record IssueOpsRecord, client *prepareOrcaFake) {
			client.worktrees = []port.OrcaWorktree{{ID: "wt-existing", Path: handoffPrepareWorktreePath(record)}}
		}},
		{name: "Orca branch collision", arrange: func(_ *testing.T, record IssueOpsRecord, client *prepareOrcaFake) {
			client.worktrees = []port.OrcaWorktree{{ID: "wt-existing", Branch: "refs/heads/" + record.Branch}}
		}},
		{name: "Orca name collision", arrange: func(_ *testing.T, record IssueOpsRecord, client *prepareOrcaFake) {
			client.worktrees = []port.OrcaWorktree{{ID: "wt-existing", Name: record.Branch}}
		}},
	}
	for _, tt := range tests {
		for _, mode := range []string{IssueOpsOrchestratorAuto, IssueOpsOrchestratorOrca} {
			t.Run(tt.name+"/"+mode, func(t *testing.T) {
				stateRoot, record := handoffPrepareRecord(t)
				client := &prepareOrcaFake{probe: port.OrcaProbeResult{Available: true, Ready: true, RuntimeID: "runtime-1", RepoID: "repo-1", RepoRemoteName: "origin"}}
				tt.arrange(t, record, client)
				got, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
					ID: record.ID, Orchestrator: mode, Agent: "codex", Confirm: true,
				}, client, handoffPrepareTestClock())
				if mode == IssueOpsOrchestratorAuto && tt.autoInline {
					if err != nil || got.ResolvedMode != "" || got.FallbackCode != "" || got.RequestedMode != "" || got.Exists != tt.autoExists {
						t.Fatalf("safe legacy collision should preserve inline flow: got=%#v err=%v", got, err)
					}
				} else if err == nil {
					t.Fatalf("unsafe or explicit collision must fail closed: got=%#v", got)
				}
				persisted, readErr := ReadIssueOps(stateRoot, record.ID)
				if readErr != nil || persisted.ExecutionHandoff != nil || client.createCalls != 0 {
					t.Fatalf("collision crossed create boundary: handoff=%#v creates=%d err=%v", persisted.ExecutionHandoff, client.createCalls, readErr)
				}
			})
		}
	}
}

func TestWorktreePrepareAdoptsExactExistingOrcaWorktree(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	worktree := handoffPrepareWorktreePath(record)
	makeGitWorktreeMarker(t, worktree)
	marker := issueOpsHandoffMarker(record.ID, "epoch-1", 1)
	client := &prepareOrcaFake{
		probe: port.OrcaProbeResult{Available: true, Ready: true, RuntimeID: "runtime-1", RepoID: "repo-1", RepoRemoteName: "origin"},
		worktrees: []port.OrcaWorktree{{
			ID: "wt-existing", InstanceID: "inst-existing", RepoID: "repo-1", Path: worktree,
			Branch: "refs/heads/" + record.Branch, Head: record.BranchPrepare.BaseSHA,
			BaseRef: "refs/remotes/origin/" + record.Branch, Issue: 16, Comment: marker,
		}},
	}

	got, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
		ID: record.ID, Orchestrator: IssueOpsOrchestratorOrca, Agent: "codex", Host: "codex", SessionID: "preparation-session", SourceCWD: record.Repo, Confirm: true,
	}, client, handoffPrepareTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if got.WorkspaceState != "ready" || got.Orca == nil || got.Orca.WorktreeID != "wt-existing" || client.createCalls != 0 {
		t.Fatalf("exact existing worktree was not adopted: result=%#v creates=%d", got, client.createCalls)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExecutionWorkspace == nil || persisted.ExecutionWorkspace.Orca == nil || persisted.ExecutionWorkspace.Orca.WorktreeID != "wt-existing" || !persisted.ExecutionWorkspace.Orca.WorktreeAdopted || persisted.ExecutionHandoff != nil {
		t.Fatalf("adopted identity was not persisted: %#v", persisted.ExecutionWorkspace)
	}
}

func TestWorktreePrepareMarksAndAdoptsUnlinkedExistingOrcaWorktree(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	worktree := handoffPrepareWorktreePath(record)
	makeGitWorktreeMarker(t, worktree)
	marker := issueOpsHandoffMarker(record.ID, "epoch-1", 1)
	client := &prepareOrcaFake{
		probe: port.OrcaProbeResult{Available: true, Ready: true, RuntimeID: "runtime-1", RepoID: "repo-1", RepoRemoteName: "origin"},
		worktrees: []port.OrcaWorktree{{
			ID: "wt-existing", InstanceID: "inst-existing", RepoID: "repo-1", Path: worktree,
			Branch: "refs/heads/" + record.Branch, Head: record.BranchPrepare.BaseSHA,
		}},
		adopt: port.OrcaWorktree{
			ID: "wt-existing", InstanceID: "inst-existing", RepoID: "repo-1", Path: worktree,
			Branch: "refs/heads/" + record.Branch, Head: record.BranchPrepare.BaseSHA,
			Issue: 16, Comment: marker,
		},
	}

	got, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
		ID: record.ID, Orchestrator: IssueOpsOrchestratorOrca, Agent: "codex", Host: "codex", SessionID: "preparation-session", SourceCWD: record.Repo, Confirm: true,
	}, client, handoffPrepareTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if got.Orca == nil || got.Orca.WorktreeID != "wt-existing" || client.adoptCalls != 1 || client.createCalls != 0 {
		t.Fatalf("unlinked existing worktree was not marked and adopted: result=%#v adopts=%d creates=%d", got, client.adoptCalls, client.createCalls)
	}
	if len(client.adoptRequests) != 1 || client.adoptRequests[0].Issue != 16 || client.adoptRequests[0].Comment != marker {
		t.Fatalf("adoption request = %#v", client.adoptRequests)
	}
}

func TestWorktreePrepareRejectsAmbiguousExistingOrcaWorktreeWithoutMutation(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	worktree := handoffPrepareWorktreePath(record)
	makeGitWorktreeMarker(t, worktree)
	row := port.OrcaWorktree{
		ID: "wt-existing", InstanceID: "inst-existing", RepoID: "repo-1", Path: worktree,
		Branch: "refs/heads/" + record.Branch, Head: record.BranchPrepare.BaseSHA,
	}
	client := &prepareOrcaFake{
		probe:     port.OrcaProbeResult{Available: true, Ready: true, RuntimeID: "runtime-1", RepoID: "repo-1", RepoRemoteName: "origin"},
		worktrees: []port.OrcaWorktree{row, row},
	}
	_, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
		ID: record.ID, Orchestrator: IssueOpsOrchestratorOrca, Agent: "codex", Confirm: true,
	}, client, handoffPrepareTestClock())
	if err == nil || !strings.Contains(err.Error(), "ambiguous") || client.adoptCalls != 0 || client.createCalls != 0 {
		t.Fatalf("ambiguous existing worktree crossed mutation boundary: err=%v adopts=%d creates=%d", err, client.adoptCalls, client.createCalls)
	}
	persisted, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil || persisted.ExecutionHandoff != nil {
		t.Fatalf("ambiguous existing worktree persisted handoff: %#v err=%v", persisted.ExecutionHandoff, readErr)
	}
}

func TestWorktreePrepareRejectsRawLegacyWorktreeMissingFromOrcaInventory(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	worktree := handoffPrepareWorktreePath(record)
	makeGitWorktreeMarker(t, worktree)
	client := &prepareOrcaFake{
		probe: port.OrcaProbeResult{Available: true, Ready: true, RuntimeID: "runtime-1", RepoID: "repo-1", RepoRemoteName: "origin"},
	}
	_, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
		ID: record.ID, Orchestrator: IssueOpsOrchestratorOrca, Agent: "codex", Confirm: true,
	}, client, handoffPrepareTestClock())
	if err == nil || !strings.Contains(err.Error(), "orca_existing_legacy_worktree") {
		t.Fatalf("raw Git worktree absent from Orca inventory must fail closed: %v", err)
	}
	if client.adoptCalls != 0 || client.createCalls != 0 {
		t.Fatalf("raw Git worktree crossed Orca mutation boundary: adopts=%d creates=%d", client.adoptCalls, client.createCalls)
	}
	persisted, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil || persisted.ExecutionHandoff != nil {
		t.Fatalf("raw Git worktree persisted handoff: %#v err=%v", persisted.ExecutionHandoff, readErr)
	}
}

func TestMigrateLegacyWorktreeReplacesCleanRemoteEqualCheckoutWithOrcaManagedIdentity(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	worktree := handoffPrepareWorktreePath(record)
	makeGitWorktreeMarker(t, worktree)
	migrationMarker := issueOpsLegacyWorktreeMigrationMarker(record.ID)
	client := &prepareOrcaFake{
		probe: port.OrcaProbeResult{Available: true, Ready: true, RuntimeID: "runtime-1", RepoID: "repo-1", RepoRemoteName: "origin"},
		create: port.OrcaWorktree{
			ID: "wt-migrated", InstanceID: "inst-migrated", RepoID: "repo-1", Path: worktree,
			Branch: "refs/heads/" + record.Branch, Head: record.BranchPrepare.BaseSHA,
			BaseRef: "refs/remotes/origin/" + record.Branch, Issue: 16, Comment: migrationMarker,
		},
	}
	materializePrepareWorktreeOnCreate(t, client, worktree)
	previous := client.beforeCreate
	client.beforeCreate = func() {
		previous()
		client.worktrees = append(client.worktrees, client.create)
	}

	got, err := MigrateIssueOpsLegacyWorktree(context.Background(), stateRoot, IssueOpsLegacyWorktreeMigrationRequest{
		ID: record.ID, Confirm: true,
	}, client, handoffPrepareTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if !got.OK || got.State != IssueOpsLegacyWorktreeMigrationStateOrcaManaged || got.Orca == nil || got.Orca.WorktreeID != "wt-migrated" || client.createCalls != 1 {
		t.Fatalf("legacy migration result = %#v, creates=%d", got, client.createCalls)
	}
	persisted, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil || persisted.LegacyWorktreeMigration == nil || persisted.LegacyWorktreeMigration.State != IssueOpsLegacyWorktreeMigrationStateOrcaManaged || persisted.ExecutionHandoff != nil {
		t.Fatalf("legacy migration persistence = %#v err=%v", persisted, readErr)
	}
	if !existingLegacyWorktreeMatches(persisted, worktree) {
		t.Fatal("migrated checkout no longer matches the exact legacy path/branch/head contract")
	}
}

func TestMigrateLegacyWorktreeCanBeAdoptedByTheFollowingSupervisedHandoff(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	worktree := handoffPrepareWorktreePath(record)
	makeGitWorktreeMarker(t, worktree)
	migrationMarker := issueOpsLegacyWorktreeMigrationMarker(record.ID)
	client := &prepareOrcaFake{
		probe: port.OrcaProbeResult{Available: true, Ready: true, RuntimeID: "runtime-1", RepoID: "repo-1", RepoRemoteName: "origin"},
		create: port.OrcaWorktree{
			ID: "wt-migrated", InstanceID: "inst-migrated", RepoID: "repo-1", Path: worktree,
			Branch: "refs/heads/" + record.Branch, Head: record.BranchPrepare.BaseSHA,
			BaseRef: "refs/remotes/origin/" + record.Branch, Issue: 16, Comment: migrationMarker,
		},
	}
	materializePrepareWorktreeOnCreate(t, client, worktree)
	previous := client.beforeCreate
	client.beforeCreate = func() {
		previous()
		client.worktrees = append(client.worktrees, client.create)
	}
	if _, err := MigrateIssueOpsLegacyWorktree(context.Background(), stateRoot, IssueOpsLegacyWorktreeMigrationRequest{
		ID: record.ID, Confirm: true,
	}, client, handoffPrepareTestClock()); err != nil {
		t.Fatal(err)
	}

	client.adopt = client.create
	client.adopt.Comment = issueOpsHandoffMarker(record.ID, "epoch-1", 1)
	got, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
		ID: record.ID, Orchestrator: IssueOpsOrchestratorOrca, Agent: "codex", Host: "codex", SessionID: "source-session", AgentID: "source-agent", SourceCWD: record.Repo, Confirm: true,
	}, client, handoffPrepareTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "ready" || got.HandoffState != "" || got.Orca == nil || !got.Orca.WorktreeAdopted || client.createCalls != 1 || client.adoptCalls != 1 {
		t.Fatalf("migrated worktree was not adopted as a ready workspace: result=%#v creates=%d adopts=%d", got, client.createCalls, client.adoptCalls)
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.LegacyWorktreeMigration == nil || persisted.LegacyWorktreeMigration.State != IssueOpsLegacyWorktreeMigrationStateOrcaManaged || persisted.ExecutionWorkspace == nil || persisted.ExecutionWorkspace.Orca == nil || !persisted.ExecutionWorkspace.Orca.WorktreeAdopted || persisted.ExecutionHandoff != nil {
		t.Fatalf("migration audit or ready workspace was not persisted: %#v", persisted)
	}
}

func TestPersistedMigratedWorktreeRejectsDifferentExecutionHandoffIdentity(t *testing.T) {
	_, record := handoffPrepareRecord(t)
	worktree := handoffPrepareWorktreePath(record)
	record.LegacyWorktreeMigration = &IssueOpsLegacyWorktreeMigration{
		State: IssueOpsLegacyWorktreeMigrationStateOrcaManaged, WorktreePath: worktree, Branch: record.Branch,
		Head: record.BranchPrepare.BaseSHA, BaseRef: "refs/remotes/origin/" + record.Branch,
		PreparedAt: "2026-07-11T01:02:03Z", GitRemovedAt: "2026-07-11T01:02:04Z", CompletedAt: "2026-07-11T01:02:05Z",
		Orca: &IssueOpsOrcaIdentity{RuntimeID: "runtime-1", RepoID: "repo-1", WorktreeID: "wt-migrated", WorktreeInstanceID: "inst-migrated", WorktreePath: worktree},
	}
	record.ExecutionHandoff = &IssueOpsExecutionHandoff{Orca: &IssueOpsOrcaIdentity{RuntimeID: "runtime-1", RepoID: "repo-1", WorktreeID: "wt-other", WorktreeInstanceID: "inst-other", WorktreePath: worktree}}
	if err := validatePersistedLegacyWorktreeMigration(record); err == nil || !strings.Contains(err.Error(), "same Orca worktree") {
		t.Fatalf("mismatched migration and handoff identity must fail closed: %v", err)
	}
}

func TestMigrateLegacyWorktreeRejectsDirtyCheckoutBeforeAnyMutation(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	worktree := handoffPrepareWorktreePath(record)
	makeGitWorktreeMarker(t, worktree)
	if err := os.WriteFile(filepath.Join(worktree, "DIRTY.md"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	client := &prepareOrcaFake{probe: port.OrcaProbeResult{Available: true, Ready: true, RuntimeID: "runtime-1", RepoID: "repo-1", RepoRemoteName: "origin"}}
	_, err := MigrateIssueOpsLegacyWorktree(context.Background(), stateRoot, IssueOpsLegacyWorktreeMigrationRequest{
		ID: record.ID, Confirm: true,
	}, client, handoffPrepareTestClock())
	if err == nil || !strings.Contains(err.Error(), "must be clean") || client.createCalls != 0 {
		t.Fatalf("dirty legacy checkout crossed mutation boundary: err=%v creates=%d", err, client.createCalls)
	}
	persisted, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil || persisted.LegacyWorktreeMigration != nil {
		t.Fatalf("dirty migration changed durable state: %#v err=%v", persisted.LegacyWorktreeMigration, readErr)
	}
}

func TestMigrateLegacyWorktreeRetriesFromDurableGitRemovedState(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	worktree := handoffPrepareWorktreePath(record)
	makeGitWorktreeMarker(t, worktree)
	client := &prepareOrcaFake{
		probe:     port.OrcaProbeResult{Available: true, Ready: true, RuntimeID: "runtime-1", RepoID: "repo-1", RepoRemoteName: "origin"},
		createErr: &port.OrcaError{Code: "timeout", Invoked: true, Timeout: true},
	}
	req := IssueOpsLegacyWorktreeMigrationRequest{ID: record.ID, Confirm: true}
	if _, err := MigrateIssueOpsLegacyWorktree(context.Background(), stateRoot, req, client, handoffPrepareTestClock()); err == nil {
		t.Fatal("expected ambiguous Orca create failure")
	}
	pending, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil || pending.LegacyWorktreeMigration == nil || pending.LegacyWorktreeMigration.State != IssueOpsLegacyWorktreeMigrationStateGitRemoved {
		t.Fatalf("git removal was not durably resumable: %#v err=%v", pending.LegacyWorktreeMigration, err)
	}
	if _, err := os.Stat(worktree); !os.IsNotExist(err) {
		t.Fatalf("raw worktree was not removed before retry: %v", err)
	}
	migrationMarker := issueOpsLegacyWorktreeMigrationMarker(record.ID)
	client.createErr = nil
	client.create = port.OrcaWorktree{
		ID: "wt-retried", InstanceID: "inst-retried", RepoID: "repo-1", Path: worktree,
		Branch: "refs/heads/" + record.Branch, Head: record.BranchPrepare.BaseSHA,
		BaseRef: "refs/remotes/origin/" + record.Branch, Issue: 16, Comment: migrationMarker,
	}
	client.beforeCreate = func() {
		makeGitWorktreeMarker(t, worktree)
		client.worktrees = append(client.worktrees, client.create)
	}
	got, err := MigrateIssueOpsLegacyWorktree(context.Background(), stateRoot, req, client, handoffPrepareTestClock())
	if err != nil {
		t.Fatal(err)
	}
	if got.State != IssueOpsLegacyWorktreeMigrationStateOrcaManaged || client.createCalls != 2 {
		t.Fatalf("migration retry = %#v creates=%d", got, client.createCalls)
	}
}

func TestWorktreePrepareRejectsSymlinkedCanonicalWorktreeBaseBeforeCreate(t *testing.T) {
	for _, mode := range []string{IssueOpsOrchestratorAuto, IssueOpsOrchestratorOrca} {
		t.Run(mode, func(t *testing.T) {
			stateRoot, record := handoffPrepareRecord(t)
			base := filepath.Dir(handoffPrepareWorktreePath(record))
			if err := os.Remove(base); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(t.TempDir(), base); err != nil {
				t.Fatal(err)
			}
			client := &prepareOrcaFake{probe: port.OrcaProbeResult{Available: true, Ready: true, RuntimeID: "runtime-1", RepoID: "repo-1", RepoRemoteName: "origin"}}
			got, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
				ID: record.ID, Orchestrator: mode, Agent: "codex", Confirm: true,
			}, client, handoffPrepareTestClock())
			if err == nil || got.ResolvedMode == IssueOpsOrchestratorInline {
				t.Fatalf("symlinked worktree base must fail closed, not fallback: got=%#v err=%v", got, err)
			}
			if client.createCalls != 0 {
				t.Fatalf("symlinked base invoked create %d times", client.createCalls)
			}
		})
	}
}

func TestWorktreePrepareMissingCanonicalBaseFallsBackOnlyInAuto(t *testing.T) {
	for _, mode := range []string{IssueOpsOrchestratorAuto, IssueOpsOrchestratorOrca} {
		t.Run(mode, func(t *testing.T) {
			stateRoot, record := handoffPrepareRecord(t)
			base := filepath.Dir(handoffPrepareWorktreePath(record))
			if err := os.Remove(base); err != nil {
				t.Fatal(err)
			}
			client := &prepareOrcaFake{probe: port.OrcaProbeResult{Available: true, Ready: true, RuntimeID: "runtime-1", RepoID: "repo-1", RepoRemoteName: "origin"}}
			got, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
				ID: record.ID, Orchestrator: mode, Agent: "codex", Confirm: true,
			}, client, handoffPrepareTestClock())
			if mode == IssueOpsOrchestratorAuto {
				if err != nil || got.ResolvedMode != "" || got.FallbackCode != "" || got.RequestedMode != "" {
					t.Fatalf("missing base auto fallback = %#v err=%v", got, err)
				}
			} else if err == nil {
				t.Fatal("explicit Orca missing base must error")
			}
			persisted, readErr := ReadIssueOps(stateRoot, record.ID)
			if readErr != nil || persisted.ExecutionHandoff != nil || client.createCalls != 0 {
				t.Fatalf("missing base crossed create boundary: handoff=%#v creates=%d err=%v", persisted.ExecutionHandoff, client.createCalls, readErr)
			}
		})
	}
}

func TestWorktreePrepareRejectsReturnedBranchPathOrInstanceMismatch(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*port.OrcaWorktree)
	}{
		{name: "branch", mutate: func(w *port.OrcaWorktree) { w.Branch = "refs/heads/wrong" }},
		{name: "path", mutate: func(w *port.OrcaWorktree) { w.Path += "-wrong" }},
		{name: "instance", mutate: func(w *port.OrcaWorktree) { w.InstanceID = "" }},
		{name: "lineage", mutate: func(w *port.OrcaWorktree) { w.Head = "wrong" }},
		{name: "issue", mutate: func(w *port.OrcaWorktree) { w.Issue = 17 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffPrepareRecord(t)
			worktree := handoffPrepareWorktreePath(record)
			created := port.OrcaWorktree{ID: "wt-1", InstanceID: "inst-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/16-demo", Path: worktree, Branch: "refs/heads/" + record.Branch, Head: record.BranchPrepare.BaseSHA, Issue: 16}
			tt.mutate(&created)
			client := &prepareOrcaFake{probe: port.OrcaProbeResult{Available: true, Ready: true, RepoID: "repo-1", RepoRemoteName: "origin"}, create: created}
			materializePrepareWorktreeOnCreate(t, client, worktree)

			_, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
				ID: record.ID, Orchestrator: "orca", Agent: "codex", Host: "codex", SessionID: "preparation-session", SourceCWD: record.Repo, Confirm: true,
			}, client, handoffPrepareTestClock())
			if err == nil {
				t.Fatal("expected validation error")
			}
			persisted, _ := ReadIssueOps(stateRoot, record.ID)
			if persisted.ExecutionWorkspace == nil || persisted.ExecutionWorkspace.State != "recovery_required" || persisted.ExecutionHandoff != nil {
				t.Fatalf("expected workspace recovery_required, got %#v", persisted)
			}
		})
	}
}

func TestWorktreePreparePathMismatchExplainsFlatLayoutRecovery(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	expected := handoffPrepareWorktreePath(record)
	nested := filepath.Join(filepath.Dir(expected), filepath.Base(record.Repo), filepath.Base(expected))
	client := &prepareOrcaFake{
		probe: port.OrcaProbeResult{Available: true, Ready: true, RepoID: "repo-1", RepoRemoteName: "origin"},
		create: port.OrcaWorktree{
			ID: "wt-nested", InstanceID: "inst-nested", RepoID: "repo-1", BaseRef: "refs/remotes/origin/16-demo", Path: nested,
			Branch: "refs/heads/" + record.Branch, Head: record.BranchPrepare.BaseSHA, Issue: 16,
			Comment: issueOpsHandoffMarker(record.ID, "epoch-1", 1),
		},
	}

	_, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
		ID: record.ID, Orchestrator: "orca", Agent: "codex", Host: "codex", SessionID: "preparation-session", SourceCWD: record.Repo, Confirm: true,
	}, client, handoffPrepareTestClock())
	if err == nil {
		t.Fatal("expected canonical path validation error")
	}
	for _, want := range []string{"Nest Workspaces", "OFF", "provider tracking ref", "cancel", "fresh IssueOps cycle"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("path mismatch diagnostic does not contain %q: %v", want, err)
		}
	}
	persisted, readErr := ReadIssueOps(stateRoot, record.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if persisted.ExecutionWorkspace == nil || persisted.ExecutionWorkspace.State != "recovery_required" || persisted.ExecutionWorkspace.Failure == nil || persisted.ExecutionHandoff != nil {
		t.Fatalf("path mismatch did not preserve workspace recovery evidence: %#v", persisted)
	}
	if !strings.Contains(persisted.ExecutionWorkspace.Failure.Message, "Nest Workspaces") {
		t.Fatalf("persisted workspace failure is not actionable: %#v", persisted.ExecutionWorkspace.Failure)
	}
	if persisted.ExecutionWorkspace.PendingOperation == nil || persisted.ExecutionWorkspace.Orca == nil || persisted.ExecutionWorkspace.Orca.WorktreeID != "" || persisted.WorktreePath == nested {
		t.Fatalf("known-invalid worktree lost its recovery journal or was adopted: record=%#v", persisted)
	}
}

func TestWorktreePrepareDoesNotGrantCleanupAuthorityToUnprovenCreateIdentity(t *testing.T) {
	tests := []struct {
		name     string
		baseline bool
		mutate   func(*port.OrcaWorktree)
	}{
		{name: "baseline id reuse", baseline: true},
		{name: "substring marker", mutate: func(w *port.OrcaWorktree) { w.Comment = "prefix " + w.Comment + " suffix" }},
		{name: "empty instance", mutate: func(w *port.OrcaWorktree) { w.InstanceID = "" }},
		{name: "empty repo", mutate: func(w *port.OrcaWorktree) { w.RepoID = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := handoffPrepareRecord(t)
			created := port.OrcaWorktree{
				ID: "wt-returned", InstanceID: "instance-returned", RepoID: "repo-1", BaseRef: "refs/remotes/origin/16-demo",
				Path: handoffPrepareWorktreePath(record) + "-mismatch", Branch: "refs/heads/" + record.Branch,
				Head: record.BranchPrepare.BaseSHA, Issue: 16, Comment: issueOpsHandoffMarker(record.ID, "epoch-1", 1),
			}
			if tt.mutate != nil {
				tt.mutate(&created)
			}
			client := &prepareOrcaFake{
				probe:  port.OrcaProbeResult{Available: true, Ready: true, RuntimeID: "runtime-1", RepoID: "repo-1", RepoRemoteName: "origin"},
				create: created,
			}
			if tt.baseline {
				client.worktrees = []port.OrcaWorktree{{ID: created.ID}}
			}
			if _, err := PrepareIssueOpsHandoffWorktree(context.Background(), stateRoot, IssueOpsHandoffPrepareRequest{
				ID: record.ID, Orchestrator: "orca", Agent: "codex", Host: "codex", SessionID: "preparation-session", SourceCWD: record.Repo, Confirm: true,
			}, client, handoffPrepareTestClock()); err == nil {
				t.Fatal("unproven create identity must require recovery")
			}
			persisted, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if persisted.ExecutionWorkspace == nil || persisted.ExecutionWorkspace.State != handoff.StateRecoveryRequired || persisted.ExecutionWorkspace.PendingOperation == nil || persisted.ExecutionHandoff != nil {
				t.Fatalf("unproven response became workspace authority: %#v", persisted)
			}
		})
	}
}

func TestValidateCreatedWorktreeRejectsResolvedButLexicallyDifferentOrSymlinkPath(t *testing.T) {
	t.Run("same resolved target different lexical path", func(t *testing.T) {
		_, record := handoffPrepareRecord(t)
		expected := handoffPrepareWorktreePath(record)
		makeGitWorktreeMarker(t, expected)
		alias := expected + "-alias"
		if err := os.Symlink(expected, alias); err != nil {
			t.Fatal(err)
		}
		created := port.OrcaWorktree{
			ID: "wt-1", InstanceID: "inst-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/16-demo",
			Path: alias, Branch: "refs/heads/" + record.Branch, Head: record.BranchPrepare.BaseSHA, Issue: 16,
		}
		if err := validateCreatedHandoffWorktree(record, expected, "repo-1", "refs/remotes/origin/16-demo", created); err == nil {
			t.Fatal("resolved equality must not replace exact lexical path identity")
		}
	})
	t.Run("expected symlink leaf", func(t *testing.T) {
		_, record := handoffPrepareRecord(t)
		expected := handoffPrepareWorktreePath(record)
		actual := expected + "-actual"
		if code, _, stderr := preflight.GitCmd(record.Repo, "worktree", "add", "-q", "-b", record.Branch, actual, "refs/remotes/origin/"+record.Branch); code != 0 {
			t.Fatalf("materialize alternate worktree: %s", stderr)
		}
		if err := os.MkdirAll(filepath.Dir(expected), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(actual, expected); err != nil {
			t.Fatal(err)
		}
		created := port.OrcaWorktree{
			ID: "wt-1", InstanceID: "inst-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/16-demo",
			Path: expected, Branch: "refs/heads/" + record.Branch, Head: record.BranchPrepare.BaseSHA, Issue: 16,
		}
		if err := validateCreatedHandoffWorktree(record, expected, "repo-1", "refs/remotes/origin/16-demo", created); err == nil {
			t.Fatal("canonical expected worktree path must be a real directory, not a symlink leaf")
		}
	})
}

func TestWorktreePrepareExactOneMarkerRecovery(t *testing.T) {
	pending := IssueOpsExecutionHandoffPendingOperation{Kind: "worktree_create", BaselineWorktreeIDs: []string{"before"}}
	tests := []struct {
		name string
		rows []port.OrcaWorktree
		ok   bool
	}{
		{name: "zero", rows: []port.OrcaWorktree{{ID: "before"}}},
		{name: "one", rows: []port.OrcaWorktree{{ID: "before"}, {ID: "new", InstanceID: "inst", Comment: "agent-harness issueops=io-demo ownership=epoch-1 attempt=1"}}, ok: true},
		{name: "substring sibling", rows: []port.OrcaWorktree{{ID: "new", Comment: "prefix agent-harness issueops=io-demo ownership=epoch-1 attempt=1 suffix"}}},
		{name: "multiple", rows: []port.OrcaWorktree{{ID: "new-1", Comment: "agent-harness issueops=io-demo ownership=epoch-1 attempt=1"}, {ID: "new-2", Comment: "agent-harness issueops=io-demo ownership=epoch-1 attempt=1"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReconcileIssueOpsHandoffWorktree(pending, "io-demo", "epoch-1", 1, tt.rows)
			if tt.ok && (err != nil || got.ID != "new") {
				t.Fatalf("expected unique candidate, got %#v err=%v", got, err)
			}
			if !tt.ok && err == nil {
				t.Fatalf("expected fail closed, got %#v", got)
			}
		})
	}
}

type prepareOrcaFake struct {
	probe          port.OrcaProbeResult
	probeErr       error
	worktrees      []port.OrcaWorktree
	worktreeErr    error
	create         port.OrcaWorktree
	createErr      error
	createCalls    int
	createRequests []port.OrcaCreateWorktreeRequest
	adopt          port.OrcaWorktree
	adoptErr       error
	adoptCalls     int
	adoptRequests  []port.OrcaAdoptWorktreeRequest
	probeRequests  []port.OrcaProbeRequest
	beforeCreate   func()
	trace          []string
}

func (f *prepareOrcaFake) Probe(_ context.Context, req port.OrcaProbeRequest) (port.OrcaProbeResult, error) {
	f.trace = append(f.trace, "probe")
	f.probeRequests = append(f.probeRequests, req)
	return f.probe, f.probeErr
}

func (f *prepareOrcaFake) ListWorktrees(context.Context, string) ([]port.OrcaWorktree, error) {
	f.trace = append(f.trace, "worktree-list")
	return append([]port.OrcaWorktree(nil), f.worktrees...), f.worktreeErr
}

func (f *prepareOrcaFake) CreateWorktree(_ context.Context, req port.OrcaCreateWorktreeRequest) (port.OrcaWorktree, error) {
	f.trace = append(f.trace, "worktree-create")
	f.createCalls++
	f.createRequests = append(f.createRequests, req)
	if f.beforeCreate != nil {
		f.beforeCreate()
	}
	return f.create, f.createErr
}

func (f *prepareOrcaFake) AdoptWorktree(_ context.Context, req port.OrcaAdoptWorktreeRequest) (port.OrcaWorktree, error) {
	f.trace = append(f.trace, "worktree-adopt")
	f.adoptCalls++
	f.adoptRequests = append(f.adoptRequests, req)
	return f.adopt, f.adoptErr
}

func materializePrepareWorktreeOnCreate(t *testing.T, client *prepareOrcaFake, worktree string) {
	t.Helper()
	previous := client.beforeCreate
	client.beforeCreate = func() {
		if previous != nil {
			previous()
		}
		makeGitWorktreeMarker(t, worktree)
	}
}

func handoffPrepareRecord(t *testing.T) (string, IssueOpsRecord) {
	t.Helper()
	stateRoot := t.TempDir()
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(repo+".worktrees", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("handoff fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.name", "Handoff Test"},
		{"config", "user.email", "handoff@example.test"},
		{"add", "README.md"},
		{"commit", "-q", "-m", "test: base"},
	} {
		if code, _, stderr := preflight.GitCmd(repo, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	baseSHA := strings.TrimSpace(preflight.GitOut(repo, "rev-parse", "HEAD"))
	for _, ref := range []string{"refs/remotes/origin/16-demo", "refs/remotes/upstream/16-demo"} {
		if code, _, stderr := preflight.GitCmd(repo, "update-ref", ref, baseSHA); code != 0 {
			t.Fatalf("update %s: %s", ref, stderr)
		}
	}
	record := IssueOpsRecord{
		SchemaVersion: IssueOpsCurrentSchemaVersion,
		ID:            NewIssueOpsID(repo, "16-demo"),
		Repo:          repo,
		Branch:        "16-demo",
		Phase:         IssueOpsPhasePlan,
		IssueURL:      "https://github.com/acme/repo/issues/16",
		DesignReview:  &IssueOpsDesignReview{Approved: true, ReviewedAt: "2026-07-11T00:00:00Z"},
		BranchPrepare: &IssueOpsBranchPrepare{Provider: "github", IssueURL: "https://github.com/acme/repo/issues/16", Branch: "16-demo", BaseBranch: "main", BaseSHA: baseSHA, LinkVerified: true, CreatedAt: "2026-07-11T00:00:00Z"},
		CreatedAt:     "2026-07-11T00:00:00Z",
		UpdatedAt:     "2026-07-11T00:00:00Z",
	}
	got, err := WriteIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, got
}

func gitLabHandoffPrepareRecord(t *testing.T) (string, IssueOpsRecord) {
	t.Helper()
	stateRoot, record := handoffPrepareRecord(t)
	record.IssueURL = "https://gitlab.example/acme/repo/-/issues/16"
	record.BranchPrepare.Provider = "gitlab"
	record.BranchPrepare.IssueURL = record.IssueURL
	updated, err := WriteIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, updated
}

func handoffPrepareWorktreePath(record IssueOpsRecord) string {
	return record.Repo + ".worktrees/" + strings.ReplaceAll(record.Branch, "/", "-")
}

func makeGitWorktreeMarker(t *testing.T, path string) {
	t.Helper()
	marker := ".worktrees" + string(filepath.Separator)
	at := strings.Index(path, marker)
	if at < 0 {
		t.Fatalf("cannot derive source repo from worktree path %q", path)
	}
	repo := path[:at]
	branch := filepath.Base(path)
	if code, _, stderr := preflight.GitCmd(repo, "worktree", "add", "-q", "-b", branch, path, "refs/remotes/origin/"+branch); code != 0 {
		t.Fatalf("materialize Git worktree: %s", stderr)
	}
}

func handoffPrepareTestClock() IssueOpsHandoffPrepareClock {
	return IssueOpsHandoffPrepareClock{
		Now:      func() time.Time { return time.Date(2026, 7, 11, 1, 2, 3, 0, time.UTC) },
		NewEpoch: func() (string, error) { return "epoch-1", nil },
	}
}
