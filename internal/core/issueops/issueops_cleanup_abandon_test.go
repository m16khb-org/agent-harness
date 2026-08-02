package issueops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/port"
)

const abandonIssueBodySHA = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

type fakeAbandonGit struct{ branchOID string }

func (g *fakeAbandonGit) run(_ string, args ...string) (int, string) {
	if len(args) > 0 && args[0] == "rev-parse" {
		if g.branchOID == "" {
			return 1, ""
		}
		return 0, g.branchOID
	}
	return 0, ""
}

// fakeAbandonOrca는 InspectIntent만 의미 있게 응답한다. InvokeIntent는 항상
// 실패를 반환해, abandon이 어떤 경로에서도 orca mutation을 부르지 않음을
// 구조적으로 고정한다(원격/외부 무접촉 계약).
type fakeAbandonOrca struct {
	inventory port.ExecutionOrcaIntentInventory
	err       error
	inspects  int
	stages    []port.ExecutionOrcaIntentStage
}

func (o *fakeAbandonOrca) Probe(context.Context, port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaProbeResult, error) {
	return port.ExecutionOrcaProbeResult{Available: true, Ready: true}, nil
}

func (o *fakeAbandonOrca) InspectIntent(_ context.Context, req port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
	o.inspects++
	o.stages = append(o.stages, req.Stage)
	return o.inventory, o.err
}

func (o *fakeAbandonOrca) InvokeIntent(context.Context, port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
	return port.ExecutionOrcaIntentReceipt{}, fmt.Errorf("cleanup abandon must never invoke an Orca mutation")
}

func abandonTestRecord(t *testing.T) (string, IssueOpsRecord) {
	t.Helper()
	stateRoot := filepath.Join(t.TempDir(), "issueops")
	repo := t.TempDir()
	record, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: repo, Branch: "106-abandon"})
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, record
}

func abandonRequest(id string, apply bool, fingerprint string) CleanupAbandonRequest {
	return CleanupAbandonRequest{
		ID: id, Reason: "폐기된 비-done 사이클 정리",
		Apply: apply, Confirm: apply, Fingerprint: fingerprint,
	}
}

func abandonDeps(git *fakeAbandonGit, orca port.ExecutionOrcaProvisioner) CleanupAbandonDeps {
	return CleanupAbandonDeps{Git: git.run, Orca: orca}
}

func authoritativeZeroOrca() *fakeAbandonOrca {
	return &fakeAbandonOrca{inventory: port.ExecutionOrcaIntentInventory{AuthoritativeZero: true}}
}

func writeAbandonIntentRow(t *testing.T, stateRoot, operationID string, payload externalOrcaIntentPayload) {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(externalIntentBucket, operationID, data); err != nil {
		t.Fatal(err)
	}
}

func abandonIntentRowExists(t *testing.T, stateRoot, operationID string) bool {
	t.Helper()
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	_, ok, err := db.Get(externalIntentBucket, operationID)
	if err != nil {
		t.Fatal(err)
	}
	return ok
}

// abandonExecution은 지정한 lease로 released-가능한 orca 실행 블록을 만든다.
// root는 부모만 존재하고 자기 자신은 없는 경로다(worktree_absent 충족).
func abandonExecution(repo, root string, lease WriteLease) *Execution {
	return &Execution{
		Mode: model.ExecutionModeOrca,
		Workspace: Workspace{
			SourceRoot: repo, Root: root, Branch: "106-abandon",
			BaseHead: "deadbeef", Driver: "orca", LinkedAt: "2026-07-24T00:00:00Z",
		},
		Lease: lease,
	}
}

