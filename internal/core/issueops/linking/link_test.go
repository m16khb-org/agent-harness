package linking

import (
	"os"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
)

type linkStoreForTest struct {
	records map[string]model.IssueOpsRecord
}

func newLinkStoreForTest(records ...model.IssueOpsRecord) (*linkStoreForTest, Store) {
	store := &linkStoreForTest{records: map[string]model.IssueOpsRecord{}}
	for _, record := range records {
		store.records[record.ID] = record
	}
	return store, Store{Read: store.read, TouchWrite: store.touchWrite}
}

func (s *linkStoreForTest) read(_ string, id string) (model.IssueOpsRecord, error) {
	record, ok := s.records[id]
	if !ok {
		return model.IssueOpsRecord{OK: false, ID: id}, os.ErrNotExist
	}
	record.OK = true
	return record, nil
}

func (s *linkStoreForTest) touchWrite(_ string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
	record.OK = true
	s.records[record.ID] = record
	return record, nil
}

func TestLinkChildPersistsProviderNeutralGraph(t *testing.T) {
	parent := model.IssueOpsRecord{
		ID:       "io-parent",
		Repo:     "/repo/example",
		Branch:   "1-demo",
		Phase:    model.IssueOpsPhasePlan,
		IssueURL: "https://github.com/example/repo/issues/10",
	}
	gitlab := model.IssueOpsRecord{
		ID:       "io-gitlab",
		Repo:     "/repo/gitlab",
		Branch:   "20-gitlab",
		Phase:    model.IssueOpsPhasePlan,
		IssueURL: "https://gitlab.example/group/project/-/issues/20",
	}
	generic := model.IssueOpsRecord{
		ID:       "io-generic",
		Repo:     "/repo/generic",
		Branch:   "10-generic",
		Phase:    model.IssueOpsPhasePlan,
		IssueURL: "https://tracker.example/acme/repo/issues/10",
	}
	linkStore, store := newLinkStoreForTest(parent, gitlab, generic)
	stateRoot := t.TempDir()

	record, err := LinkChild(store, stateRoot, parent.ID, "https://github.com/example/repo/issues/11", "write child graph tests")
	if err != nil {
		t.Fatal(err)
	}
	if len(record.IssueLinks) != 1 {
		t.Fatalf("expected one child issue link, got %+v", record.IssueLinks)
	}
	link := record.IssueLinks[0]
	if link.Type != "child" || link.URL != "https://github.com/example/repo/issues/11" || link.Title != "write child graph tests" || link.Provider != "github" {
		t.Fatalf("unexpected child issue link: %+v", link)
	}
	if link.CreatedAt == "" {
		t.Fatalf("child issue link should record created_at: %+v", link)
	}

	reloaded := linkStore.records[parent.ID]
	if len(reloaded.IssueLinks) != 1 || reloaded.IssueLinks[0].URL != link.URL {
		t.Fatalf("reloaded child issue links mismatch: %+v", reloaded.IssueLinks)
	}
	if _, err := LinkChild(store, stateRoot, parent.ID, link.URL, "duplicate"); err == nil || !strings.Contains(err.Error(), "already linked") {
		t.Fatalf("expected duplicate child link rejection, got %v", err)
	}
	if _, err := LinkChild(store, stateRoot, parent.ID, "https://tracker.example/acme/repo/issues/12", "generic tracker child"); err == nil || !strings.Contains(err.Error(), "provider") {
		t.Fatalf("generic child under GitHub parent should be rejected as provider mismatch, got %v", err)
	}
	if _, err := LinkChild(store, stateRoot, parent.ID, "https://github.com/other/repo/issues/12", "other repo child"); err == nil || !strings.Contains(err.Error(), "parent issue project") {
		t.Fatalf("GitHub child from another repo should be rejected, got %v", err)
	}
	if _, err := LinkChild(store, stateRoot, parent.ID, "https://github.com/example/repo/issues/not-a-number", "bad child"); err == nil || !strings.Contains(err.Error(), "numeric github issue URL") {
		t.Fatalf("GitHub child with nonnumeric issue should be rejected, got %v", err)
	}
	if _, err := LinkChild(store, stateRoot, gitlab.ID, "https://gitlab.example/other/project/-/issues/21", "other project child"); err == nil || !strings.Contains(err.Error(), "parent issue project") {
		t.Fatalf("GitLab child from another project should be rejected, got %v", err)
	}
	if _, err := LinkChild(store, stateRoot, gitlab.ID, "https://gitlab.example/group/project/-/issues/not-a-number", "bad child"); err == nil || !strings.Contains(err.Error(), "numeric gitlab issue URL") {
		t.Fatalf("GitLab child with nonnumeric issue should be rejected, got %v", err)
	}
	if _, err := LinkChild(store, stateRoot, gitlab.ID, "https://gitlab.example/group/project/-/issues/21", "same project child"); err != nil {
		t.Fatalf("GitLab child in same project should be accepted: %v", err)
	}
	generic, err = LinkChild(store, stateRoot, generic.ID, "https://tracker.example/acme/repo/issues/12", "generic tracker child")
	if err != nil {
		t.Fatal(err)
	}
	if got := generic.IssueLinks[0].Provider; got != "" {
		t.Fatalf("generic issue URL should not infer a provider, got %q", got)
	}
	if _, err := LinkChild(store, stateRoot, parent.ID, "not-a-url", "bad"); err == nil || !strings.Contains(err.Error(), "child_url") {
		t.Fatalf("expected child URL validation error, got %v", err)
	}
}
