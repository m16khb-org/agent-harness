package issueops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"agent-harness/internal/core/preflight"
	"agent-harness/internal/port"
)

type publicationRefFake struct {
	localHead  string
	remoteHead string
	localRef   string
	remote     string
	remoteRef  string
	pushCalls  int
	pushHead   string
}

func (f *publicationRefFake) PushExact(_ context.Context, _, _, _, finalHead string) error {
	f.pushCalls++
	f.pushHead = finalHead
	return nil
}

func TestAcceptedHandoffPublicationPersistsAmbiguousInventoryRecovery(t *testing.T) {
	stateRoot, record := acceptedPublicationHandoff(t, "github")
	client := handoffDispatchFake(record)
	client.terminalListErr = errors.New("truncated terminal inventory")
	finalHead := record.ExecutionHandoff.Result.FinalHead
	reader := &publicationRefFake{localHead: finalHead, remoteHead: finalHead}
	if _, err := RecordIssueOpsHandoffPublishReceipt(context.Background(), stateRoot, IssueOpsHandoffPublishRequest{ID: record.ID, Confirm: true}, reader, client, handoffPrepareTestClock()); err == nil {
		t.Fatal("ambiguous publication inventory was accepted")
	}
	persisted, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ExecutionHandoff.PublicationRecovery == nil || persisted.ExecutionHandoff.PublicationRecovery.Code != "publication_inventory_ambiguous" || persisted.ExecutionHandoff.PublishReceipt != nil {
		t.Fatalf("publication ambiguity was not persisted: %#v", persisted.ExecutionHandoff)
	}
}

func TestLegacyNilHandoffPublishRefusesWithoutMutation(t *testing.T) {
	stateRoot, record := handoffPrepareRecord(t)
	before := rawIssueOpsBytesForTest(t, stateRoot, record.ID)
	reader := &publicationRefFake{}
	client := handoffDispatchFake(record)
	if _, err := RecordIssueOpsHandoffPublishReceipt(context.Background(), stateRoot, IssueOpsHandoffPublishRequest{ID: record.ID, Confirm: true}, reader, client, handoffPrepareTestClock()); err == nil || !strings.Contains(err.Error(), "closed accepted execution handoff") {
		t.Fatalf("legacy nil-handoff publish error = %v", err)
	}
	if reader.pushCalls != 0 {
		t.Fatalf("legacy nil-handoff crossed push boundary: calls=%d", reader.pushCalls)
	}
	if after := rawIssueOpsBytesForTest(t, stateRoot, record.ID); !reflect.DeepEqual(after, before) {
		t.Fatal("legacy nil-handoff publish changed durable state")
	}
}

func (f *publicationRefFake) LocalRefHead(_ context.Context, _ string, ref string) (string, error) {
	f.localRef = ref
	return f.localHead, nil
}

func (f *publicationRefFake) RemoteRefHead(_ context.Context, _ string, remote, ref string) (string, error) {
	f.remote, f.remoteRef = remote, ref
	return f.remoteHead, nil
}

func TestGitPublicationPushUsesImmutableFinalHeadArgv(t *testing.T) {
	bin := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "push-argv")
	envLogPath := filepath.Join(t.TempDir(), "push-env")
	script := filepath.Join(bin, "git")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$PUSH_ARGV_LOG\"\nprintf '%s' \"$GIT_TERMINAL_PROMPT\" > \"$PUSH_ENV_LOG\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("PUSH_ARGV_LOG", logPath)
	t.Setenv("PUSH_ENV_LOG", envLogPath)
	finalHead := strings.Repeat("a", 40)
	if err := (GitIssueOpsHandoffPublicationReader{}).PushExact(context.Background(), t.TempDir(), "origin", "16-demo", finalHead); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "push\n--\norigin\n" + finalHead + ":refs/heads/16-demo\n"
	if string(got) != want {
		t.Fatalf("push argv = %q, want %q", got, want)
	}
	if got, err := os.ReadFile(envLogPath); err != nil || string(got) != "0" {
		t.Fatalf("GIT_TERMINAL_PROMPT = %q, err=%v", got, err)
	}
}

