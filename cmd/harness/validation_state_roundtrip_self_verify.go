package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/core"
)

type validateStateRoundtripSelfVerifyInput struct {
	binary      string
	root        string
	seed        int64
	tempState   string
	key         string
	env         []string
	started     time.Time
	stdoutParts []string
	commands    []string
	deps        stateRoundtripValidationDeps
}

func validateStateRoundtripSelfVerifyDeps(input validateStateRoundtripSelfVerifyInput) StepResult {
	stdoutParts := input.stdoutParts
	commands := input.commands
	run := func(label string, command ...string) StepResult {
		step := input.deps.run(input.root, label, 30*time.Second, "", input.env, command...)
		stdoutParts = append(stdoutParts, step.Stdout)
		commands = append(commands, step.Command)
		return step
	}
	fail := func(errs ...string) StepResult {
		return assertionStepWithOutput("state roundtrip", input.started, errs, stdoutParts, commands)
	}
	key := input.key
	baselineCompareKey := key + "-compare-base"
	candidateCompareKey := key + "-compare-candidate"
	compareSummary := SelfAugmentSummary{
		TotalRuns:    10,
		TotalSteps:   20,
		PassedSteps:  20,
		StepLabels:   []string{"go test", "MCP smoke"},
		SlowestSteps: []SelfAugmentSlowStep{{Iteration: 1, Seed: input.seed, Label: "go test", DurationMS: 1000}},
	}
	for _, snapshot := range []struct {
		key       string
		elapsedMS int64
		at        string
	}{
		{baselineCompareKey, 1000, "2000-01-01T00:00:00Z"},
		{candidateCompareKey, 1100, "2000-01-01T00:01:00Z"},
	} {
		if err := input.deps.writeSnapshot(input.tempState, snapshot.key, SelfAugmentStateSnapshot{
			SchemaVersion: 1,
			Kind:          selfVerificationSummaryKind,
			OK:            true,
			Iterations:    10,
			BaseSeed:      input.seed,
			ElapsedMS:     snapshot.elapsedMS,
			HarnessRoot:   input.root,
			GeneratedAt:   snapshot.at,
			Summary:       compareSummary,
		}); err != nil {
			return fail(err.Error())
		}
	}

	compareOK := run("self verify compare ok", input.binary, "self-verify", "compare", "--baseline-key", baselineCompareKey, "--candidate-key", candidateCompareKey, "--json")
	if !compareOK.OK {
		return combineFailedStep("state roundtrip", input.started, compareOK, stdoutParts, commands)
	}
	var compareOKResult SelfAugmentCompareResult
	if err := json.Unmarshal([]byte(compareOK.Stdout), &compareOKResult); err != nil {
		return fail(err.Error())
	}
	if !compareOKResult.OK || compareOKResult.Regressed || compareOKResult.ElapsedDeltaMS != 100 {
		return fail("self-verify compare reported unexpected non-regression result")
	}

	compareRegression := run("self verify compare regression", input.binary, "self-verify", "compare", "--baseline-key", baselineCompareKey, "--candidate-key", candidateCompareKey, "--max-elapsed-regression-pct", "5", "--json")
	if !compareRegression.OK {
		return combineFailedStep("state roundtrip", input.started, compareRegression, stdoutParts, commands)
	}
	var compareRegressionResult SelfAugmentCompareResult
	if err := json.Unmarshal([]byte(compareRegression.Stdout), &compareRegressionResult); err != nil {
		return fail(err.Error())
	}
	if !compareRegressionResult.OK || !compareRegressionResult.Regressed || len(compareRegressionResult.Regressions) == 0 {
		return fail("self-verify compare did not report expected elapsed regression")
	}

	promotedBaselineKey := key + "-promoted-baseline"
	promoteDry := run("self verify promote dry-run", input.binary, "self-verify", "promote", "--from-key", candidateCompareKey, "--baseline-key", promotedBaselineKey, "--json")
	if !promoteDry.OK {
		return combineFailedStep("state roundtrip", input.started, promoteDry, stdoutParts, commands)
	}
	var promoteDryResult SelfAugmentPromoteResult
	if err := json.Unmarshal([]byte(promoteDry.Stdout), &promoteDryResult); err != nil {
		return fail(err.Error())
	}
	if !promoteDryResult.OK || !promoteDryResult.DryRun || promoteDryResult.Promoted {
		return fail("self-verify promote dry-run mutated state or did not report dry-run")
	}
	if _, err := input.deps.stateRead(promotedBaselineKey); err == nil {
		return fail("self-verify promote dry-run wrote baseline unexpectedly")
	}

	promoteConfirm := run("self verify promote confirm", input.binary, "self-verify", "promote", "--from-key", candidateCompareKey, "--baseline-key", promotedBaselineKey, "--confirm", "--json")
	if !promoteConfirm.OK {
		return combineFailedStep("state roundtrip", input.started, promoteConfirm, stdoutParts, commands)
	}
	var promoteConfirmResult SelfAugmentPromoteResult
	if err := json.Unmarshal([]byte(promoteConfirm.Stdout), &promoteConfirmResult); err != nil {
		return fail(err.Error())
	}
	if !promoteConfirmResult.OK || promoteConfirmResult.DryRun || !promoteConfirmResult.Promoted {
		return fail("self-verify promote confirm did not write baseline")
	}

	comparePromoted := run("self verify compare promoted", input.binary, "self-verify", "compare", "--baseline-key", promotedBaselineKey, "--candidate-key", candidateCompareKey, "--json")
	if !comparePromoted.OK {
		return combineFailedStep("state roundtrip", input.started, comparePromoted, stdoutParts, commands)
	}
	var comparePromotedResult SelfAugmentCompareResult
	if err := json.Unmarshal([]byte(comparePromoted.Stdout), &comparePromotedResult); err != nil {
		return fail(err.Error())
	}
	if !comparePromotedResult.OK || comparePromotedResult.Regressed || comparePromotedResult.ElapsedDeltaMS != 0 {
		return fail("promoted baseline did not compare cleanly with candidate")
	}

	history := run("self verify history", input.binary, "self-verify", "history", "--prefix", key+"-", "--json")
	if !history.OK {
		return combineFailedStep("state roundtrip", input.started, history, stdoutParts, commands)
	}
	var historyResult SelfAugmentHistoryResult
	if err := json.Unmarshal([]byte(history.Stdout), &historyResult); err != nil {
		return fail(err.Error())
	}
	historyKeys := []string{}
	for _, entry := range historyResult.Entries {
		historyKeys = append(historyKeys, entry.Key)
	}
	if !historyResult.OK || historyResult.TotalMatches < 3 || !containsString(historyKeys, baselineCompareKey) || !containsString(historyKeys, candidateCompareKey) || !containsString(historyKeys, promotedBaselineKey) {
		return fail("self-verify history did not list saved baseline/candidate/promoted summaries")
	}

	retentionDry := run("self verify history retention dry-run", input.binary, "self-verify", "history", "--prefix", key+"-", "--retention-limit", "1", "--prune-retention", "--json")
	if !retentionDry.OK {
		return combineFailedStep("state roundtrip", input.started, retentionDry, stdoutParts, commands)
	}
	var retentionDryResult SelfAugmentHistoryResult
	if err := json.Unmarshal([]byte(retentionDry.Stdout), &retentionDryResult); err != nil {
		return fail(err.Error())
	}
	if retentionDryResult.Retention == nil || !retentionDryResult.Retention.DryRun || retentionDryResult.Retention.Confirm || retentionDryResult.Retention.Limit != 1 || len(retentionDryResult.Retention.CandidateKeys) == 0 || len(retentionDryResult.Retention.DeletedKeys) != 0 {
		return fail("self-verify history retention dry-run did not classify prune candidates safely")
	}

	retentionConfirm := run("self verify history retention confirm", input.binary, "self-verify", "history", "--prefix", key+"-", "--retention-limit", "1", "--prune-retention", "--confirm", "--json")
	if !retentionConfirm.OK {
		return combineFailedStep("state roundtrip", input.started, retentionConfirm, stdoutParts, commands)
	}
	var retentionConfirmResult SelfAugmentHistoryResult
	if err := json.Unmarshal([]byte(retentionConfirm.Stdout), &retentionConfirmResult); err != nil {
		return fail(err.Error())
	}
	if retentionConfirmResult.Retention == nil || retentionConfirmResult.Retention.DryRun || !retentionConfirmResult.Retention.Confirm || len(retentionConfirmResult.Retention.DeletedKeys) == 0 {
		return fail("self-verify history retention confirm did not delete prune candidates")
	}

	historyAfterRetention := run("self verify history after retention", input.binary, "self-verify", "history", "--prefix", key+"-", "--json")
	if !historyAfterRetention.OK {
		return combineFailedStep("state roundtrip", input.started, historyAfterRetention, stdoutParts, commands)
	}
	var historyAfterRetentionResult SelfAugmentHistoryResult
	if err := json.Unmarshal([]byte(historyAfterRetention.Stdout), &historyAfterRetentionResult); err != nil {
		return fail(err.Error())
	}
	if historyAfterRetentionResult.TotalMatches > 1 {
		return fail("self-verify history retention confirm left too many matching summaries")
	}

	corruptPath := filepath.Join(input.tempState, "corrupt.json")
	if err := input.deps.writeFile(corruptPath, []byte("{not json\n"), 0o600); err != nil {
		return fail(err.Error())
	}
	doctor := run("state doctor", input.binary, "state", "doctor", "--json")
	if !doctor.OK {
		return combineFailedStep("state roundtrip", input.started, doctor, stdoutParts, commands)
	}
	var doctorResult core.StateDoctorResult
	if err := json.Unmarshal([]byte(doctor.Stdout), &doctorResult); err != nil {
		return fail(err.Error())
	}
	if !doctorResult.OK || doctorResult.Healthy || !containsString(doctorResult.ValidKeys, key) || !stateDoctorHasIssueCode(doctorResult.Issues, "invalid_json") {
		return fail("state doctor did not report corrupt fixture and preserve valid key")
	}

	stdoutText, stdoutTruncated, stdoutBytes := tailWithBudget(strings.Join(stdoutParts, "\n"), selfVerifyAggregateOutputBudgetBytes)
	return StepResult{
		Label:           "state roundtrip",
		Command:         strings.Join(commands, " && "),
		OK:              true,
		DurationMS:      time.Since(input.started).Milliseconds(),
		Stdout:          stdoutText,
		StdoutBytes:     stdoutBytes,
		StdoutTruncated: stdoutTruncated,
	}
}
