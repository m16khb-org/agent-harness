package installutil

import (
	"errors"
	"testing"

	"agent-harness/internal/port"
)

func TestJoinErrors(t *testing.T) {
	if err := JoinErrors(nil); err != nil {
		t.Fatalf("JoinErrors(nil) = %v", err)
	}
	if err := JoinErrors([]error{nil, nil}); err != nil {
		t.Fatalf("JoinErrors(all-nil) = %v", err)
	}
	err := JoinErrors([]error{errors.New("one"), nil, errors.New("two")})
	if err == nil || err.Error() != "one; two" {
		t.Fatalf("JoinErrors = %v", err)
	}
}

func TestPlanAccumulatesAndFinishesOK(t *testing.T) {
	plan := NewPlan("codex", true)
	plan.Message("first")
	plan.Messages([]string{"second", "third"})
	plan.File(port.InstallFile{Path: "a", Kind: "k"}, nil)
	plan.Files([]port.InstallFile{{Path: "b"}})
	plan.Link(port.InstallLink{Path: "l"}, nil)
	plan.Links([]port.InstallLink{{Path: "m"}})
	plan.Err(nil)
	plan.Errs(nil)

	result, err := plan.Finish()
	if err != nil {
		t.Fatalf("Finish err = %v", err)
	}
	if !result.OK || result.Host != "codex" || !result.DryRun {
		t.Fatalf("unexpected result header: %+v", result)
	}
	if len(result.Messages) != 3 || result.Messages[0] != "first" || result.Messages[2] != "third" {
		t.Fatalf("messages not preserved in order: %+v", result.Messages)
	}
	if len(result.Files) != 2 || len(result.Links) != 2 {
		t.Fatalf("files/links not accumulated: %+v", result)
	}
}

func TestPlanFinishFoldsErrors(t *testing.T) {
	plan := NewPlan("claude", false)
	plan.File(port.InstallFile{Path: "a"}, errors.New("boom"))
	plan.Errs([]error{nil, errors.New("bang")})
	result, err := plan.Finish()
	if err == nil || err.Error() != "boom; bang" {
		t.Fatalf("Finish error fan-in = %v", err)
	}
	if result.OK {
		t.Fatalf("result should not be OK when errors present: %+v", result)
	}
}

func TestHookGroupContainsAgentHarness(t *testing.T) {
	harness := map[string]any{"hooks": []any{map[string]any{"command": "'/bin/agent-harness' hook stop"}}}
	if !HookGroupContainsAgentHarness(harness) {
		t.Fatalf("should detect agent-harness hook group")
	}
	thirdParty := map[string]any{"hooks": []any{map[string]any{"command": "echo keep"}}}
	if HookGroupContainsAgentHarness(thirdParty) {
		t.Fatalf("should not flag third-party hook group")
	}
	if HookGroupContainsAgentHarness("not a map") {
		t.Fatalf("non-map group should be false")
	}
}
