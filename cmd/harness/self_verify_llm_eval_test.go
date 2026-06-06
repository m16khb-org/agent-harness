package main

import (
	"strings"
	"testing"
)

func TestRunSelfVerifyRejectsUnknownLLMEvalMode(t *testing.T) {
	err := runSelfVerify([]string{"--llm-eval", "--llm-eval-mode=unknown", "--json"})
	if err == nil || !strings.Contains(err.Error(), "llm-eval-mode") {
		t.Fatalf("expected llm-eval-mode validation error, got %v", err)
	}
}

func TestResolveSelfVerifyRunModeDefaultsQuick(t *testing.T) {
	mode, err := resolveSelfVerifyRunMode(false, false, 10)
	if err != nil {
		t.Fatal(err)
	}
	if mode.Full || mode.Iterations != 1 || !strings.Contains(mode.ContractLabel, "quick") {
		t.Fatalf("default self-verify should run quick one-iteration mode, got %+v", mode)
	}
}

func TestResolveSelfVerifyRunModeFullUsesTenIterations(t *testing.T) {
	mode, err := resolveSelfVerifyRunMode(true, false, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !mode.Full || mode.Iterations != 10 || !strings.Contains(mode.ContractLabel, "full") {
		t.Fatalf("--full should run full ten-iteration mode, got %+v", mode)
	}
}

func TestResolveSelfVerifyRunModeFullAllowsExplicitIterations(t *testing.T) {
	mode, err := resolveSelfVerifyRunMode(true, true, 12)
	if err != nil {
		t.Fatal(err)
	}
	if !mode.Full || mode.Iterations != 12 {
		t.Fatalf("--full --iterations=12 should run 12 iterations, got %+v", mode)
	}
}

func TestResolveSelfVerifyRunModeRejectsIterationsWithoutFull(t *testing.T) {
	_, err := resolveSelfVerifyRunMode(false, true, 3)
	if err == nil || !strings.Contains(err.Error(), "--full") {
		t.Fatalf("expected --iterations without --full to be rejected, got %v", err)
	}
}
