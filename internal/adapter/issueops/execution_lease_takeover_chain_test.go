package issueops

import (
	"context"
	"strings"
	"testing"

	"issueops/internal/port"
)

// 인수 경로의 첫 명령은 `replace --preview`이고, 그 뒤 단계는 preview가 돌려주는
// next_command로만 이어져야 한다(라우터 `## 단계 표`의 takeover 행, issueops-abandon).
// active lease에서 next_command가 비어 있으면 그 체인이 첫 걸음에서 끊긴다 —
// 죽은 홀더를 인수하려던 세션이 다음 명령을 스스로 지어내야 한다.
func TestReplacePreviewRendersTheRevokeStepForAnActiveLease(t *testing.T) {
	stateRoot, record := rolloverExecutionFixture(t)
	requester := executionActor("codex", "replacement-owner")
	deps := ExecutionReplaceDependencies{
		OrcaOwner:        &rolloverOwnerInspector{inventory: port.ExecutionOrcaOwnerInventory{RuntimeID: "runtime-sealed"}},
		inspectWorkspace: quiescentWorkspaceInspector(),
	}

	preview, err := ReplaceExecutionWithDependencies(context.Background(), stateRoot, ExecutionReplaceRequest{
		ID: record.ID, Action: ExecutionReplacePreview, ExpectedGeneration: 1,
		Actor: requester, CWD: record.Execution.Workspace.SourceRoot,
	}, deps)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if strings.TrimSpace(preview.NextCommand) == "" {
		t.Fatal("active lease preview must render the revoke step; an empty next_command ends the takeover chain")
	}
	for _, want := range []string{
		"execution replace", "--revoke", "--reason", "--confirm",
		"--inventory-fingerprint " + preview.InventoryFingerprint,
	} {
		if !strings.Contains(preview.NextCommand, want) {
			t.Fatalf("revoke step %q missing from next_command %q", want, preview.NextCommand)
		}
	}
}
