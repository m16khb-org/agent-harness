package lifecycle

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"agent-harness/internal/core/commandparse"
	"agent-harness/internal/core/issueops/handoff"
	issueopsmodel "agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/preflight"
	"agent-harness/internal/core/sqlstore"
)

func TestShellGuidanceQuoteAlwaysProducesOneLiteralArgvValue(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "pwned")
	values := []string{
		"", "plain", "a;touch " + sentinel, "$(touch " + sentinel + ")", "`touch " + sentinel + "`", "*.go", "line1\nline2", "single'quote", "$HOME",
	}
	for _, value := range values {
		quoted := shellGuidanceQuote(value)
		if !strings.HasPrefix(quoted, "'") || !strings.HasSuffix(quoted, "'") {
			t.Fatalf("dynamic argv is not unconditionally single-quoted: %q", quoted)
		}
		output, err := exec.Command("sh", "-c", "printf '%s' "+quoted).Output()
		if err != nil {
			t.Fatalf("quoted argv did not parse: value=%q quote=%q err=%v", value, quoted, err)
		}
		if string(output) != value {
			t.Fatalf("quoted argv round trip = %q, want %q", output, value)
		}
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatalf("guidance quoting executed injected shell content: %v", err)
	}
}

func TestHandoffGuardBlocksIdentifiableFutureAndInvalidRawEnvelope(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(*issueopsmodel.IssueOpsExecutionHandoff)
	}{
		{name: "future protocol", mutate: func(h *issueopsmodel.IssueOpsExecutionHandoff) { h.ProtocolVersion = handoff.ProtocolVersion + 1 }},
		{name: "invalid state", mutate: func(h *issueopsmodel.IssueOpsExecutionHandoff) { h.State = "future_state" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
			tt.mutate(record.ExecutionHandoff)
			putRawLifecycleIssueOpsRecord(t, record)
			req := handoffEditRequest(record, worktree, "codex", "session-1", filepath.Join(worktree, "internal", "x.go"))
			got := BuildLifecyclePreToolUseDecision(req)
			if got.Decision != "block" || !strings.Contains(got.Reason, "invalid supervised IssueOps handoff envelope") {
				t.Fatalf("identifiable invalid envelope must fail closed: %#v", got)
			}
		})
	}
}

func TestHandoffGuardInvalidEnvelopeDiagnosticIsBoundedAndRedacted(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	secret := "super-secret-token"
	apiSecret := "super-secret-value"
	record.ExecutionHandoff.State = "Authorization: Bearer " + secret + strings.Repeat("x", 16*1024)
	record.ExecutionHandoff.PendingOperation = &issueopsmodel.IssueOpsExecutionHandoffPendingOperation{Kind: "api_key=" + apiSecret + strings.Repeat("y", 16*1024)}
	putRawLifecycleIssueOpsRecord(t, record)
	req := handoffEditRequest(record, worktree, "codex", "session-1", filepath.Join(worktree, "internal", "x.go"))
	got := BuildLifecyclePreToolUseDecision(req)
	if got.Decision != "block" || strings.Contains(got.Reason, secret) || strings.Contains(got.Reason, apiSecret) || len(got.Reason) > 768 {
		t.Fatalf("invalid envelope reason must be bounded and redacted: decision=%s len=%d", got.Decision, len(got.Reason))
	}
}

func TestSessionStartInvalidEnvelopeNeverRendersWorkerLifecycleCommand(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateDispatched)
	record.ExecutionHandoff.ProtocolVersion = handoff.ProtocolVersion + 1
	putRawLifecycleIssueOpsRecord(t, record)
	guidance := BuildIssueOpsHandoffSessionGuidance(worktree, "codex", "session-1", "worker-1")
	if guidance == "" || !strings.Contains(strings.ToLower(guidance), "remain read-only") || !strings.Contains(strings.ToLower(guidance), "coordinator recovery") {
		t.Fatalf("invalid SessionStart guidance is not fail-closed: %q", guidance)
	}
	for _, forbidden := range []string{"handoff claim", "handoff heartbeat", "handoff finish"} {
		if strings.Contains(guidance, forbidden) {
			t.Fatalf("invalid envelope guidance rendered %q command: %s", forbidden, guidance)
		}
	}
	if len(guidance) > 768 {
		t.Fatalf("invalid envelope guidance exceeded bound: %d", len(guidance))
	}
}

func TestSessionStartGuidanceSelectsExactWorkerAndFailsClosedOnSourceAmbiguity(t *testing.T) {
	repo, first, _ := lifecycleHandoffRecord(t, handoff.StateDispatched)
	second := first
	second.ID = "io-second-cycle"
	second.Branch = "2-demo"
	second.WorktreePath = makeIssueOpsGuardWorktreeForTest(t, repo, second.Branch)
	second.ExecutionHandoff = cloneLifecycleHandoffForTest(t, first.ExecutionHandoff)
	second.ExecutionHandoff.WorkerRoot = second.WorktreePath
	second.ExecutionHandoff.OwnershipEpoch = "epoch-2"
	second.ExecutionHandoff.Orca.WorktreeID = "wt-2"
	second.ExecutionHandoff.Orca.WorktreePath = second.WorktreePath
	second.ExecutionHandoff.Orca.TaskID = "task-2"
	second.ExecutionHandoff.Orca.DispatchID = "dispatch-2"
	if _, err := writeIssueOps(IssueOpsStateRoot(), second); err != nil {
		t.Fatal(err)
	}

	workerGuidance := BuildIssueOpsHandoffSessionGuidance(second.WorktreePath, "codex", "session-2", "worker-2")
	if !strings.Contains(workerGuidance, "handoff claim") || !strings.Contains(workerGuidance, second.ID) || strings.Contains(workerGuidance, first.ID) {
		t.Fatalf("exact second worker root did not select its cycle: %s", workerGuidance)
	}

	sourceGuidance := BuildIssueOpsHandoffSessionGuidance(repo, "codex", "coordinator", "")
	for _, want := range []string{"multiple", first.ID, second.ID, "status", "resume", "--id"} {
		if !strings.Contains(strings.ToLower(sourceGuidance), strings.ToLower(want)) {
			t.Fatalf("ambiguous source guidance missing %q: %s", want, sourceGuidance)
		}
	}
	for _, forbidden := range []string{"handoff claim", "handoff start"} {
		if strings.Contains(sourceGuidance, forbidden) {
			t.Fatalf("ambiguous source guidance rendered %q: %s", forbidden, sourceGuidance)
		}
	}
	if len(sourceGuidance) > 1024 {
		t.Fatalf("ambiguous source guidance exceeded bound: %d", len(sourceGuidance))
	}
}

func TestSessionStartGuidanceSelectsOnlyCoordinatorPreparingSourceCycle(t *testing.T) {
	_, dispatchedTemplate, _ := lifecycleHandoffRecord(t, handoff.StateDispatched)
	repo, preparing, _ := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	dispatched := preparing
	dispatched.ID = "io-dispatched-cycle"
	dispatched.Branch = "2-dispatched"
	dispatched.WorktreePath = makeIssueOpsGuardWorktreeForTest(t, repo, dispatched.Branch)
	dispatched.ExecutionHandoff = cloneLifecycleHandoffForTest(t, dispatchedTemplate.ExecutionHandoff)
	dispatched.ExecutionHandoff.CoordinatorRoot = repo
	dispatched.ExecutionHandoff.WorkerRoot = dispatched.WorktreePath
	dispatched.ExecutionHandoff.OwnershipEpoch = "epoch-dispatched"
	dispatched.ExecutionHandoff.Orca.WorktreeID = "wt-dispatched"
	dispatched.ExecutionHandoff.Orca.WorktreePath = dispatched.WorktreePath
	dispatched.ExecutionHandoff.Orca.TaskID = "task-dispatched"
	dispatched.ExecutionHandoff.Orca.DispatchID = "dispatch-dispatched"
	if _, err := writeIssueOps(IssueOpsStateRoot(), dispatched); err != nil {
		t.Fatal(err)
	}

	guidance := BuildIssueOpsHandoffSessionGuidance(repo, "codex", "coordinator", "agent-1")
	for _, want := range []string{"role=coordinator", "handoff start", preparing.ID, "--coordinator-session-id 'coordinator'"} {
		if !strings.Contains(guidance, want) {
			t.Fatalf("unique coordinator_preparing source guidance missing %q: %s", want, guidance)
		}
	}
	if strings.Contains(guidance, dispatched.ID) || strings.Contains(strings.ToLower(guidance), "multiple active") {
		t.Fatalf("non-preparing cycle kept coordinator dispatch unreachable: %s", guidance)
	}

	secondPreparing := dispatched
	secondPreparing.ID = "io-second-preparing"
	secondPreparing.Branch = "3-preparing"
	secondPreparing.WorktreePath = makeIssueOpsGuardWorktreeForTest(t, repo, secondPreparing.Branch)
	secondPreparing.ExecutionHandoff = cloneLifecycleHandoffForTest(t, preparing.ExecutionHandoff)
	secondPreparing.ExecutionHandoff.WorkerRoot = secondPreparing.WorktreePath
	secondPreparing.ExecutionHandoff.OwnershipEpoch = "epoch-second-preparing"
	secondPreparing.ExecutionHandoff.Orca.WorktreeID = "wt-second-preparing"
	secondPreparing.ExecutionHandoff.Orca.WorktreePath = secondPreparing.WorktreePath
	if _, err := writeIssueOps(IssueOpsStateRoot(), secondPreparing); err != nil {
		t.Fatal(err)
	}

	ambiguous := BuildIssueOpsHandoffSessionGuidance(repo, "codex", "coordinator", "agent-1")
	if !strings.Contains(strings.ToLower(ambiguous), "multiple") || !strings.Contains(ambiguous, preparing.ID) || !strings.Contains(ambiguous, secondPreparing.ID) || strings.Contains(ambiguous, "handoff start") {
		t.Fatalf("multiple coordinator_preparing cycles must remain fail-closed: %s", ambiguous)
	}
}

func TestSessionStartWorkerGuidanceEmitsUsageModelUserDecisionBoundary(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateDispatched)
	guidance := strings.ToLower(BuildIssueOpsHandoffSessionGuidance(worktree, "codex", "session-1", "worker-1"))
	for _, want := range []string{"usage-limit", "rate-limit", "reset", "model-selection", "user-decision", "dismiss or stop", "never auto switch", "reset usage"} {
		if !strings.Contains(guidance, want) {
			t.Fatalf("SessionStart worker guidance missing %q: %s", want, guidance)
		}
	}
	if !strings.Contains(guidance, record.ID) {
		t.Fatalf("SessionStart worker guidance lost exact handoff identity: %s", guidance)
	}
}

func cloneLifecycleHandoffForTest(t *testing.T, source *issueopsmodel.IssueOpsExecutionHandoff) *issueopsmodel.IssueOpsExecutionHandoff {
	t.Helper()
	b, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	var cloned issueopsmodel.IssueOpsExecutionHandoff
	if err := json.Unmarshal(b, &cloned); err != nil {
		t.Fatal(err)
	}
	return &cloned
}

func putRawLifecycleIssueOpsRecord(t *testing.T, record IssueOpsRecord) {
	t.Helper()
	b, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlstore.Open(IssueOpsStateRoot())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put("issueops", record.ID, b); err != nil {
		t.Fatal(err)
	}
}

func TestHandoffGuardBlocksBeforeClaim(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateDispatched)
	got := BuildLifecyclePreToolUseDecision(handoffEditRequest(record, worktree, "codex", "session-1", filepath.Join(worktree, "internal", "x.go")))
	if got.Decision != "block" || !strings.Contains(got.Reason, "claim") {
		t.Fatalf("pre-claim mutation should block: %#v", got)
	}
}

func TestHandoffGuardAllowsMatchingClaimedWorkerInTree(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	got := BuildLifecyclePreToolUseDecision(handoffEditRequest(record, worktree, "codex", "session-1", filepath.Join(worktree, "internal", "x.go")))
	if got.Decision != "allow" {
		t.Fatalf("claimed worker in tree should pass: %#v", got)
	}
}

func TestHandoffCancellationTombstoneKeepsClaimedWorkerGuardClosed(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	record.ExecutionHandoff.State = handoff.StateRecoveryRequired
	record.ExecutionHandoff.Cancellation = &issueopsmodel.IssueOpsExecutionHandoffCancellation{RequestedAt: "2026-07-11T02:00:00Z", Reason: "verified stale worker"}
	record.ExecutionHandoff.Failure = &issueopsmodel.IssueOpsExecutionHandoffFailure{Code: "cancellation_requested", Message: "verified stale worker", At: "2026-07-11T02:00:00Z"}
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	got := BuildLifecyclePreToolUseDecision(handoffEditRequest(record, worktree, "codex", "session-1", filepath.Join(worktree, "internal", "x.go")))
	if got.Decision != "block" || !strings.Contains(got.Reason, "remain read-only") {
		t.Fatalf("cancellation tombstone released the claimed-worker mutation guard: %#v", got)
	}
	guidance := BuildIssueOpsHandoffSessionGuidance(worktree, "codex", "session-1", "worker-1")
	for _, forbidden := range []string{"handoff claim", "handoff heartbeat", "handoff finish"} {
		if strings.Contains(guidance, forbidden) {
			t.Fatalf("cancellation tombstone rendered worker command %q: %s", forbidden, guidance)
		}
	}
}

func TestHandoffGuardBlocksCoordinatorAbsolutePathIntoWorkerTree(t *testing.T) {
	repo, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	got := BuildLifecyclePreToolUseDecision(handoffEditRequest(record, repo, "codex", "coordinator", filepath.Join(worktree, "internal", "x.go")))
	if got.Decision != "block" {
		t.Fatalf("coordinator absolute-path edit should block: %#v", got)
	}
}

func TestHandoffGuardFailsClosedForRealFutureSchemaWrongSessionMutation(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	record.SchemaVersion = issueopsmodel.IssueOpsCurrentSchemaVersion + 1
	raw, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlstore.Open(IssueOpsStateRoot())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put("issueops", record.ID, raw); err != nil {
		t.Fatal(err)
	}
	before, ok, err := db.Get("issueops", record.ID)
	if err != nil || !ok {
		t.Fatalf("read future-schema fixture: ok=%v err=%v", ok, err)
	}

	req := handoffEditRequest(record, worktree, "codex", "wrong-session", filepath.Join(worktree, "internal", "x.go"))
	got := BuildLifecyclePreToolUseDecision(req)
	if got.Decision != "block" || !strings.Contains(got.Reason, "invalid supervised IssueOps") || !strings.Contains(got.Reason, "schema_version") {
		t.Fatalf("future-schema wrong-session mutation did not fail closed with ownership reason: %#v", got)
	}
	after, ok, err := db.Get("issueops", record.ID)
	if err != nil || !ok || string(after) != string(before) {
		t.Fatalf("read-only hook changed future-schema bytes: ok=%v err=%v", ok, err)
	}
}

