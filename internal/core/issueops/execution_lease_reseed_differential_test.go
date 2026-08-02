package issueops_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	leaseinbound "agent-harness/internal/adapter/inbound/issueopslease"
	leaseoutbound "agent-harness/internal/adapter/outbound/issueopslease"
	leaseapp "agent-harness/internal/application/issueopslease"
	issueopscontract "agent-harness/internal/contract/issueops"
	leasecontract "agent-harness/internal/contract/issueopslease"
	"agent-harness/internal/core/issueops"
	"agent-harness/internal/core/sqlstore"
	"agent-harness/internal/port"
)

func TestReseedDifferentialClonedDirectStateInvokesRealVertical(t *testing.T) {
	for _, testCase := range []struct {
		name         string
		status       issueopscontract.LeaseStatus
		legacyRecord bool
	}{
		{name: "schema-v1-claimable", status: issueopscontract.LeaseStatusClaimable},
		{name: "schema-v1-released", status: issueopscontract.LeaseStatusReleased},
		{name: "legacy-claimable", status: issueopscontract.LeaseStatusClaimable, legacyRecord: true},
		{name: "legacy-released", status: issueopscontract.LeaseStatusReleased, legacyRecord: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			legacy := newDifferentialFixture(t, testCase.name, testCase.status, issueopscontract.ExecutionModeDirect, testCase.legacyRecord)
			vertical := cloneDifferentialFixture(t, legacy)
			assertDifferentialRawClone(t, legacy, vertical)
			if testCase.legacyRecord {
				assertDifferentialLegacySeed(t, legacy)
				assertDifferentialLegacySeed(t, vertical)
			}
			actor := differentialActor(t)
			legacyPreview := differentialPreview(t, legacy, actor, issueops.ExecutionReplaceDependencies{})
			verticalPreview := differentialPreview(t, vertical, actor, issueops.ExecutionReplaceDependencies{})
			legacyResult, err := issueops.ReseedExecutionCompatibilityOracleForTest(context.Background(), legacy.stateRoot, issueops.ExecutionReplaceRequest{
				ID: legacy.id, Action: issueops.ExecutionReplaceReseed, ExpectedGeneration: 1,
				InventoryFingerprint: legacyPreview.InventoryFingerprint, Reason: "differential reseed",
				Actor: actor, CWD: legacy.worktree, Confirm: true,
			}, issueops.ExecutionReplaceDependencies{})
			if err != nil {
				t.Fatalf("legacy reseed: %v", err)
			}
			cleanupDifferentialArtifacts(t, legacyResult)
			verticalAny, err := issueops.ExecuteExecution(context.Background(), vertical.stateRoot, issueops.ExecutionActionRequest{
				Action: issueops.ExecutionActionReplace, ReplaceAction: issueops.ExecutionReplaceReseed, ID: vertical.id,
				ExpectedGeneration: 1, InventoryFingerprint: verticalPreview.InventoryFingerprint,
				Reason: "differential reseed", Actor: actor, CWD: vertical.worktree, Confirm: true,
			}, issueops.ExecutionActionDependencies{Reseed: realVerticalReseedHandler(t, nil, nil)})
			if err != nil {
				t.Fatalf("real vertical reseed: %v", err)
			}
			verticalResult, ok := verticalAny.(issueops.ExecutionReplaceResult)
			if !ok {
				t.Fatalf("vertical result type=%T", verticalAny)
			}
			assertDifferentialProjection(t, legacyResult, verticalResult)
			assertDifferentialToken(t, legacyResult.ClaimTokenPath)
			assertDifferentialToken(t, verticalResult.ClaimTokenPath)
			assertDifferentialRecord(t, legacy)
			assertDifferentialRecord(t, vertical)
			assertDifferentialPersistedProjection(t, legacy, vertical)
		})
	}
}

