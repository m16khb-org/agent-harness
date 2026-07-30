package issueops

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/port"
)

func TestExecutionIssueSnapshotEvidenceSealsPrepareAndClaimWithoutFallback(t *testing.T) {
	stateRoot, record := gitLabExecutionSnapshotRecord(t)
	evidence := validGitLabExecutionSnapshotEvidence()
	fallbackCalls := 0
	fallback := func(context.Context, string, port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
		fallbackCalls++
		return port.ExecutionIssueSnapshot{}, errors.New("must not run")
	}

	preparedAny, err := ExecuteExecution(context.Background(), stateRoot, ExecutionActionRequest{
		Action: ExecutionActionPrepare, ID: record.ID, Mode: "orca", CWD: record.Repo,
		Actor: executionActor("codex", "snapshot-coordinator"), OwnerHost: "claude", OwnerModel: "caller-model",
		Confirm: true, IssueSnapshot: evidence,
	}, ExecutionActionDependencies{Orca: readyOrcaFake(), ReadIssue: fallback})
	if err != nil {
		t.Fatal(err)
	}
	prepared, ok := preparedAny.(ExecutionPrepareResult)
	if !ok {
		t.Fatalf("prepare result type = %T", preparedAny)
	}
	if prepared.IssueSnapshotSource != "glab_mcp" {
		t.Fatalf("prepare snapshot source = %q", prepared.IssueSnapshotSource)
	}
	if prepared.IssueBodySHA256 != digestExecutionOwnerBytes([]byte(evidence.Body)) {
		t.Fatalf("prepare sealed a different issue body: %q", prepared.IssueBodySHA256)
	}
	if fallbackCalls != 0 {
		t.Fatalf("valid supplied evidence called fallback %d times", fallbackCalls)
	}

	claimedAny, err := ExecuteExecution(context.Background(), stateRoot, ExecutionActionRequest{
		Action: ExecutionActionClaim, ID: record.ID, Generation: 1,
		Actor: executionActor("claude", "snapshot-owner"), CWD: prepared.Workspace.Root,
		TokenFile: prepared.ClaimTokenPath, IssueBodySHA256: prepared.IssueBodySHA256,
		ContextPacketSHA256: prepared.ContextPacketSHA256, IssueSnapshot: evidence,
	}, ExecutionActionDependencies{ReadIssue: fallback, Claim: claimViaVerticalHandler})
	if err != nil {
		t.Fatal(err)
	}
	claimed, ok := claimedAny.(ExecutionResult)
	if !ok {
		t.Fatalf("claim result type = %T", claimedAny)
	}
	if claimed.IssueSnapshotSource != "glab_mcp" {
		t.Fatalf("claim snapshot source = %q", claimed.IssueSnapshotSource)
	}
	if fallbackCalls != 0 {
		t.Fatalf("valid supplied evidence called fallback during claim: %d", fallbackCalls)
	}
}