// abandonOrcaPendingRecord는 실측 io-ff5300b4aa0b 형태를 재현한다: released
// lease + worktree_create pending + external_operation_ambiguous failure +
// 디스크에 없는 workspace root.
func abandonOrcaPendingRecord(t *testing.T, kind string, writeRow bool) (string, IssueOpsRecord, string, string) {
	t.Helper()
	stateRoot, record := abandonTestRecord(t)
	root := filepath.Join(t.TempDir(), "absent-worktree")
	operationID := "op-abandon-worktree"
	issueURL := "https://github.com/m16khb/agent-harness/issues/106"
	record.IssueURL = issueURL
	record.BranchPrepare = &IssueOpsBranchPrepare{
		Provider: "github", IssueURL: issueURL, Branch: "106-abandon",
		BaseBranch: "main", BaseSHA: "deadbeef", LinkVerified: true,
	}
	marker, err := renderOrcaIntentMarker(orcaIntentMarkerIdentity{
		Purpose: orcaIntentPurposePrepare, LifecycleID: record.ID,
		Generation: 1, OperationID: operationID, Provider: "github", Issue: 106,
	})
	if err != nil {
		t.Fatal(err)
	}
	if writeRow {
		writeAbandonIntentRow(t, stateRoot, operationID, externalOrcaIntentPayload{
			SchemaVersion: model.IssueOpsSchemaVersion, OperationID: operationID, LifecycleID: record.ID,
			Generation: 1, Stage: intentContractStage(port.ExecutionOrcaIntentWorktree), Marker: marker,
			StartedAt: "2026-07-24T00:00:00Z", InvocationState: orcaIntentNotInvoked,
			Workspace: intentContractWorkspaceRequest(port.ExecutionWorkspaceRequest{
				LifecycleID: record.ID, SourceRoot: record.Repo, Root: root,
				Branch: "106-abandon", BaseBranch: "main", BaseHead: "deadbeef",
			}),
			Probe: intentContractProbeRequest(port.ExecutionOrcaProbeRequest{
				Repo: record.Repo, Host: "codex", Model: "gpt-5.4", Marker: marker,
				Provider: "github", Issue: 106,
			}),
			IssueBodySHA256: abandonIssueBodySHA,
		})
	}
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *IssueOpsRecord) {
		rec.IssueURL = record.IssueURL
		rec.BranchPrepare = record.BranchPrepare
		rec.Execution = abandonExecution(rec.Repo, root, WriteLease{Generation: 1, Status: model.LeaseStatusReleased})
		rec.Execution.Pending = &model.ExternalIntent{
			OperationID: operationID, Kind: kind, Marker: marker, StartedAt: "2026-07-24T00:00:00Z",
		}
		rec.Execution.Failure = &model.ExecutionFailure{
			OperationID: operationID, Code: "external_operation_ambiguous", At: "2026-07-24T00:00:00Z",
		}
	})
	updated, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, updated, operationID, root
}

// AC-02 성공 경로: preview가 삭제 대상 레코드 전문과 fingerprint를 내고,
// apply가 레코드를 삭제하며, 동일 (repo, branch) start가 새 사이클을 연다.
func TestCleanupAbandonPreviewThenApplyDeletesRecord(t *testing.T) {
	stateRoot, record := abandonTestRecord(t)
	deps := abandonDeps(&fakeAbandonGit{}, authoritativeZeroOrca())

	preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Preview || preview.Fingerprint == "" || preview.RecordDeleted {
		t.Fatalf("preview must issue a fingerprint without mutating: %+v", preview)
	}
	// brooks F7 완화책: 삭제 전 캡처가 가능해야 한다.
	if preview.Record == nil || preview.Record.ID != record.ID || preview.Record.Repo != record.Repo {
		t.Fatalf("preview must carry the full record about to be deleted: %+v", preview.Record)
	}
	if preview.Reason != "폐기된 비-done 사이클 정리" {
		t.Fatalf("preview must echo the reason: %q", preview.Reason)
	}

	applied, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, true, preview.Fingerprint), deps)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.RecordDeleted || applied.AbandonedAt == "" || applied.Record == nil {
		t.Fatalf("apply must delete the record and echo the snapshot and time: %+v", applied)
	}
	if _, err := ReadIssueOps(stateRoot, record.ID); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("record must be gone after abandon: %v", err)
	}
	fresh, err := StartIssueOps(stateRoot, IssueOpsStartRequest{Repo: record.Repo, Branch: "106-abandon"})
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Phase != IssueOpsPhaseProblem || fresh.Execution != nil {
		t.Fatalf("abandon must unlock same-branch rework with a fresh cycle: %+v", fresh)
	}
}