func TestReseedDifferentialClonedOrcaStateInvokesRealVertical(t *testing.T) {
	for _, testCase := range []struct {
		name         string
		status       issueopscontract.LeaseStatus
		legacyRecord bool
	}{
		{name: "schema-v1-claimable", status: issueopscontract.LeaseStatusClaimable},
		{name: "schema-v1-released", status: issueopscontract.LeaseStatusReleased},
		{name: "legacy-claimable", status: issueopscontract.LeaseStatusClaimable, legacyRecord: true},
		{name: "legacy-released", status: issueopscontract.LeaseStatusReleased, legacyRecord: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			legacy := newDifferentialFixture(t, "orca-"+testCase.name, testCase.status, issueopscontract.ExecutionModeOrca, testCase.legacyRecord)
			vertical := cloneDifferentialFixture(t, legacy)
			assertDifferentialRawClone(t, legacy, vertical)
			if testCase.legacyRecord {
				assertDifferentialLegacySeed(t, legacy)
				assertDifferentialLegacySeed(t, vertical)
			}
			actor := differentialActor(t)
			owner := differentialOrcaOwner{}
			reader := differentialIssueReader
			deps := issueops.ExecutionReplaceDependencies{OrcaOwner: owner, ReadIssue: reader}
			legacyPreview := differentialPreview(t, legacy, actor, deps)
			verticalPreview := differentialPreview(t, vertical, actor, deps)
			legacyResult, err := issueops.ReseedExecutionCompatibilityOracleForTest(context.Background(), legacy.stateRoot, issueops.ExecutionReplaceRequest{ID: legacy.id, Action: issueops.ExecutionReplaceReseed, ExpectedGeneration: 1, InventoryFingerprint: legacyPreview.InventoryFingerprint, Reason: "differential reseed", Actor: actor, CWD: legacy.worktree, Confirm: true}, deps)
			if err != nil {
				t.Fatalf("legacy Orca reseed: %v", err)
			}
			cleanupDifferentialArtifacts(t, legacyResult)
			verticalAny, err := issueops.ExecuteExecution(context.Background(), vertical.stateRoot, issueops.ExecutionActionRequest{Action: issueops.ExecutionActionReplace, ReplaceAction: issueops.ExecutionReplaceReseed, ID: vertical.id, ExpectedGeneration: 1, InventoryFingerprint: verticalPreview.InventoryFingerprint, Reason: "differential reseed", Actor: actor, CWD: vertical.worktree, Confirm: true}, issueops.ExecutionActionDependencies{ReadIssue: reader, Reseed: realVerticalReseedHandler(t, owner, reader)})
			if err != nil {
				t.Fatalf("real vertical Orca reseed: %v", err)
			}
			verticalResult, ok := verticalAny.(issueops.ExecutionReplaceResult)
			if !ok {
				t.Fatalf("vertical Orca result type=%T", verticalAny)
			}
			assertDifferentialProjection(t, legacyResult, verticalResult)
			assertDifferentialToken(t, legacyResult.ClaimTokenPath)
			assertDifferentialToken(t, verticalResult.ClaimTokenPath)
			assertDifferentialRecord(t, legacy)
			assertDifferentialRecord(t, vertical)
			assertDifferentialPersistedProjection(t, legacy, vertical)
		})
	}
}

func TestReseedDifferentialOrcaSnapshotEvidenceReachesRealVerticalWithoutFallback(t *testing.T) {
	fixture := newDifferentialFixture(t, "orca-snapshot", issueopscontract.LeaseStatusClaimable, issueopscontract.ExecutionModeOrca, false)
	record, err := issueops.ReadIssueOps(fixture.stateRoot, fixture.id)
	if err != nil {
		t.Fatal(err)
	}
	record.IssueURL = "https://gitlab.example.com/acme/repo/-/work_items/16"
	record.BranchPrepare.Provider = "gitlab"
	record.BranchPrepare.IssueURL = record.IssueURL
	if _, err := issueops.WriteIssueOps(fixture.stateRoot, record); err != nil {
		t.Fatal(err)
	}
	actor := differentialActor(t)
	owner := differentialOrcaOwner{}
	preview := differentialPreview(t, fixture, actor, issueops.ExecutionReplaceDependencies{OrcaOwner: owner})
	evidence := &port.ExecutionIssueSnapshotEvidence{
		Provider: "gitlab", Source: "glab_mcp", WebURL: "https://gitlab.example.com/acme/repo/-/issues/16",
		Body: "## acceptance criteria\n\n- [ ] AC-09: production reseed snapshot authority\n\n## verification\n\n```bash\ngo test ./internal/core/issueops -count=1\n```\n", State: "opened",
	}
	fallbackCalls := 0
	resultAny, err := issueops.ExecuteExecution(context.Background(), fixture.stateRoot, issueops.ExecutionActionRequest{
		Action: issueops.ExecutionActionReplace, ReplaceAction: issueops.ExecutionReplaceReseed, ID: fixture.id,
		ExpectedGeneration: 1, InventoryFingerprint: preview.InventoryFingerprint, Reason: "snapshot reseed",
		Actor: actor, CWD: fixture.worktree, Confirm: true, IssueSnapshot: evidence,
	}, issueops.ExecutionActionDependencies{
		ReadIssue: func(context.Context, string, port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
			fallbackCalls++
			return port.ExecutionIssueSnapshot{}, errors.New("fallback reader must not run")
		},
		Reseed: realVerticalReseedHandler(t, owner, nil),
	})
	if err != nil {
		t.Fatalf("reseed with supplied snapshot evidence: %v", err)
	}
	result, ok := resultAny.(issueops.ExecutionReplaceResult)
	if !ok || !result.OK || result.IssueSnapshotSource != "glab_mcp" {
		t.Fatalf("reseed result=%#v", resultAny)
	}
	if fallbackCalls != 0 {
		t.Fatalf("supplied snapshot evidence invoked fallback %d times", fallbackCalls)
	}
}

