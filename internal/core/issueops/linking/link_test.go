package linking

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"agent-harness/internal/core/issueops/model"
)

type linkStoreForTest struct {
	records               map[string]model.IssueOpsRecord
	branchEvidenceMissing []string
	designReviewMissing   []string
}

func newLinkStoreForTest(records ...model.IssueOpsRecord) (*linkStoreForTest, Store) {
	store := &linkStoreForTest{records: map[string]model.IssueOpsRecord{}}
	for _, record := range records {
		store.records[record.ID] = record
	}
	return store, Store{
		Read:                   store.read,
		TouchWrite:             store.touchWrite,
		PlanReadiness:          store.planReadiness,
		PhaseRank:              model.IssueOpsPhaseRank,
		BranchEvidenceMissing:  store.branchEvidenceMissingFor,
		DesignReviewMissing:    store.designReviewMissingFor,
		PlanPathExists:         store.planPathExists,
		PlanPathInsideWorktree: store.planPathInsideWorktree,
		WorktreePathValid:      store.worktreePathValid,
		UniqueSorted:           uniqueSortedForTest,
	}
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

func (s *linkStoreForTest) planReadiness(record model.IssueOpsRecord) model.IssueOpsReadiness {
	return model.IssueOpsReadiness{OK: true, Ready: strings.TrimSpace(record.IssueURL) != ""}
}

func (s *linkStoreForTest) branchEvidenceMissingFor(model.IssueOpsRecord) []string {
	return append([]string(nil), s.branchEvidenceMissing...)
}

func (s *linkStoreForTest) designReviewMissingFor(model.IssueOpsRecord) []string {
	return append([]string(nil), s.designReviewMissing...)
}

func (s *linkStoreForTest) planPathExists(_ string, path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (s *linkStoreForTest) planPathInsideWorktree(worktree, planPath string) bool {
	rel, err := filepath.Rel(worktree, planPath)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (s *linkStoreForTest) worktreePathValid(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func uniqueSortedForTest(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func TestLinkIssuePersistsURLAndAdvancesReadyRecord(t *testing.T) {
	record := model.IssueOpsRecord{
		ID:     "io-link-issue",
		Repo:   "/repo/example",
		Branch: "feature/link-issue",
		Phase:  model.IssueOpsPhaseProblem,
	}
	linkStore, store := newLinkStoreForTest(record)

	got, err := LinkIssue(store, t.TempDir(), record.ID, " https://github.com/example/repo/issues/10 ")
	if err != nil {
		t.Fatal(err)
	}
	if got.IssueURL != "https://github.com/example/repo/issues/10" {
		t.Fatalf("IssueURL=%q", got.IssueURL)
	}
	if got.Phase != model.IssueOpsPhasePlan {
		t.Fatalf("Phase=%q, want %q", got.Phase, model.IssueOpsPhasePlan)
	}
	if reloaded := linkStore.records[record.ID]; reloaded.IssueURL != got.IssueURL || reloaded.Phase != got.Phase {
		t.Fatalf("persisted record mismatch: %+v", reloaded)
	}
}

func TestLinkIssueRejectsInvalidURL(t *testing.T) {
	_, store := newLinkStoreForTest(model.IssueOpsRecord{ID: "io-bad-url"})
	if _, err := LinkIssue(store, t.TempDir(), "io-bad-url", "not-a-url"); err == nil || !strings.Contains(err.Error(), "http(s) URL") {
		t.Fatalf("expected issue URL validation error, got %v", err)
	}
}

func TestLinkPlanValidatesReadinessAndPersistsAbsolutePath(t *testing.T) {
	repo, worktree := issueOpsRepoAndWorktreeFixture(t, "feature/plan")
	planDir := filepath.Join(worktree, "docs")
	if err := os.MkdirAll(planDir, 0o700); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(planDir, "plan.md")
	if err := os.WriteFile(planPath, []byte("# plan\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	record := model.IssueOpsRecord{
		ID:           "io-link-plan",
		Repo:         repo,
		Branch:       "feature/plan",
		Phase:        model.IssueOpsPhasePlan,
		IssueURL:     "https://github.com/example/repo/issues/10",
		WorktreePath: worktree,
	}
	_, store := newLinkStoreForTest(record)

	got, err := LinkPlan(store, t.TempDir(), record.ID, filepath.Join("docs", "plan.md"))
	if err != nil {
		t.Fatal(err)
	}
	if got.PlanPath != planPath {
		t.Fatalf("PlanPath=%q, want %q", got.PlanPath, planPath)
	}
	if got.Phase != model.IssueOpsPhasePlan {
		t.Fatalf("Phase=%q, want %q", got.Phase, model.IssueOpsPhasePlan)
	}
}

func TestLinkPlanRejectsBoundaryViolations(t *testing.T) {
	repo, worktree := issueOpsRepoAndWorktreeFixture(t, "feature/plan-boundary")
	outsidePlanPath := filepath.Join(filepath.Dir(worktree), "outside-plan.md")
	if err := os.WriteFile(outsidePlanPath, []byte("# outside plan\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	record := model.IssueOpsRecord{
		ID:           "io-link-plan-boundary",
		Repo:         repo,
		Branch:       "feature/plan-boundary",
		Phase:        model.IssueOpsPhasePlan,
		IssueURL:     "https://github.com/example/repo/issues/10",
		WorktreePath: worktree,
	}
	for _, tc := range []struct {
		name    string
		path    string
		mutate  func(*linkStoreForTest)
		wantErr string
	}{
		{name: "empty path", path: " ", wantErr: "plan_path is required"},
		{name: "path traversal", path: "../plan.md", wantErr: "path traversal"},
		{name: "missing branch evidence", path: "plan.md", mutate: func(s *linkStoreForTest) {
			s.branchEvidenceMissing = []string{"branch_exists"}
		}, wantErr: "before branch evidence"},
		{name: "missing design review", path: "plan.md", mutate: func(s *linkStoreForTest) {
			s.designReviewMissing = []string{"risks", "design_approval", "risks"}
		}, wantErr: "design_approval, risks"},
		{name: "missing file", path: "missing.md", wantErr: "plan_path does not exist"},
		{name: "absolute path outside worktree", path: outsidePlanPath, wantErr: "plan_path must be inside linked worktree"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			linkStore, store := newLinkStoreForTest(record)
			if tc.mutate != nil {
				tc.mutate(linkStore)
			}
			_, err := LinkPlan(store, t.TempDir(), record.ID, tc.path)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error=%v, want substring %q", err, tc.wantErr)
			}
		})
	}

	noWorktree := record
	noWorktree.ID = "io-no-worktree"
	noWorktree.WorktreePath = ""
	_, store := newLinkStoreForTest(noWorktree)
	if _, err := LinkPlan(store, t.TempDir(), noWorktree.ID, "plan.md"); err == nil || !strings.Contains(err.Error(), "before linked worktree") {
		t.Fatalf("expected linked worktree error, got %v", err)
	}
}

func TestLinkWorktreeValidatesIsolationBranchAndExistingPlan(t *testing.T) {
	repo, worktree := issueOpsRepoAndWorktreeFixture(t, "feature/worktree")
	planPath := filepath.Join(worktree, "plan.md")
	if err := os.WriteFile(planPath, []byte("# plan\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	record := model.IssueOpsRecord{
		ID:       "io-link-worktree",
		Repo:     repo,
		Branch:   "feature/worktree",
		Phase:    model.IssueOpsPhasePlan,
		PlanPath: planPath,
	}
	_, store := newLinkStoreForTest(record)

	got, err := LinkWorktree(store, t.TempDir(), record.ID, worktree)
	if err != nil {
		t.Fatal(err)
	}
	if got.WorktreePath != worktree {
		t.Fatalf("WorktreePath=%q, want %q", got.WorktreePath, worktree)
	}

	otherRepo, otherWorktree := issueOpsRepoAndWorktreeFixture(t, "feature/other")
	otherPlan := filepath.Join(filepath.Dir(otherWorktree), "outside-plan.md")
	if err := os.WriteFile(otherPlan, []byte("# plan\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	badPlan := record
	badPlan.ID = "io-bad-plan"
	badPlan.Repo = otherRepo
	badPlan.Branch = "feature/other"
	badPlan.PlanPath = otherPlan
	_, store = newLinkStoreForTest(badPlan)
	if _, err := LinkWorktree(store, t.TempDir(), badPlan.ID, otherWorktree); err == nil || !strings.Contains(err.Error(), "plan_path must be inside linked worktree") {
		t.Fatalf("expected plan/worktree boundary error, got %v", err)
	}
}

func TestLinkWorktreeRejectsBoundaryViolations(t *testing.T) {
	repo, worktree := issueOpsRepoAndWorktreeFixture(t, "feature/worktree-boundary")
	record := model.IssueOpsRecord{
		ID:     "io-link-worktree-boundary",
		Repo:   repo,
		Branch: "feature/worktree-boundary",
		Phase:  model.IssueOpsPhasePlan,
	}
	for _, tc := range []struct {
		name    string
		path    string
		mutate  func(*linkStoreForTest)
		wantErr string
	}{
		{name: "empty path", path: " ", wantErr: "worktree_path is required"},
		{name: "path traversal", path: "../worktree", wantErr: "path traversal"},
		{name: "missing branch evidence", path: worktree, mutate: func(s *linkStoreForTest) {
			s.branchEvidenceMissing = []string{"branch_head"}
		}, wantErr: "before branch evidence"},
		{name: "missing directory", path: filepath.Join(filepath.Dir(worktree), "missing"), wantErr: "does not exist or is not a directory"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			linkStore, store := newLinkStoreForTest(record)
			if tc.mutate != nil {
				tc.mutate(linkStore)
			}
			_, err := LinkWorktree(store, t.TempDir(), record.ID, tc.path)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error=%v, want substring %q", err, tc.wantErr)
			}
		})
	}

	mismatchRepo, mismatchWorktree := issueOpsRepoAndWorktreeFixture(t, "feature/actual")
	mismatch := record
	mismatch.ID = "io-branch-mismatch"
	mismatch.Repo = mismatchRepo
	mismatch.Branch = "feature/expected"
	_, store := newLinkStoreForTest(mismatch)
	if _, err := LinkWorktree(store, t.TempDir(), mismatch.ID, mismatchWorktree); err == nil || !strings.Contains(err.Error(), "does not match IssueOps branch") {
		t.Fatalf("expected branch mismatch error, got %v", err)
	}
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
	if _, err := LinkChild(store, stateRoot, parent.ID, "https://github.com/example/repo/issues/not-a-number", "bad child"); err == nil || !strings.Contains(err.Error(), "numeric github issue or work item URL") {
		t.Fatalf("GitHub child with nonnumeric issue should be rejected, got %v", err)
	}
	if _, err := LinkChild(store, stateRoot, gitlab.ID, "https://gitlab.example/other/project/-/issues/21", "other project child"); err == nil || !strings.Contains(err.Error(), "parent issue project") {
		t.Fatalf("GitLab child from another project should be rejected, got %v", err)
	}
	if _, err := LinkChild(store, stateRoot, gitlab.ID, "https://gitlab.example/group/project/-/issues/not-a-number", "bad child"); err == nil || !strings.Contains(err.Error(), "numeric gitlab issue or work item URL") {
		t.Fatalf("GitLab child with nonnumeric issue should be rejected, got %v", err)
	}
	if _, err := LinkChild(store, stateRoot, gitlab.ID, "https://gitlab.example/group/project/-/issues/21", "same project child"); err != nil {
		t.Fatalf("GitLab child in same project should be accepted: %v", err)
	}
	gitlabWorkItem := model.IssueOpsRecord{
		ID:       "io-gitlab-work-item",
		Repo:     "/repo/gitlab",
		Branch:   "21-gitlab",
		Phase:    model.IssueOpsPhasePlan,
		IssueURL: "https://gitlab.example/group/project/-/issues/20",
	}
	_, workItemStore := newLinkStoreForTest(gitlabWorkItem)
	if _, err := LinkChild(workItemStore, stateRoot, gitlabWorkItem.ID, "https://gitlab.example/group/project/-/work_items/22", "same project work item child"); err != nil {
		t.Fatalf("GitLab child work item in same project should be accepted: %v", err)
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

func TestLinkRelatedPersistsProviderAndRejectsDuplicates(t *testing.T) {
	record := model.IssueOpsRecord{
		ID:       "io-related",
		Repo:     "/repo/example",
		Branch:   "feature/related",
		Phase:    model.IssueOpsPhasePlan,
		IssueURL: "https://github.com/example/repo/issues/10",
	}
	_, store := newLinkStoreForTest(record)

	got, err := LinkRelated(store, t.TempDir(), record.ID, " follows-up ", " https://github.com/example/repo/issues/11 ", " follow up ")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.IssueLinks) != 1 {
		t.Fatalf("IssueLinks=%+v", got.IssueLinks)
	}
	link := got.IssueLinks[0]
	if link.Type != "follows-up" || link.URL != "https://github.com/example/repo/issues/11" || link.Title != "follow up" || link.Provider != "github" {
		t.Fatalf("unexpected related link: %+v", link)
	}
	if link.CreatedAt == "" {
		t.Fatalf("related link should include CreatedAt: %+v", link)
	}
	if _, err := LinkRelated(store, t.TempDir(), record.ID, "follows-up", link.URL, "duplicate"); err == nil || !strings.Contains(err.Error(), "already linked") {
		t.Fatalf("expected duplicate related link rejection, got %v", err)
	}
	if _, err := LinkRelated(store, t.TempDir(), record.ID, "parent", "https://github.com/example/repo/issues/12", "bad type"); err == nil || !strings.Contains(err.Error(), "invalid link type") {
		t.Fatalf("expected invalid link type rejection, got %v", err)
	}
	if _, err := LinkRelated(store, t.TempDir(), record.ID, "blocks", "not-a-url", "bad url"); err == nil || !strings.Contains(err.Error(), "related_url") {
		t.Fatalf("expected related URL validation error, got %v", err)
	}
}

func TestValidateIssueURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid github https", "https://github.com/user/repo/issues/1", false},
		{"valid gitlab https", "https://gitlab.com/user/repo/-/issues/1", false},
		{"valid http", "http://example.com/issues/1", false},
		{"empty", "", true},
		{"not url", "not-a-url", true},
		{"no scheme", "github.com/user/repo/issues/1", true},
		{"ftp scheme", "ftp://example.com/issues/1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateIssueURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateIssueURL(%q) error = %v, wantErr = %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestValidateIsolatedWorktreePath(t *testing.T) {
	repo, worktree := issueOpsRepoAndWorktreeFixture(t, "feature/isolation")
	record := model.IssueOpsRecord{Repo: repo}
	if err := ValidateIsolatedWorktreePath(record, worktree); err != nil {
		t.Fatalf("valid worktree rejected: %v", err)
	}
	if err := ValidateIsolatedWorktreePath(record, repo); err == nil || !strings.Contains(err.Error(), "isolated") {
		t.Fatalf("expected source checkout isolation error, got %v", err)
	}
	outside := filepath.Join(filepath.Dir(repo), "outside")
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := ValidateIsolatedWorktreePath(record, outside); err == nil || !strings.Contains(err.Error(), "sibling worktree directory") {
		t.Fatalf("expected sibling worktree directory error, got %v", err)
	}
	symlink := filepath.Join(filepath.Dir(worktree), "linked")
	if err := os.Symlink(worktree, symlink); err != nil {
		t.Skipf("symlink fixture unavailable: %v", err)
	}
	if err := ValidateIsolatedWorktreePath(record, symlink); err == nil || !strings.Contains(err.Error(), "must not be a symlink") {
		t.Fatalf("expected symlink rejection, got %v", err)
	}
}

func TestValidateWorktreeBranch(t *testing.T) {
	_, worktree := issueOpsRepoAndWorktreeFixture(t, "feature/branch")
	if err := ValidateWorktreeBranch(model.IssueOpsRecord{Branch: "feature/branch"}, worktree); err != nil {
		t.Fatalf("matching branch rejected: %v", err)
	}
	if err := ValidateWorktreeBranch(model.IssueOpsRecord{}, worktree); err != nil {
		t.Fatalf("empty expected branch should be allowed: %v", err)
	}
	if err := ValidateWorktreeBranch(model.IssueOpsRecord{Branch: "feature/other"}, worktree); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected branch mismatch, got %v", err)
	}
	noGit := t.TempDir()
	if err := ValidateWorktreeBranch(model.IssueOpsRecord{Branch: "feature/branch"}, noGit); err == nil || !strings.Contains(err.Error(), "must be a git worktree") {
		t.Fatalf("expected git worktree error, got %v", err)
	}
}

func issueOpsRepoAndWorktreeFixture(t *testing.T, branch string) (string, string) {
	t.Helper()
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	worktree := filepath.Join(root, "repo.worktrees", strings.ReplaceAll(branch, "/", "-"))
	writeGitHeadForTest(t, repo, branch)
	writeGitHeadForTest(t, worktree, branch)
	return repo, worktree
}

func writeGitHeadForTest(t *testing.T, path, branch string) {
	t.Helper()
	gitDir := filepath.Join(path, ".git")
	if err := os.MkdirAll(gitDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/"+branch+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestIsValidLinkType(t *testing.T) {
	valid := []string{"depends-on", "blocks", "supersedes", "follows-up", "duplicates", "splits-from", "implements"}
	for _, lt := range valid {
		if !isValidLinkType(lt) {
			t.Errorf("expected %q to be valid", lt)
		}
	}
	invalid := []string{"", "parent", "child", "related", "unknown"}
	for _, lt := range invalid {
		if isValidLinkType(lt) {
			t.Errorf("expected %q to be invalid", lt)
		}
	}
}