func TestHandoffGuardAllowsOnlyCoordinatorPlanEditsBeforeDispatch(t *testing.T) {
	repo, record, worktree := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	plan := filepath.Join(worktree, "docs", "superpowers", "plans", "2026-07-11-demo.md")
	req := handoffEditRequest(record, repo, "codex", "coordinator", plan)
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("coordinator plan edit from source checkout should pass: %#v", got)
	}

	workerReq := handoffEditRequest(record, worktree, "codex", "unclaimed-worker", plan)
	if got := BuildLifecyclePreToolUseDecision(workerReq); got.Decision != "block" {
		t.Fatalf("unclaimed worker must not use the coordinator plan exception: %#v", got)
	}
	codeReq := handoffEditRequest(record, repo, "codex", "coordinator", filepath.Join(worktree, "internal", "x.go"))
	if got := BuildLifecyclePreToolUseDecision(codeReq); got.Decision != "block" {
		t.Fatalf("coordinator exception must not allow implementation edits: %#v", got)
	}
	mixedReq := req
	mixedReq.Paths = append(mixedReq.Paths, filepath.Join(worktree, "internal", "x.go"))
	if got := BuildLifecyclePreToolUseDecision(mixedReq); got.Decision != "block" {
		t.Fatalf("every target must be a planning path: %#v", got)
	}
	for _, target := range []string{
		filepath.Join(worktree, "internal", "plans", "rogue.md"),
		filepath.Join(worktree, "foo", "plans", "rogue.md"),
		filepath.Join(worktree, "docs", "superpowers", "plans", "rogue.txt"),
	} {
		invalid := handoffEditRequest(record, repo, "codex", "coordinator", target)
		if got := BuildLifecyclePreToolUseDecision(invalid); got.Decision != "block" {
			t.Fatalf("non-convention plan target %q must block: %#v", target, got)
		}
	}

	exact := filepath.Join(worktree, "custom", "linked-plan.md")
	record.PlanPath = exact
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	exactReq := handoffEditRequest(record, repo, "codex", "coordinator", exact)
	if got := BuildLifecyclePreToolUseDecision(exactReq); got.Decision != "allow" {
		t.Fatalf("exact linked plan path should pass: %#v", got)
	}
	otherPlan := handoffEditRequest(record, repo, "codex", "coordinator", plan)
	if got := BuildLifecyclePreToolUseDecision(otherPlan); got.Decision != "block" {
		t.Fatalf("linked plan must narrow the exception to the exact path: %#v", got)
	}
}

func TestHandoffGuardAllowsCycleNamedPlanCorrectionAndExactCoordinatorCommit(t *testing.T) {
	repo, record, worktree := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	legacyPlan := filepath.Join(worktree, "docs", "superpowers", "plans", "legacy.md")
	if err := os.MkdirAll(filepath.Dir(legacyPlan), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPlan, []byte("# unrelated legacy plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	record.PlanPath = legacyPlan
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	cyclePlan := filepath.Join(worktree, ".agent-harness", "plans", record.ID+"-live-e2e.md")
	if err := os.MkdirAll(filepath.Dir(cyclePlan), 0o755); err != nil {
		t.Fatal(err)
	}
	edit := handoffEditRequest(record, repo, "codex", "coordinator", cyclePlan)
	if got := BuildLifecyclePreToolUseDecision(edit); got.Decision != "allow" {
		t.Fatalf("cycle-named corrective plan edit should pass before context seal: %#v", got)
	}
	featureRoot := filepath.Join(filepath.Dir(worktree), "unrelated-feature-worktree")
	if err := os.MkdirAll(featureRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, roots := range []struct {
		name string
		cwd  string
		repo string
	}{
		{name: "feature cwd and repo", cwd: featureRoot, repo: featureRoot},
		{name: "feature cwd", cwd: featureRoot, repo: repo},
		{name: "feature repo", cwd: repo, repo: featureRoot},
	} {
		t.Run(roots.name, func(t *testing.T) {
			wrongRoot := handoffEditRequest(record, roots.cwd, "codex", "feature-session", cyclePlan)
			wrongRoot.Repo = roots.repo
			if got := BuildLifecyclePreToolUseDecision(wrongRoot); got.Decision != "block" {
				t.Fatalf("plan edit outside the source coordinator root must block: %#v", got)
			}
			if _, err := os.Lstat(cyclePlan); !os.IsNotExist(err) {
				t.Fatalf("blocked plan edit must not create the target, lstat err=%v", err)
			}
		})
	}

	for _, command := range []string{
		"git -C " + worktree + " add -- " + cyclePlan,
		"git -C " + worktree + " commit --only -m 'docs: record current cycle plan' -- " + cyclePlan,
	} {
		req := handoffEditRequest(record, repo, "codex", "coordinator", "")
		req.Tool = "Bash"
		req.Command = command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("exact coordinator plan command %q should pass: %#v", command, got)
		}
	}

	for _, command := range []string{
		"git -C " + worktree + " add -- " + legacyPlan + " internal/x.go",
		"git -C " + worktree + " commit -m 'missing only path fence'",
		"git -C " + repo + " add -- " + cyclePlan,
	} {
		req := handoffEditRequest(record, repo, "codex", "coordinator", "")
		req.Tool = "Bash"
		req.Command = command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
			t.Fatalf("unsafe coordinator plan command %q must block: %#v", command, got)
		}
	}
}

func TestHandoffGuardBlocksCoordinatorPlanSymlinkEscape(t *testing.T) {
	repo, record, worktree := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	outside := t.TempDir()
	planRoot := filepath.Join(worktree, "docs", "superpowers", "plans")
	if err := os.MkdirAll(filepath.Dir(planRoot), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, planRoot); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(planRoot, "escape.md")
	req := handoffEditRequest(record, repo, "codex", "coordinator", target)
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
		t.Fatalf("nonexistent plan leaf below an escaping symlink must block: %#v", got)
	}
	record.PlanPath = target
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
		t.Fatalf("exact linked plan path must still block when a component escapes: %#v", got)
	}
}

func TestHandoffGuardRestrictsPrepareToolsToCoordinatorSourceCheckout(t *testing.T) {
	repo, record, worktree := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	command := "agent-harness issueops worktree prepare-tools --id " + record.ID + " --json"
	coordinator := handoffEditRequest(record, repo, "codex", "coordinator", "")
	coordinator.Tool = "Bash"
	coordinator.Command = command
	if got := BuildLifecyclePreToolUseDecision(coordinator); got.Decision != "allow" {
		t.Fatalf("coordinator prepare-tools from source checkout should pass: %#v", got)
	}
	worker := handoffEditRequest(record, worktree, "codex", "unclaimed-worker", "")
	worker.Tool = "Bash"
	worker.Command = command
	if got := BuildLifecyclePreToolUseDecision(worker); got.Decision != "block" {
		t.Fatalf("unclaimed worker prepare-tools must be blocked: %#v", got)
	}
}

func TestHandoffGuardBlocksCommonMutationFamiliesBeforeOrOutsideClaim(t *testing.T) {
	mutating := []string{
		"git add .", "git commit -m test", "git reset --hard HEAD", "git restore internal/x.go",
		"git checkout -- internal/x.go", "git switch feature", "go mod tidy", "go generate ./...",
		"git -C /tmp/repo add .", "git -C=/tmp/repo commit -m test", "go -C /tmp/repo mod tidy", "go -C=/tmp/repo generate ./...",
		"npm install", "pnpm add example", "yarn remove example", "bun install", "cargo fmt",
		"bash -c 'touch internal/x.go'", "bash -lc 'touch internal/x.go'", "sh -ec 'touch internal/x.go'", "zsh -lc 'touch internal/x.go'", `python -c 'open("internal/x.go", "w").write("x")'`,
		`node -e 'require("fs").writeFileSync("internal/x.go", "x")'`,
		"rsync source.txt destination.txt", "install source.txt destination.txt", "truncate -s 0 internal/x.go",
		"dd if=/dev/null of=internal/x.go", "./scripts/custom-write.sh",
	}
	for _, state := range []string{handoff.StateDispatched, handoff.StateClaimed} {
		t.Run(state, func(t *testing.T) {
			_, record, worktree := lifecycleHandoffRecord(t, state)
			for _, tool := range []string{"Bash", "shell_command", "exec_command", "unified_exec"} {
				t.Run(tool, func(t *testing.T) {
					for _, command := range mutating {
						t.Run(command, func(t *testing.T) {
							req := handoffEditRequest(record, worktree, "codex", "wrong-session", "")
							req.Tool = tool
							req.Command = command
							if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
								t.Fatalf("unowned mutation command must block: %#v", got)
							}
						})
					}
				})
			}
		})
	}
}

func TestHandoffGuardKeepsRepresentativeReadOnlyCommandsAllowed(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateDispatched)
	readOnly := []string{
		"git status --short", "git diff --stat", "git log -1", "git show --stat HEAD", "git rev-parse HEAD",
		"rg -n handoff internal", "pwd",
		"agent-harness issueops status --id " + record.ID + " --json",
		"agent-harness issueops resume --repo " + record.Repo + " --id " + record.ID + " --json",
		"orca terminal list --json", "orca orchestration task-list --json",
	}
	for _, tool := range []string{"Bash", "shell_command", "exec_command", "unified_exec"} {
		t.Run(tool, func(t *testing.T) {
			for _, command := range readOnly {
				t.Run(command, func(t *testing.T) {
					req := handoffEditRequest(record, worktree, "codex", "unclaimed-worker", "")
					req.Tool = tool
					req.Command = command
					if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
						t.Fatalf("representative read-only command should pass: %#v", got)
					}
				})
			}
		})
	}
}

func TestHandoffGuardRunsGoTestsOnlyAfterExactWorkerClaim(t *testing.T) {
	for _, state := range []string{handoff.StateCoordinatorPreparing, handoff.StateDispatched} {
		_, record, worktree := lifecycleHandoffRecord(t, state)
		for _, command := range []string{"go test ./...", "go test ./... -run TestMainSentinel"} {
			req := handoffEditRequest(record, worktree, "codex", "unclaimed-worker", "")
			req.Tool, req.Command = "exec_command", command
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("%s may execute TestMain/init and must block before claim: %#v", command, got)
			}
		}
	}
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	req := handoffEditRequest(record, worktree, "codex", "session-1", "")
	req.Tool, req.Command = "exec_command", "go test ./... -run TestFocused"
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("claimed matching worker should run verification: %#v", got)
	}
}

func TestHandoffGuardRejectsActiveOutputRedirectsAcrossShellAliases(t *testing.T) {
	commands := []string{
		"rg handoff > target.txt",
		"git status >>target.txt",
		"go test ./... 1> target.txt",
		"pwd 2>target.txt",
		"orca orchestration task-list --json 2>>target.txt",
		"rg --files &> target.txt",
	}
	for _, state := range []string{handoff.StateDispatched, handoff.StateClaimed} {
		_, record, worktree := lifecycleHandoffRecord(t, state)
		for _, tool := range []string{"Bash", "shell_command", "exec_command", "unified_exec"} {
			for _, command := range commands {
				t.Run(state+"/"+tool+"/"+command, func(t *testing.T) {
					req := handoffEditRequest(record, worktree, "codex", "session-1", "")
					req.Tool, req.Command = tool, command
					if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
						t.Fatalf("active output redirect must block: %#v", got)
					}
				})
			}
		}
	}
}

func TestHandoffGuardRejectsActiveParameterAndTildeExpansionInEveryRole(t *testing.T) {
	commands := []string{`rm "$HOME/out"`, `rm "$TMPDIR/x"`, `touch ${TMPDIR}/x`, `touch ~/x`, `cd ~`}
	for _, state := range []string{handoff.StateCoordinatorPreparing, handoff.StateDispatched, handoff.StateClaimed} {
		repo, record, worktree := lifecycleHandoffRecord(t, state)
		cwd, session := worktree, "session-1"
		if state == handoff.StateCoordinatorPreparing {
			cwd, session = repo, "coordinator"
		}
		for _, command := range commands {
			req := handoffEditRequest(record, cwd, "codex", session, "")
			req.Tool, req.Command = "exec_command", command
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("%s active expansion %q must block: %#v", state, command, got)
			}
		}
	}
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	for _, command := range []string{
		"touch " + filepath.Join(worktree, "internal", "explicit.txt"),
		`touch '$HOME-literal'`, `touch \$HOME-literal`,
	} {
		req := handoffEditRequest(record, worktree, "codex", "session-1", "")
		req.Tool, req.Command = "exec_command", command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("explicit/literal worker path %q should pass: %#v", command, got)
		}
	}
}

func TestHandoffGuardRejectsActiveProcessSubstitution(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	for _, tool := range []string{"Bash", "shell_command", "exec_command", "unified_exec"} {
		for _, command := range []string{`diff <(git status) <(gh pr create --title x)`, `tool --input >(outside-command)`} {
			req := handoffEditRequest(record, worktree, "codex", "session-1", "")
			req.Tool, req.Command = tool, command
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("active process substitution must block: tool=%s command=%q got=%#v", tool, command, got)
			}
		}
		for _, command := range []string{`tool --input '<(literal)'`, `tool --input "<(literal)"`, `tool --input ">(literal)"`, `tool --input \<(literal)`} {
			req := handoffEditRequest(record, worktree, "codex", "session-1", "")
			req.Tool, req.Command = tool, command
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
				t.Fatalf("literal process-substitution syntax should pass: tool=%s command=%q got=%#v", tool, command, got)
			}
		}
	}
}

func TestHandoffReadOnlyRequiresBareTrustedExecutable(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateDispatched)
	binDir := t.TempDir()
	sentinel := filepath.Join(t.TempDir(), "executed")
	for _, name := range []string{"git", "rg", "go", "orca"} {
		path := filepath.Join(binDir, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\ntouch '"+sentinel+"'\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	commands := []string{
		"./rg --files",
		filepath.Join(binDir, "git") + " status --short",
		"./go test ./...",
		filepath.Join(binDir, "orca") + " orchestration task-list --json",
	}
	for _, command := range commands {
		req := handoffEditRequest(record, worktree, "codex", "unclaimed-worker", "")
		req.Tool, req.Command = "exec_command", command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
			t.Fatalf("shadow executable %q must not inherit read-only trust: %#v", command, got)
		}
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatalf("hook evaluation executed a shadow binary: %v", err)
	}
}

