package runmode

import (
	"strings"
	"testing"
)

func TestResolveDefaultsQuick(t *testing.T) {
	mode, err := Resolve(false, false, 10)
	if err != nil {
		t.Fatal(err)
	}
	if mode.Full || mode.Iterations != 1 || !strings.Contains(mode.ContractLabel, "quick") {
		t.Fatalf("default self-verify should run quick one-iteration mode, got %+v", mode)
	}
}

func TestResolveFullUsesTenIterations(t *testing.T) {
	mode, err := Resolve(true, false, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !mode.Full || mode.Iterations != 10 || !strings.Contains(mode.ContractLabel, "full") {
		t.Fatalf("--full should run full ten-iteration mode, got %+v", mode)
	}
}

func TestResolveFullAllowsExplicitIterations(t *testing.T) {
	mode, err := Resolve(true, true, 12)
	if err != nil {
		t.Fatal(err)
	}
	if !mode.Full || mode.Iterations != 12 {
		t.Fatalf("--full --iterations=12 should run 12 iterations, got %+v", mode)
	}
}

func TestResolveRejectsIterationsWithoutFull(t *testing.T) {
	_, err := Resolve(false, true, 3)
	if err == nil || !strings.Contains(err.Error(), "--full") {
		t.Fatalf("expected --iterations without --full to be rejected, got %v", err)
	}
}
