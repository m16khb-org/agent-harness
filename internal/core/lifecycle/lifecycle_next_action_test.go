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