func TestClaimedWorkerRoleBlocksCoordinatorOwnedCommandsAndChecksBranch(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	blocked := []string{
		"git push origin HEAD", "git remote set-url origin https://example.invalid/repo.git",
		"git switch other", "git checkout other", "git branch -D other", "git reset --hard HEAD", "git worktree remove ../other",
		"gh pr create --title x --body y", "gh pr merge 1", "gh pr close 1",
		"glab mr create --title x", "glab mr merge 1", "glab mr close 1",
		"agent-harness issueops phase --id " + record.ID + " --to feedback",
		"agent-harness issueops handoff start --id " + record.ID + " --confirm",
		"agent-harness issueops handoff accept --id " + record.ID + " --attempt 1 --ownership-epoch epoch-1 --context-sha256 " + strings.Repeat("a", 64) + " --final-head deadbeef",
		"agent-harness issueops handoff recover --id " + record.ID + " --action cancel --confirm",
		"agent-harness issueops worktree prepare-tools --id " + record.ID,
		"agent-harness issueops unknown-side-effect --id " + record.ID,
		"orca worktree create --repo path:/tmp --name rogue --json",
		"orca orchestration task-create --spec rogue --task-title rogue --display-name rogue --json",
		"orca orchestration dispatch --task task-1 --to term-1 --inject --json",
		"orca terminal create --worktree id:wt-1 --command codex --json",
		"env git push origin HEAD", "./git push origin HEAD", "/tmp/gh pr create --title x",
	}
	for _, command := range blocked {
		req := handoffEditRequest(record, worktree, "codex", "session-1", "")
		req.Tool, req.Command = "shell_command", command
		got := BuildLifecyclePreToolUseDecision(req)
		if got.Decision != "block" || !strings.Contains(got.Reason, "worker") {
			t.Fatalf("coordinator-owned command %q must block with role guidance: %#v", command, got)
		}
	}
	for _, command := range []string{"git add .", "git commit -m local", "git status --short", "git diff --stat", "go test ./...", "go build ./..."} {
		req := handoffEditRequest(record, worktree, "codex", "session-1", "")
		req.Tool, req.Command = "shell_command", command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("worker implementation command %q should remain allowed: %#v", command, got)
		}
	}

	headPath := filepath.Join(worktree, ".git", "HEAD")
	for name, head := range map[string]string{
		"mismatch": "ref: refs/heads/other\n",
		"detached": "0123456789012345678901234567890123456789\n",
		"unknown":  "",
	} {
		t.Run(name, func(t *testing.T) {
			if head == "" {
				if err := os.Remove(headPath); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(headPath, []byte(head), 0o644); err != nil {
				t.Fatal(err)
			}
			got := BuildLifecyclePreToolUseDecision(handoffEditRequest(record, worktree, "codex", "session-1", filepath.Join(worktree, "internal", "x.go")))
			if got.Decision != "block" || !strings.Contains(got.Reason, "branch") {
				t.Fatalf("%s branch evidence must fail closed: %#v", name, got)
			}
			if err := os.WriteFile(headPath, []byte("ref: refs/heads/1-demo\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestHandoffGuardBlocksRawTerminalSteeringOutsideSourceCoordinator(t *testing.T) {
	repo, record, _ := lifecycleHandoffRecord(t, handoff.StateClaimed)
	featureRoot := filepath.Join(filepath.Dir(repo), "unrelated-feature-session")
	gitDir := filepath.Join(repo, ".git", "worktrees", "unrelated-feature-session")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/unrelated-feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(featureRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(featureRoot, ".git"), []byte("gitdir: "+gitDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(featureRoot, "terminal-steering-sentinel")
	commands := map[string]string{
		"send":          "orca terminal send --terminal term-1 --text 'touch " + sentinel + "' --enter --json",
		"stop":          "orca terminal stop --worktree id:wt-1 --json",
		"create":        "orca terminal create --worktree id:wt-1 --command codex --json",
		"switch":        "orca terminal switch --terminal term-1 --json",
		"focus":         "orca terminal focus --terminal term-1 --json",
		"close":         "orca terminal close --terminal term-1 --json",
		"rename":        "orca terminal rename --terminal term-1 --title worker --json",
		"split":         "orca terminal split --terminal term-1 --direction horizontal --json",
		"write alias":   "orca terminal write --terminal term-1 --text 'touch " + sentinel + "' --json",
		"input alias":   "orca terminal input --terminal term-1 --text 'touch " + sentinel + "' --json",
		"type alias":    "orca terminal type --terminal term-1 --text 'touch " + sentinel + "' --json",
		"paste alias":   "orca terminal paste --terminal term-1 --text 'touch " + sentinel + "' --json",
		"shadowed orca": "/tmp/orca terminal send --terminal term-1 --text 'touch " + sentinel + "' --enter --json",
	}
	for name, command := range commands {
		t.Run(name, func(t *testing.T) {
			req := handoffEditRequest(record, featureRoot, "codex", "feature-session", "")
			req.SourceCheckout = ""
			req.Tool, req.Command = "exec_command", command
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" || !strings.Contains(got.Reason, "terminal steering") {
				t.Fatalf("non-source terminal steering %q must block before execution: %#v", command, got)
			}
			if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
				t.Fatalf("blocked terminal steering must not create a sentinel: %v", err)
			}
		})
	}

	for _, tool := range []string{
		"mcp__orca__terminal_send", "mcp__orca__terminal_stop", "mcp__orca__terminal_create", "mcp__orca__terminal_switch",
		"mcp__orca__terminal_focus", "mcp__orca__terminal_close", "mcp__orca__terminal_rename", "mcp__orca__terminal_split",
		"terminal_write", "terminal_input", "terminal_type", "terminal_paste",
	} {
		req := handoffEditRequest(record, featureRoot, "codex", "feature-session", "")
		req.SourceCheckout = ""
		req.Tool, req.Command = tool, ""
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" || !strings.Contains(got.Reason, "terminal steering") {
			t.Fatalf("non-source terminal control tool %q must block before execution: %#v", tool, got)
		}
		if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
			t.Fatalf("blocked terminal control tool must not create a sentinel: %v", err)
		}
	}

	for _, command := range []string{
		"orca terminal list --worktree id:wt-1 --json",
		"orca terminal show --terminal term-1 --json",
		"orca terminal read --terminal term-1 --json",
		"orca terminal wait --terminal term-1 --for tui-idle --timeout-ms 100 --json",
	} {
		req := handoffEditRequest(record, featureRoot, "codex", "feature-session", "")
		req.SourceCheckout = ""
		req.Tool, req.Command = "exec_command", command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("installed read-only terminal command %q should remain allowed: %#v", command, got)
		}
	}
}

func TestHandoffGuardAllowsOnlyLiteralSafeClaimedWorkerSteeringFromSourceCoordinator(t *testing.T) {
	t.Run("claimed safe guidance", func(t *testing.T) {
		repo, record, _ := lifecycleHandoffRecord(t, handoff.StateClaimed)
		record.ExecutionHandoff.Orca.WorkerTerminalHandle = "term_live"
		var err error
		record, err = writeIssueOps(IssueOpsStateRoot(), record)
		if err != nil {
			t.Fatal(err)
		}
		req := handoffEditRequest(record, repo, "codex", "coordinator", "")
		req.Tool = "exec_command"
		req.Command = "orca terminal send --terminal term_live --text '# agent-harness guidance: retry the exact report patch once' --enter --json"
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("source coordinator literal-safe claimed-worker guidance should pass: %#v", got)
		}

		req.Command = "orca terminal send --terminal term_other --text '# agent-harness guidance: retry the exact report patch once' --enter --json"
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
			t.Fatalf("source coordinator guidance must bind the persisted worker terminal handle: %#v", got)
		}

		for _, command := range []string{
			"orca terminal list --worktree id:wt-1 --limit 32 --json",
			"orca terminal show --terminal term_live --json",
			"orca terminal read --terminal term_live --cursor 1 --limit 32 --json",
			"orca terminal wait --terminal term_live --for tui-idle --timeout-ms 100 --json",
		} {
			req.Command = command
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
				t.Fatalf("source coordinator read-only terminal command %q should pass: %#v", command, got)
			}
		}
	})

	t.Run("claimed guidance rejects terminal control bytes", func(t *testing.T) {
		repo, record, _ := lifecycleHandoffRecord(t, handoff.StateClaimed)
		record.ExecutionHandoff.Orca.WorkerTerminalHandle = "term_live"
		var err error
		record, err = writeIssueOps(IssueOpsStateRoot(), record)
		if err != nil {
			t.Fatal(err)
		}
		base := handoffEditRequest(record, repo, "codex", "coordinator", "")
		base.Tool = "exec_command"
		for name, control := range map[string]string{
			"backspace": "\b",
			"tab":       "\t",
			"escape":    "\x1b",
			"delete":    "\x7f",
		} {
			t.Run(name, func(t *testing.T) {
				req := base
				req.Command = "orca terminal send --terminal term_live --text '# agent-harness guidance: keep" + control + "going' --enter --json"
				if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
					t.Fatalf("decoded terminal control byte %q must block: %#v", control, got)
				}
			})
		}
		base.Command = "orca terminal send --terminal term_live --text '# agent-harness guidance: 정확한 보고서 패치를 한 번만 다시 시도하세요 ✅' --enter --json"
		if got := BuildLifecyclePreToolUseDecision(base); got.Decision != "allow" {
			t.Fatalf("ordinary Korean and Unicode guidance should pass: %#v", got)
		}
	})

	for _, state := range []string{handoff.StateCoordinatorPreparing, handoff.StateDispatched} {
		t.Run(state, func(t *testing.T) {
			repo, record, _ := lifecycleHandoffRecord(t, state)
			req := handoffEditRequest(record, repo, "codex", "coordinator", "")
			req.Tool = "exec_command"
			req.Command = "orca terminal send --terminal term_live --text '# agent-harness guidance: create or dispatch now' --enter --json"
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("%s must use issueops handoff start rather than raw terminal steering: %#v", state, got)
			}
		})
	}

	t.Run("claimed shell payload", func(t *testing.T) {
		repo, record, _ := lifecycleHandoffRecord(t, handoff.StateClaimed)
		req := handoffEditRequest(record, repo, "codex", "coordinator", "")
		req.Tool = "exec_command"
		req.Command = "orca terminal send --terminal term_live --text 'apply_patch internal/x.go' --enter --json"
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
			t.Fatalf("source coordinator must not inject an arbitrary terminal mutation: %#v", got)
		}
	})
}

func TestHandoffGuardEnforcesInstalledOrchestrationMessageTypes(t *testing.T) {
	repo, record, _ := lifecycleHandoffRecord(t, handoff.StateClaimed)
	accepted := []string{"status", "dispatch", "worker_done", "merge_ready", "escalation", "handoff", "decision_gate", "heartbeat"}
	req := handoffEditRequest(record, repo, "codex", "coordinator", "")
	req.Tool, req.Command = "exec_command", "orca orchestration send --to term-coordinator --type progress --subject update --json"
	got := BuildLifecyclePreToolUseDecision(req)
	if got.Decision != "block" {
		t.Fatalf("active-handoff invalid explicit message type must block before record authority: %#v", got)
	}
	for _, want := range accepted {
		if !strings.Contains(got.Reason, want) {
			t.Fatalf("block reason must list accepted type %q: %s", want, got.Reason)
		}
	}

	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	for _, messageType := range accepted {
		request := HookToolUseLifecycleRequest{
			Repo: t.TempDir(), CWD: t.TempDir(), Tool: "exec_command",
			Command: "orca orchestration send --to term-coordinator --type=" + messageType + " --subject update --json",
		}
		if got := BuildLifecyclePreToolUseDecision(request); got.Decision != "allow" {
			t.Fatalf("valid no-record type %q must fall through unchanged: %#v", messageType, got)
		}
	}
	request := HookToolUseLifecycleRequest{Repo: t.TempDir(), CWD: t.TempDir(), Tool: "exec_command", Command: "orca orchestration send --to term-coordinator --subject update --json"}
	if got := BuildLifecyclePreToolUseDecision(request); got.Decision != "allow" {
		t.Fatalf("no-type send must fall through unchanged: %#v", got)
	}
	request.Command = "orca orchestration send --to term-coordinator --type status --type progress --subject update --json"
	got = BuildLifecyclePreToolUseDecision(request)
	if got.Decision != "block" {
		t.Fatalf("duplicate explicit message type must block without a supervised record: %#v", got)
	}
	for _, want := range accepted {
		if !strings.Contains(got.Reason, want) {
			t.Fatalf("duplicate-type reason must list accepted type %q: %s", want, got.Reason)
		}
	}
}

func TestHandoffGuardBlocksExplicitHistoricalMailboxInjection(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	base := HookToolUseLifecycleRequest{Repo: t.TempDir(), CWD: t.TempDir(), Tool: "exec_command"}
	for name, command := range map[string]string{
		"implicit unread default": "orca orchestration check --inject --json",
		"unread then inject":      "orca orchestration check --unread --inject --json",
		"inject then unread":      "orca orchestration check --inject --unread --json",
		"all history":             "orca orchestration check --all --inject --json",
		"equals form":             "orca orchestration check --all --inject=true --json",
	} {
		t.Run(name, func(t *testing.T) {
			req := base
			req.Command = command
			got := BuildLifecyclePreToolUseDecision(req)
			if got.Decision != "block" || !strings.Contains(strings.ToLower(got.Reason), "mailbox") || !strings.Contains(got.Reason, "orca orchestration check --all --json") {
				t.Fatalf("explicit mailbox injection %q must block with safe inspection guidance: %#v", command, got)
			}
		})
	}
	for _, command := range []string{
		"orca orchestration check --all --json",
		"orca orchestration check --terminal term-current --all --json",
		"orca orchestration task-list --json",
	} {
		req := base
		req.Command = command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("unrelated observation %q must fall through unchanged: %#v", command, got)
		}
	}
}

func TestHandoffGuardBlocksWorkerDecisionGatesWithoutExecutionHandoff(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	source := t.TempDir()
	gitDir := filepath.Join(source, ".git", "worktrees", "legacy-worker")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	worker := t.TempDir()
	if err := os.WriteFile(filepath.Join(worker, ".git"), []byte("gitdir: "+gitDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, command := range []string{
		"orca orchestration ask --to term-coordinator --question blocked --json",
		"orca orchestration gate-create --task task-1 --question blocked --json",
	} {
		req := HookToolUseLifecycleRequest{Repo: worker, CWD: worker, Tool: "exec_command", Command: command}
		got := BuildLifecyclePreToolUseDecision(req)
		if got.Decision != "block" || !strings.Contains(got.Reason, "linked worktree") {
			t.Fatalf("legacy linked worker decision command %q must block without a handoff record: %#v", command, got)
		}
	}

	workerList := HookToolUseLifecycleRequest{Repo: worker, CWD: worker, Tool: "exec_command", Command: "orca orchestration gate-list --json"}
	if got := BuildLifecyclePreToolUseDecision(workerList); got.Decision != "allow" {
		t.Fatalf("linked worker read-only gate-list must remain allowed: %#v", got)
	}
	for _, command := range []string{
		"orca orchestration ask --to term-worker --question allowed --json",
		"orca orchestration gate-create --task task-1 --question allowed --json",
		"orca orchestration gate-list --json",
	} {
		req := HookToolUseLifecycleRequest{Repo: source, CWD: source, Tool: "exec_command", Command: command}
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("source coordinator command %q must remain allowed: %#v", command, got)
		}
	}
}

func TestHandoffGuardDeduplicatesSourceDiscoveryBeforeTerminalSteeringAmbiguity(t *testing.T) {
	repo, record, _ := lifecycleHandoffRecord(t, handoff.StateClaimed)
	info, err := os.Stat(filepath.Join(repo, ".git"))
	if err != nil || !info.IsDir() {
		t.Fatalf("fixture must model a main checkout with a .git directory: info=%v err=%v", info, err)
	}
	req := handoffEditRequest(record, repo, "codex", "coordinator", "")
	req.Tool = "exec_command"
	req.Command = "orca terminal send --terminal term-1 --text '# agent-harness guidance: retry the exact report patch once' --enter --json"
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("duplicate discovery of one stable cycle ID must not create false ambiguity: %#v", got)
	}

	b, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	var second IssueOpsRecord
	if err := json.Unmarshal(b, &second); err != nil {
		t.Fatal(err)
	}
	second.ID = newIssueOpsID(repo, "2-demo")
	second.Branch = "2-demo"
	second.WorktreePath = makeIssueOpsGuardWorktreeForTest(t, repo, second.Branch)
	second.BranchPrepare.Branch = second.Branch
	second.ExecutionHandoff.WorkerRoot = second.WorktreePath
	second.ExecutionHandoff.OwnershipEpoch = "epoch-2"
	second.ExecutionHandoff.Orca.BaseRef = "refs/remotes/origin/2-demo"
	second.ExecutionHandoff.Orca.WorktreeID = "wt-2"
	second.ExecutionHandoff.Orca.WorktreeInstanceID = "inst-2"
	second.ExecutionHandoff.Orca.WorktreePath = second.WorktreePath
	second.ExecutionHandoff.Orca.WorkerPTYID = "pty-2"
	second.ExecutionHandoff.Orca.WorkerMailboxHandle = "term-2"
	second.ExecutionHandoff.Orca.WorkerTerminalHandle = "term-2"
	second.ExecutionHandoff.Orca.TaskID = "task-2"
	second.ExecutionHandoff.Orca.DispatchID = "dispatch-2"
	if _, err := writeIssueOps(IssueOpsStateRoot(), second); err != nil {
		t.Fatal(err)
	}
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("two active cycles with distinct handles must select the exact claimed handle: %#v", got)
	}

	unknown := req
	unknown.Command = "orca terminal send --terminal term-unknown --text '# agent-harness guidance: retry once' --enter --json"
	if got := BuildLifecyclePreToolUseDecision(unknown); got.Decision != "block" {
		t.Fatalf("an unknown terminal handle must fail closed: %#v", got)
	}

	second.ExecutionHandoff.Orca.WorkerTerminalHandle = "term-1"
	if _, err := writeIssueOps(IssueOpsStateRoot(), second); err != nil {
		t.Fatal(err)
	}
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" || !strings.Contains(got.Reason, "ambiguous") {
		t.Fatalf("duplicate persisted terminal handles must be ambiguous: %#v", got)
	}
}