// AC-02: LeaseStatus 4값 전수 테이블. released만 통과하고 revoking을 포함한
// 나머지는 전부 lease_terminal로 거부된다(brooks F5).
func TestCleanupAbandonLeaseAllowlistCoversEveryStatus(t *testing.T) {
	holder := &NativeActor{
		Host: "codex", SessionID: "s-1",
		SessionProcess: &NativeProcessReceipt{PID: 4321, StartedAt: "2026-07-24T00:00:00Z", Executable: "/usr/bin/codex"},
	}
	cases := []struct {
		name  string
		lease WriteLease
		ready bool
	}{
		{"released", WriteLease{Generation: 1, Status: model.LeaseStatusReleased}, true},
		// claimable은 #140에서 통과로 바뀌었다. validateWriteLease가 홀더 부재를
		// 강제하므로 released와 같은 성질이고, 거부하면 운영자가 claim→release로
		// lease를 한 바퀴 돌리는 우회를 하게 된다(#139에서 실측). 자원 잔여는
		// 게이트 ⑤·⑥·⑦·⑨가 각각 막는다.
		{"claimable", WriteLease{Generation: 1, Status: model.LeaseStatusClaimable, ClaimTokenSHA256: abandonIssueBodySHA}, true},
		{"active", WriteLease{Generation: 1, Status: model.LeaseStatusActive, Holder: holder, ClaimedAt: "2026-07-24T00:00:00Z"}, false},
		{"revoking", WriteLease{Generation: 1, Status: model.LeaseStatusRevoking, Holder: holder}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stateRoot, record := abandonTestRecord(t)
			root := filepath.Join(t.TempDir(), "absent-worktree")
			mutateFinishRecord(t, stateRoot, record.ID, func(rec *IssueOpsRecord) {
				rec.Execution = abandonExecution(rec.Repo, root, tc.lease)
			})
			deps := abandonDeps(&fakeAbandonGit{}, authoritativeZeroOrca())
			result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), deps)
			if tc.ready {
				if err != nil || result.Fingerprint == "" {
					t.Fatalf("released lease must be ready: %v %+v", err, result)
				}
				return
			}
			if err == nil || !containsString(result.Missing, "lease_terminal") {
				t.Fatalf("%s lease must block abandon: %v %v", tc.name, err, result.Missing)
			}
		})
	}
}

