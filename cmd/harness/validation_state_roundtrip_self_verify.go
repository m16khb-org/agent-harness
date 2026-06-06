package main

import (
	"encoding/json"
	"strings"
	"time"
)

func validateStateRoundtripSelfVerifyDeps(input validateStateRoundtripSelfVerifyInput) StepResult {
	session := newStateRoundtripSelfVerifySession(input)
	key := input.key
	baselineCompareKey := key + "-compare-base"
	candidateCompareKey := key + "-compare-candidate"
	if step := session.writeCompareSnapshots(input, baselineCompareKey, candidateCompareKey); !step.OK {
		return step
	}

	compareOK := session.run("self verify compare ok", input.binary, "self-verify", "compare", "--baseline-key", baselineCompareKey, "--candidate-key", candidateCompareKey, "--json")
	if !compareOK.OK {
		return session.combineFailed(compareOK)
	}
	var compareOKResult SelfAugmentCompareResult
	if err := json.Unmarshal([]byte(compareOK.Stdout), &compareOKResult); err != nil {
		return session.fail(err.Error())
	}
	if !compareOKResult.OK || compareOKResult.Regressed || compareOKResult.ElapsedDeltaMS != 100 {
		return session.fail("self-verify compare reported unexpected non-regression result")
	}

	compareRegression := session.run("self verify compare regression", input.binary, "self-verify", "compare", "--baseline-key", baselineCompareKey, "--candidate-key", candidateCompareKey, "--max-elapsed-regression-pct", "5", "--json")
	if !compareRegression.OK {
		return session.combineFailed(compareRegression)
	}
	var compareRegressionResult SelfAugmentCompareResult
	if err := json.Unmarshal([]byte(compareRegression.Stdout), &compareRegressionResult); err != nil {
		return session.fail(err.Error())
	}
	if !compareRegressionResult.OK || !compareRegressionResult.Regressed || len(compareRegressionResult.Regressions) == 0 {
		return session.fail("self-verify compare did not report expected elapsed regression")
	}

	promotedBaselineKey := key + "-promoted-baseline"
	promoteDry := session.run("self verify promote dry-run", input.binary, "self-verify", "promote", "--from-key", candidateCompareKey, "--baseline-key", promotedBaselineKey, "--json")
	if !promoteDry.OK {
		return session.combineFailed(promoteDry)
	}
	var promoteDryResult SelfAugmentPromoteResult
	if err := json.Unmarshal([]byte(promoteDry.Stdout), &promoteDryResult); err != nil {
		return session.fail(err.Error())
	}
	if !promoteDryResult.OK || !promoteDryResult.DryRun || promoteDryResult.Promoted {
		return session.fail("self-verify promote dry-run mutated state or did not report dry-run")
	}
	if _, err := input.deps.stateRead(promotedBaselineKey); err == nil {
		return session.fail("self-verify promote dry-run wrote baseline unexpectedly")
	}

	promoteConfirm := session.run("self verify promote confirm", input.binary, "self-verify", "promote", "--from-key", candidateCompareKey, "--baseline-key", promotedBaselineKey, "--confirm", "--json")
	if !promoteConfirm.OK {
		return session.combineFailed(promoteConfirm)
	}
	var promoteConfirmResult SelfAugmentPromoteResult
	if err := json.Unmarshal([]byte(promoteConfirm.Stdout), &promoteConfirmResult); err != nil {
		return session.fail(err.Error())
	}
	if !promoteConfirmResult.OK || promoteConfirmResult.DryRun || !promoteConfirmResult.Promoted {
		return session.fail("self-verify promote confirm did not write baseline")
	}

	comparePromoted := session.run("self verify compare promoted", input.binary, "self-verify", "compare", "--baseline-key", promotedBaselineKey, "--candidate-key", candidateCompareKey, "--json")
	if !comparePromoted.OK {
		return session.combineFailed(comparePromoted)
	}
	var comparePromotedResult SelfAugmentCompareResult
	if err := json.Unmarshal([]byte(comparePromoted.Stdout), &comparePromotedResult); err != nil {
		return session.fail(err.Error())
	}
	if !comparePromotedResult.OK || comparePromotedResult.Regressed || comparePromotedResult.ElapsedDeltaMS != 0 {
		return session.fail("promoted baseline did not compare cleanly with candidate")
	}

	if step := session.validateHistoryAndRetention(baselineCompareKey, candidateCompareKey, promotedBaselineKey); !step.OK {
		return step
	}

	stdoutText, stdoutTruncated, stdoutBytes := tailWithBudget(strings.Join(session.stdoutParts, "\n"), selfVerifyAggregateOutputBudgetBytes)
	return StepResult{
		Label:           "state roundtrip",
		Command:         strings.Join(session.commands, " && "),
		OK:              true,
		DurationMS:      time.Since(input.started).Milliseconds(),
		Stdout:          stdoutText,
		StdoutBytes:     stdoutBytes,
		StdoutTruncated: stdoutTruncated,
	}
}