func TestClaimedWorkerMutationOperandsAndSymlinksStayWithinRoot(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "outside.txt")
	if err := os.WriteFile(outsideFile, []byte("outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	blocked := []string{
		"sed -i '' " + outsideFile,
		"perl -pi -e s/x/y/ " + outsideFile,
		"find " + outside + " -delete",
		"chmod 600 " + outsideFile,
		"chown nobody " + outsideFile,
		"ln -s internal/x.go " + filepath.Join(outside, "link"),
		"make -C " + outside + " build",
		"npm --prefix " + outside + " install",
		"tar -C " + outside + " -xf archive.tar",
		`awk 'BEGIN { system("touch ` + outsideFile + `") }'`,
		`perl -e 'open(F, ">` + outsideFile + `")'`,
	}
	for _, command := range blocked {
		req := handoffEditRequest(record, worktree, "codex", "session-1", "")
		req.Tool, req.Command = "exec_command", command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
			t.Fatalf("outside/eval mutation %q must block: %#v", command, got)
		}
	}
	for _, command := range []string{
		"sed -i '' internal/x.go", "perl -pi -e s/x/y/ internal/x.go", "find internal -delete",
		"chmod 600 internal/x.go", "ln -s x.go internal/link", "make -C . build", "npm --prefix . install", "tar -C . -xf archive.tar",
	} {
		req := handoffEditRequest(record, worktree, "codex", "session-1", "")
		req.Tool, req.Command = "exec_command", command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("in-worktree implementation mutation %q should pass: %#v", command, got)
		}
	}
	link := filepath.Join(worktree, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	req := handoffEditRequest(record, worktree, "codex", "session-1", "")
	req.Tool, req.Command = "exec_command", "touch escape/sentinel"
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
		t.Fatalf("pre-existing symlink parent escape must block: %#v", got)
	}
}

func TestHandoffGuardStateRoleMatrixReturnsAuthorityAfterAccept(t *testing.T) {
	t.Run("submitted", func(t *testing.T) {
		repo, record, worktree := lifecycleTerminalHandoffRecord(t, handoff.StateSubmitted, "")
		workerEdit := handoffEditRequest(record, worktree, "codex", "session-1", filepath.Join(worktree, "internal", "late.go"))
		if got := BuildLifecyclePreToolUseDecision(workerEdit); got.Decision != "block" {
			t.Fatalf("submitted worker mutation must block: %#v", got)
		}
		for _, command := range []string{
			"agent-harness issueops handoff accept --id " + record.ID + " --attempt 1 --ownership-epoch epoch-1 --context-sha256 " + strings.Repeat("a", 64) + " --final-head " + record.ExecutionHandoff.Result.FinalHead + " --host codex --session-id coordinator --agent-id worker-1 --source-cwd " + repo,
			"agent-harness issueops handoff recover --id " + record.ID + " --action cancel --confirm",
		} {
			req := handoffEditRequest(record, repo, "codex", "coordinator", "")
			req.Tool, req.Command = "exec_command", command
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
				t.Fatalf("submitted coordinator command %q should pass: %#v", command, got)
			}
		}
		push := handoffEditRequest(record, repo, "codex", "coordinator", "")
		push.Tool, push.Command = "exec_command", "git push origin HEAD"
		if got := BuildLifecyclePreToolUseDecision(push); got.Decision != "block" {
			t.Fatalf("submitted handoff must not publish before accept: %#v", got)
		}
	})

	t.Run("closed accepted", func(t *testing.T) {
		repo, record, worktree := lifecycleTerminalHandoffRecord(t, handoff.StateClosed, handoff.DispositionAccepted)
		for _, command := range []string{
			"agent-harness issueops phase --id " + record.ID + " --to ai-slop-clean --json",
			"agent-harness issueops feedback add --id " + record.ID + " --source review --body accepted --classification defect --json",
			"orca orchestration task-update --id task-1 --status completed --result accepted --json",
		} {
			req := handoffEditRequest(record, repo, "codex", "coordinator", "")
			req.Tool, req.Command = "exec_command", command
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
				t.Fatalf("accepted coordinator command %q should pass: %#v", command, got)
			}
		}
		for _, command := range []string{
			"git push origin 1-demo",
			"orca orchestration task-update --id task-other --status completed --json",
			"orca orchestration task-update --id task-1 --status failed --json",
			"orca terminal send --terminal term-1 --text exit --enter --json",
			"git -C " + worktree + " commit -m late",
		} {
			req := handoffEditRequest(record, repo, "codex", "coordinator", "")
			req.Tool, req.Command = "exec_command", command
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("accepted command %q must remain blocked: %#v", command, got)
			}
		}
		if got := BuildLifecyclePreToolUseDecision(handoffEditRequest(record, worktree, "codex", "session-1", filepath.Join(worktree, "internal", "late.go"))); got.Decision != "block" {
			t.Fatalf("worker cannot edit after accepted authority return: %#v", got)
		}
	})

	for _, disposition := range []string{handoff.DispositionWorkerFailed, handoff.DispositionCancelled} {
		t.Run("closed "+disposition, func(t *testing.T) {
			repo, record, worktree := lifecycleTerminalHandoffRecord(t, handoff.StateClosed, disposition)
			command := "agent-harness issueops handoff recover --id " + record.ID + " --action retry --confirm"
			req := handoffEditRequest(record, repo, "codex", "coordinator", "")
			req.Tool, req.Command = "exec_command", command
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
				t.Fatalf("%s recovery command %q should pass to core validation: %#v", disposition, command, got)
			}
			for _, command := range []string{
				"orca orchestration task-update --id task-1 --status failed --json",
				"orca terminal close --terminal term-1 --json",
				"orca worktree rm --worktree id:wt-1 --force --json",
			} {
				req := handoffEditRequest(record, repo, "codex", "coordinator", "")
				req.Tool, req.Command = "exec_command", command
				if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
					t.Fatalf("%s unapproved cleanup command %q should block: %#v", disposition, command, got)
				}
			}
			for _, command := range []string{"git push origin HEAD", "gh pr create --title x --body y", "go build ./..."} {
				req := handoffEditRequest(record, repo, "codex", "coordinator", "")
				req.Tool, req.Command = "exec_command", command
				if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
					t.Fatalf("%s must not publish/implement via %q: %#v", disposition, command, got)
				}
			}
			if got := BuildLifecyclePreToolUseDecision(handoffEditRequest(record, worktree, "codex", "session-1", filepath.Join(worktree, "internal", "late.go"))); got.Decision != "block" {
				t.Fatalf("%s worker mutation must block: %#v", disposition, got)
			}
		})
	}
}

func TestHandoffGuardBlocksManualSubmittedWorkerDoneAndRetryGuidance(t *testing.T) {
	repo, record, worktree := lifecycleTerminalHandoffRecord(t, handoff.StateSubmitted, "")
	report := ".agent-harness/research/report.md"
	record.ExecutionHandoff.Result.ChangedFiles = []string{report}
	var err error
	record, err = writeIssueOps(IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}
	absoluteReport := filepath.Join(worktree, filepath.FromSlash(report))
	command := "orca orchestration send --to term_coordinator --type worker_done --subject complete --body 'Implementation completed. Verification passed. Cleanup handed off.' --task-id task-1 --dispatch-id dispatch-1 --files-modified " + report + " --report-path " + absoluteReport

	exact := handoffEditRequest(record, worktree, "codex", "session-1", "")
	exact.Tool, exact.Command = "exec_command", command
	if got := BuildLifecyclePreToolUseDecision(exact); got.Decision != "block" {
		t.Fatalf("submitted worker_done must be projected automatically, never by shell: %#v", got)
	}

	wrongSession := exact
	wrongSession.SessionID = "session-other"
	wrongAgent := exact
	wrongAgent.AgentID = "worker-other"
	wrongHost := exact
	wrongHost.Host = "claude"
	for name, req := range map[string]HookToolUseLifecycleRequest{
		"wrong host":         wrongHost,
		"wrong session":      wrongSession,
		"wrong agent":        wrongAgent,
		"group target":       withHandoffCommand(exact, strings.Replace(command, "--to term_coordinator", "--to @all", 1)),
		"wrong task":         withHandoffCommand(exact, strings.Replace(command, "--task-id task-1", "--task-id task-other", 1)),
		"wrong dispatch":     withHandoffCommand(exact, strings.Replace(command, "--dispatch-id dispatch-1", "--dispatch-id dispatch-other", 1)),
		"extra file":         withHandoffCommand(exact, strings.Replace(command, "--files-modified "+report, "--files-modified "+report+",internal/x.go", 1)),
		"external report":    withHandoffCommand(exact, strings.Replace(command, absoluteReport, filepath.Join(t.TempDir(), "report.md"), 1)),
		"duplicate task":     withHandoffCommand(exact, command+" --task-id task-1"),
		"wrong message type": withHandoffCommand(exact, strings.Replace(command, "--type worker_done", "--type status", 1)),
		"two sentence body":  withHandoffCommand(exact, strings.Replace(command, "Implementation completed. Verification passed. Cleanup handed off.", "Verification passed. Cleanup handed off.", 1)),
		"extra payload":      withHandoffCommand(exact, command+" --payload '{}'"),
	} {
		t.Run(name, func(t *testing.T) {
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("submitted worker_done %s must block: %#v", name, got)
			}
		})
	}

	lateEdit := handoffEditRequest(record, worktree, "codex", "session-1", filepath.Join(worktree, "internal", "late.go"))
	if got := BuildLifecyclePreToolUseDecision(lateEdit); got.Decision != "block" {
		t.Fatalf("submitted worker repository mutation must remain blocked: %#v", got)
	}

	guidance := handoffEditRequest(record, repo, "codex", "coordinator", "")
	guidance.Tool = "exec_command"
	guidance.Command = "orca terminal send --terminal term-1 --text '# agent-harness guidance: retry the exact worker_done command once' --enter --json"
	if got := BuildLifecyclePreToolUseDecision(guidance); got.Decision != "block" {
		t.Fatalf("submitted retry guidance must not authorize a second external send: %#v", got)
	}
	guidance.Command = "orca terminal send --terminal term-other --text '# agent-harness guidance: retry the exact worker_done command once' --enter --json"
	if got := BuildLifecyclePreToolUseDecision(guidance); got.Decision != "block" {
		t.Fatalf("submitted guidance to a non-persisted handle must block: %#v", got)
	}
}

func TestHandoffGuardAllowsOnlyExactClaimedWorkerProgressMessages(t *testing.T) {
	_, record, worktree := lifecycleTerminalHandoffRecord(t, handoff.StateClaimed, "")
	base := "orca orchestration send --to term_coordinator --subject alive --task-id task-1 --dispatch-id dispatch-1"
	for _, command := range []string{
		base + " --type heartbeat --phase implementing",
		base + " --type status --body 'Tests are running' --json",
		base + " --type escalation --body 'Blocked on coordinator action'",
	} {
		req := handoffEditRequest(record, worktree, "codex", "session-1", "")
		req.Tool, req.Command = "exec_command", command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("exact claimed progress message %q should pass: %#v", command, got)
		}
	}
	for _, command := range []string{
		strings.Replace(base+" --type status --body ok", "term_coordinator", "@all", 1),
		strings.Replace(base+" --type status --body ok", "task-1", "task-other", 1),
		strings.Replace(base+" --type status --body ok", "dispatch-1", "dispatch-other", 1),
		base + " --type worker_done --body done",
		base + " --type heartbeat --phase implementing --body extra",
		base + " --type escalation --body blocked --payload '{}'",
	} {
		req := handoffEditRequest(record, worktree, "codex", "session-1", "")
		req.Tool, req.Command = "exec_command", command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
			t.Fatalf("inexact claimed progress message %q must block: %#v", command, got)
		}
	}
}

func withHandoffCommand(req HookToolUseLifecycleRequest, command string) HookToolUseLifecycleRequest {
	req.Command = command
	return req
}