func TestExecutionIssueSnapshotEvidencePreviewReportsOnlyValidatedSource(t *testing.T) {
	stateRoot, record := gitLabExecutionSnapshotRecord(t)
	fallbackCalls := 0
	gotAny, err := ExecuteExecution(context.Background(), stateRoot, ExecutionActionRequest{
		Action: ExecutionActionPrepare, ID: record.ID, Mode: "orca", CWD: record.Repo,
		OwnerHost: "claude", Confirm: false, IssueSnapshot: validGitLabExecutionSnapshotEvidence(),
	}, ExecutionActionDependencies{
		Orca: readyOrcaFake(),
		ReadIssue: func(context.Context, string, port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
			fallbackCalls++
			return port.ExecutionIssueSnapshot{}, errors.New("must not run")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := gotAny.(ExecutionPrepareResult)
	if !got.Preview || got.IssueSnapshotSource != "glab_mcp" || got.IssueBodySHA256 != "" {
		t.Fatalf("preview source/seal = %#v", got)
	}
	if fallbackCalls != 0 {
		t.Fatalf("preview called fallback %d times", fallbackCalls)
	}
}

func TestExecutionIssueSnapshotEvidenceRejectsDriftBeforeFallback(t *testing.T) {
	tests := map[string]func(*port.ExecutionIssueSnapshotEvidence){
		"authority": func(e *port.ExecutionIssueSnapshotEvidence) {
			e.WebURL = "https://other.example.com/acme/repo/-/issues/16"
		},
		"port": func(e *port.ExecutionIssueSnapshotEvidence) {
			e.WebURL = "https://gitlab.example.com:8443/acme/repo/-/issues/16"
		},
		"project": func(e *port.ExecutionIssueSnapshotEvidence) {
			e.WebURL = "https://gitlab.example.com/acme/other/-/issues/16"
		},
		"iid": func(e *port.ExecutionIssueSnapshotEvidence) {
			e.WebURL = "https://gitlab.example.com/acme/repo/-/issues/17"
		},
		"provider": func(e *port.ExecutionIssueSnapshotEvidence) {
			e.Provider = "github"
		},
		"source": func(e *port.ExecutionIssueSnapshotEvidence) {
			e.Source = "personal_wrapper"
		},
		"state": func(e *port.ExecutionIssueSnapshotEvidence) {
			e.State = "unknown"
		},
		"empty_body": func(e *port.ExecutionIssueSnapshotEvidence) {
			e.Body = " "
		},
		"oversized_body": func(e *port.ExecutionIssueSnapshotEvidence) {
			e.Body = strings.Repeat("x", (1<<19)+1)
		},
		"http_scheme": func(e *port.ExecutionIssueSnapshotEvidence) {
			e.WebURL = "http://gitlab.example.com/acme/repo/-/issues/16"
		},
		"userinfo": func(e *port.ExecutionIssueSnapshotEvidence) {
			e.WebURL = "https://user@gitlab.example.com/acme/repo/-/issues/16"
		},
		"query": func(e *port.ExecutionIssueSnapshotEvidence) {
			e.WebURL += "?view=full"
		},
		"fragment": func(e *port.ExecutionIssueSnapshotEvidence) {
			e.WebURL += "#note"
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			stateRoot, record := gitLabExecutionSnapshotRecord(t)
			evidence := validGitLabExecutionSnapshotEvidence()
			mutate(evidence)
			fallbackCalls := 0
			_, err := ExecuteExecution(context.Background(), stateRoot, ExecutionActionRequest{
				Action: ExecutionActionPrepare, ID: record.ID, Mode: "orca", CWD: record.Repo,
				OwnerHost: "claude", Confirm: false, IssueSnapshot: evidence,
			}, ExecutionActionDependencies{
				Orca: readyOrcaFake(),
				ReadIssue: func(context.Context, string, port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
					fallbackCalls++
					return port.ExecutionIssueSnapshot{}, nil
				},
			})
			if err == nil {
				t.Fatal("drifted snapshot evidence was accepted")
			}
			if fallbackCalls != 0 {
				t.Fatalf("invalid supplied evidence called fallback %d times", fallbackCalls)
			}
		})
	}
}

func TestExecutionIssueSnapshotEvidenceSealsFinalizeAndReplacementClaimWithoutFallback(t *testing.T) {
	issueBody := "## acceptance criteria\n\n- [ ] AC-01: first\n\n## 검증 명령\n\n```bash\ngo test ./internal/core/issueops -count=1\n```\n"
	fixture := newRevokingSealedOrcaCycle(t, issueBody)
	current, err := ReadIssueOps(fixture.stateRoot, fixture.record.ID)
	if err != nil {
		t.Fatal(err)
	}
	current.IssueURL = "https://gitlab.example.com/acme/repo/-/work_items/16"
	current.BranchPrepare.Provider = "gitlab"
	current.BranchPrepare.IssueURL = current.IssueURL
	if _, err := writeIssueOps(fixture.stateRoot, current); err != nil {
		t.Fatal(err)
	}
	evidence := validGitLabExecutionSnapshotEvidence()

	raw, err := ExecuteExecution(context.Background(), fixture.stateRoot, ExecutionActionRequest{
		Action: ExecutionActionReplace, ID: fixture.record.ID,
		ReplaceAction: ExecutionReplaceFinalize, ExpectedGeneration: 2,
		QuiescenceFingerprint: fixture.preview.QuiescenceFingerprint,
		Actor:                 fixture.requester, CWD: fixture.prepared.Workspace.Root, Confirm: true,
		IssueSnapshot: evidence,
	}, ExecutionActionDependencies{OrcaOwner: fixture.deps.OrcaOwner})
	if err != nil {
		t.Fatalf("주입된 GitLab snapshot으로 finalize하지 못했다: %v", err)
	}
	finalized, ok := raw.(ExecutionReplaceResult)
	if !ok || finalized.IssueSnapshotSource != "glab_mcp" ||
		strings.TrimSpace(finalized.ContextPacketSHA256) == "" ||
		finalized.IssueBodySHA256 != digestOwnerFixture([]byte(evidence.Body)) {
		t.Fatalf("finalize가 검증된 snapshot 봉인 증거를 반환하지 않았다: %#v", raw)
	}

	raw, err = ExecuteExecution(context.Background(), fixture.stateRoot, ExecutionActionRequest{
		Action: ExecutionActionClaim, ID: fixture.record.ID, Generation: 2,
		Actor: executionActor("claude", "gitlab-replacement-owner"),
		CWD:   fixture.prepared.Workspace.Root, TokenFile: finalized.ClaimTokenPath,
		IssueBodySHA256: finalized.IssueBodySHA256, ContextPacketSHA256: finalized.ContextPacketSHA256,
		IssueSnapshot: evidence,
	}, ExecutionActionDependencies{Claim: claimViaVerticalHandler})
	if err != nil {
		t.Fatalf("finalize가 봉인한 GitLab snapshot으로 claim하지 못했다: %v", err)
	}
	claimed, ok := raw.(ExecutionResult)
	if !ok || claimed.Execution.Lease.Status != model.LeaseStatusActive || claimed.IssueSnapshotSource != "glab_mcp" {
		t.Fatalf("주입 snapshot claim 결과가 불완전하다: %#v", raw)
	}
}

func TestExecutionIssueSnapshotEvidenceRejectsUnsupportedActions(t *testing.T) {
	stateRoot, record := gitLabExecutionSnapshotRecord(t)
	evidence := validGitLabExecutionSnapshotEvidence()
	for _, req := range []ExecutionActionRequest{
		{Action: ExecutionActionStatus, ID: record.ID},
		{Action: ExecutionActionRelease, ID: record.ID},
		{Action: ExecutionActionResume, ID: record.ID},
		{Action: ExecutionActionComplete, ID: record.ID},
		{Action: ExecutionActionReplace, ID: record.ID, ReplaceAction: ExecutionReplacePreview},
		{Action: ExecutionActionReconcile, ID: record.ID, Preview: true},
		{Action: ExecutionActionReconcile, ID: record.ID, Confirm: true},
	} {
		t.Run(req.Action+"/"+req.ReplaceAction, func(t *testing.T) {
			req.IssueSnapshot = evidence
			if _, err := ExecuteExecution(context.Background(), stateRoot, req, ExecutionActionDependencies{}); err == nil || !strings.Contains(err.Error(), "issue_snapshot") {
				t.Fatalf("unsupported snapshot action was not rejected: %v", err)
			}
		})
	}
}

func TestExecutionIssueSnapshotEvidenceAllowsOnlyWorktreeReconcileConfirm(t *testing.T) {
	_, record := gitLabExecutionSnapshotRecord(t)
	record.Execution = &model.Execution{
		Mode:    model.ExecutionModeOrca,
		Pending: &model.ExternalIntent{Kind: "worktree_create"},
	}
	req := ExecutionActionRequest{Action: ExecutionActionReconcile, Confirm: true}
	if err := validateExecutionIssueSnapshotAction(req); err != nil {
		t.Fatal(err)
	}
	if err := validateExecutionIssueSnapshotRecord(req, record); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"owner_launch", "dispatch"} {
		record.Execution.Pending.Kind = kind
		if err := validateExecutionIssueSnapshotRecord(req, record); err == nil {
			t.Fatalf("reconcile stage %q accepted unused issue_snapshot", kind)
		}
	}
}

func TestExecutionIssueSnapshotFallbackIsValidatedAndObserved(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		stateRoot, record := gitLabExecutionSnapshotRecord(t)
		fallbackCalls := 0
		gotAny, err := ExecuteExecution(context.Background(), stateRoot, ExecutionActionRequest{
			Action: ExecutionActionPrepare, ID: record.ID, Mode: "orca", CWD: record.Repo,
			Actor: executionActor("codex", "cli-coordinator"), OwnerHost: "claude", OwnerModel: "caller-model", Confirm: true,
		}, ExecutionActionDependencies{
			Orca: readyOrcaFake(),
			ReadIssue: func(context.Context, string, port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
				fallbackCalls++
				return port.ExecutionIssueSnapshot{
					URL:   "https://gitlab.example.com/acme/repo/-/issues/16",
					Body:  validGitLabExecutionSnapshotEvidence().Body,
					State: "opened",
				}, nil
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		got := gotAny.(ExecutionPrepareResult)
		if got.IssueSnapshotSource != "glab_cli" || fallbackCalls == 0 {
			t.Fatalf("fallback source/calls = %q/%d", got.IssueSnapshotSource, fallbackCalls)
		}
	})

	t.Run("transport_error", func(t *testing.T) {
		stateRoot, record := gitLabExecutionSnapshotRecord(t)
		orca := readyOrcaFake()
		_, err := ExecuteExecution(context.Background(), stateRoot, ExecutionActionRequest{
			Action: ExecutionActionPrepare, ID: record.ID, Mode: "orca", CWD: record.Repo,
			Actor: executionActor("codex", "cli-error"), OwnerHost: "claude", Confirm: true,
		}, ExecutionActionDependencies{
			Orca: orca,
			ReadIssue: func(context.Context, string, port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error) {
				return port.ExecutionIssueSnapshot{}, errors.New("credential unavailable")
			},
		})
		if err == nil || !strings.Contains(err.Error(), "gitlab_issue_snapshot_unavailable") {
			t.Fatalf("fallback transport error = %v", err)
		}
		if orca.prepareCalls != 0 {
			t.Fatalf("snapshot transport failure happened after Orca mutation: %d", orca.prepareCalls)
		}
	})
}

func TestWithExecutionIssueSnapshotSourceCoversEveryExecutionResult(t *testing.T) {
	if got := withExecutionIssueSnapshotSource(ExecutionPrepareResult{}, "glab_mcp").(ExecutionPrepareResult); got.IssueSnapshotSource != "glab_mcp" {
		t.Fatalf("prepare source = %q", got.IssueSnapshotSource)
	}
	if got := withExecutionIssueSnapshotSource(ExecutionResult{}, "glab_mcp").(ExecutionResult); got.IssueSnapshotSource != "glab_mcp" {
		t.Fatalf("execution source = %q", got.IssueSnapshotSource)
	}
	if got := withExecutionIssueSnapshotSource(ExecutionReplaceResult{}, "glab_mcp").(ExecutionReplaceResult); got.IssueSnapshotSource != "glab_mcp" {
		t.Fatalf("replace source = %q", got.IssueSnapshotSource)
	}
	if got := withExecutionIssueSnapshotSource(ExecutionReconcileResult{}, "glab_mcp").(ExecutionReconcileResult); got.IssueSnapshotSource != "glab_mcp" {
		t.Fatalf("reconcile source = %q", got.IssueSnapshotSource)
	}
}

func gitLabExecutionSnapshotRecord(t *testing.T) (string, IssueOpsRecord) {
	t.Helper()
	stateRoot, record := orcaPrepareRecord(t)
	record.IssueURL = "https://gitlab.example.com/acme/repo/-/work_items/16"
	record.BranchPrepare.Provider = "gitlab"
	record.BranchPrepare.IssueURL = record.IssueURL
	written, err := writeIssueOps(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, written
}

func validGitLabExecutionSnapshotEvidence() *port.ExecutionIssueSnapshotEvidence {
	return &port.ExecutionIssueSnapshotEvidence{
		Provider: "gitlab",
		Source:   "glab_mcp",
		WebURL:   "https://gitlab.example.com/acme/repo/-/issues/16",
		Body:     "## acceptance criteria\n\n- [ ] AC-16: exact identity\n\n## 검증 명령\n\n```bash\ngo test ./internal/core/issueops -run ExecutionIssueSnapshot -count=1\n```\n",
		State:    "opened",
	}
}
