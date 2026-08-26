package issueops

import (
	"strings"
	"testing"
)

func validSyncBaseNativeActor() NativeActor {
	return NativeActor{
		Host:           "codex",
		SessionID:      "session-1",
		SessionProcess: &NativeProcessReceipt{PID: 42, StartedAt: "2026-08-25T00:00:00Z", Executable: "/usr/local/bin/codex"},
	}
}

func fullOID() string { return strings.Repeat("a", 40) }

func TestValidateNativeActorRequiresReuseSafeProcessReceipt(t *testing.T) {
	valid := validSyncBaseNativeActor()
	if err := ValidateNativeActor(valid); err != nil {
		t.Fatalf("valid actor rejected: %v", err)
	}

	tests := []struct {
		name    string
		mutate  func(*NativeActor)
		wantErr string
	}{
		{"unknown host", func(a *NativeActor) { a.Host = "gemini" }, "host must be codex"},
		{"blank session", func(a *NativeActor) { a.SessionID = "  " }, "session_id is required"},
		{"missing process", func(a *NativeActor) { a.SessionProcess = nil }, "session_process receipt"},
		{"non positive pid", func(a *NativeActor) { a.SessionProcess.PID = 0 }, "session_process receipt"},
		{"missing started at", func(a *NativeActor) { a.SessionProcess.StartedAt = "" }, "session_process receipt"},
		{"missing executable", func(a *NativeActor) { a.SessionProcess.Executable = "" }, "session_process receipt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actor := valid
			tt.mutate(&actor)
			err := ValidateNativeActor(actor)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateWriteLeaseStatusMatrix(t *testing.T) {
	holder := func() *NativeActor {
		actor := validSyncBaseNativeActor()
		return &actor
	}
	tests := []struct {
		name    string
		lease   WriteLease
		wantErr string
	}{
		{"zero generation", WriteLease{Generation: 0, Status: LeaseStatusClaimable}, "generation must start at 1"},
		{"unsupported status", WriteLease{Generation: 1, Status: LeaseStatus("frozen")}, "unsupported lease status"},
		{"claimable with holder", WriteLease{Generation: 1, Status: LeaseStatusClaimable, Holder: holder()}, "claimable lease requires no holder"},
		{"claimable without token", WriteLease{Generation: 1, Status: LeaseStatusClaimable}, "claimable lease requires no holder and one token hash"},
		{"active without holder", WriteLease{Generation: 1, Status: LeaseStatusActive}, "active lease requires one holder"},
		{"active keeps token", WriteLease{Generation: 1, Status: LeaseStatusActive, Holder: holder(), ClaimTokenSHA256: strings.Repeat("b", 64), ClaimedAt: "t"}, "no token hash"},
		{"revoking without holder", WriteLease{Generation: 1, Status: LeaseStatusRevoking}, "revoking lease requires the fenced holder"},
		{"released retains token", WriteLease{Generation: 1, Status: LeaseStatusReleased, ClaimTokenSHA256: strings.Repeat("c", 64)}, "released lease must not retain"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWriteLease(tt.lease)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}

	happy := []WriteLease{
		{Generation: 1, Status: LeaseStatusClaimable, ClaimTokenSHA256: strings.Repeat("b", 64)},
		{Generation: 1, Status: LeaseStatusActive, Holder: holder(), ClaimedAt: "t"},
		{Generation: 1, Status: LeaseStatusRevoking, Holder: holder()},
		{Generation: 1, Status: LeaseStatusReleased},
	}
	for _, lease := range happy {
		if err := validateWriteLease(lease); err != nil {
			t.Fatalf("valid lease %+v rejected: %v", lease, err)
		}
	}
}

func TestValidateExecutionSyncBaseResolutionBindsReleasedCompletion(t *testing.T) {
	execution := Execution{
		Lease:      WriteLease{Generation: 3, Status: LeaseStatusReleased},
		Completion: &ExecutionCompletion{Generation: 3},
	}
	valid := ExecutionSyncBaseResolution{
		Generation:           3,
		CompletionGeneration: 3,
		BaseOID:              fullOID(),
		StartedAt:            "2026-08-25T00:00:00Z",
		ConflictFiles:        []string{"internal/a.go", "internal/b.go"},
		Actor:                validSyncBaseNativeActor(),
	}
	if err := validateExecutionSyncBaseResolution(execution, valid); err != nil {
		t.Fatalf("valid resolution rejected: %v", err)
	}

	invalid := []struct {
		name    string
		mutate  func(*ExecutionSyncBaseResolution)
		wantErr string
	}{
		{"wrong generation", func(r *ExecutionSyncBaseResolution) { r.Generation = 4 }, "must bind the released current completion"},
		{"wrong completion generation", func(r *ExecutionSyncBaseResolution) { r.CompletionGeneration = 9 }, "must bind the released current completion"},
		{"short base oid", func(r *ExecutionSyncBaseResolution) { r.BaseOID = "abc" }, "is incomplete"},
		{"no conflict files", func(r *ExecutionSyncBaseResolution) { r.ConflictFiles = nil }, "is incomplete"},
		{"duplicate conflict file", func(r *ExecutionSyncBaseResolution) { r.ConflictFiles = []string{"a.go", "a.go"} }, "conflict path is invalid"},
		{"absolute conflict file", func(r *ExecutionSyncBaseResolution) { r.ConflictFiles = []string{"/etc/passwd"} }, "conflict path is invalid"},
		{"escaping conflict file", func(r *ExecutionSyncBaseResolution) { r.ConflictFiles = []string{"../secret"} }, "conflict path is invalid"},
		{"unclean conflict path", func(r *ExecutionSyncBaseResolution) { r.ConflictFiles = []string{"./a.go"} }, "conflict path is invalid"},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			resolution := valid
			tt.mutate(&resolution)
			err := validateExecutionSyncBaseResolution(execution, resolution)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}

	unbound := execution
	unbound.Lease.Status = LeaseStatusActive
	if err := validateExecutionSyncBaseResolution(unbound, valid); err == nil ||
		!strings.Contains(err.Error(), "must bind the released current completion") {
		t.Fatal("resolution on non-released lease must be rejected")
	}
}

func TestValidateExecutionSyncBaseEventContract(t *testing.T) {
	valid := ExecutionSyncBaseEvent{
		Mode:          ExecutionSyncBaseEventApply,
		BaseOID:       fullOID(),
		MergeCommit:   strings.Repeat("b", 40),
		BaseBranch:    "main",
		Actor:         "codex",
		At:            "2026-08-25T00:00:00Z",
		ConflictFiles: 2,
	}
	if err := validateExecutionSyncBaseEvent(valid); err != nil {
		t.Fatalf("valid event rejected: %v", err)
	}

	invalid := []struct {
		name    string
		mutate  func(*ExecutionSyncBaseEvent)
		wantErr string
	}{
		{"unknown mode", func(e *ExecutionSyncBaseEvent) { e.Mode = "revert" }, "mode must be apply or finalize"},
		{"short merge commit", func(e *ExecutionSyncBaseEvent) { e.MergeCommit = "zz" }, "full base and merge commit"},
		{"empty branch", func(e *ExecutionSyncBaseEvent) { e.BaseBranch = " " }, "event is incomplete"},
		{"negative conflicts", func(e *ExecutionSyncBaseEvent) { e.ConflictFiles = -1 }, "must not be negative"},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			event := valid
			tt.mutate(&event)
			err := validateExecutionSyncBaseEvent(event)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
	finalize := valid
	finalize.Mode = ExecutionSyncBaseEventFinalize
	if err := validateExecutionSyncBaseEvent(finalize); err != nil {
		t.Fatalf("finalize mode rejected: %v", err)
	}
}

func TestBaseSyncRequiredErrorCarriesReseedFreeNextCommand(t *testing.T) {
	err := NewBaseSyncRequiredError("io-9'x", 7)

	if got := err.Error(); !strings.Contains(got, "io-9'x") || !strings.Contains(got, "completion generation 7") {
		t.Fatalf("error message mismatch: %q", got)
	}
	if !strings.Contains(err.NextCommand, "'io-9'\\''x'") || !strings.Contains(err.NextCommand, "--completion-generation 7") {
		t.Fatalf("next command quoting wrong: %q", err.NextCommand)
	}
	fields := err.IssueOpsErrorFields()
	if fields["code"] != "post_completion_sync_base_required" || fields["completion_generation"] != uint64(7) {
		t.Fatalf("fields mismatch: %+v", fields)
	}
	if fields["next_command"] != err.NextCommand {
		t.Fatal("next_command field must match error field")
	}
}