func TestHandoffGuardAllowsOnlyExactClosedFailedTerminalCleanup(t *testing.T) {
	for _, disposition := range []string{handoff.DispositionWorkerFailed, handoff.DispositionCancelled} {
		t.Run(disposition, func(t *testing.T) {
			repo, record, worktree := lifecycleTerminalHandoffRecord(t, handoff.StateClosed, disposition)
			record.ExecutionHandoff.Orca.WorkerMailboxHandle = "term_historical"
			record.ExecutionHandoff.Orca.WorkerTerminalHandle = "term_live"
			record.ExecutionHandoff.Cleanup = &issueopsmodel.IssueOpsExecutionHandoffCleanup{
				Disposition: "remove", Reason: "discard terminal resources", ApprovedAt: "2026-07-11T02:01:00Z",
				Receipts: []issueopsmodel.IssueOpsExecutionHandoffCleanupReceipt{{
					Step: "task_terminal", TaskID: "task-1", DispatchID: "dispatch-1", RecordedAt: "2026-07-11T02:02:00Z",
				}},
			}
			var err error
			record, err = writeIssueOps(IssueOpsStateRoot(), record)
			if err != nil {
				t.Fatal(err)
			}
			allowed := handoffEditRequest(record, repo, "codex", "coordinator", "")
			allowed.Tool = "exec_command"
			allowed.Command = "orca terminal close --terminal term_live --json"
			if got := BuildLifecyclePreToolUseDecision(allowed); got.Decision != "allow" {
				t.Fatalf("source coordinator exact live terminal cleanup should pass: %#v", got)
			}
			stop := allowed
			stop.Command = "orca terminal stop --worktree id:wt-1 --json"
			if got := BuildLifecyclePreToolUseDecision(stop); got.Decision != "allow" {
				t.Fatalf("source coordinator exact worktree terminal stop should pass: %#v", got)
			}

			for name, mutate := range map[string]func(*HookToolUseLifecycleRequest){
				"wrong handle": func(req *HookToolUseLifecycleRequest) {
					req.Command = "orca terminal close --terminal term-other --json"
				},
				"historical mailbox": func(req *HookToolUseLifecycleRequest) {
					req.Command = "orca terminal close --terminal term_historical --json"
				},
				"extra close flag": func(req *HookToolUseLifecycleRequest) { req.Command += " --force" },
				"stop by terminal": func(req *HookToolUseLifecycleRequest) { req.Command = "orca terminal stop --terminal term_live --json" },
				"wrong stop worktree": func(req *HookToolUseLifecycleRequest) {
					req.Command = "orca terminal stop --worktree id:wt-other --json"
				},
				"stop without id selector": func(req *HookToolUseLifecycleRequest) {
					req.Command = "orca terminal stop --worktree wt-1 --json"
				},
				"extra stop flag": func(req *HookToolUseLifecycleRequest) {
					req.Command = "orca terminal stop --worktree id:wt-1 --json --force"
				},
				"create": func(req *HookToolUseLifecycleRequest) {
					req.Command = "orca terminal create --worktree id:wt-1 --command codex --json"
				},
				"worker cwd":  func(req *HookToolUseLifecycleRequest) { req.CWD = worktree },
				"worker repo": func(req *HookToolUseLifecycleRequest) { req.Repo = worktree },
			} {
				t.Run(name, func(t *testing.T) {
					req := allowed
					mutate(&req)
					if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
						t.Fatalf("%s must not authorize terminal cleanup: %#v", name, got)
					}
				})
			}
		})
	}

	for _, tc := range []struct {
		name        string
		state       string
		disposition string
	}{
		{name: "claimed", state: handoff.StateClaimed},
		{name: "dispatched", state: handoff.StateDispatched},
		{name: "accepted", state: handoff.StateClosed, disposition: handoff.DispositionAccepted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var repo string
			var record IssueOpsRecord
			if tc.state == handoff.StateClosed {
				repo, record, _ = lifecycleTerminalHandoffRecord(t, tc.state, tc.disposition)
			} else {
				repo, record, _ = lifecycleHandoffRecord(t, tc.state)
			}
			req := handoffEditRequest(record, repo, "codex", "coordinator", "")
			req.Tool = "exec_command"
			req.Command = "orca terminal close --terminal term-1 --json"
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("%s terminal close must remain blocked: %#v", tc.name, got)
			}
		})
	}
}

func TestHandoffGuardEnforcesCleanupApprovalAndReceiptOrder(t *testing.T) {
	repo, record, _ := lifecycleTerminalHandoffRecord(t, handoff.StateClosed, handoff.DispositionWorkerFailed)
	request := handoffEditRequest(record, repo, "codex", "coordinator", "")
	request.Tool = "exec_command"
	request.Command = "orca orchestration task-update --id task-1 --status failed --json"
	if got := BuildLifecyclePreToolUseDecision(request); got.Decision != "block" {
		t.Fatalf("task cleanup without approval should block: %#v", got)
	}
	record.ExecutionHandoff.Cleanup = &issueopsmodel.IssueOpsExecutionHandoffCleanup{
		Disposition: "remove", Reason: "discard failed attempt", ApprovedAt: "2026-07-11T02:01:00Z",
	}
	var err error
	record, err = writeIssueOps(IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}
	if got := BuildLifecyclePreToolUseDecision(request); got.Decision != "allow" {
		t.Fatalf("approved task cleanup should pass: %#v", got)
	}
	record.ExecutionHandoff.Cleanup.Receipts = append(record.ExecutionHandoff.Cleanup.Receipts, issueopsmodel.IssueOpsExecutionHandoffCleanupReceipt{
		Step: "task_terminal", TaskID: "task-1", DispatchID: "dispatch-1", RecordedAt: "2026-07-11T02:02:00Z",
	})
	record, err = writeIssueOps(IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}
	request.Command = "orca terminal close --terminal term-1 --json"
	if got := BuildLifecyclePreToolUseDecision(request); got.Decision != "allow" {
		t.Fatalf("terminal cleanup after task receipt should pass: %#v", got)
	}
	record.ExecutionHandoff.Cleanup.Receipts = append(record.ExecutionHandoff.Cleanup.Receipts, issueopsmodel.IssueOpsExecutionHandoffCleanupReceipt{
		Step: "terminal_quiescent", TerminalHandle: "term-1", PTYID: "pty-1", WorktreeID: "wt-1", RecordedAt: "2026-07-11T02:03:00Z",
	})
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	request.Command = "orca worktree rm --worktree id:wt-1 --force --json"
	if got := BuildLifecyclePreToolUseDecision(request); got.Decision != "block" {
		t.Fatalf("stale terminal quiescence receipt must not authorize raw worktree removal: %#v", got)
	}
}

func TestTerminalQuiescentReceiptCannotAuthorizeWorktreeRemovalAfterCompetitorRace(t *testing.T) {
	repo, record, _ := lifecycleTerminalHandoffRecord(t, handoff.StateClosed, handoff.DispositionCancelled)
	record.ExecutionHandoff.Cleanup = &issueopsmodel.IssueOpsExecutionHandoffCleanup{
		Disposition: "remove", Reason: "user will decide external deletion", ApprovedAt: "2026-07-11T02:01:00Z",
		Receipts: []issueopsmodel.IssueOpsExecutionHandoffCleanupReceipt{
			{Step: "task_terminal", TaskID: "task-1", DispatchID: "dispatch-1", RecordedAt: "2026-07-11T02:02:00Z"},
			{Step: "terminal_quiescent", TerminalHandle: "term-1", PTYID: "pty-1", WorktreeID: "wt-1", RecordedAt: "2026-07-11T02:03:00Z"},
		},
	}
	if _, err := writeIssueOps(IssueOpsStateRoot(), record); err != nil {
		t.Fatal(err)
	}
	req := handoffEditRequest(record, repo, "codex", "coordinator", "")
	req.Tool = "exec_command"
	req.Command = "orca worktree rm --worktree id:wt-1 --force --json"
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
		t.Fatalf("a competitor may appear after the stale receipt; raw removal must remain blocked: %#v", got)
	}
}

func TestAcceptedCoordinatorPublishAuthorityExcludesDestructiveRemoteActions(t *testing.T) {
	repo, record, _ := lifecycleTerminalHandoffRecord(t, handoff.StateClosed, handoff.DispositionAccepted)
	allowed := []string{
		"gh pr view 16", "gh pr list", "gh pr status", "gh pr checks 16", "gh pr diff 16",
		"glab mr view 16", "glab mr list", "glab mr diff 16",
	}
	for _, command := range allowed {
		req := handoffEditRequest(record, repo, "codex", "coordinator", "")
		req.Tool, req.Command = "exec_command", command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("non-destructive accepted publish command %q should pass: %#v", command, got)
		}
	}
	blocked := []string{
		"git push origin 1-demo", "git push --set-upstream origin 1-demo", "git push -u origin refs/heads/1-demo:refs/heads/1-demo",
		"git push origin HEAD", "git push origin other", "git push origin refs/heads/other:refs/heads/1-demo",
		"git push --all origin", "git push --mirror origin", "git push --prune origin", "git push --tags origin",
		"git push --force origin " + record.Branch, "git push --force-with-lease origin " + record.Branch, "git push --delete origin " + record.Branch, "git push origin :" + record.Branch,
		"gh pr merge 16", "gh pr close 16", "gh pr reopen 16",
		"gh pr create --head 1-demo --base main --draft --title draft --body body",
		"glab mr create --source-branch 1-demo --target-branch main --draft --title draft --description body",
		"gh pr review 16 --approve", "glab mr approve 16", "glab mr merge 16", "glab mr close 16", "glab mr reopen 16",
		"gh pr create --base main --draft --title missing-head", "gh pr create --head 1-demo --draft --title missing-base",
		"gh pr create --head 1-demo --head other --base main --draft", "gh pr create --head owner:1-demo --base main --draft", "gh pr create --head 1-demo --base other --draft",
		"gh pr create --head 1-demo --base main --title missing-draft", "gh pr create --head 1-demo --base main --draft --web", "gh pr create --head 1-demo --base main --draft --fill", "gh pr create --head 1-demo --base main --draft --fill-first", "gh pr create --head 1-demo --base main --draft --recover key",
		"glab mr create --target-branch main --draft", "glab mr create --source-branch 1-demo --draft", "glab mr create --source-branch 1-demo --source-branch other --target-branch main --draft",
		"glab mr create --source-branch owner:1-demo --target-branch main --draft", "glab mr create --source-branch 1-demo --target-branch other --draft", "glab mr create --source-branch 1-demo --target-branch main",
		"glab mr create --source-branch 1-demo --target-branch main --draft --push", "glab mr create --source-branch 1-demo --target-branch main --draft --fill", "glab mr create --source-branch 1-demo --target-branch main --draft --create-source-branch", "glab mr create --source-branch 1-demo --target-branch main --draft --web", "glab mr create --source-branch 1-demo --target-branch main --draft --recover key",
	}
	for _, command := range blocked {
		req := handoffEditRequest(record, repo, "codex", "coordinator", "")
		req.Tool, req.Command = "exec_command", command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
			t.Fatalf("destructive/unapproved remote command %q must block: %#v", command, got)
		}
	}
}

func TestAcceptedCoordinatorPRWrapperRequiresMatchingPublishReceipt(t *testing.T) {
	repo, record, _ := lifecycleTerminalHandoffRecord(t, handoff.StateClosed, handoff.DispositionAccepted)
	command := "agent-harness issueops remote create-pr --id " + record.ID + " --provider github --title draft --body rendered --head 1-demo --base main --label bug --assignee octocat --confirm --json"
	req := handoffEditRequest(record, repo, "codex", "coordinator", "")
	req.Tool, req.Command = "exec_command", command
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
		t.Fatalf("missing publish receipt authorized PR wrapper: %#v", got)
	}
	record.ExecutionHandoff.PublishReceipt = &issueopsmodel.IssueOpsExecutionHandoffPublishReceipt{
		Provider: "github", ProjectKey: "github.com/example/repo", Remote: "origin", PushTargetSHA256: strings.Repeat("a", 64), Branch: record.Branch, Base: "main", RemoteRef: "refs/heads/" + record.Branch,
		FinalHead: record.ExecutionHandoff.Result.FinalHead, VerifiedAt: "2026-07-11T02:01:00Z",
	}
	var err error
	record, err = writeIssueOps(IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}
	if !acceptedIssueOpsDownstreamCommand(req, record) {
		t.Fatal("matching publish receipt failed the exact downstream parser")
	}
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("matching publish receipt did not authorize exact PR wrapper: %#v", got)
	}
	for _, mismatch := range []string{
		strings.Replace(command, "--provider github", "--provider gitlab", 1),
		strings.Replace(command, "--head 1-demo", "--head other", 1),
		strings.Replace(command, "--body rendered", "--body-file /tmp/untrusted", 1),
		strings.Replace(command, " --provider github", "", 1),
		strings.Replace(command, " --body rendered", "", 1),
		strings.Replace(command, " --head 1-demo", "", 1),
		strings.Replace(command, " --base main", "", 1),
		strings.Replace(command, " --label bug", "", 1),
		strings.Replace(command, " --assignee octocat", "", 1),
	} {
		req.Command = mismatch
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
			t.Fatalf("mismatched PR wrapper %q should block: %#v", mismatch, got)
		}
	}
}

func TestAcceptedCoordinatorRemoteCreateReconcileRequiresExactClaimAndSealedRecipient(t *testing.T) {
	repo, record, _ := lifecycleTerminalHandoffRecord(t, handoff.StateClosed, handoff.DispositionAccepted)
	record.Phase = IssueOpsPhasePR
	fingerprint := strings.Repeat("a", 64)
	record.ExecutionHandoff.PublishReceipt = &issueopsmodel.IssueOpsExecutionHandoffPublishReceipt{
		Provider: "github", ProjectKey: "github.com/example/repo", Remote: "origin", PushTargetSHA256: fingerprint,
		Branch: record.Branch, Base: "main", RemoteRef: "refs/heads/" + record.Branch,
		FinalHead: record.ExecutionHandoff.Result.FinalHead, VerifiedAt: "2026-07-11T02:01:00Z",
	}
	record.RemoteCreateClaim = &issueopsmodel.IssueOpsRemoteCreateClaim{
		ClaimID: "claim_00000000000000000000000000000000", Provider: "github", Kind: "pr", ProjectKey: "github.com/example/repo",
		Remote: "origin", RemoteRef: "refs/heads/" + record.Branch, PushTargetSHA256: fingerprint,
		Head: record.Branch, Base: "main", FinalHead: record.ExecutionHandoff.Result.FinalHead,
		Title: "title", Body: "", BodySHA256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		Labels: []string{"bug"}, Assignees: []string{"octocat"}, Draft: true, State: "unknown", InvocationState: "unknown", ClaimedAt: "2026-07-11T02:02:00Z",
	}
	var err error
	record, err = writeIssueOps(IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}
	command := "agent-harness issueops remote reconcile-create --id " + record.ID + " --claim-id " + record.RemoteCreateClaim.ClaimID + " --coordinator-recipient term_coordinator --host codex --session-id coordinator --agent-id worker-1 --source-cwd " + repo + " --confirm --json"
	req := handoffEditRequest(record, repo, "codex", "coordinator", "")
	req.Tool, req.Command = "exec_command", command
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("exact coordinator remote-create reconcile should pass: %#v", got)
	}
	for _, mismatch := range []string{
		strings.Replace(command, record.RemoteCreateClaim.ClaimID, "claim_11111111111111111111111111111111", 1),
		strings.Replace(command, "term_coordinator", "term_worker", 1),
		strings.Replace(command, " --confirm", "", 1),
		strings.Replace(command, " --json", " --unknown", 1),
	} {
		req.Command = mismatch
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
			t.Fatalf("mismatched remote-create reconcile %q should block: %#v", mismatch, got)
		}
	}
}

