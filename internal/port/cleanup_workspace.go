package port

import issueopscontract "agent-harness/internal/contract/issueops"

// CleanupWorkspaceOccupancy는 cleanup finish/abandon이 한 번의 관측으로 얻는
// 워크트리 점유 상태다. 점유자와 요청자(현재 프로세스)의 계보를 같은 프로세스
// 스냅샷에서 읽어야 요청자 거부와 PID 재사용 방어가 서로 모순되지 않는다(#477).
type CleanupWorkspaceOccupancy struct {
	Occupants []issueopscontract.CleanupWorkspaceProcess `json:"occupants"`
	// Ancestry는 점유자와 요청자 pid의 조상 pid를 가까운 순서로 담는다. 요청자
	// 항목이 없으면 관측 실패이며 게이트는 fail-closed다.
	Ancestry map[int][]int `json:"-"`
}
