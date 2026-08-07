package active

import (
	"testing"

	model "agent-harness/internal/contract/issueops"
)

func TestUmbrellaCycleForChildIssueFindsLinkingParent(t *testing.T) {
	store := newActiveTestStore(t)
	repo := t.TempDir()
	store.writeRecord(t, model.IssueOpsRecord{
		ID:       "io-umbrella",
		OK:       true,
		Repo:     repo,
		Branch:   "78-umbrella",
		IssueURL: "https://github.com/example/repo/issues/78",
		Phase:    model.IssueOpsPhasePlan,
		IssueLinks: []model.IssueOpsIssueLink{
			{Type: "child", URL: "https://github.com/example/repo/issues/79"},
		},
	})

	got, ok := UmbrellaCycleForChildIssue(store.issueOpsStore(), repo, "https://github.com/example/repo/issues/79")
	if !ok || got.Branch != "78-umbrella" {
		t.Fatalf("UmbrellaCycleForChildIssue() = %+v, %v; want the linking umbrella cycle", got, ok)
	}
}

// 우산 사이클 자신은 자기 부모가 아니다. 우산의 IssueURL은 자기 자식 링크에
// 나타나지 않으므로 역조회가 자신을 잡아 base_branch를 자기 브랜치로 강요하는
// 일이 없어야 한다 — 그러면 우산이 자체 PR로 main에 합류할 수 없다(#129 AC-04).
func TestUmbrellaCycleForChildIssueDoesNotMatchItself(t *testing.T) {
	store := newActiveTestStore(t)
	repo := t.TempDir()
	store.writeRecord(t, model.IssueOpsRecord{
		ID:       "io-umbrella",
		OK:       true,
		Repo:     repo,
		Branch:   "78-umbrella",
		IssueURL: "https://github.com/example/repo/issues/78",
		Phase:    model.IssueOpsPhasePlan,
		IssueLinks: []model.IssueOpsIssueLink{
			{Type: "child", URL: "https://github.com/example/repo/issues/79"},
		},
	})

	if got, ok := UmbrellaCycleForChildIssue(store.issueOpsStore(), repo, "https://github.com/example/repo/issues/78"); ok {
		t.Fatalf("an umbrella cycle must not resolve as its own parent: %+v", got)
	}
}

// child가 아닌 링크 종류는 부모 관계가 아니다.
func TestUmbrellaCycleForChildIssueIgnoresNonChildLinks(t *testing.T) {
	store := newActiveTestStore(t)
	repo := t.TempDir()
	store.writeRecord(t, model.IssueOpsRecord{
		ID:       "io-related",
		OK:       true,
		Repo:     repo,
		Branch:   "78-related",
		IssueURL: "https://github.com/example/repo/issues/78",
		Phase:    model.IssueOpsPhasePlan,
		IssueLinks: []model.IssueOpsIssueLink{
			{Type: "depends-on", URL: "https://github.com/example/repo/issues/79"},
		},
	})

	if got, ok := UmbrellaCycleForChildIssue(store.issueOpsStore(), repo, "https://github.com/example/repo/issues/79"); ok {
		t.Fatalf("a depends-on link must not establish umbrella topology: %+v", got)
	}
}

// 우산이 정리된 뒤에는 대조 기준이 없다. done 사이클은 잡지 않는다.
func TestUmbrellaCycleForChildIssueSkipsDoneParents(t *testing.T) {
	store := newActiveTestStore(t)
	repo := t.TempDir()
	store.writeRecord(t, model.IssueOpsRecord{
		ID:       "io-umbrella",
		OK:       true,
		Repo:     repo,
		Branch:   "78-umbrella",
		IssueURL: "https://github.com/example/repo/issues/78",
		Phase:    model.IssueOpsPhaseDone,
		IssueLinks: []model.IssueOpsIssueLink{
			{Type: "child", URL: "https://github.com/example/repo/issues/79"},
		},
	})

	if got, ok := UmbrellaCycleForChildIssue(store.issueOpsStore(), repo, "https://github.com/example/repo/issues/79"); ok {
		t.Fatalf("a done umbrella must not strand its children: %+v", got)
	}
}

func TestUmbrellaCycleForChildIssueRejectsBlankChildURL(t *testing.T) {
	store := newActiveTestStore(t)
	if got, ok := UmbrellaCycleForChildIssue(store.issueOpsStore(), t.TempDir(), "  "); ok {
		t.Fatalf("a blank child url must not resolve an umbrella: %+v", got)
	}
}
