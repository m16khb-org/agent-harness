package orca

import (
	"context"
	"os"
	"testing"
	"time"

	"issueops/internal/port"
)

// TestClientSettleTaskConvergesAgainstTheLiveRelay는 #325의 dogfood 조건이다.
//
// fake runner는 내가 relay의 동작이라고 *믿는 것*을 고정할 뿐이다. fence가
// 실제로 어떤 코드로 오는지, run-use가 정말 authority를 회복하는지, 그리고
// 종결이 서버 쪽에 실제로 반영되는지는 실물에서만 확인된다.
//
// 이 테스트는 배포되는 코드 경로 그대로를 쓴다 — ExecRunner로 진짜 orca CLI를
// 부르는 Client다. 시나리오는 결함의 발생 조건을 그대로 재현한다:
//
//	Run A 생성 → A에 task 생성 → Run B 생성(= coordinator 바인딩이 A에서 떠남)
//	→ SettleTask(A, task)  ← 예전에는 여기서 consumer_fenced로 잃었다
//	→ task-list로 A의 task 상태를 되읽어 completed 확인
//
// 기본은 skip이다. 실제 Orca runtime과 coordinator terminal이 필요하고,
// Run 레코드를 만드는 부작용이 있다(Orca는 Run 삭제를 제공하지 않는다).
// 실행:
//
//	ISSUEOPS_ORCA_LIVE=1 go test ./internal/adapter/orca -run LiveRelay -count=1 -v
func TestClientSettleTaskConvergesAgainstTheLiveRelay(t *testing.T) {
	if os.Getenv("ISSUEOPS_ORCA_LIVE") != "1" {
		t.Skip("실물 Orca가 필요하다: ISSUEOPS_ORCA_LIVE=1로 실행한다")
	}
	client := NewClient(ExecRunner{})
	if !client.Available() {
		t.Skip("이 호스트에 orca CLI가 없다")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	sealed, err := client.CreateRun(ctx, port.OrcaCreateRunRequest{Objective: "issue 325 live settle: sealed run"})
	if err != nil {
		t.Fatalf("봉인될 Run을 만들지 못했다: %v", err)
	}
	task, err := client.CreateTask(ctx, port.OrcaCreateTaskRequest{
		RunID: sealed.ID, Spec: "issue 325 live settle probe",
		Title: "settle probe", DisplayName: "settle probe",
	})
	if err != nil {
		t.Fatalf("task를 만들지 못했다: %v", err)
	}

	// coordinator 바인딩을 봉인된 Run에서 떼어낸다. run-create가 곧 bind이므로
	// 이것이 실제 lifecycle에서 fence가 생기는 방식 그대로다.
	if _, err := client.CreateRun(ctx, port.OrcaCreateRunRequest{Objective: "issue 325 live settle: fencing run"}); err != nil {
		t.Fatalf("fence를 유발할 Run을 만들지 못했다: %v", err)
	}

	// 결함이 이 환경에 실재함을 먼저 고정한다. 이것이 없으면 fence가 나지
	// 않는 relay에서도 아래 단언이 조용히 통과해, 수정을 되돌려도 GREEN인
	// 테스트가 된다. 예전 SettleTask는 정확히 이 호출 하나였다.
	naive := client.UpdateTask(ctx, sealed.ID, task.ID, taskStatusCompleted, "")
	if !isConsumerFenced(naive) {
		t.Fatalf("바인딩이 떠난 Run의 task-update는 fence돼야 한다 — 이 관측이 없으면 아래 단언은 무의미하다: %v", naive)
	}

	if err := client.SettleTask(ctx, sealed.ID, task.ID); err != nil {
		t.Fatalf("봉인된 Run에 다시 바인딩해 종결시켜야 한다: %v", err)
	}

	// 성공 반환만으로는 부족하다. 서버가 실제로 종결을 기록했는지 되읽는다.
	settled, err := client.ListAllTasks(ctx)
	if err != nil {
		t.Fatalf("task를 되읽지 못했다: %v", err)
	}
	for _, candidate := range settled {
		if candidate.ID != task.ID {
			continue
		}
		if candidate.Status != taskStatusCompleted {
			t.Fatalf("종결이 서버에 반영돼야 한다: status=%q", candidate.Status)
		}
		t.Logf("live readback ok: run=%s task=%s status=%s", sealed.ID, task.ID, candidate.Status)
		return
	}
	t.Fatalf("종결시킨 task를 인벤토리에서 찾지 못했다: %s", task.ID)
}
