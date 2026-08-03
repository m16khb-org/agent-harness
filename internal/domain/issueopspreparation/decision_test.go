package issueopspreparation

import (
	"testing"

	leasecontract "agent-harness/internal/contract/issueopslease"
	preparationcontract "agent-harness/internal/contract/issueopspreparation"
)

func TestDecisionMatrix(t *testing.T) {
	tests := []struct {
		name          string
		mode          string
		confirm       bool
		execution     *leasecontract.Execution
		rootConflict  *preparationcontract.RootClaim
		orca          OrcaReadiness
		wantCode      Code
		wantRequested string
		wantResolved  string
		wantFallback  string
		wantDenial    DenialReason
	}{
		{name: "auto preview ready", mode: "auto", orca: OrcaReadiness{Available: true, Ready: true}, wantCode: CodePreviewOrca, wantRequested: "auto", wantResolved: "orca"},
		{name: "auto confirm ready", mode: "auto", confirm: true, orca: OrcaReadiness{Available: true, Ready: true}, wantCode: CodeApplyOrca, wantRequested: "auto", wantResolved: "orca"},
		{name: "auto ready without availability", mode: "auto", orca: OrcaReadiness{Ready: true}, wantCode: CodePreviewDirect, wantRequested: "auto", wantResolved: "direct", wantFallback: "orca_probe_failed"},
		{name: "auto preview unavailable", mode: "", orca: OrcaReadiness{Code: "orca_adapter_unavailable"}, wantCode: CodePreviewDirect, wantRequested: "auto", wantResolved: "direct", wantFallback: "orca_adapter_unavailable"},
		{name: "auto confirm unavailable", mode: "auto", confirm: true, orca: OrcaReadiness{Code: "orca_probe_failed"}, wantCode: CodeApplyDirect, wantRequested: "auto", wantResolved: "direct", wantFallback: "orca_probe_failed"},
		{name: "direct preview", mode: "direct", orca: OrcaReadiness{Ready: true}, wantCode: CodePreviewDirect, wantRequested: "direct", wantResolved: "direct"},
		{name: "direct confirm", mode: "direct", confirm: true, wantCode: CodeApplyDirect, wantRequested: "direct", wantResolved: "direct"},
		{name: "orca preview", mode: "orca", orca: OrcaReadiness{Available: true, Ready: true}, wantCode: CodePreviewOrca, wantRequested: "orca", wantResolved: "orca"},
		{name: "orca confirm", mode: "orca", confirm: true, orca: OrcaReadiness{Available: true, Ready: true}, wantCode: CodeApplyOrca, wantRequested: "orca", wantResolved: "orca"},
		{name: "explicit Orca unavailable", mode: "orca", orca: OrcaReadiness{Code: "orca_adapter_unavailable"}, wantRequested: "orca", wantDenial: DenialOrcaUnavailable},
		{name: "pending precedes mismatch", mode: "direct", confirm: true, execution: preparedExecution("orca", "active", true, true), wantCode: CodePendingReconcile, wantRequested: "direct", wantResolved: "orca"},
		{name: "existing explicit same", mode: "direct", confirm: true, execution: preparedExecution("direct", "active", true, false), wantCode: CodeExisting, wantRequested: "direct", wantResolved: "direct"},
		{name: "existing auto", mode: "auto", confirm: true, execution: preparedExecution("orca", "active", true, false), wantCode: CodeExisting, wantRequested: "auto", wantResolved: "orca"},
		{name: "explicit mismatch", mode: "orca", confirm: true, execution: preparedExecution("direct", "active", true, false), wantCode: CodeModeMismatch, wantRequested: "orca", wantResolved: "direct"},
		{name: "claimable writerless", mode: "auto", confirm: true, execution: preparedExecution("orca", "claimable", false, false), wantCode: CodeWriterless, wantRequested: "auto", wantResolved: "orca"},
		{name: "released writerless", mode: "direct", confirm: true, execution: preparedExecution("direct", "released", false, false), wantCode: CodeWriterless, wantRequested: "direct", wantResolved: "direct"},
		{name: "revoking writerless", mode: "auto", confirm: true, execution: preparedExecution("direct", "revoking", true, false), wantCode: CodeWriterless, wantRequested: "auto", wantResolved: "direct"},
		{name: "preview exposes writerless as existing", mode: "auto", execution: preparedExecution("orca", "claimable", false, false), wantCode: CodeExisting, wantRequested: "auto", wantResolved: "orca"},
		{name: "root conflict", mode: "direct", confirm: true, rootConflict: rootClaim(), wantCode: CodeRootConflict, wantRequested: "direct"},
		{name: "root conflict precedes Orca denial", mode: "orca", confirm: true, rootConflict: rootClaim(), orca: OrcaReadiness{Code: "orca_probe_failed"}, wantCode: CodeRootConflict, wantRequested: "orca"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			directReason := ""
			if test.mode == preparationcontract.ModeDirect && test.execution == nil && test.rootConflict == nil {
				directReason = "planned recovery"
			}
			decision, err := Decide(DecisionInput{
				Command: preparationcontract.Command{ID: "io-prepare", Mode: test.mode, DirectReason: directReason, Confirm: test.confirm},
				Snapshot: preparationcontract.Snapshot{
					Record:        leasecontract.Record{ID: "io-prepare", Execution: test.execution},
					CanonicalRoot: "/repo.worktrees/199-prepare", RootConflict: test.rootConflict,
				},
				Orca: test.orca,
			})
			if got := DenialReasonOf(err); got != test.wantDenial {
				t.Fatalf("denial=%q want=%q decision=%+v err=%v", got, test.wantDenial, decision, err)
			}
			if test.wantDenial != "" {
				return
			}
			if err != nil || decision.Code != test.wantCode || decision.RequestedMode != test.wantRequested ||
				decision.ResolvedMode != test.wantResolved || decision.FallbackCode != test.wantFallback {
				t.Fatalf("decision=%+v err=%v", decision, err)
			}
			if test.wantCode == CodeRootConflict && decision.RootConflict == nil {
				t.Fatal("root-conflict decision lost the conflicting claim")
			}
		})
	}
}