func TestAcceptedCoordinatorMCPRemoteCreateAndReconcileRequireNativeSourceIdentity(t *testing.T) {
	repo, record, worktree := lifecycleTerminalHandoffRecord(t, handoff.StateClosed, handoff.DispositionAccepted)
	record.Phase = IssueOpsPhasePR
	record.ExecutionHandoff.PublishReceipt = &issueopsmodel.IssueOpsExecutionHandoffPublishReceipt{
		Provider: "github", ProjectKey: "github.com/example/repo", Remote: "origin", PushTargetSHA256: strings.Repeat("a", 64),
		Branch: record.Branch, Base: "main", RemoteRef: "refs/heads/" + record.Branch, FinalHead: record.ExecutionHandoff.Result.FinalHead, VerifiedAt: "2026-07-11T02:01:00Z",
	}
	record, _ = writeIssueOps(IssueOpsStateRoot(), record)
	req := handoffEditRequest(record, repo, "codex", "coordinator", "")
	req.Tool = "mcp__agent_harness__issueops_remote_create_pr"
	req.ToolInput = map[string]any{"id": record.ID, "title": "draft", "body": "rendered", "provider": "github", "head": record.Branch, "base": "main", "labels": []any{"bug"}, "assignees": []any{"octocat"}, "confirm": true}
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("exact coordinator MCP create blocked: %#v", got)
	}
	for _, key := range []string{"provider", "body", "head", "base", "labels", "assignees"} {
		bad := req
		bad.ToolInput = map[string]any{}
		for inputKey, value := range req.ToolInput {
			bad.ToolInput[inputKey] = value
		}
		delete(bad.ToolInput, key)
		if got := BuildLifecyclePreToolUseDecision(bad); got.Decision != "block" {
			t.Fatalf("MCP create defaulted omitted %s unlike shell: %#v", key, got)
		}
	}
	for _, key := range []string{"provider", "body", "head", "base"} {
		bad := req
		bad.ToolInput = map[string]any{}
		for inputKey, value := range req.ToolInput {
			bad.ToolInput[inputKey] = value
		}
		bad.ToolInput[key] = ""
		if got := BuildLifecyclePreToolUseDecision(bad); got.Decision != "block" {
			t.Fatalf("MCP create accepted empty %s unlike shell: %#v", key, got)
		}
	}
	worker := req
	worker.CWD, worker.Repo, worker.SessionID = worktree, worktree, "session-1"
	if got := BuildLifecyclePreToolUseDecision(worker); got.Decision != "block" {
		t.Fatalf("worker MCP create allowed with copied payload: %#v", got)
	}
	record.RemoteCreateClaim = &issueopsmodel.IssueOpsRemoteCreateClaim{
		ClaimID: "claim_00000000000000000000000000000000", Provider: "github", Kind: "pr", ProjectKey: "github.com/example/repo", Remote: "origin", RemoteRef: "refs/heads/" + record.Branch,
		PushTargetSHA256: strings.Repeat("a", 64), Head: record.Branch, Base: "main", FinalHead: record.ExecutionHandoff.Result.FinalHead, Title: "draft", Body: "",
		BodySHA256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", Labels: []string{"bug"}, Assignees: []string{"octocat"}, Draft: true, State: "unknown", InvocationState: "unknown", ClaimedAt: "2026-07-11T02:02:00Z",
	}
	record, _ = writeIssueOps(IssueOpsStateRoot(), record)
	req.Tool = "mcp__agent_harness__issueops_remote_reconcile_create"
	req.ToolInput = map[string]any{"id": record.ID, "claim_id": record.RemoteCreateClaim.ClaimID, "coordinator_recipient": "term_coordinator", "host": "codex", "session_id": "coordinator", "agent_id": "worker-1", "source_cwd": repo, "confirm": true}
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("exact coordinator MCP reconcile blocked: %#v", got)
	}
	for _, mutate := range []func(*HookToolUseLifecycleRequest){
		func(r *HookToolUseLifecycleRequest) { r.CWD, r.Repo = worktree, worktree },
		func(r *HookToolUseLifecycleRequest) { r.ToolInput["session_id"] = "worker-session" },
	} {
		bad := req
		bad.ToolInput = map[string]any{}
		for key, value := range req.ToolInput {
			bad.ToolInput[key] = value
		}
		mutate(&bad)
		if got := BuildLifecyclePreToolUseDecision(bad); got.Decision != "block" {
			t.Fatalf("non-source MCP reconcile allowed: %#v", got)
		}
	}
}

func TestHandoffLifecycleBlocksForeignMCPNameCollisions(t *testing.T) {
	repo, record, _ := lifecycleTerminalHandoffRecord(t, handoff.StateClosed, handoff.DispositionAccepted)
	record.Phase = IssueOpsPhasePR
	record.ExecutionHandoff.PublishReceipt = &issueopsmodel.IssueOpsExecutionHandoffPublishReceipt{
		Provider: "github", ProjectKey: "github.com/example/repo", Remote: "origin", PushTargetSHA256: strings.Repeat("a", 64),
		Branch: record.Branch, Base: "main", RemoteRef: "refs/heads/" + record.Branch, FinalHead: record.ExecutionHandoff.Result.FinalHead, VerifiedAt: "2026-07-11T02:01:00Z",
	}
	record.RemoteCreateClaim = &issueopsmodel.IssueOpsRemoteCreateClaim{
		ClaimID: "claim_00000000000000000000000000000000", Provider: "github", Kind: "pr", ProjectKey: "github.com/example/repo", Remote: "origin", RemoteRef: "refs/heads/" + record.Branch,
		PushTargetSHA256: strings.Repeat("a", 64), Head: record.Branch, Base: "main", FinalHead: record.ExecutionHandoff.Result.FinalHead, Title: "draft", Body: "",
		BodySHA256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", Labels: []string{"bug"}, Assignees: []string{"octocat"}, Draft: true, State: "unknown", InvocationState: "unknown", ClaimedAt: "2026-07-11T02:02:00Z",
	}
	var err error
	record, err = writeIssueOps(IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}
	req := handoffEditRequest(record, repo, "codex", "coordinator", "")
	for _, tt := range []struct {
		tool  string
		input map[string]any
	}{
		{
			tool:  "mcp__evil__issueops_remote_create_pr",
			input: map[string]any{"id": record.ID, "title": "draft", "body": "rendered", "provider": "github", "head": record.Branch, "base": "main", "labels": []any{"bug"}, "assignees": []any{"octocat"}, "confirm": true},
		},
		{
			tool:  "mcp__evil__issueops_remote_reconcile_create",
			input: map[string]any{"id": record.ID, "claim_id": record.RemoteCreateClaim.ClaimID, "coordinator_recipient": "term_coordinator", "host": "codex", "session_id": "coordinator", "agent_id": "worker-1", "source_cwd": repo, "confirm": true},
		},
		{
			tool:  "MCP__AGENT_HARNESS__ISSUEOPS_REMOTE_RECONCILE_CREATE",
			input: map[string]any{"id": record.ID, "claim_id": record.RemoteCreateClaim.ClaimID, "coordinator_recipient": "term_coordinator", "host": "codex", "session_id": "coordinator", "agent_id": "worker-1", "source_cwd": repo, "confirm": true},
		},
		{
			tool:  " mcp__agent_harness__issueops_remote_reconcile_create",
			input: map[string]any{"id": record.ID, "claim_id": record.RemoteCreateClaim.ClaimID, "coordinator_recipient": "term_coordinator", "host": "codex", "session_id": "coordinator", "agent_id": "worker-1", "source_cwd": repo, "confirm": true},
		},
		{
			tool:  "mcp__agent_harness__issueops_remote_reconcile_create ",
			input: map[string]any{"id": record.ID, "claim_id": record.RemoteCreateClaim.ClaimID, "coordinator_recipient": "term_coordinator", "host": "codex", "session_id": "coordinator", "agent_id": "worker-1", "source_cwd": repo, "confirm": true},
		},
		{
			tool:  "prefix_mcp__agent_harness__issueops_remote_reconcile_create",
			input: map[string]any{"id": record.ID, "claim_id": record.RemoteCreateClaim.ClaimID, "coordinator_recipient": "term_coordinator", "host": "codex", "session_id": "coordinator", "agent_id": "worker-1", "source_cwd": repo, "confirm": true},
		},
		{
			tool:  "mcp__agent_harness__issueops_remote_reconcile_create_suffix",
			input: map[string]any{"id": record.ID, "claim_id": record.RemoteCreateClaim.ClaimID, "coordinator_recipient": "term_coordinator", "host": "codex", "session_id": "coordinator", "agent_id": "worker-1", "source_cwd": repo, "confirm": true},
		},
	} {
		req.Tool, req.ToolInput = tt.tool, tt.input
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
			t.Fatalf("foreign MCP name collision %q bypassed supervised authority: %#v", tt.tool, got)
		}
	}
}

func TestLiteralV5CoordinatorSealRequiresExplicitSourceApprovalInShellAndMCP(t *testing.T) {
	repo, record, worktree := lifecycleTerminalHandoffRecord(t, handoff.StateClosed, handoff.DispositionAccepted)
	record.SchemaVersion = 5
	record.Invalid = true
	record.ExecutionHandoff.CoordinatorSession = nil
	record.ExecutionHandoff.PublishReceipt = &issueopsmodel.IssueOpsExecutionHandoffPublishReceipt{
		Provider: "github", Remote: "origin", Branch: record.Branch, RemoteRef: "refs/heads/" + record.Branch,
		FinalHead: record.ExecutionHandoff.Result.FinalHead, VerifiedAt: "2026-07-11T02:01:00Z",
	}
	putRawLifecycleIssueOpsRecord(t, record)
	req := handoffEditRequest(record, repo, "codex", "legacy-coordinator", "")
	req.Tool = "exec_command"
	req.Command = "agent-harness issueops handoff publish --id " + record.ID + " --host codex --session-id legacy-coordinator --agent-id worker-1 --source-cwd " + repo + " --confirm --json"
	if allowedExactHandoffLifecycleCommand(req, record) {
		t.Fatal("literal v5 publish allowed an implicit coordinator seal")
	}
	req.Command = strings.Replace(req.Command, " --confirm", " --approve-legacy-coordinator-seal --confirm", 1)
	if !allowedExactHandoffLifecycleCommand(req, record) {
		t.Fatal("literal v5 publish blocked an explicit source-coordinator seal")
	}
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("lifecycle guard blocked an explicit source-coordinator seal: %#v", got)
	}
	withoutApproval := req
	withoutApproval.Command = strings.Replace(req.Command, " --approve-legacy-coordinator-seal", "", 1)
	if got := BuildLifecyclePreToolUseDecision(withoutApproval); got.Decision != "block" {
		t.Fatalf("lifecycle guard allowed an implicit coordinator seal: %#v", got)
	}
	mcp := req
	mcp.Tool = "mcp__agent_harness__issueops_handoff"
	mcp.Command = ""
	mcp.ToolInput = map[string]any{
		"action": "publish", "id": record.ID, "host": "codex", "session_id": "legacy-coordinator", "agent_id": "worker-1", "source_cwd": repo,
		"approve_legacy_coordinator_seal": true, "confirm": true,
	}
	if !allowedHandoffMCPTool(mcp, record) {
		t.Fatal("literal v5 MCP publish blocked an explicit source-coordinator seal")
	}
	if got := BuildLifecyclePreToolUseDecision(mcp); got.Decision != "allow" {
		t.Fatalf("lifecycle guard blocked an explicit MCP source-coordinator seal: %#v", got)
	}
	withoutMCPApproval := mcp
	withoutMCPApproval.ToolInput = map[string]any{}
	for key, value := range mcp.ToolInput {
		withoutMCPApproval.ToolInput[key] = value
	}
	delete(withoutMCPApproval.ToolInput, "approve_legacy_coordinator_seal")
	if got := BuildLifecyclePreToolUseDecision(withoutMCPApproval); got.Decision != "block" {
		t.Fatalf("lifecycle guard allowed an implicit MCP coordinator seal: %#v", got)
	}
	mcp.CWD, mcp.Repo = worktree, worktree
	if allowedHandoffMCPTool(mcp, record) {
		t.Fatal("worker MCP publish authorized a copied legacy coordinator seal")
	}
}

func TestCoordinatorMCPStartRejectsCopiedNativeIdentity(t *testing.T) {
	repo, record, _ := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	req := handoffEditRequest(record, repo, "codex", "victim-session", "")
	req.Tool = "mcp__agent_harness__issueops_handoff"
	req.ToolInput = map[string]any{
		"action": "start", "id": record.ID, "coordinator_recipient": "term_coordinator",
		"coordinator_host": "codex", "coordinator_session_id": "victim-session", "coordinator_agent_id": "worker-1", "source_cwd": repo,
	}
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("MCP start blocked exact native coordinator identity: %#v", got)
	}
	req.SessionID = "attacker-session"
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
		t.Fatalf("MCP start accepted copied coordinator identity: %#v", got)
	}
}

func TestCoordinatorAcceptAndPublishRequireSealedNativeSessionInShellAndMCP(t *testing.T) {
	for _, action := range []string{"accept", "publish"} {
		t.Run(action, func(t *testing.T) {
			state, disposition := handoff.StateSubmitted, ""
			if action == "publish" {
				state, disposition = handoff.StateClosed, handoff.DispositionAccepted
			}
			repo, record, _ := lifecycleTerminalHandoffRecord(t, state, disposition)
			good := handoffEditRequest(record, repo, "codex", "coordinator", "")
			good.Tool = "mcp__agent_harness__issueops_handoff"
			good.ToolInput = map[string]any{
				"action": action, "id": record.ID, "host": "codex", "session_id": "coordinator", "agent_id": "worker-1", "source_cwd": repo,
				"attempt": record.ExecutionHandoff.Attempt, "ownership_epoch": record.ExecutionHandoff.OwnershipEpoch, "context_sha256": record.ExecutionHandoff.ContextSHA256, "confirm": true,
			}
			if got := BuildLifecyclePreToolUseDecision(good); got.Decision != "allow" {
				t.Fatalf("MCP %s blocked sealed coordinator session: %#v", action, got)
			}
			bad := good
			bad.SessionID = "different-coordinator-session"
			bad.ToolInput = map[string]any{}
			for key, value := range good.ToolInput {
				bad.ToolInput[key] = value
			}
			bad.ToolInput["session_id"] = "different-coordinator-session"
			if got := BuildLifecyclePreToolUseDecision(bad); got.Decision != "block" {
				t.Fatalf("MCP %s accepted different coordinator session: %#v", action, got)
			}
			goodShell := good
			goodShell.Tool = "exec_command"
			goodShell.Command = "agent-harness issueops handoff " + action + " --id " + record.ID + " --host codex --session-id coordinator --agent-id worker-1 --source-cwd " + repo
			if action == "accept" {
				goodShell.Command += " --attempt 1 --ownership-epoch epoch-1 --context-sha256 " + record.ExecutionHandoff.ContextSHA256 + " --final-head " + record.ExecutionHandoff.Result.FinalHead
			} else {
				goodShell.Command += " --confirm"
			}
			if got := BuildLifecyclePreToolUseDecision(goodShell); got.Decision != "allow" {
				t.Fatalf("shell %s blocked sealed coordinator session: %#v", action, got)
			}
			bad = goodShell
			bad.SessionID = "different-coordinator-session"
			bad.Command = strings.Replace(goodShell.Command, "--session-id coordinator", "--session-id different-coordinator-session", 1)
			if got := BuildLifecyclePreToolUseDecision(bad); got.Decision != "block" {
				t.Fatalf("shell %s accepted different coordinator session: %#v", action, got)
			}
		})
	}
}

func TestAcceptedCoordinatorPushRequiresLocalBranchAtFinalHead(t *testing.T) {
	repo, record, _ := lifecycleTerminalHandoffRecord(t, handoff.StateClosed, handoff.DispositionAccepted)
	record.ExecutionHandoff.Result.FinalHead = strings.Repeat("f", 40)
	var err error
	record, err = writeIssueOps(IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}
	req := handoffEditRequest(record, repo, "codex", "coordinator", "")
	req.Tool, req.Command = "exec_command", "git push origin 1-demo"
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
		t.Fatalf("local branch ref drift authorized push: %#v", got)
	}
}

func TestAcceptedCoordinatorRejectsArbitraryBodyFileAndCleanupBypass(t *testing.T) {
	repo, record, _ := lifecycleTerminalHandoffRecord(t, handoff.StateClosed, handoff.DispositionAccepted)
	for _, command := range []string{
		"gh pr create --head 1-demo --base main --draft --title draft --body-file /tmp/untrusted.md",
		"orca worktree rm --worktree id:wt-1 --force --json",
	} {
		req := handoffEditRequest(record, repo, "codex", "coordinator", "")
		req.Tool, req.Command = "exec_command", command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
			t.Fatalf("accepted boundary bypass %q should block: %#v", command, got)
		}
	}
}

