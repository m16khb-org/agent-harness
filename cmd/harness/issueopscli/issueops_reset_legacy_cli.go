package issueopscli

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	orcaadapter "agent-harness/internal/adapter/orca"
	"agent-harness/internal/adapter/provider"
	"agent-harness/internal/core"
)

func runIssueOpsResetLegacy(args []string) error {
	fs := flag.NewFlagSet("issueops reset-legacy", flag.ContinueOnError)
	targetSchema := fs.Int("target-schema", 0, "required reset target schema; only 1 is supported")
	preview := fs.Bool("preview", false, "build the exact read-only legacy manifest")
	status := fs.Bool("status", false, "show the preview plus journal or receipt state")
	reconcileRemote := fs.Bool("reconcile-remote", false, "verify exactly one legacy remote-create candidate")
	reconcileOrca := fs.Bool("reconcile-orca", false, "verify one legacy Orca authority is absent or terminal")
	drainCycle := fs.Bool("drain-cycle", false, "record that one active legacy cycle is drained")
	activationBegin := fs.Bool("activation-begin", false, "seal a same-directory native activation candidate before atomic replacement")
	confirm := fs.Bool("confirm", false, "confirm destructive reset against the exact fingerprint")
	expectedFingerprint := fs.String("expected-fingerprint", "", "exact fingerprint returned by preview")
	id := fs.String("id", "", "exact legacy lifecycle id")
	claimID := fs.String("claim-id", "", "exact legacy remote-create claim id")
	runtimeID := fs.String("runtime-id", "", "exact legacy Orca runtime id")
	taskID := fs.String("task-id", "", "exact legacy Orca task id")
	dispatchID := fs.String("dispatch-id", "", "exact legacy Orca dispatch id")
	harnessRoot := fs.String("harness-root", "", "physical agent-harness root for native activation")
	targetBinary := fs.String("target-binary", "", "canonical harness binary target for native activation")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseIssueOpsFlags(fs, args); help || err != nil {
		return err
	}
	fullConfirm := *confirm && !*reconcileRemote && !*reconcileOrca && !*drainCycle
	actions := 0
	for _, selected := range []bool{*preview, *status, *reconcileRemote, *reconcileOrca, *drainCycle, *activationBegin, fullConfirm} {
		if selected {
			actions++
		}
	}
	if actions != 1 {
		return fmt.Errorf("issueops reset-legacy requires exactly one action: --preview, --status, --reconcile-remote, --reconcile-orca, --drain-cycle, --activation-begin, or reset --confirm")
	}
	if *targetSchema != 1 {
		return fmt.Errorf("issueops reset-legacy requires --target-schema 1")
	}
	if (*reconcileRemote || *reconcileOrca || *drainCycle) && !*confirm {
		return fmt.Errorf("legacy reconcile and drain actions require --confirm")
	}
	if (*preview || *status) && strings.TrimSpace(*expectedFingerprint) != "" {
		return fmt.Errorf("--expected-fingerprint is only valid with --confirm")
	}
	if (*confirm || *reconcileRemote || *reconcileOrca || *drainCycle) && !*activationBegin && strings.TrimSpace(*expectedFingerprint) == "" {
		return fmt.Errorf("issueops reset-legacy --confirm requires --expected-fingerprint")
	}
	if *activationBegin && (strings.TrimSpace(*harnessRoot) == "" || strings.TrimSpace(*targetBinary) == "") {
		return fmt.Errorf("legacy reset activation begin requires --harness-root and --target-binary")
	}
	if (*reconcileRemote || *reconcileOrca || *drainCycle) && strings.TrimSpace(*id) == "" {
		return fmt.Errorf("legacy reconcile and drain actions require --id")
	}
	if *reconcileRemote && strings.TrimSpace(*claimID) == "" {
		return fmt.Errorf("legacy remote reconciliation requires --claim-id")
	}
	if *reconcileOrca && strings.TrimSpace(*runtimeID) == "" && strings.TrimSpace(*taskID) == "" && strings.TrimSpace(*dispatchID) == "" {
		return fmt.Errorf("legacy Orca reconciliation requires at least one exact Orca identity")
	}
	stateDir := filepath.Dir(core.IssueOpsStateRoot())
	var value any
	var err error
	switch {
	case *preview:
		value, err = core.PreviewLegacyReset(stateDir, *targetSchema)
	case *status:
		value, err = core.StatusLegacyReset(stateDir, *targetSchema)
	case *activationBegin:
		value, err = core.BeginLegacyResetActivation(stateDir, core.LegacyResetActivationBeginRequest{
			TargetSchema: *targetSchema, HarnessRoot: *harnessRoot, TargetBinary: *targetBinary,
		})
	case *reconcileRemote:
		value, err = core.ReconcileLegacyRemoteClaim(context.Background(), stateDir, core.LegacyResetRemoteReconcileRequest{
			TargetSchema: *targetSchema, ExpectedFingerprint: *expectedFingerprint,
			LifecycleID: *id, ClaimID: *claimID, Confirm: true,
		}, core.LegacyResetRemoteDependencies{
			Reconcile: func(providerName string, request core.IssueProviderReconcilePullRequestRequest) (core.IssueProviderReconcilePullRequestResult, error) {
				prov, resolveErr := provider.Resolve(providerName)
				if resolveErr != nil {
					return core.IssueProviderReconcilePullRequestResult{}, resolveErr
				}
				return core.ReconcileRemotePullRequest(request, prov)
			},
			Verify: verifyIssueOpsRemoteArtifactLive,
		})
	case *reconcileOrca:
		client := orcaadapter.New()
		value, err = core.ReconcileLegacyOrcaTask(context.Background(), stateDir, core.LegacyResetOrcaReconcileRequest{
			TargetSchema: *targetSchema, ExpectedFingerprint: *expectedFingerprint, LifecycleID: *id,
			RuntimeID: *runtimeID, TaskID: *taskID, DispatchID: *dispatchID, Confirm: true,
		}, core.LegacyResetOrcaDependencies{
			Status: client.Status, ListTasks: client.ListAllTasks, ShowDispatch: client.ShowDispatch,
		})
	case *drainCycle:
		client := orcaadapter.New()
		value, err = core.DrainLegacyCycleWithOrca(context.Background(), stateDir, core.LegacyResetDrainCycleRequest{
			TargetSchema: *targetSchema, ExpectedFingerprint: *expectedFingerprint, LifecycleID: *id, Confirm: true,
		}, core.LegacyResetOrcaDependencies{
			Status: client.Status, ListTasks: client.ListAllTasks, ShowDispatch: client.ShowDispatch,
		})
	case fullConfirm:
		client := orcaadapter.New()
		value, err = core.ConfirmLegacyResetWithOrca(context.Background(), stateDir, *targetSchema, *expectedFingerprint, core.LegacyResetOrcaDependencies{
			Status: client.Status, ListTasks: client.ListAllTasks, ShowDispatch: client.ShowDispatch,
		})
	}
	if err != nil {
		if *jsonOut {
			_ = printIssueOpsErrorJSON(err)
		}
		return err
	}
	if *jsonOut {
		return printJSON(value)
	}
	fmt.Printf("%+v\n", value)
	return nil
}
