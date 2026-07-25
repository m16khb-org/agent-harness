package cleanupchildren

import (
	"fmt"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/remote"
	"agent-harness/internal/port"
)

const (
	evidenceParentMergeVerified   = "parent_merge_verified"
	evidenceChildrenAlreadyClosed = "children_already_closed"
)

type Store struct {
	Read       func(stateRoot, id string) (model.IssueOpsRecord, error)
	TouchWrite func(stateRoot string, record model.IssueOpsRecord) (model.IssueOpsRecord, error)
	Provider   func(provider string) (port.IssueProvider, error)
}

func ByID(store Store, stateRoot, id string, req model.IssueOpsCloseChildrenRequest) (model.IssueOpsCloseChildrenResult, error) {
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return model.IssueOpsCloseChildrenResult{OK: false, ID: id}, err
	}
	result := model.IssueOpsCloseChildrenResult{
		OK:        true,
		ID:        record.ID,
		Merged:    req.Merged,
		Confirmed: req.Confirm,
		DryRun:    !req.Confirm,
	}
	if strings.TrimSpace(record.IssueURL) == "" && (req.Merged || req.MergeEvidenceRequested) {
		result.Missing = []string{"parent_issue"}
		return result, fmt.Errorf("cannot close child tasks before linked parent issue")
	}
	basis, err := resolveEvidenceBasis(store, record, req)
	if err != nil {
		result.Missing = []string{"merge_evidence"}
		return result, err
	}
	result.EvidenceBasis = basis
	if strings.TrimSpace(record.IssueURL) == "" {
		result.Missing = []string{"parent_issue"}
		return result, fmt.Errorf("cannot close child tasks before linked parent issue")
	}

	changed := false
	for index, link := range record.IssueLinks {
		if link.Type != "child" {
			continue
		}
		childResult, linkChanged, err := closeChild(store, record, link, req)
		result.Children = append(result.Children, childResult)
		if err != nil {
			return result, err
		}
		if linkChanged {
			now := time.Now().UTC().Format(time.RFC3339Nano)
			if strings.TrimSpace(record.IssueLinks[index].ClosedAt) == "" {
				record.IssueLinks[index].ClosedAt = now
			}
			record.IssueLinks[index].CloseVerifiedAt = now
			record.IssueLinks[index].CloseReason = "completed"
			changed = true
		}
		if childResult.Closed {
			result.ClosedCount++
		}
	}
	if req.Confirm && changed {
		if _, err := store.TouchWrite(stateRoot, record); err != nil {
			return result, err
		}
	}
	return result, nil
}

// resolveEvidenceBasis는 이 정리 실행이 무엇을 근거로 진행될 수 있는지를
// 판정한다.
//
// 우선순위는 부모 머지 증거다. 그것이 없을 때만 대체 증거를 찾는다: 위상 규약
// (#129) 이전에 만들어진 우산 레코드는 자체 브랜치와 PR을 갖지 않아 부모 머지를
// 증명할 방법이 없고, 그 결과 abandon / close-children / finish 세 경로가 서로를
// 막아 레코드가 영구 잔여물이 됐다.
//
// 대체 증거는 "모든 자식이 원격에서 이미 닫혀 있다"는 관측이다. close-children의
// 목적이 고아 자식 방지이므로, 이미 닫힌 자식은 이 게이트가 보호하려는 대상이
// 아니다. 관측은 provider의 preview 경로로 하며 읽기 전용이다.
//
// 상태를 관측하지 못하면 통과가 아니라 거부다. 미상을 통과로 다루면 네트워크
// 실패 한 번으로 게이트가 무력해진다.
func resolveEvidenceBasis(store Store, record model.IssueOpsRecord, req model.IssueOpsCloseChildrenRequest) (string, error) {
	if req.Merged {
		return evidenceParentMergeVerified, nil
	}
	if !req.MergeEvidenceRequested {
		return "", fmt.Errorf("cannot close child tasks without merge evidence")
	}
	if record.RemoteArtifact != nil {
		// 부모가 원격 artifact를 가졌는데도 머지가 검증되지 않았다면 그것이
		// 진짜 결론이다. 대체 증거로 우회해서는 안 된다.
		return "", fmt.Errorf("cannot close child tasks without merge evidence: parent artifact is not verified merged")
	}
	for _, link := range record.IssueLinks {
		if link.Type != "child" {
			continue
		}
		state, err := observeChildState(store, record, link)
		if err != nil {
			return "", fmt.Errorf("cannot close child tasks without merge evidence: %w", err)
		}
		if !strings.EqualFold(state, "closed") {
			return "", fmt.Errorf("cannot close child tasks without merge evidence: child %s is %s remotely",
				link.URL, childStateForMessage(state))
		}
	}
	return evidenceChildrenAlreadyClosed, nil
}

// observeChildState는 provider preview로 자식의 현재 원격 상태를 읽는다.
// 빈 문자열은 관측 실패다.
func observeChildState(store Store, record model.IssueOpsRecord, link model.IssueOpsIssueLink) (string, error) {
	providerName := firstNonEmpty(link.Provider, remote.ProviderFromURL(link.URL), remote.ProviderFromURL(record.IssueURL))
	if providerName == "" {
		return "", fmt.Errorf("cannot determine provider for child %s", link.URL)
	}
	prov, err := store.Provider(providerName)
	if err != nil {
		return "", err
	}
	result, err := prov.CloseChild(port.IssueProviderCloseChildRequest{
		Repo:           record.Repo,
		ParentIssueURL: record.IssueURL,
		ChildURL:       link.URL,
		Confirm:        false,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.State), nil
}

func childStateForMessage(state string) string {
	if strings.TrimSpace(state) == "" {
		return "unobserved"
	}
	return state
}

func closeChild(store Store, record model.IssueOpsRecord, link model.IssueOpsIssueLink, req model.IssueOpsCloseChildrenRequest) (model.IssueOpsCloseChildResult, bool, error) {
	providerName := firstNonEmpty(link.Provider, remote.ProviderFromURL(link.URL), remote.ProviderFromURL(record.IssueURL))
	if providerName == "" {
		return model.IssueOpsCloseChildResult{URL: link.URL, Error: "cannot determine provider"}, false, fmt.Errorf("cannot determine provider for child %s", link.URL)
	}
	prov, err := store.Provider(providerName)
	if err != nil {
		return model.IssueOpsCloseChildResult{URL: link.URL, Provider: providerName, Error: err.Error()}, false, err
	}
	providerResult, err := prov.CloseChild(port.IssueProviderCloseChildRequest{
		Repo:           record.Repo,
		ParentIssueURL: record.IssueURL,
		ChildURL:       link.URL,
		Confirm:        req.Confirm,
	})
	childResult := model.IssueOpsCloseChildResult{
		URL:               firstNonEmpty(providerResult.ChildURL, link.URL),
		Provider:          firstNonEmpty(providerResult.Provider, providerName),
		Closed:            providerResult.Closed,
		AlreadyClosed:     providerResult.AlreadyClosed,
		HierarchyVerified: providerResult.HierarchyVerified,
		State:             providerResult.State,
		Preview:           providerResult.Preview,
	}
	if err != nil {
		childResult.Error = err.Error()
		return childResult, false, err
	}
	if req.Confirm && (!providerResult.HierarchyVerified || !providerResult.Closed) {
		err := fmt.Errorf("provider did not verify child close for %s", link.URL)
		childResult.Error = err.Error()
		return childResult, false, err
	}
	return childResult, req.Confirm, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