// AC-02: 나머지 레코드 게이트(phase/artifact/children/reason).
func TestCleanupAbandonRecordGatesRejectUnsafeRecords(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*IssueOpsRecord)
		request func(id string) CleanupAbandonRequest
		missing string
	}{
		{
			name:    "done phase belongs to finish",
			mutate:  func(rec *IssueOpsRecord) { rec.Phase = IssueOpsPhaseDone },
			missing: "phase_not_done",
		},
		{
			name: "remote artifact belongs to reflect then finish",
			mutate: func(rec *IssueOpsRecord) {
				rec.RemoteArtifact = &IssueOpsRemoteArtifactVerification{
					Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/9",
				}
			},
			missing: "no_remote_artifact",
		},
		{
			name: "child cycles would be orphaned",
			mutate: func(rec *IssueOpsRecord) {
				rec.ChildCycles = append(rec.ChildCycles, IssueOpsChildCycleRef{CycleID: "io-child000000", Branch: "child"})
			},
			missing: "no_children",
		},
		{
			name: "unclosed child issue link",
			mutate: func(rec *IssueOpsRecord) {
				rec.IssueLinks = append(rec.IssueLinks, IssueOpsIssueLink{Type: "child", URL: "https://github.com/acme/repo/issues/91"})
			},
			missing: "no_children",
		},
		{
			name:    "blank reason",
			request: func(id string) CleanupAbandonRequest { r := abandonRequest(id, false, ""); r.Reason = "   "; return r },
			missing: "reason_required",
		},
		{
			name: "control character in reason",
			request: func(id string) CleanupAbandonRequest {
				r := abandonRequest(id, false, "")
				r.Reason = "abandon\nnow"
				return r
			},
			missing: "reason_required",
		},
		{
			name: "active shell character in reason",
			request: func(id string) CleanupAbandonRequest {
				r := abandonRequest(id, false, "")
				r.Reason = "abandon $(rm -rf /)"
				return r
			},
			missing: "reason_required",
		},
		{
			name: "reason over the byte limit",
			request: func(id string) CleanupAbandonRequest {
				r := abandonRequest(id, false, "")
				r.Reason = strings.Repeat("a", cleanupAbandonReasonLimit+1)
				return r
			},
			missing: "reason_required",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stateRoot, record := abandonTestRecord(t)
			if tc.mutate != nil {
				mutateFinishRecord(t, stateRoot, record.ID, tc.mutate)
			}
			req := abandonRequest(record.ID, false, "")
			if tc.request != nil {
				req = tc.request(record.ID)
			}
			deps := abandonDeps(&fakeAbandonGit{}, authoritativeZeroOrca())
			result, err := CleanupAbandon(context.Background(), stateRoot, req, deps)
			if err == nil || !containsString(result.Missing, tc.missing) {
				t.Fatalf("expected missing %q: err=%v missing=%v", tc.missing, err, result.Missing)
			}
		})
	}
}

// AC-02: 로컬 잔여물(워크트리 디렉토리, 브랜치 ref, 경로 불일치)은 abandon을
// 막는다 — abandon은 아무것도 지우지 않는 경로이기 때문이다.
func TestCleanupAbandonRejectsLocalResidue(t *testing.T) {
	t.Run("worktree present", func(t *testing.T) {
		stateRoot, record := abandonTestRecord(t)
		root := filepath.Join(t.TempDir(), "live-worktree")
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		mutateFinishRecord(t, stateRoot, record.ID, func(rec *IssueOpsRecord) {
			rec.Execution = abandonExecution(rec.Repo, root, WriteLease{Generation: 1, Status: model.LeaseStatusReleased})
		})
		result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), abandonDeps(&fakeAbandonGit{}, authoritativeZeroOrca()))
		if err == nil || !containsString(result.Missing, "worktree_absent") {
			t.Fatalf("present worktree must block: %v %v", err, result.Missing)
		}
	})
	t.Run("branch ref present", func(t *testing.T) {
		stateRoot, record := abandonTestRecord(t)
		result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), abandonDeps(&fakeAbandonGit{branchOID: "abc123"}, authoritativeZeroOrca()))
		if err == nil || !containsString(result.Missing, "branch_absent") {
			t.Fatalf("present branch must block: %v %v", err, result.Missing)
		}
	})
	t.Run("worktree identity conflict", func(t *testing.T) {
		stateRoot, record := abandonTestRecord(t)
		root := filepath.Join(t.TempDir(), "absent-worktree")
		mutateFinishRecord(t, stateRoot, record.ID, func(rec *IssueOpsRecord) {
			rec.Execution = abandonExecution(rec.Repo, root, WriteLease{Generation: 1, Status: model.LeaseStatusReleased})
			rec.WorktreePath = filepath.Join(t.TempDir(), "other-worktree")
		})
		result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), abandonDeps(&fakeAbandonGit{}, authoritativeZeroOrca()))
		if err == nil || !containsString(result.Missing, "worktree_identity_conflict") {
			t.Fatalf("conflicting worktree identity must block: %v %v", err, result.Missing)
		}
	})
}