type differentialFixture struct {
	stateRoot string
	id        string
	worktree  string
}

func newDifferentialFixture(t *testing.T, branch string, status issueopscontract.LeaseStatus, mode issueopscontract.ExecutionMode, legacyRecord bool) differentialFixture {
	t.Helper()
	branch = "192-" + branch
	stateRoot := t.TempDir()
	repo := filepath.Join(t.TempDir(), "repo")
	worktree := filepath.Join(t.TempDir(), "worktree")
	runDifferentialGit(t, "", "init", "-q", "-b", "main", repo)
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("# differential fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runDifferentialGit(t, repo, "add", "README.md")
	runDifferentialGit(t, repo, "-c", "user.name=IssueOps Test", "-c", "user.email=issueops@example.invalid", "commit", "-q", "-m", "test: seed differential fixture")
	runDifferentialGit(t, repo, "worktree", "add", "-q", "-b", branch, worktree, "main")
	baseHead := strings.TrimSpace(runDifferentialGit(t, worktree, "rev-parse", "HEAD"))
	record, err := issueops.StartIssueOps(stateRoot, issueopscontract.IssueOpsStartRequest{Repo: repo, Branch: branch})
	if err != nil {
		t.Fatal(err)
	}
	record.WorktreePath = worktree
	record.Phase = issueops.IssueOpsPhaseImplement
	record.IssueURL = "https://github.com/example/agent-harness/issues/192"
	record.BranchPrepare = &issueopscontract.IssueOpsBranchPrepare{Provider: "github", IssueURL: record.IssueURL, Branch: branch, BaseBranch: "main", BaseSHA: baseHead, LinkVerified: true}
	driver := "git"
	if mode == issueopscontract.ExecutionModeOrca {
		driver = "orca"
	}
	record.Execution = &issueopscontract.Execution{Mode: mode, Workspace: issueopscontract.Workspace{SourceRoot: repo, Root: worktree, Branch: branch, BaseHead: baseHead, Driver: driver, LinkedAt: "2026-07-30T09:00:00Z"}, Lease: issueopscontract.WriteLease{Generation: 1, Status: status}}
	if mode == issueopscontract.ExecutionModeOrca {
		record.Execution.Orca = &issueopscontract.OrcaBinding{RuntimeID: "runtime-1", RepoID: "repo-1", WorktreeID: "worktree-1", LeaseGeneration: 1, OwnerHost: "codex", OwnerModel: "model", TaskID: "task-1", DispatchID: "dispatch-1", TerminalPTYID: "pty-1"}
	}
	if status == issueopscontract.LeaseStatusClaimable {
		record.Execution.Lease.ClaimTokenSHA256 = strings.Repeat("a", 64)
	}
	if status == issueopscontract.LeaseStatusReleased {
		record.Execution.Lease.ReleasedAt = "2026-07-30T09:00:00Z"
	}
	if _, err := issueops.WriteIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	if legacyRecord {
		seedDifferentialLegacyRecord(t, stateRoot, record.ID)
	}
	return differentialFixture{stateRoot: stateRoot, id: record.ID, worktree: worktree}
}

func cloneDifferentialFixture(t *testing.T, source differentialFixture) differentialFixture {
	t.Helper()
	targetRoot := t.TempDir()
	raw := differentialRawRecord(t, source)
	db, err := sqlstore.Open(targetRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put("issueops_v1", source.id, raw); err != nil {
		t.Fatal(err)
	}
	return differentialFixture{stateRoot: targetRoot, id: source.id, worktree: source.worktree}
}

func differentialRawRecord(t *testing.T, fixture differentialFixture) []byte {
	t.Helper()
	db, err := sqlstore.Open(fixture.stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	raw, ok, err := db.Get("issueops_v1", fixture.id)
	if err != nil || !ok {
		t.Fatalf("read differential record ok=%t err=%v", ok, err)
	}
	return raw
}

func assertDifferentialRawClone(t *testing.T, source, clone differentialFixture) {
	t.Helper()
	if !bytes.Equal(differentialRawRecord(t, source), differentialRawRecord(t, clone)) {
		t.Fatal("oracle and vertical fixtures must begin from identical raw record bytes")
	}
}

func seedDifferentialLegacyRecord(t *testing.T, stateRoot, id string) {
	t.Helper()
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	raw, ok, err := db.Get("issueops_v1", id)
	if err != nil || !ok {
		t.Fatalf("read v1 fixture record ok=%t err=%v", ok, err)
	}
	legacy := bytes.Replace(raw, []byte("  \"schema_version\": 1,\n"), nil, 1)
	if bytes.Equal(legacy, raw) {
		t.Fatal("fixture did not contain schema_version=1")
	}
	if err := db.Put("issueops_v1", id, legacy); err != nil {
		t.Fatal(err)
	}
}

func assertDifferentialLegacySeed(t *testing.T, fixture differentialFixture) {
	t.Helper()
	db, err := sqlstore.Open(fixture.stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	raw, ok, err := db.Get("issueops_v1", fixture.id)
	if err != nil || !ok {
		t.Fatalf("read legacy fixture record ok=%t err=%v", ok, err)
	}
	if bytes.Contains(raw, []byte("\"schema_version\": 1")) {
		t.Fatalf("legacy fixture retained schema v1: %s", raw)
	}
}

func differentialPreview(t *testing.T, fixture differentialFixture, actor issueopscontract.NativeActor, deps issueops.ExecutionReplaceDependencies) issueops.ExecutionReplaceResult {
	t.Helper()
	preview, err := issueops.ReplaceExecutionWithDependencies(context.Background(), fixture.stateRoot, issueops.ExecutionReplaceRequest{ID: fixture.id, Action: issueops.ExecutionReplacePreview, ExpectedGeneration: 1, Actor: actor, CWD: fixture.worktree}, deps)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	return preview
}

func realVerticalReseedHandler(t *testing.T, owner port.ExecutionOrcaOwnerInspector, readIssue issueops.ExecutionIssueSnapshotReadFunc) issueops.ExecutionReseedHandler {
	t.Helper()
	return func(ctx context.Context, stateRoot string, request issueops.ExecutionReseedRequest) (issueops.ExecutionReplaceResult, error) {
		resolvedReader := readIssue
		if request.ReadIssue != nil {
			resolvedReader = request.ReadIssue
		}
		db, err := sqlstore.Open(stateRoot)
		if err != nil {
			return issueops.ExecutionReplaceResult{ID: request.ID, Action: issueops.ExecutionReplaceReseed}, err
		}
		fence, err := leaseoutbound.NewSQLiteReseedFence(stateRoot, func(root string) (port.TransactionalRecordStore, error) { return sqlstore.Open(root) })
		if err != nil {
			return issueops.ExecutionReplaceResult{ID: request.ID, Action: issueops.ExecutionReplaceReseed}, err
		}
		inventory := leaseoutbound.NewReseedInventory(owner, leaseoutbound.InspectNativeProcess)
		artifacts := leaseoutbound.NewReseedArtifacts(func(ctx context.Context, record leasecontract.Record) (leasecontract.ReseedReceipt, error) {
			if record.Execution.Mode != "orca" {
				return leasecontract.ReseedReceipt{}, nil
			}
			execution, err := differentialReseedExecution(record)
			if err != nil {
				return leasecontract.ReseedReceipt{}, err
			}
			prepared, err := issueops.PrepareExecutionReseedOwnerArtifacts(ctx, stateRoot, record.ID, execution, resolvedReader)
			if err != nil {
				return leasecontract.ReseedReceipt{}, err
			}
			return leasecontract.ReseedReceipt{IssueBodySHA256: prepared.IssueBodySHA256, ContextPacketPath: prepared.ContextPacketPath, ContextPacketSHA256: prepared.ContextPacketSHA256, OwnerPromptPath: prepared.OwnerPromptPath, OwnerPromptSHA256: prepared.OwnerPromptSHA256}, nil
		})
		service := leaseapp.NewReseedService(fence, leaseoutbound.NewReseedRepository(db), inventory, artifacts, leaseoutbound.UTCClock{}, leaseoutbound.InspectNativeProcess, leaseoutbound.FilesystemPathMatcher{})
		return leaseinbound.NewReseedHandler(service)(ctx, stateRoot, request)
	}
}

func differentialReseedExecution(record leasecontract.Record) (issueopscontract.Execution, error) {
	data, err := json.Marshal(record.Execution)
	if err != nil {
		return issueopscontract.Execution{}, err
	}
	var execution issueopscontract.Execution
	if err := json.Unmarshal(data, &execution); err != nil {
		return issueopscontract.Execution{}, err
	}
	return execution, nil
}

func differentialActor(t *testing.T) issueopscontract.NativeActor {
	t.Helper()
	receipt, err := issueops.ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	return issueopscontract.NativeActor{Host: "codex", SessionID: "reseed-differential", SessionProcess: &receipt, ProcessAncestry: []issueopscontract.NativeProcessReceipt{receipt}}
}

type differentialOrcaOwner struct{}

func (differentialOrcaOwner) InspectOwner(context.Context, port.ExecutionOrcaOwnerInventoryRequest) (port.ExecutionOrcaOwnerInventory, error) {
	return port.ExecutionOrcaOwnerInventory{RuntimeID: "runtime-1", TaskStatus: "completed", DispatchStatus: "failed"}, nil
}

func differentialIssueReader(_ context.Context, _ string, request port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
	return port.ExecutionIssueSnapshot{URL: request.URL, Body: "## acceptance criteria\n\n- [ ] AC-01: differential\n\n## verification\n\n```bash\ngo test ./internal/core/issueops -count=1\n```\n"}, nil
}

func assertDifferentialToken(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("target token %q: %v", path, err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("target token mode=%#o", info.Mode().Perm())
	}
}

func cleanupDifferentialArtifacts(t *testing.T, result issueops.ExecutionReplaceResult) {
	t.Helper()
	for _, path := range []string{result.ClaimTokenPath, result.ContextPacketPath, result.OwnerPromptPath} {
		if path == "" {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			t.Fatalf("remove oracle artifact %q: %v", path, err)
		}
	}
}

func assertDifferentialProjection(t *testing.T, oracle, vertical issueops.ExecutionReplaceResult) {
	t.Helper()
	if oracle.Execution.Lease.Generation != 2 || vertical.Execution.Lease.Generation != 2 {
		t.Fatalf("reseed generation oracle=%d vertical=%d", oracle.Execution.Lease.Generation, vertical.Execution.Lease.Generation)
	}
	if oracle.Execution.Lease.ClaimTokenSHA256 == "" || vertical.Execution.Lease.ClaimTokenSHA256 == "" {
		t.Fatalf("reseed must return a new claim token hash oracle=%q vertical=%q", oracle.Execution.Lease.ClaimTokenSHA256, vertical.Execution.Lease.ClaimTokenSHA256)
	}
	if got, want := normalizeDifferentialResult(oracle), normalizeDifferentialResult(vertical); !reflect.DeepEqual(got, want) {
		t.Fatalf("public reseed projection differs\noracle=%+v\nvertical=%+v", got, want)
	}
}

func normalizeDifferentialResult(result issueops.ExecutionReplaceResult) issueops.ExecutionReplaceResult {
	result.Execution.Lease.ClaimTokenSHA256 = ""
	result.Execution.Lease.ReplacedAt = ""
	return result
}

func assertDifferentialRecord(t *testing.T, fixture differentialFixture) {
	t.Helper()
	db, err := sqlstore.Open(fixture.stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	raw, ok, err := db.Get("issueops_v1", fixture.id)
	if err != nil || !ok {
		t.Fatalf("read canonical bytes ok=%t err=%v", ok, err)
	}
	record, err := leasecontract.Decode(fixture.id, raw)
	if err != nil {
		t.Fatalf("decode canonical bytes: %v", err)
	}
	if record.SchemaVersion != 1 || record.Execution.Lease.Generation != 2 || record.Execution.Lease.Status != "claimable" || record.Execution.Lease.ReplacementReason != "differential reseed" {
		t.Fatalf("canonical differential record=%+v", record.Execution.Lease)
	}
}

func assertDifferentialPersistedProjection(t *testing.T, oracle, vertical differentialFixture) {
	t.Helper()
	oracleRecord := normalizeDifferentialRecord(t, oracle)
	verticalRecord := normalizeDifferentialRecord(t, vertical)
	if !reflect.DeepEqual(oracleRecord, verticalRecord) {
		t.Fatalf("persisted reseed projection differs\noracle=%+v\nvertical=%+v", oracleRecord, verticalRecord)
	}
}

func normalizeDifferentialRecord(t *testing.T, fixture differentialFixture) leasecontract.Record {
	t.Helper()
	record, err := leasecontract.Decode(fixture.id, differentialRawRecord(t, fixture))
	if err != nil {
		t.Fatalf("decode differential projection: %v", err)
	}
	record.UpdatedAt = ""
	record.Execution.Lease.ClaimTokenSHA256 = ""
	record.Execution.Lease.ReplacedAt = ""
	return record
}

func runDifferentialGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}