func TestAcceptedCoordinatorIssueOpsAuthorityUsesExactSubcommandsAndFlags(t *testing.T) {
	repo, record, _ := lifecycleTerminalHandoffRecord(t, handoff.StateClosed, handoff.DispositionAccepted)
	allowed := []string{
		"agent-harness issueops phase --id " + record.ID + " --to ai-slop-clean --json",
		"agent-harness issueops feedback add --id " + record.ID + " --source review --body accepted --classification defect --json",
		"agent-harness issueops feedback resolve --id " + record.ID + " --index 0 --resolution valid-defect --json",
		"agent-harness issueops feedback mark-issue-updated --id " + record.ID + " --json",
		"agent-harness issueops pr-readiness --id " + record.ID + " --strict --json",
		"agent-harness issueops ai-slop-clean record --id " + record.ID + " --category comments --category duplication --verification go-test --json",
		"agent-harness issueops cleanup status --id " + record.ID + " --merged --json",
		"agent-harness issueops remote verify-artifact --id " + record.ID + " --provider github --kind pr --url https://github.com/acme/repo/pull/16 --label bug --assignee octocat --json",
	}
	for _, command := range allowed {
		req := handoffEditRequest(record, repo, "codex", "coordinator", "")
		req.Tool, req.Command = "exec_command", command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("exact accepted IssueOps command %q should pass: %#v", command, got)
		}
	}
	blocked := []string{
		"agent-harness issueops remote create-issue --id " + record.ID + " --confirm",
		"agent-harness issueops remote create-child --id " + record.ID + " --confirm",
		"agent-harness issueops remote create-pr --id " + record.ID + " --confirm",
		"agent-harness issueops remote sync-graph --id " + record.ID,
		"agent-harness issueops remote reflect-devils-advocate --id " + record.ID,
		"agent-harness issueops cleanup close-children --id " + record.ID + " --merged --confirm",
		"agent-harness issueops force-release --id " + record.ID + " --reason bypass",
		"agent-harness issueops regress --id " + record.ID + " --reason bypass",
		"agent-harness issueops phase --id " + record.ID + " --to done --force",
		"agent-harness issueops unknown --id " + record.ID,
		"agent-harness issueops feedback unknown --id " + record.ID,
		"agent-harness issueops feedback add --id " + record.ID + " --id other --source review --body accepted",
		"agent-harness issueops pr-readiness --id " + record.ID + " --mystery",
	}
	for _, command := range blocked {
		req := handoffEditRequest(record, repo, "codex", "coordinator", "")
		req.Tool, req.Command = "exec_command", command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
			t.Fatalf("unapproved accepted IssueOps command %q must block: %#v", command, got)
		}
	}
}

func TestClaimedWorkerCannotEscapeControllerRoleWithShellQuoting(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	commands := []string{
		`\g\i\t push origin HEAD`, `\g\h pr merge 16`, `\o\r\c\a worktree rm --worktree id:wt-1 --force --json`,
		`$'git' push origin HEAD`, `$"gh" pr merge 16`, `$'orca' worktree rm --worktree id:wt-1 --force --json`,
	}
	for _, command := range commands {
		req := handoffEditRequest(record, worktree, "codex", "session-1", "")
		req.Tool, req.Command = "exec_command", command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
			t.Fatalf("quoted/escaped controller executable %q must block: %#v", command, got)
		}
	}
	if got := commandparse.SplitCommandTokens(`finish --verification evidence\ value`); len(got) != 3 || got[2] != "evidence value" {
		t.Fatalf("escaped literal evidence did not remain one argv value: %#v", got)
	}
}

func TestClaimedWorkerCannotReinterpretControllerThroughEvalOrSource(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	for _, command := range []string{
		`eval 'git push origin 1-demo'`, `builtin eval 'gh pr merge 16'`, `source scripts/controller.sh`, `. scripts/controller.sh`,
	} {
		req := handoffEditRequest(record, worktree, "codex", "session-1", "")
		req.Tool, req.Command = "exec_command", command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
			t.Fatalf("shell reinterpretation primitive %q must block: %#v", command, got)
		}
	}
	req := handoffEditRequest(record, worktree, "codex", "session-1", "")
	req.Tool, req.Command = "exec_command", "./scripts/test.sh"
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("ordinary in-worktree executable must remain allowed: %#v", got)
	}
}

func TestClaimedWorkerCannotUseZshEqualsExpansion(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	for _, command := range []string{`=git push origin 1-demo`, `tool --input =(print -r -- marker)`} {
		req := handoffEditRequest(record, worktree, "codex", "session-1", "")
		req.Tool, req.Command = "exec_command", command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
			t.Fatalf("active zsh equals expansion %q must block: %#v", command, got)
		}
	}
	for _, command := range []string{`tool '=git'`, `tool "=(literal)"`, `NAME=value ./scripts/test.sh`} {
		req := handoffEditRequest(record, worktree, "codex", "session-1", "")
		req.Tool, req.Command = "exec_command", command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("literal equals data or ordinary assignment %q should pass: %#v", command, got)
		}
	}
}

func TestHandoffGuardRetainsNonterminalLeaseAfterWorkerWorktreeDisappears(t *testing.T) {
	repo, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	if err := os.RemoveAll(worktree); err != nil {
		t.Fatal(err)
	}
	sourceEdit := handoffEditRequest(record, repo, "codex", "coordinator", filepath.Join(repo, "internal", "x.go"))
	if got := BuildLifecyclePreToolUseDecision(sourceEdit); got.Decision != "block" {
		t.Fatalf("missing worker tree must not silently release source guard authority: %#v", got)
	}
	staleWorker := handoffEditRequest(record, worktree, "codex", "session-1", filepath.Join(worktree, "internal", "x.go"))
	if got := BuildLifecyclePreToolUseDecision(staleWorker); got.Decision != "block" {
		t.Fatalf("stale worker mutation must fail closed after worktree loss: %#v", got)
	}
	for _, command := range []string{
		"agent-harness issueops status --id " + record.ID + " --json",
		"agent-harness issueops handoff recover --id " + record.ID + " --action cancel --confirm --force --reason 'worker tree disappeared'",
	} {
		req := handoffEditRequest(record, repo, "codex", "coordinator", "")
		req.Tool, req.Command = "exec_command", command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("missing-worktree coordinator recovery command %q should pass: %#v", command, got)
		}
	}
}

func TestCoordinatorPreparingAllowsOnlyExactPreDispatchCancel(t *testing.T) {
	repo, record, _ := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	allow := handoffEditRequest(record, repo, "codex", "coordinator", "")
	allow.Tool = "exec_command"
	allow.Command = "agent-harness issueops handoff recover --id " + record.ID + " --action cancel --confirm"
	if got := BuildLifecyclePreToolUseDecision(allow); got.Decision != "allow" {
		t.Fatalf("exact pre-dispatch cancel = %#v", got)
	}

	for _, command := range []string{
		"agent-harness issueops handoff recover --id " + record.ID + " --action cancel",
		"agent-harness issueops handoff recover --id " + record.ID + " --action finalize-cancel --confirm",
		"agent-harness issueops handoff recover --id " + record.ID + " --action cancel --confirm --json",
		"agent-harness issueops handoff recover --id " + record.ID + " --action cancel --confirm --force",
		"agent-harness issueops handoff recover --id " + record.ID + " --action cancel --confirm --reason 'unneeded reason'",
	} {
		req := handoffEditRequest(record, repo, "codex", "coordinator", "")
		req.Tool, req.Command = "exec_command", command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
			t.Fatalf("non-exact pre-dispatch recovery %q = %#v", command, got)
		}
	}
	mismatched := handoffEditRequest(record, repo, "codex", "different-session", "")
	mismatched.Tool, mismatched.Command = "exec_command", allow.Command
	if got := BuildLifecyclePreToolUseDecision(mismatched); got.Decision != "block" {
		t.Fatalf("mismatched coordinator cancel = %#v", got)
	}
	edit := handoffEditRequest(record, repo, "codex", "coordinator", filepath.Join(repo, "internal", "x.go"))
	if got := BuildLifecyclePreToolUseDecision(edit); got.Decision != "block" {
		t.Fatalf("source edit during coordinator preparation = %#v", got)
	}
}

func TestClaimedWorkerCannotDestroyCanonicalRootOrGitMetadata(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	outside := t.TempDir()
	for _, command := range []string{
		"rm -rf .", "rm -rf " + worktree, "rm -rf .git", "mv .git .git-old",
		"mv " + worktree + " " + filepath.Join(outside, "moved"), "chmod 000 .", "chown nobody " + worktree, "find . -delete",
	} {
		req := handoffEditRequest(record, worktree, "codex", "session-1", "")
		req.Tool, req.Command = "exec_command", command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" || !strings.Contains(got.Reason, "root") {
			t.Fatalf("protected-root command %q must block: %#v", command, got)
		}
	}
	req := handoffEditRequest(record, worktree, "codex", "session-1", "")
	req.Tool, req.Command = "exec_command", "rm -f internal/scoped.tmp"
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("scoped in-worktree file removal should remain allowed: %#v", got)
	}
}

func TestClaimedWorkerGitRepositoryOverridesMustStayInsideWorkerRoot(t *testing.T) {
	repo, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	outsideGit := filepath.Join(repo, ".git")
	outsideCommands := []string{
		"git --git-dir=" + outsideGit + " --work-tree=" + repo + " add .",
		"git --git-dir " + outsideGit + " --work-tree " + repo + " add .",
		"GIT_DIR=" + outsideGit + " GIT_WORK_TREE=" + repo + " git add .",
		"env GIT_DIR=" + outsideGit + " GIT_WORK_TREE=" + repo + " git add .",
		"GIT_DIR=~/.git GIT_WORK_TREE=:~/source git add .",
		"env GIT_DIR=~/.git GIT_WORK_TREE=~/source git add .",
	}
	for _, command := range outsideCommands {
		req := handoffEditRequest(record, worktree, "codex", "session-1", "")
		req.Tool, req.Command = "exec_command", command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" || !strings.Contains(command, "~") && !strings.Contains(got.Reason, "outside") {
			t.Fatalf("outside Git repository override %q must block: %#v", command, got)
		}
	}

	workerGit := filepath.Join(worktree, ".git")
	insideCommands := []string{
		"git --git-dir=" + workerGit + " --work-tree=" + worktree + " add .",
		"GIT_DIR=" + workerGit + " GIT_WORK_TREE=" + worktree + " git add .",
		"env GIT_DIR=" + workerGit + " GIT_WORK_TREE=" + worktree + " git add .",
	}
	for _, command := range insideCommands {
		req := handoffEditRequest(record, worktree, "codex", "session-1", "")
		req.Tool, req.Command = "exec_command", command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("exact worker-root Git override %q should pass: %#v", command, got)
		}
	}
}

func TestHandoffGuardRejectsActiveBraceAndGlobExpansion(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	for _, command := range []string{
		`touch {..,inside}/outside.txt`, `chmod {..,inside}/target`,
		`touch {..,"inside"}/outside.txt`, `touch {"..",inside}/outside.txt`,
		`touch {1"."."3"}/outside.txt`, `touch ["a"]/outside.txt`,
		`touch o*/pwned`, `rm file?.tmp`, `cp [ab].txt internal/`,
	} {
		req := handoffEditRequest(record, worktree, "codex", "session-1", "")
		req.Tool, req.Command = "exec_command", command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
			t.Fatalf("active brace/glob expansion %q must block: %#v", command, got)
		}
	}
	for _, command := range []string{
		`touch '{..,inside}/literal'`, `touch "o*/literal"`, `touch \{one,two\}`, `touch o\*/literal`,
		`touch {one","two}`, `touch {one\,two}`,
	} {
		req := handoffEditRequest(record, worktree, "codex", "session-1", "")
		req.Tool, req.Command = "exec_command", command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("literal brace/glob data %q should pass: %#v", command, got)
		}
	}
}

func TestHandoffGlobSymlinkEscapeReproducesOutsideMutationBeforeGuard(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	outside := t.TempDir()
	tests := []struct {
		name, link, shell, command, sentinel string
	}{
		{name: "star glob", link: "out", shell: "bash", command: "touch o*/pwned-star", sentinel: "pwned-star"},
		{name: "zsh mixed range", link: "2", shell: "zsh", command: `touch {1"."."3"}/pwned-range`, sentinel: "pwned-range"},
		{name: "mixed bracket", link: "a", shell: "bash", command: `touch ["a"]/pwned-bracket`, sentinel: "pwned-bracket"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.Symlink(outside, filepath.Join(worktree, tt.link)); err != nil {
				t.Fatal(err)
			}
			sentinel := filepath.Join(outside, tt.sentinel)
			req := handoffEditRequest(record, worktree, "codex", "session-1", "")
			req.Tool, req.Command = "exec_command", tt.command
			got := BuildLifecyclePreToolUseDecision(req)
			if got.Decision != "block" {
				cmd := exec.Command(tt.shell, "-c", req.Command)
				cmd.Dir = worktree
				if err := cmd.Run(); err != nil {
					t.Fatalf("reproduce pathname escape: %v", err)
				}
				if _, err := os.Stat(sentinel); err != nil {
					t.Fatalf("hook allowed pathname command but fixture did not reproduce escape: %v", err)
				}
				t.Fatalf("hook allowed pathname expansion through worker symlink and wrote outside sentinel %s", sentinel)
			}
			if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
				t.Fatalf("blocked hook must leave outside sentinel absent: %v", err)
			}
		})
	}
}

func TestHandoffGuardBlocksWrongOrRestartedSession(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	for _, session := range []string{"other", ""} {
		got := BuildLifecyclePreToolUseDecision(handoffEditRequest(record, worktree, "codex", session, filepath.Join(worktree, "internal", "x.go")))
		if got.Decision != "block" {
			t.Fatalf("session %q should block: %#v", session, got)
		}
	}
}

func TestHandoffGuardBlocksWorkerEscape(t *testing.T) {
	repo, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	got := BuildLifecyclePreToolUseDecision(handoffEditRequest(record, worktree, "codex", "session-1", filepath.Join(repo, "internal", "x.go")))
	if got.Decision != "block" || !strings.Contains(got.Reason, "outside") {
		t.Fatalf("worker escape should block: %#v", got)
	}
}

func TestHandoffGuardAllowsExactLifecycleCommandsOnly(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateDispatched)
	claim := handoffEditRequest(record, worktree, "codex", "session-1", "")
	claim.Tool = "Bash"
	claim.Command = handoffClaimCommand(record, worktree, "codex", "session-1", "worker-1", "wt-1")
	if got := BuildLifecyclePreToolUseDecision(claim); got.Decision != "allow" {
		t.Fatalf("exact claim command should pass: %#v", got)
	}
	claim.Command += " && touch internal/x.go"
	if got := BuildLifecyclePreToolUseDecision(claim); got.Decision != "block" {
		t.Fatalf("compound lifecycle command should block: %#v", got)
	}
}