// AC-02: pending kind별 허용/거부 + InspectIntent 분기 전수.
func TestCleanupAbandonPendingIntentGate(t *testing.T) {
	t.Run("remote kind is never abandonable", func(t *testing.T) {
		stateRoot, record, _, _ := abandonOrcaPendingRecord(t, externalIntentRemotePR, true)
		orca := authoritativeZeroOrca()
		result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), abandonDeps(&fakeAbandonGit{}, orca))
		if err == nil || !containsString(result.Missing, "pending_intent_safe") {
			t.Fatalf("remote pending kind must block: %v %v", err, result.Missing)
		}
		if orca.inspects != 0 {
			t.Fatalf("remote kind must be rejected before any Orca call: %d", orca.inspects)
		}
		if !strings.Contains(result.PendingIntentError, "execution reconcile") {
			t.Fatalf("rejection must point at execution reconcile: %q", result.PendingIntentError)
		}
	})
	t.Run("missing intent row is ambiguous", func(t *testing.T) {
		stateRoot, record, _, _ := abandonOrcaPendingRecord(t, "worktree_create", false)
		result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), abandonDeps(&fakeAbandonGit{}, authoritativeZeroOrca()))
		if err == nil || !containsString(result.Missing, "pending_intent_safe") {
			t.Fatalf("absent intent row must block the gate: %v %v", err, result.Missing)
		}
	})
	t.Run("orca inspector absent", func(t *testing.T) {
		stateRoot, record, _, _ := abandonOrcaPendingRecord(t, "worktree_create", true)
		result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), CleanupAbandonDeps{Git: (&fakeAbandonGit{}).run})
		if err == nil || !containsString(result.Missing, "pending_intent_safe") {
			t.Fatalf("missing Orca inspector must block: %v %v", err, result.Missing)
		}
	})
	t.Run("inspect transport failure", func(t *testing.T) {
		stateRoot, record, _, _ := abandonOrcaPendingRecord(t, "worktree_create", true)
		orca := &fakeAbandonOrca{err: fmt.Errorf("orca runtime unreachable")}
		result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), abandonDeps(&fakeAbandonGit{}, orca))
		if err == nil || !containsString(result.Missing, "pending_intent_safe") {
			t.Fatalf("inspect failure must block: %v %v", err, result.Missing)
		}
	})
	t.Run("candidate found", func(t *testing.T) {
		stateRoot, record, _, _ := abandonOrcaPendingRecord(t, "worktree_create", true)
		orca := &fakeAbandonOrca{inventory: port.ExecutionOrcaIntentInventory{
			Candidates:        []port.ExecutionOrcaIntentReceipt{{Workspace: &port.ExecutionOrcaWorkspaceReceipt{WorktreeID: "wt-1"}}},
			AuthoritativeZero: false,
		}}
		result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), abandonDeps(&fakeAbandonGit{}, orca))
		if err == nil || !containsString(result.Missing, "pending_intent_safe") {
			t.Fatalf("orca candidate must block: %v %v", err, result.Missing)
		}
	})
	t.Run("non authoritative zero", func(t *testing.T) {
		stateRoot, record, _, _ := abandonOrcaPendingRecord(t, "worktree_create", true)
		orca := &fakeAbandonOrca{inventory: port.ExecutionOrcaIntentInventory{AuthoritativeZero: false}}
		result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), abandonDeps(&fakeAbandonGit{}, orca))
		if err == nil || !containsString(result.Missing, "pending_intent_safe") {
			t.Fatalf("non-authoritative zero must block: %v %v", err, result.Missing)
		}
	})
	t.Run("workspace root still on disk", func(t *testing.T) {
		stateRoot, record, _, root := abandonOrcaPendingRecord(t, "worktree_create", true)
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		orca := authoritativeZeroOrca()
		result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), abandonDeps(&fakeAbandonGit{}, orca))
		if err == nil || !containsString(result.Missing, "pending_intent_safe") || !containsString(result.Missing, "worktree_absent") {
			t.Fatalf("present workspace root must block both gates: %v %v", err, result.Missing)
		}
		if orca.inspects != 0 {
			t.Fatalf("disk residue must be rejected before any Orca call: %d", orca.inspects)
		}
	})
}