func TestDecisionRejectsUnsupportedMode(t *testing.T) {
	_, err := Decide(DecisionInput{Command: preparationcontract.Command{Mode: "remote"}})
	if got := DenialReasonOf(err); got != DenialInvalidMode {
		t.Fatalf("denial=%q err=%v", got, err)
	}
}

func TestDecisionSelectionEvidenceAndDirectReason(t *testing.T) {
	decision, err := Decide(DecisionInput{
		Command: preparationcontract.Command{Mode: "auto"},
		Orca:    OrcaReadiness{Available: true, Ready: false, Code: "orca_unready"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !decision.ProbeAttempted || !decision.ProbeAvailable || decision.ProbeReady ||
		decision.ProbeCode != "orca_unready" || decision.FallbackCode != "orca_unready" || decision.ReadinessFingerprint == "" {
		t.Fatalf("decision lost readiness evidence: %+v", decision)
	}

	_, err = Decide(DecisionInput{Command: preparationcontract.Command{Mode: "direct"}})
	if got := DenialReasonOf(err); got != DenialDirectReasonRequired {
		t.Fatalf("missing direct reason denial=%q err=%v", got, err)
	}
	decision, err = Decide(DecisionInput{Command: preparationcontract.Command{Mode: "direct", DirectReason: "  planned recovery  "}})
	if err != nil || decision.ExplicitDirectReason != "planned recovery" || decision.ProbeAttempted || decision.ReadinessFingerprint == "" {
		t.Fatalf("explicit direct decision=%+v err=%v", decision, err)
	}
}

func TestReadinessFingerprintBindsProviderIssueOnlyWhenProbed(t *testing.T) {
	command := preparationcontract.Command{Mode: "auto", OwnerHost: "codex", OwnerModel: "model", OwnerEffort: "high"}
	first, err := Decide(DecisionInput{Command: command, Orca: OrcaReadiness{Available: true, Ready: true, Provider: "github", Issue: 248}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Decide(DecisionInput{Command: command, Orca: OrcaReadiness{Available: true, Ready: true, Provider: "github", Issue: 249}})
	if err != nil {
		t.Fatal(err)
	}
	if first.ReadinessFingerprint == second.ReadinessFingerprint {
		t.Fatal("provider issue drift did not change probed readiness fingerprint")
	}

	directCommand := preparationcontract.Command{Mode: "direct", DirectReason: "planned recovery"}
	directA, err := Decide(DecisionInput{Command: directCommand, Orca: OrcaReadiness{Provider: "github", Issue: 248}})
	if err != nil {
		t.Fatal(err)
	}
	directB, err := Decide(DecisionInput{Command: directCommand, Orca: OrcaReadiness{Provider: "gitlab", Issue: 999}})
	if err != nil {
		t.Fatal(err)
	}
	if directA.ReadinessFingerprint != directB.ReadinessFingerprint {
		t.Fatal("explicit direct fingerprint read provider identity")
	}
}

func preparedExecution(mode, status string, holder, pending bool) *leasecontract.Execution {
	execution := &leasecontract.Execution{Mode: mode, Lease: leasecontract.Lease{Generation: 1, Status: status}}
	if holder {
		execution.Lease.Holder = &leasecontract.Actor{Host: "codex", SessionID: "session"}
	}
	if pending {
		execution.Pending = &leasecontract.ExternalIntent{OperationID: "operation", Kind: "owner_launch", Marker: "marker"}
	}
	return execution
}

func rootClaim() *preparationcontract.RootClaim {
	return &preparationcontract.RootClaim{LifecycleID: "io-other", Branch: "other", Root: "/repo.worktrees/199-prepare"}
}