func TestHandoffGuardRejectsWrappedDuplicateAndTrailingLifecycleCommands(t *testing.T) {
	repo, record, _ := lifecycleHandoffRecord(t, handoff.StateCoordinatorPreparing)
	base := handoffEditRequest(record, repo, "codex", "coordinator", "")
	base.Tool = "Bash"
	flags := " --id " + record.ID + " --coordinator-recipient term_coordinator --coordinator-host codex --coordinator-session-id coordinator --coordinator-agent-id worker-1 --source-cwd " + repo + " --expected-context-sha256 " + strings.Repeat("a", 64) + " --allow-codex-hook-trust-bypass --confirm --json"
	valid := "agent-harness issueops handoff start" + flags
	for _, command := range []string{valid, "./bin/agent-harness issueops handoff start" + flags} {
		req := base
		req.Command = command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
			t.Fatalf("exact lifecycle command should pass: command=%q got=%#v", command, got)
		}
	}
	for _, command := range []string{
		"bash -lc 'touch internal/x.go' agent-harness issueops handoff start --id " + record.ID + " --confirm",
		valid + " --id other",
		valid + " touch internal/x.go",
		"env agent-harness issueops handoff start --id " + record.ID + " --confirm",
	} {
		req := base
		req.Command = command
		if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
			t.Fatalf("non-exact lifecycle command must block: command=%q got=%#v", command, got)
		}
	}
}

func TestHandoffGuardAllowsQuotedFinishEvidenceAndBlocksUnquotedControlOperators(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	base := handoffEditRequest(record, worktree, "codex", "session-1", "")
	base.Tool = "Bash"
	base.Command = "agent-harness issueops handoff finish --id " + record.ID +
		" --attempt 1 --ownership-epoch epoch-1 --context-sha256 " + strings.Repeat("a", 64) +
		" --host codex --session-id session-1 --agent-id worker-1" +
		" --verification 'commit parent is exact; tree clean & verified | complete'" +
		` --cleanup-receipt "no temp; coordinator owns task & worktree | branch" --verification 'literal > evidence data'`
	if got := BuildLifecyclePreToolUseDecision(base); got.Decision != "allow" {
		t.Fatalf("quoted evidence punctuation must remain argument data: %#v", got)
	}

	for _, suffix := range []string{
		"; touch escaped.go",
		" & touch escaped.go",
		" | touch escaped.go",
		"\ntouch escaped.go",
		"\rtouch escaped.go",
	} {
		t.Run(strconv.Quote(suffix), func(t *testing.T) {
			req := base
			req.Command += suffix
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("unquoted shell control %q must fail closed: %#v", suffix, got)
			}
		})
	}
}

func TestHandoffGuardAllowsExactNoChangeFinishAndRejectsUnsafeFlags(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	base := handoffEditRequest(record, worktree, "codex", "session-1", "")
	base.Tool = "Bash"
	base.Command = "agent-harness issueops handoff finish --id " + record.ID +
		" --attempt 1 --ownership-epoch epoch-1 --context-sha256 " + strings.Repeat("a", 64) +
		" --host codex --session-id session-1 --agent-id worker-1" +
		" --no-change --verification 'focused handoff regression test passed' --json"
	if got := BuildLifecyclePreToolUseDecision(base); got.Decision != "allow" {
		t.Fatalf("exact no-change finish should pass: %#v", got)
	}

	for _, suffix := range []string{
		" --changed-file internal/unsafe.go",
		" --turing-report forged.md",
		" --cleanup-receipt forged",
		" --task-id task-other",
		" --unknown value",
	} {
		t.Run(strconv.Quote(suffix), func(t *testing.T) {
			req := base
			req.Command += suffix
			if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "block" {
				t.Fatalf("unsafe no-change finish flags must block: %#v", got)
			}
		})
	}

	identityMismatch := base
	identityMismatch.SessionID = "other-session"
	if got := BuildLifecyclePreToolUseDecision(identityMismatch); got.Decision != "block" {
		t.Fatalf("native identity mismatch must block: %#v", got)
	}
}

func TestHandoffGuardAuthenticatesClaimFlagsAgainstNativeIdentity(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateDispatched)
	tests := []struct {
		name, host, session, agent, cwd, worktreeID string
		allow                                       bool
	}{
		{name: "exact", host: "codex", session: "session-1", agent: "worker-1", cwd: worktree, worktreeID: "wt-1", allow: true},
		{name: "session mismatch", host: "codex", session: "other", agent: "worker-1", cwd: worktree, worktreeID: "wt-1"},
		{name: "host mismatch", host: "claude", session: "session-1", agent: "worker-1", cwd: worktree, worktreeID: "wt-1"},
		{name: "agent mismatch", host: "codex", session: "session-1", agent: "other", cwd: worktree, worktreeID: "wt-1"},
		{name: "cwd mismatch", host: "codex", session: "session-1", agent: "worker-1", cwd: filepath.Join(worktree, "nested"), worktreeID: "wt-1"},
		{name: "worktree id mismatch", host: "codex", session: "session-1", agent: "worker-1", cwd: worktree, worktreeID: "other"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := handoffEditRequest(record, worktree, "codex", "session-1", "")
			req.Tool = "Bash"
			req.Command = handoffClaimCommand(record, tt.cwd, tt.host, tt.session, tt.agent, tt.worktreeID)
			got := BuildLifecyclePreToolUseDecision(req)
			if tt.allow && got.Decision != "allow" {
				t.Fatalf("exact native identity should pass: %#v", got)
			}
			if !tt.allow && got.Decision != "block" {
				t.Fatalf("self-asserted claim identity should block: %#v", got)
			}
		})
	}
}

func TestHandoffGuardAllowsClaimWithoutAgentFlagWhenNativeAgentIsEmpty(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateDispatched)
	req := handoffEditRequest(record, worktree, "codex", "session-1", "")
	req.AgentID = ""
	req.Tool = "Bash"
	req.Command = handoffClaimCommand(record, worktree, "codex", "session-1", "", "wt-1")
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("claim with absent native agent identity should pass without --agent-id: %#v", got)
	}
}

func TestExactFlagsRejectsFollowingFlagAsValue(t *testing.T) {
	command, ok := commandparse.ParseExactIssueOpsCommand("agent-harness issueops handoff claim --agent-id --cwd /worker")
	if !ok {
		t.Fatal("exact command shape should parse before flag validation")
	}
	values, booleans, repeatable, ok := commandparse.IssueOpsCommandSpec(command.Path)
	if !ok {
		t.Fatal("claim command spec missing")
	}
	if flags, ok := commandparse.ExactFlags(command, values, booleans, repeatable); ok {
		t.Fatalf("flag token must not become another flag's value: %#v", flags)
	}
}

func TestWorktreePrepareCommandSpecAllowsBoundedInlineReasonFlag(t *testing.T) {
	command, ok := commandparse.ParseExactIssueOpsCommand("agent-harness issueops worktree prepare --id io-demo --orchestrator inline --inline-reason recovery --json")
	if !ok {
		t.Fatal("worktree prepare command should parse")
	}
	values, booleans, repeatable, ok := commandparse.IssueOpsCommandSpec(command.Path)
	if !ok {
		t.Fatal("worktree prepare command spec missing")
	}
	flags, ok := commandparse.ExactFlags(command, values, booleans, repeatable)
	if !ok || len(flags["--inline-reason"]) != 1 || flags["--inline-reason"][0] != "recovery" {
		t.Fatalf("inline authorization flag rejected by lifecycle parser: %#v", flags)
	}
}

func TestSessionStartRendersClaimWithoutMutation(t *testing.T) {
	_, record, worktree := lifecycleHandoffRecord(t, handoff.StateDispatched)
	before, _ := json.Marshal(record)
	guidance := BuildIssueOpsHandoffSessionGuidance(worktree, "codex", "session-1", "worker-1")
	for _, want := range []string{"role=worker", "handoff claim", "--id '" + record.ID + "'", "--attempt 1", "--ownership-epoch 'epoch-1'", "--context-sha256 '" + strings.Repeat("a", 64) + "'", "--session-id 'session-1'"} {
		if !strings.Contains(guidance, want) {
			t.Fatalf("guidance missing %q: %s", want, guidance)
		}
	}
	afterRecord, err := ReadIssueOps(IssueOpsStateRoot(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(afterRecord)
	if string(before) != string(after) {
		t.Fatal("SessionStart guidance mutated IssueOps state")
	}
}

func TestSessionStartOmitsEmptyAgentIDFromClaim(t *testing.T) {
	_, _, worktree := lifecycleHandoffRecord(t, handoff.StateDispatched)
	guidance := BuildIssueOpsHandoffSessionGuidance(worktree, "codex", "session-1", "")
	if strings.Contains(guidance, "--agent-id") {
		t.Fatalf("empty native agent id must be omitted so the next flag cannot be consumed: %s", guidance)
	}
	for _, want := range []string{"--session-id 'session-1'", "--cwd '" + worktree + "'", "--orca-worktree-id 'wt-1'"} {
		if !strings.Contains(guidance, want) {
			t.Fatalf("guidance missing %q: %s", want, guidance)
		}
	}
}

func lifecycleHandoffRecord(t *testing.T, state string) (string, IssueOpsRecord, string) {
	t.Helper()
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := guardRepoWithCycle(t, "1-demo", IssueOpsPhaseImplement)
	record, ok := ActiveIssueOpsCycleForBranch(repo, "1-demo")
	if !ok {
		t.Fatal("active record missing")
	}
	worktree := makeIssueOpsGuardWorktreeForTest(t, repo, "1-demo")
	linkIssueOpsBranchEvidenceForTest(t, repo, "1-demo")
	if _, err := LinkIssueOpsWorktree(IssueOpsStateRoot(), record.ID, worktree); err != nil {
		t.Fatal(err)
	}
	record, err := ReadIssueOps(IssueOpsStateRoot(), record.ID)
	if err != nil {
		t.Fatal(err)
	}
	contextSHA := strings.Repeat("a", 64)
	if state == handoff.StateCoordinatorPreparing {
		contextSHA = ""
	}
	record.ExecutionHandoff = &issueopsmodel.IssueOpsExecutionHandoff{
		ProtocolVersion: handoff.ProtocolVersion, State: state, Attempt: 1, OwnershipEpoch: "epoch-1", ContextSHA256: contextSHA,
		AttemptBaseHead: strings.Repeat("b", 40), Driver: "orca", Agent: "codex", CoordinatorRoot: repo, WorkerRoot: worktree, Orca: &issueopsmodel.IssueOpsOrcaIdentity{
			RuntimeID: "runtime-1", RepoID: "repo-1", BaseRef: "refs/remotes/origin/1-demo", WorktreeID: "wt-1", WorktreeInstanceID: "inst-1", WorktreePath: worktree,
		},
	}
	record.ExecutionHandoff.CoordinatorMailboxHandle = "term_coordinator"
	record.ExecutionHandoff.CoordinatorSession = &issueopsmodel.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "coordinator", AgentID: "worker-1"}
	if state != handoff.StateCoordinatorPreparing {
		record.ExecutionHandoff.ContextVersion = handoff.ContextVersion
		record.ExecutionHandoff.ContextSourceSHA256 = strings.Repeat("d", 64)
		record.ExecutionHandoff.ContextOptions = &issueopsmodel.IssueOpsExecutionHandoffContextOptions{}
		record.ExecutionHandoff.DeliveryMode = "inject"
		record.ExecutionHandoff.Orca.WorkerPTYID = "pty-1"
		record.ExecutionHandoff.Orca.WorkerTerminalHandle = "term-1"
		record.ExecutionHandoff.Orca.WorkerMailboxHandle = "term-1"
		record.ExecutionHandoff.Orca.TaskID = "task-1"
		record.ExecutionHandoff.Orca.DispatchID = "dispatch-1"
	}
	if state == handoff.StateClaimed {
		record.ExecutionHandoff.WorkerSession = &issueopsmodel.IssueOpsHostSessionIdentity{Host: "codex", SessionID: "session-1", AgentID: "worker-1"}
	}
	record, err = writeIssueOps(IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}
	return repo, record, worktree
}

func lifecycleTerminalHandoffRecord(t *testing.T, state, disposition string) (string, IssueOpsRecord, string) {
	t.Helper()
	repo, record, worktree := lifecycleHandoffRecord(t, handoff.StateClaimed)
	report := filepath.Join(worktree, ".agent-harness", "research", "report.md")
	if err := os.MkdirAll(filepath.Dir(report), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(report, []byte("# evidence\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	record.ExecutionHandoff.State = state
	record.ExecutionHandoff.ClosedDisposition = disposition
	if state == handoff.StateSubmitted || disposition == handoff.DispositionAccepted {
		finalHead := ensureLifecyclePublicationRef(t, repo)
		record.ExecutionHandoff.Result = &issueopsmodel.IssueOpsExecutionHandoffResult{
			Outcome: handoff.OutcomeCompleted, FinalHead: finalHead, ChangedFiles: []string{"internal/x.go", ".agent-harness/research/report.md"},
			TuringReportPath: ".agent-harness/research/report.md", Verification: []string{"go test: pass"}, CleanupReceipts: []string{"worker resources handed off"}, TaskID: "task-1", DispatchID: "dispatch-1",
		}
	} else if disposition == handoff.DispositionWorkerFailed {
		record.ExecutionHandoff.Result = &issueopsmodel.IssueOpsExecutionHandoffResult{
			Outcome: handoff.OutcomeFailed, Verification: []string{"failure reproduced"}, CleanupReceipts: []string{"worker stopped"}, TaskID: "task-1", DispatchID: "dispatch-1",
		}
	}
	if state == handoff.StateClosed && disposition == handoff.DispositionAccepted {
		record.ExecutionHandoff.AcceptedAt = "2026-07-11T02:00:00Z"
	}
	var err error
	record, err = writeIssueOps(IssueOpsStateRoot(), record)
	if err != nil {
		t.Fatal(err)
	}
	return repo, record, worktree
}

func ensureLifecyclePublicationRef(t *testing.T, repo string) string {
	t.Helper()
	if head := strings.TrimSpace(preflight.GitOut(repo, "rev-parse", "refs/heads/1-demo")); head != "" {
		return head
	}
	if code, _, stderr := preflight.GitCmd(repo, "init", "-q"); code != 0 {
		t.Fatalf("git init failed: %s", stderr)
	}
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("publication fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"config", "user.name", "Lifecycle Test"},
		{"config", "user.email", "lifecycle@example.test"},
		{"add", "README.md"},
		{"commit", "-q", "-m", "test: publication fixture"},
	} {
		if code, _, stderr := preflight.GitCmd(repo, args...); code != 0 {
			t.Fatalf("git %v failed: %s", args, stderr)
		}
	}
	return strings.TrimSpace(preflight.GitOut(repo, "rev-parse", "refs/heads/1-demo"))
}

func handoffEditRequest(record IssueOpsRecord, cwd, host, session, target string) HookToolUseLifecycleRequest {
	paths := []string(nil)
	if target != "" {
		paths = []string{target}
	}
	return HookToolUseLifecycleRequest{
		Repo: cwd, CWD: cwd, Host: host, SessionID: session, AgentID: "worker-1", Tool: "Edit", Paths: paths,
		EnforceWorktree: true, ExpectedWorktree: record.WorktreePath, SourceCheckout: record.Repo,
	}
}

func handoffClaimCommand(record IssueOpsRecord, cwd, host, session, agent, worktreeID string) string {
	command := "agent-harness issueops handoff claim --id " + record.ID +
		" --attempt 1 --ownership-epoch epoch-1 --context-sha256 " + strings.Repeat("a", 64) +
		" --host " + host + " --session-id " + session
	if agent != "" {
		command += " --agent-id " + agent
	}
	return command + " --cwd " + cwd + " --orca-worktree-id " + worktreeID
}