// 이미 병합된 child 작업의 worktree와 Orca 자원이 외부에서 정리된 경우,
// resume의 owner_launch/dispatch pending은 봉인된 모든 단계가 authoritative
// zero일 때만 abandon할 수 있어야 한다.
func TestCleanupAbandonAllowsStaleOwnerIntentAfterEveryOrcaStageIsAbsent(t *testing.T) {
	for _, tc := range []struct {
		name          string
		stage         port.ExecutionOrcaIntentStage
		kind          string
		wantInspects  int
		terminalPTYID string
		runID         string
		runBound      bool
		taskID        string
	}{
		{name: "owner launch", stage: port.ExecutionOrcaIntentTerminal, kind: "owner_launch", wantInspects: 2},
		{name: "Run create", stage: port.ExecutionOrcaIntentRun, kind: "owner_launch", wantInspects: 2, terminalPTYID: "pty-current"},
		{name: "Run bind", stage: port.ExecutionOrcaIntentRunBind, kind: "owner_launch", wantInspects: 2, terminalPTYID: "pty-current", runID: "run-current"},
		{name: "task", stage: port.ExecutionOrcaIntentTask, kind: "owner_launch", wantInspects: 3, terminalPTYID: "pty-current", runID: "run-current", runBound: true},
		{name: "dispatch", stage: port.ExecutionOrcaIntentDispatch, kind: "dispatch", wantInspects: 4, terminalPTYID: "pty-current", runID: "run-current", runBound: true, taskID: "task-current"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stateRoot, record, payload := legacyResumeIntentFixture(t, "gitlab", 2646)
			payload.Stage = intentContractStage(tc.stage)
			payload.TerminalPTYID = tc.terminalPTYID
			payload.RunID = tc.runID
			payload.RunBound = tc.runBound
			payload.TaskID = tc.taskID
			data, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			record.Execution.Pending.Kind = tc.kind
			record, err = persistExecutionTransitionWithMutations(stateRoot, record, nil, []sqlstore.Mutation{{
				Bucket: externalIntentBucket, ID: payload.OperationID, Data: data,
			}})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.RemoveAll(record.Execution.Workspace.Root); err != nil {
				t.Fatal(err)
			}
			orca := authoritativeZeroOrca()
			owner := &fakeOwnerInspector{}
			deps := CleanupAbandonDeps{Git: (&fakeAbandonGit{}).run, Orca: orca, OrcaOwner: owner}

			result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), deps)
			if err != nil {
				t.Fatalf("모든 Orca 자원이 사라진 stale %s intent를 정리하지 못했다: %v (%+v)", tc.kind, err, result)
			}
			if orca.inspects != tc.wantInspects {
				t.Fatalf("현재 단계까지의 모든 봉인 intent를 조회하지 않았다: got=%d want=%d", orca.inspects, tc.wantInspects)
			}
			for _, stage := range orca.stages {
				if stage == port.ExecutionOrcaIntentRun || stage == port.ExecutionOrcaIntentRunBind {
					t.Fatalf("삭제할 수 없는 Run을 cleanup residue로 조회했다: %+v", orca.stages)
				}
			}
			if len(owner.calls) != 1 {
				t.Fatalf("이전 generation의 owner 자원을 조회하지 않았다: %+v", owner.calls)
			}
		})
	}
}

