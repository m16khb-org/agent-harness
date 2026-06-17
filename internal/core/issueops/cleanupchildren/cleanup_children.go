package cleanupchildren

import (
	"fmt"
	"strings"
	"time"

	"agent-harness/internal/core/issueops/model"
	"agent-harness/internal/core/issueops/remote"
	"agent-harness/internal/port"
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
	if !req.Merged {
		result.Missing = []string{"merge_evidence"}
		return result, fmt.Errorf("cannot close child tasks without merge evidence")
	}
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
