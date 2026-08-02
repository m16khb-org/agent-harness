package branchprepare

import (
	"fmt"
	"strings"
	"testing"

	model "agent-harness/internal/contract/issueops"
)

// umbrellaStore는 자식 사이클 하나와 그 부모 우산 사이클 하나를 갖는 최소 스토어다.
type umbrellaStore struct {
	child    model.IssueOpsRecord
	umbrella model.IssueOpsRecord
	// lookups는 역조회가 실제로 자식 이슈 URL로 호출됐는지 검증한다.
	lookups []string
}

func (s *umbrellaStore) store() Store {
	return Store{
		Read: func(string, string) (model.IssueOpsRecord, error) {
			return s.child, nil
		},
		TouchWrite: func(_ string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			s.child = record
			return record, nil
		},
		ValidateIssueURL: func(issueURL string) error {
			if strings.TrimSpace(issueURL) == "" {
				return fmt.Errorf("issue_url is required")
			}
			return nil
		},
		UmbrellaForChildIssue: func(repo, childIssueURL string) (model.IssueOpsRecord, bool) {
			s.lookups = append(s.lookups, repo+" "+childIssueURL)
			for _, link := range s.umbrella.IssueLinks {
				if link.Type == "child" && strings.TrimSpace(link.URL) == strings.TrimSpace(childIssueURL) {
					return s.umbrella, true
				}
			}
			return model.IssueOpsRecord{}, false
		},
	}
}

func newUmbrellaStore() *umbrellaStore {
	return &umbrellaStore{
		child: model.IssueOpsRecord{
			ID:       "io-child",
			OK:       true,
			Repo:     "/repo/example",
			Branch:   "79-child-task",
			IssueURL: "https://github.com/example/repo/issues/79",
		},
		umbrella: model.IssueOpsRecord{
			ID:       "io-umbrella",
			OK:       true,
			Repo:     "/repo/example",
			Branch:   "78-umbrella",
			IssueURL: "https://github.com/example/repo/issues/78",
			IssueLinks: []model.IssueOpsIssueLink{
				{Type: "child", URL: "https://github.com/example/repo/issues/79"},
			},
		},
	}
}

func TestPrepareRejectsChildBaseBranchOutsideUmbrella(t *testing.T) {
	store := newUmbrellaStore()

	_, err := Prepare(store.store(), t.TempDir(), "io-child", model.IssueOpsBranchPrepareRequest{
		Provider:   "github",
		IssueURL:   "https://github.com/example/repo/issues/79",
		Branch:     "79-child-task",
		BaseBranch: "main",
	})
	if err == nil {
		t.Fatal("a child cycle must not branch from main while its umbrella cycle owns a branch")
	}
	if !strings.Contains(err.Error(), "78-umbrella") {
		t.Fatalf("error %q must name the umbrella branch the child has to use", err)
	}
	if len(store.lookups) == 0 {
		t.Fatal("branch prepare must look the umbrella cycle up by the child issue url")
	}
}

func TestPrepareAcceptsChildBaseBranchOnUmbrella(t *testing.T) {
	store := newUmbrellaStore()

	record, err := Prepare(store.store(), t.TempDir(), "io-child", model.IssueOpsBranchPrepareRequest{
		Provider:   "github",
		IssueURL:   "https://github.com/example/repo/issues/79",
		Branch:     "79-child-task",
		BaseBranch: "78-umbrella",
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.BranchPrepare == nil || record.BranchPrepare.BaseBranch != "78-umbrella" {
		t.Fatalf("child branch prepare must record the umbrella base: %+v", record.BranchPrepare)
	}
}

// 부모를 찾지 못하면 통과한다. 자식이 아니거나 우산이 이미 정리된 경우이며,
// 근거를 잃은 검증이 일상 사이클을 막아서는 안 된다.
func TestPrepareAllowsCycleWithoutUmbrella(t *testing.T) {
	store := newUmbrellaStore()
	store.umbrella.IssueLinks = nil

	if _, err := Prepare(store.store(), t.TempDir(), "io-child", model.IssueOpsBranchPrepareRequest{
		Provider:   "github",
		IssueURL:   "https://github.com/example/repo/issues/79",
		Branch:     "79-child-task",
		BaseBranch: "main",
	}); err != nil {
		t.Fatalf("a cycle with no umbrella parent must keep its own base branch choice: %v", err)
	}
}

// 역조회 표면이 주입되지 않은 호출자는 종전대로 동작한다.
func TestPrepareWithoutUmbrellaLookupIsUnchanged(t *testing.T) {
	store := newBranchPrepareTestStore(model.IssueOpsRecord{
		ID:       "io-plain",
		OK:       true,
		Repo:     "/repo/example",
		Branch:   "456-plain-cycle",
		IssueURL: "https://github.com/example/repo/issues/456",
	})

	if _, err := Prepare(store.issueOpsStore(), t.TempDir(), "io-plain", model.IssueOpsBranchPrepareRequest{
		Provider:   "github",
		IssueURL:   "https://github.com/example/repo/issues/456",
		Branch:     "456-plain-cycle",
		BaseBranch: "main",
	}); err != nil {
		t.Fatalf("nil umbrella lookup must not block branch prepare: %v", err)
	}
}