// 현재 pending 단계만 비어 있어도 이전 단계 자원이 남아 있으면 레코드를
// 지우면 안 된다. terminal pending에서는 먼저 생성된 worktree를 함께 확인한다.
func TestCleanupAbandonRejectsStaleOwnerIntentWithEarlierStageResidue(t *testing.T) {
	stateRoot, record, _ := legacyResumeIntentFixture(t, "gitlab", 2646)
	if err := os.RemoveAll(record.Execution.Workspace.Root); err != nil {
		t.Fatal(err)
	}
	orca := &fakeAbandonOrca{inventory: port.ExecutionOrcaIntentInventory{
		Candidates: []port.ExecutionOrcaIntentReceipt{{Workspace: &port.ExecutionOrcaWorkspaceReceipt{
			WorktreeID: "worktree-still-present",
		}}},
	}}
	result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), CleanupAbandonDeps{
		Git: (&fakeAbandonGit{}).run, Orca: orca, OrcaOwner: &fakeOwnerInspector{},
	})
	if err == nil || !containsString(result.Missing, "pending_intent_safe") ||
		!strings.Contains(result.PendingIntentError, "candidate") {
		t.Fatalf("이전 단계 Orca 잔여를 허용했다: err=%v result=%+v", err, result)
	}
	if orca.inspects != 1 {
		t.Fatalf("잔여를 찾은 단계에서 즉시 중단하지 않았다: %d", orca.inspects)
	}
}

// 현재 generation의 intent가 비어도 이전 binding의 task/terminal이 살아 있으면
// 별도 owner gate가 계속 fail-closed해야 한다.
func TestCleanupAbandonRejectsStaleOwnerIntentWithPriorOwnerResidue(t *testing.T) {
	stateRoot, record, _ := legacyResumeIntentFixture(t, "gitlab", 2646)
	if err := os.RemoveAll(record.Execution.Workspace.Root); err != nil {
		t.Fatal(err)
	}
	result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), CleanupAbandonDeps{
		Git: (&fakeAbandonGit{}).run, Orca: authoritativeZeroOrca(),
		OrcaOwner: &fakeOwnerInspector{inventory: port.ExecutionOrcaOwnerInventory{
			TaskLive: true, TaskStatus: "running",
		}},
	})
	if err == nil || containsString(result.Missing, "pending_intent_safe") ||
		!containsString(result.Missing, "orca_resources_absent") {
		t.Fatalf("이전 owner 잔여 판정이 분리되지 않았다: err=%v result=%+v", err, result)
	}
}

// AC-01/AC-02 성공 경로: 실측 형태(worktree_create pending + ambiguous failure)가
// preview·apply 양쪽에서 orca를 실조회하고, 레코드와 intent 행을 같은 배치로
// 지운다.
func TestCleanupAbandonDeletesRecordAndIntentRowAtomically(t *testing.T) {
	stateRoot, record, operationID, _ := abandonOrcaPendingRecord(t, "worktree_create", true)
	orca := authoritativeZeroOrca()
	deps := abandonDeps(&fakeAbandonGit{}, orca)

	preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	if preview.PendingOperationID != operationID {
		t.Fatalf("preview must surface the pending operation id: %+v", preview)
	}
	applied, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, true, preview.Fingerprint), deps)
	if err != nil {
		t.Fatal(err)
	}
	if orca.inspects != 2 {
		t.Fatalf("preview and apply must each inspect the sealed intent: %d", orca.inspects)
	}
	if !applied.RecordDeleted || len(applied.IntentRowsDeleted) != 1 || applied.IntentRowsDeleted[0] != operationID {
		t.Fatalf("apply must delete the record and its intent row: %+v", applied)
	}
	if abandonIntentRowExists(t, stateRoot, operationID) {
		t.Fatal("external intent row must not survive the record")
	}
	if _, err := ReadIssueOps(stateRoot, record.ID); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("record must be gone: %v", err)
	}
}

// 멱등: Failure만 참조하는 행이 이미 없으면 성공이다
// (normalizeOrcaRemoveWorktreeErr 계약 동형).
func TestCleanupAbandonTreatsAbsentIntentRowAsSuccess(t *testing.T) {
	stateRoot, record := abandonTestRecord(t)
	root := filepath.Join(t.TempDir(), "absent-worktree")
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *IssueOpsRecord) {
		rec.Execution = abandonExecution(rec.Repo, root, WriteLease{Generation: 1, Status: model.LeaseStatusReleased})
		rec.Execution.Failure = &model.ExecutionFailure{
			OperationID: "op-already-collected", Code: "external_operation_ambiguous", At: "2026-07-24T00:00:00Z",
		}
	})
	deps := abandonDeps(&fakeAbandonGit{}, authoritativeZeroOrca())
	preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	applied, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, true, preview.Fingerprint), deps)
	if err != nil {
		t.Fatalf("absent intent row must be idempotent success: %v", err)
	}
	if !applied.RecordDeleted || len(applied.IntentRowsDeleted) != 0 {
		t.Fatalf("no row should be reported deleted: %+v", applied)
	}
}

