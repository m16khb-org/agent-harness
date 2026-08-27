package issueops

import "testing"

// cleanup finish/abandon의 새 파괴 단계 workspace_processes_stop이 실패하면 그
// 단계 이름이 failure receipt에 남는다. codec이 이 값을 모르면 receipt 쓰기를
// 거부해 finish는 resumable receipt를 무음 유실하고 abandon은
// cleanup_failure_inventory로 영구 차단된다(#477 brooks 1차 finding 2).
func TestKnownCleanupFailureStepsAdmitWorkspaceProcessesStop(t *testing.T) {
	if CleanupFailureStepWorkspaceProcessesStop != "workspace_processes_stop" {
		t.Fatalf("failure step constant = %q, want workspace_processes_stop", CleanupFailureStepWorkspaceProcessesStop)
	}
	if !knownCleanupFinishFailureStep(CleanupFailureStepWorkspaceProcessesStop) {
		t.Fatal("finish failure step validator must admit workspace_processes_stop")
	}
	if !knownCleanupAbandonFailureStep(CleanupFailureStepWorkspaceProcessesStop) {
		t.Fatal("abandon failure step validator must admit workspace_processes_stop")
	}
	if knownCleanupFinishFailureStep("workspace_processes_quiescent") || knownCleanupAbandonFailureStep("workspace_processes_quiescent") {
		t.Fatal("a preview gate slug is not an apply failure step")
	}
}