func TestGitPublicationAdapterBoundsRedactsTimesOutAndRejectsUnsafeTokens(t *testing.T) {
	bin := t.TempDir()
	script := filepath.Join(bin, "git")
	body := "#!/bin/sh\n" +
		"if [ \"$GIT_TEST_MODE\" = timeout ]; then exec sleep 2; fi\n" +
		"if [ \"$GIT_TEST_MODE\" = stderr ]; then { printf 'api_key=abcdefghijklmnopqrstuvwxyz123456\\n'; i=0; while [ $i -lt 6000 ]; do printf x; i=$((i+1)); done; printf '\\n'; } >&2; exit 1; fi\n" +
		"printf '%s\\n' \"$@\" > \"$PUSH_ARGV_LOG\"\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	logPath := filepath.Join(t.TempDir(), "argv")
	t.Setenv("PUSH_ARGV_LOG", logPath)
	reader := GitIssueOpsHandoffPublicationReader{}

	t.Setenv("GIT_TEST_MODE", "stderr")
	_, err := reader.LocalRefHead(context.Background(), t.TempDir(), "refs/heads/16-demo")
	if err == nil || len(err.Error()) > publicationDiagnosticLimit+256 || strings.Contains(err.Error(), "abcdefghijklmnopqrstuvwxyz123456") {
		t.Fatalf("oversized secret diagnostic was not bounded/redacted: len=%d err=%v", len(errString(err)), err)
	}

	t.Setenv("GIT_TEST_MODE", "timeout")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	if _, err := reader.RemoteRefHead(ctx, t.TempDir(), "origin", "refs/heads/16-demo"); err == nil || time.Since(started) > time.Second {
		t.Fatalf("publication timeout was not honored promptly: elapsed=%s err=%v", time.Since(started), err)
	}

	t.Setenv("GIT_TEST_MODE", "")
	_ = os.Remove(logPath)
	if err := reader.PushExact(context.Background(), t.TempDir(), "--upload-pack=evil", "16-demo", strings.Repeat("a", 40)); err == nil {
		t.Fatal("option-like remote was accepted")
	}
	if _, err := reader.RemoteRefHead(context.Background(), t.TempDir(), "origin", "refs/heads/bad\nref"); err == nil {
		t.Fatal("control-containing ref was accepted")
	}
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("unsafe token reached git execution: %v", err)
	}
}

