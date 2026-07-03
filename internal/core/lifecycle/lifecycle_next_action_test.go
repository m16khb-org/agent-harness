package lifecycle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecordStopNextActionRelaySuppressesWhenStateCannotPersist(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	written, err := InitProjectLifecycleState(repo, true)
	if err != nil {
		t.Fatal(err)
	}
	var profile ProjectLifecycleProfile
	b, err := os.ReadFile(written.ProjectJSONPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &profile); err != nil {
		t.Fatal(err)
	}
	profile.Fingerprint.RepoRoot = filepath.Join(t.TempDir(), "other")
	b, err = json.MarshalIndent(profile, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(written.ProjectJSONPath, append(b, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	trigger := BuildNextActionJudgementTrigger(strings.Join([]string{
		"선택지:",
		"1. (추천) 사용자가 외부 담당자에게 문의문을 전달하고 답변을 공유한다.",
		"2. 임시 조치 변경안을 검토하라고 지시한다.",
		"3. 식별자 분리 설계안을 검토하라고 지시한다.",
	}, "\n"))
	got := RecordStopNextActionRelay(repo, trigger)
	if got.ShouldRelay || !containsString(got.Warnings, "project_lifecycle_namespace_mismatch") {
		t.Fatalf("expected unpersistable Stop relay to fail closed, got %+v", got)
	}
}

func TestClearStopNextActionRelayIfPresentSkipsProfileReadWhenRelayMissing(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	plan, err := InitProjectLifecycleState(repo, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plan.ProjectJSONPath, []byte("{not-json"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := ClearStopNextActionRelayIfPresent(repo)
	if !got.OK || got.Reason != "no_next_action_relay" || len(got.Warnings) != 0 {
		t.Fatalf("missing relay should skip lifecycle profile validation, got %+v", got)
	}
	if got.Path != filepath.Join(plan.ProjectStateDir, stopNextActionRelayFile) {
		t.Fatalf("relay path = %q, want %q", got.Path, filepath.Join(plan.ProjectStateDir, stopNextActionRelayFile))
	}
}