// 소유자 가드: 다른 lifecycle이 소유한 행은 하드 에러이고 레코드는 보존된다
// (execution_state.go:150-159 규율 준용).
func TestCleanupAbandonRefusesToDeleteAnotherLifecyclesIntentRow(t *testing.T) {
	stateRoot, record, operationID, root := abandonOrcaPendingRecord(t, "worktree_create", true)
	foreign := "op-foreign-row"
	writeAbandonIntentRow(t, stateRoot, foreign, externalOrcaIntentPayload{
		SchemaVersion: model.IssueOpsSchemaVersion, OperationID: foreign, LifecycleID: "io-someoneelse",
		Generation: 1, Stage: intentContractStage(port.ExecutionOrcaIntentWorktree), Marker: "m",
		StartedAt: "2026-07-24T00:00:00Z", InvocationState: orcaIntentNotInvoked,
		Workspace: intentContractWorkspaceRequest(port.ExecutionWorkspaceRequest{
			LifecycleID: "io-someoneelse", SourceRoot: record.Repo, Root: root,
			Branch: "other", BaseBranch: "main", BaseHead: "deadbeef",
		}),
		Probe:           intentContractProbeRequest(port.ExecutionOrcaProbeRequest{Repo: record.Repo, Host: "codex", Model: "gpt-5.4", Marker: "m"}),
		IssueBodySHA256: abandonIssueBodySHA,
	})
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *IssueOpsRecord) {
		rec.Execution.Failure.OperationID = foreign
	})
	deps := abandonDeps(&fakeAbandonGit{}, authoritativeZeroOrca())
	preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, true, preview.Fingerprint), deps)
	if err == nil || !strings.Contains(err.Error(), "another lifecycle") || result.FailedStep != "record_delete" {
		t.Fatalf("foreign intent row must hard-fail: %v %+v", err, result)
	}
	if _, err := ReadIssueOps(stateRoot, record.ID); err != nil {
		t.Fatalf("record must survive the refusal: %v", err)
	}
	if !abandonIntentRowExists(t, stateRoot, operationID) || !abandonIntentRowExists(t, stateRoot, foreign) {
		t.Fatal("no row may be deleted when the batch is refused")
	}
}

// TOCTOU: preview 이후 fingerprint 입력이 바뀌면 apply는 거부된다. apply
// without --confirm도 함께 고정한다.
func TestCleanupAbandonApplyRejectsStaleFingerprintAndMissingConfirm(t *testing.T) {
	stateRoot, record := abandonTestRecord(t)
	deps := abandonDeps(&fakeAbandonGit{}, authoritativeZeroOrca())
	preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	noConfirm := abandonRequest(record.ID, true, preview.Fingerprint)
	noConfirm.Confirm = false
	if _, err := CleanupAbandon(context.Background(), stateRoot, noConfirm, deps); err == nil || !strings.Contains(err.Error(), "--confirm") {
		t.Fatalf("apply without confirm must be rejected: %v", err)
	}
	// phase 이동은 게이트를 통과하지만 fingerprint 입력을 바꾼다.
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *IssueOpsRecord) { rec.Phase = IssueOpsPhasePlan })
	if _, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, true, preview.Fingerprint), deps); err == nil ||
		!strings.Contains(err.Error(), "stale cleanup fingerprint") {
		t.Fatalf("stale fingerprint must be rejected: %v", err)
	}
	if _, err := ReadIssueOps(stateRoot, record.ID); err != nil {
		t.Fatalf("record must survive a stale apply: %v", err)
	}
}
