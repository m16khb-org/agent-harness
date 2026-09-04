package cleanupchildren

import (
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// 위상 규약 이전에 만들어진 우산 레코드는 자체 PR을 갖지 않아 부모 머지 증거를
// 만들 수 없다. 그 사이클의 자식이 원격에서 이미 모두 닫혀 있다면 고아 자식
// 위험이 없으므로 정리가 진행되어야 한다(#129 AC-05).
func TestCloseChildrenAcceptsAlreadyClosedChildrenWithoutParentMergeEvidence(t *testing.T) {
	record := model.IssueOpsRecord{
		ID:       "io-legacy-umbrella",
		Repo:     "/repo",
		IssueURL: "https://github.com/acme/repo/issues/78",
		IssueLinks: []model.IssueOpsIssueLink{
			{Type: "child", URL: "https://github.com/acme/repo/issues/79", Provider: "github"},
			{Type: "child", URL: "https://github.com/acme/repo/issues/80", Provider: "github"},
		},
	}
	store := newCloseChildrenStoreForTest(record)
	store.provider.previewState = "closed"

	result, err := ByID(store.store(), t.TempDir(), record.ID, model.IssueOpsCloseChildrenRequest{
		Merged:                 false,
		MergeEvidenceRequested: true,
		Confirm:                true,
	})
	if err != nil {
		t.Fatalf("children already closed remotely must satisfy the cleanup gate: %v", err)
	}
	if !result.OK || result.ClosedCount != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.EvidenceBasis != "children_already_closed" {
		t.Fatalf("the result must name what it passed on, got %q", result.EvidenceBasis)
	}
	for _, link := range store.records[record.ID].IssueLinks {
		if link.CloseVerifiedAt == "" {
			t.Fatalf("close evidence not recorded for %s: %+v", link.URL, link)
		}
	}
}

// 자식이 하나라도 원격에서 열려 있으면 차단은 유지된다. 열린 자식을 부모 정리와
// 함께 닫으면 그 작업이 검토 없이 사라진다(#129 AC-06).
func TestCloseChildrenRejectsOpenChildWithoutParentMergeEvidence(t *testing.T) {
	record := model.IssueOpsRecord{
		ID:       "io-legacy-umbrella",
		Repo:     "/repo",
		IssueURL: "https://github.com/acme/repo/issues/78",
		IssueLinks: []model.IssueOpsIssueLink{
			{Type: "child", URL: "https://github.com/acme/repo/issues/79", Provider: "github"},
			{Type: "child", URL: "https://github.com/acme/repo/issues/80", Provider: "github"},
		},
	}
	store := newCloseChildrenStoreForTest(record)
	store.provider.previewState = "closed"
	store.provider.previewStateByURL = map[string]string{
		"https://github.com/acme/repo/issues/80": "open",
	}

	result, err := ByID(store.store(), t.TempDir(), record.ID, model.IssueOpsCloseChildrenRequest{
		Merged:                 false,
		MergeEvidenceRequested: true,
		Confirm:                true,
	})
	if err == nil || !strings.Contains(err.Error(), "merge evidence") {
		t.Fatalf("an open child must keep the gate closed, got result=%+v err=%v", result, err)
	}
	if got := store.records[record.ID].IssueLinks[0].CloseVerifiedAt; got != "" {
		t.Fatalf("a rejected run must not record close evidence: %+v", store.records[record.ID].IssueLinks[0])
	}
}

// 원격 상태를 읽지 못하면 통과가 아니라 거부다. 미상을 통과로 다루면 게이트가
// 네트워크 실패 한 번으로 무력해진다.
func TestCloseChildrenRejectsUnknownChildStateWithoutParentMergeEvidence(t *testing.T) {
	record := model.IssueOpsRecord{
		ID:       "io-legacy-umbrella",
		Repo:     "/repo",
		IssueURL: "https://github.com/acme/repo/issues/78",
		IssueLinks: []model.IssueOpsIssueLink{
			{Type: "child", URL: "https://github.com/acme/repo/issues/79", Provider: "github"},
		},
	}
	store := newCloseChildrenStoreForTest(record)
	store.provider.previewState = ""

	if _, err := ByID(store.store(), t.TempDir(), record.ID, model.IssueOpsCloseChildrenRequest{
		Merged:                 false,
		MergeEvidenceRequested: true,
		Confirm:                true,
	}); err == nil || !strings.Contains(err.Error(), "merge evidence") {
		t.Fatalf("an unobserved child state must not open the gate, got %v", err)
	}
}

// --merged를 요청하지 않은 호출은 원격을 조회하지 않는다. 대체 증거 탐색은
// 운영자가 정리 의도를 명시했을 때만 일어난다.
func TestCloseChildrenWithoutRequestDoesNotProbeRemote(t *testing.T) {
	record := model.IssueOpsRecord{
		ID:       "io-legacy-umbrella",
		Repo:     "/repo",
		IssueURL: "https://github.com/acme/repo/issues/78",
		IssueLinks: []model.IssueOpsIssueLink{
			{Type: "child", URL: "https://github.com/acme/repo/issues/79", Provider: "github"},
		},
	}
	store := newCloseChildrenStoreForTest(record)
	store.provider.previewState = "closed"

	if _, err := ByID(store.store(), t.TempDir(), record.ID, model.IssueOpsCloseChildrenRequest{
		Merged: false, MergeEvidenceRequested: false, Confirm: true,
	}); err == nil {
		t.Fatal("expected the merge evidence gate to reject the request")
	}
	if len(store.provider.calls) != 0 {
		t.Fatalf("provider must not be probed without an explicit cleanup request: %+v", store.provider.calls)
	}
}

// 부모 머지 증거가 검증된 경우에는 종전 경로가 그대로 쓰인다.
func TestCloseChildrenKeepsParentMergeEvidenceBasis(t *testing.T) {
	record := model.IssueOpsRecord{
		ID:       "io-umbrella",
		Repo:     "/repo",
		IssueURL: "https://github.com/acme/repo/issues/78",
		IssueLinks: []model.IssueOpsIssueLink{
			{Type: "child", URL: "https://github.com/acme/repo/issues/79", Provider: "github"},
		},
	}
	store := newCloseChildrenStoreForTest(record)
	store.provider.result = port.IssueProviderCloseChildResult{
		OK: true, Provider: "github", HierarchyVerified: true, Closed: true, State: "closed",
	}

	result, err := ByID(store.store(), t.TempDir(), record.ID, model.IssueOpsCloseChildrenRequest{
		Merged: true, MergeEvidenceRequested: true, Confirm: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.EvidenceBasis != "parent_merge_verified" {
		t.Fatalf("verified parent merge must stay the recorded basis, got %q", result.EvidenceBasis)
	}
}