func TestGitPublicationLocalRefHeadUsesRealGitEndOfOptions(t *testing.T) {
	repo := initIssueOpsRepo(t)
	want := strings.TrimSpace(preflight.GitOut(repo, "rev-parse", "refs/heads/main"))
	got, err := (GitIssueOpsHandoffPublicationReader{}).LocalRefHead(context.Background(), repo, "refs/heads/main")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("local ref head = %s, want %s", got, want)
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestAcceptedHandoffPublicationReceiptRequiresExactLocalAndRemoteFinalHead(t *testing.T) {
	for _, provider := range []string{"github", "gitlab"} {
		t.Run(provider, func(t *testing.T) {
			stateRoot, record := acceptedPublicationHandoff(t, provider)
			finalHead := record.ExecutionHandoff.Result.FinalHead
			for _, tt := range []struct {
				name       string
				localHead  string
				remoteHead string
			}{
				{name: "wrong local", localHead: strings.Repeat("1", 40), remoteHead: finalHead},
				{name: "wrong remote", localHead: finalHead, remoteHead: strings.Repeat("2", 40)},
			} {
				t.Run(tt.name, func(t *testing.T) {
					reader := &publicationRefFake{localHead: tt.localHead, remoteHead: tt.remoteHead}
					client := handoffDispatchFake(record)
					client.terminals = nil
					if _, err := RecordIssueOpsHandoffPublishReceipt(context.Background(), stateRoot, IssueOpsHandoffPublishRequest{ID: record.ID, Confirm: true}, reader, client, handoffPrepareTestClock()); err == nil {
						t.Fatalf("%s publication accepted mismatched heads", provider)
					}
					persisted, err := ReadIssueOps(stateRoot, record.ID)
					if err != nil {
						t.Fatal(err)
					}
					if persisted.ExecutionHandoff.PublishReceipt != nil {
						t.Fatalf("mismatched head persisted receipt: %#v", persisted.ExecutionHandoff.PublishReceipt)
					}
				})
			}
		})
	}
}

func TestAcceptedHandoffPublicationReceiptHasGitHubGitLabParityAndFailsStale(t *testing.T) {
	for _, provider := range []string{"github", "gitlab"} {
		t.Run(provider, func(t *testing.T) {
			stateRoot, record := acceptedPublicationHandoff(t, provider)
			finalHead := record.ExecutionHandoff.Result.FinalHead
			reader := &publicationRefFake{localHead: finalHead, remoteHead: finalHead}
			client := handoffDispatchFake(record)
			client.terminals = nil
			persisted, err := RecordIssueOpsHandoffPublishReceipt(context.Background(), stateRoot, IssueOpsHandoffPublishRequest{ID: record.ID, Confirm: true}, reader, client, handoffPrepareTestClock())
			if err != nil {
				t.Fatal(err)
			}
			receipt := persisted.ExecutionHandoff.PublishReceipt
			if receipt == nil || receipt.Provider != provider || receipt.Branch != record.Branch || receipt.FinalHead != finalHead || receipt.Remote != "origin" || receipt.RemoteRef != "refs/heads/"+record.Branch {
				t.Fatalf("%s receipt = %#v", provider, receipt)
			}
			if reader.pushCalls != 1 || reader.pushHead != finalHead {
				t.Fatalf("%s push did not use immutable final head: %#v", provider, reader)
			}
			if reader.localRef != "refs/heads/"+record.Branch || reader.remote != "origin" || reader.remoteRef != "refs/heads/"+record.Branch {
				t.Fatalf("%s ref verification = %#v", provider, reader)
			}
			if err := ValidateIssueOpsHandoffPublication(context.Background(), stateRoot, persisted, provider, record.Branch, record.BranchPrepare.BaseBranch, reader, client); err != nil {
				t.Fatalf("%s matching publication receipt: %v", provider, err)
			}
			reader.remoteHead = strings.Repeat("3", 40)
			if err := ValidateIssueOpsHandoffPublication(context.Background(), stateRoot, persisted, provider, record.Branch, record.BranchPrepare.BaseBranch, reader, client); err == nil {
				t.Fatalf("%s stale remote receipt authorized publication", provider)
			}
			reader.remoteHead = finalHead
			for _, mismatch := range []struct{ provider, head, base string }{
				{provider: otherProvider(provider), head: record.Branch, base: record.BranchPrepare.BaseBranch},
				{provider: provider, head: "wrong-branch", base: record.BranchPrepare.BaseBranch},
				{provider: provider, head: record.Branch, base: "wrong-base"},
			} {
				if err := ValidateIssueOpsHandoffPublication(context.Background(), stateRoot, persisted, mismatch.provider, mismatch.head, mismatch.base, reader, client); err == nil {
					t.Fatalf("%s caller mismatch authorized publication: %#v", provider, mismatch)
				}
			}
		})
	}
}

func TestAcceptedHandoffPublicationRequiresDurableReceipt(t *testing.T) {
	stateRoot, record := acceptedPublicationHandoff(t, "github")
	finalHead := record.ExecutionHandoff.Result.FinalHead
	reader := &publicationRefFake{localHead: finalHead, remoteHead: finalHead}
	client := handoffDispatchFake(record)
	client.terminals = nil
	if err := ValidateIssueOpsHandoffPublication(context.Background(), stateRoot, record, "github", record.Branch, record.BranchPrepare.BaseBranch, reader, client); err == nil || !strings.Contains(err.Error(), "receipt") {
		t.Fatalf("missing receipt error = %v", err)
	}
}

func TestAcceptedHandoffPublicationRejectsAnyPossibleWriterAndDispatchedAssignment(t *testing.T) {
	for _, tt := range []struct {
		name  string
		setup func(IssueOpsRecord, *dispatchOrcaFake)
	}{
		{name: "connected only terminal", setup: func(record IssueOpsRecord, client *dispatchOrcaFake) {
			client.terminals = []port.OrcaTerminal{{Handle: "term-other", PTYID: "pty-other", WorktreeID: record.ExecutionHandoff.Orca.WorktreeID, WorktreePath: record.ExecutionHandoff.WorkerRoot, Connected: true}}
		}},
		{name: "writable only terminal", setup: func(record IssueOpsRecord, client *dispatchOrcaFake) {
			client.terminals = []port.OrcaTerminal{{Handle: "term-other", PTYID: "pty-other", WorktreeID: record.ExecutionHandoff.Orca.WorktreeID, WorktreePath: record.ExecutionHandoff.WorkerRoot, Writable: true}}
		}},
		{name: "dispatched persisted terminal", setup: func(record IssueOpsRecord, client *dispatchOrcaFake) {
			client.dispatchedTasks = []port.OrcaTask{{ID: "task-other", Status: "dispatched"}}
			client.dispatchByTask = map[string]port.OrcaDispatch{"task-other": {ID: "dispatch-other", TaskID: "task-other", AssigneeHandle: record.ExecutionHandoff.Orca.WorkerTerminalHandle, Status: "dispatched"}}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			stateRoot, record := acceptedPublicationHandoff(t, "github")
			client := handoffDispatchFake(record)
			client.terminals = nil
			tt.setup(record, client)
			finalHead := record.ExecutionHandoff.Result.FinalHead
			reader := &publicationRefFake{localHead: finalHead, remoteHead: finalHead}
			if _, err := RecordIssueOpsHandoffPublishReceipt(context.Background(), stateRoot, IssueOpsHandoffPublishRequest{ID: record.ID, Confirm: true}, reader, client, handoffPrepareTestClock()); err == nil {
				t.Fatal("possible writer authorized publication")
			}
			if reader.pushCalls != 0 {
				t.Fatalf("possible writer crossed push boundary: calls=%d", reader.pushCalls)
			}
			persisted, err := ReadIssueOps(stateRoot, record.ID)
			if err != nil {
				t.Fatal(err)
			}
			if persisted.ExecutionHandoff.PublicationRecovery == nil || persisted.ExecutionHandoff.PublicationRecovery.Code != "publication_writer_conflict" {
				t.Fatalf("known writer conflict was not persisted: %#v", persisted.ExecutionHandoff.PublicationRecovery)
			}
		})
	}
}

func acceptedPublicationHandoff(t *testing.T, provider string) (string, IssueOpsRecord) {
	t.Helper()
	if provider == "github" {
		stateRoot, record, claim, finish, _ := submittedGitHandoff(t, ".agent-harness/research/report.md", true)
		accepted, err := AcceptIssueOpsHandoff(stateRoot, IssueOpsHandoffAcceptRequest{
			ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch,
			ContextSHA256: claim.ContextSHA256, FinalHead: finish.FinalHead,
		})
		if err != nil {
			t.Fatal(err)
		}
		return stateRoot, accepted
	}

	stateRoot, preparing := handoffDispatchRecord(t)
	preparing.IssueURL = "https://gitlab.example/acme/repo/-/issues/16"
	preparing.BranchPrepare.Provider = "gitlab"
	preparing.BranchPrepare.IssueURL = preparing.IssueURL
	preparing.ExecutionHandoff.Orca.ProviderIssueLinkStatus = "gitlab_native_unavailable"
	preparing, err := WriteIssueOps(stateRoot, preparing)
	if err != nil {
		t.Fatal(err)
	}
	client := handoffDispatchFake(preparing)
	if _, err := StartIssueOpsHandoff(context.Background(), stateRoot, attestedCodexStart(t, stateRoot, preparing.ID), client, handoffStartTestClock()); err != nil {
		t.Fatal(err)
	}
	record, err := ReadIssueOps(stateRoot, preparing.ID)
	if err != nil {
		t.Fatal(err)
	}
	claim := handoffClaimRequest(record)
	claimed, err := ClaimIssueOpsHandoff(stateRoot, claim)
	if err != nil {
		t.Fatal(err)
	}
	writeIssueOpsFile(t, record.WorktreePath, "internal/demo.go", "package internal\n")
	report := filepath.Join(record.WorktreePath, ".agent-harness", "research", "report.md")
	if err := os.MkdirAll(filepath.Dir(report), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(report, []byte("# Turing evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", "test: worker result"}} {
		if code, _, stderr := preflight.GitCmd(record.WorktreePath, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	finish := handoffFinishRequest(claim, claimed)
	finish.FinalHead = strings.TrimSpace(preflight.GitOut(record.WorktreePath, "rev-parse", "HEAD"))
	finish.TuringReportPath = ".agent-harness/research/report.md"
	finish.ChangedFiles = []string{"internal/demo.go", ".agent-harness/research/report.md"}
	if _, err := finishIssueOpsHandoffWithoutProjection(stateRoot, finish); err != nil {
		t.Fatal(err)
	}
	accepted, err := AcceptIssueOpsHandoff(stateRoot, IssueOpsHandoffAcceptRequest{
		ID: record.ID, Attempt: claim.Attempt, OwnershipEpoch: claim.OwnershipEpoch,
		ContextSHA256: claim.ContextSHA256, FinalHead: finish.FinalHead,
	})
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, accepted
}

func otherProvider(provider string) string {
	if provider == "github" {
		return "gitlab"
	}
	return "github"
}
