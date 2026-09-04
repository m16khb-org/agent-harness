package intentdesign

import (
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestCleanTextValues(t *testing.T) {
	tests := []struct {
		name   string
		input  []string
		expect []string
	}{
		{"empty", nil, nil},
		{"single", []string{"hello"}, []string{"hello"}},
		{"trims whitespace", []string{"  hello  "}, []string{"hello"}},
		{"deduplicates", []string{"a", "a", "b"}, []string{"a", "b"}},
		{"filters empty", []string{"", "a", ""}, []string{"a"}},
		{"filters null byte", []string{"a\x00b", "c"}, []string{"c"}},
		{"multiple non-goals", []string{"no auth", "no db"}, []string{"no auth", "no db"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanTextValues(tt.input)
			if !stringSlicesEqual(got, tt.expect) {
				t.Errorf("CleanTextValues(%v) = %v, want %v", tt.input, got, tt.expect)
			}
		})
	}
}

func TestHasDesignReviewEvidence(t *testing.T) {
	tests := []struct {
		values   []string
		expected bool
	}{
		{[]string{"design review checked alternatives and risks"}, true},
		{[]string{"design audit complete"}, true},
		{[]string{"design evaluated"}, true},
		{[]string{"설계 검토 완료"}, true},
		{[]string{"설계 검수 완료"}, true},
		{[]string{"no evidence here"}, false},
		{nil, false},
		{[]string{""}, false},
		{[]string{"code review done", "design review done"}, true},
	}
	for _, tt := range tests {
		got := HasDesignReviewEvidence(tt.values)
		if got != tt.expected {
			t.Errorf("HasDesignReviewEvidence(%v) = %v, want %v", tt.values, got, tt.expected)
		}
	}
}

func TestMateriallyDifferentIntent(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		interpreted string
		expect      bool
	}{
		{"completely different", "add login", "build authentication system with session management", true},
		{"same text different", "add login button to the page", "implement a login button on the main page", true},
		{"very similar", "add login feature for users to sign in", "add login feature for users to sign in with email", true},
		{"too short raw", "add", "add login feature for users", true},
		{"too short interpreted", "add login feature for users", "add", true},
		{"identical", "add login feature for all users", "add login feature for all users", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := materiallyDifferentIntent(tt.raw, tt.interpreted)
			if got != tt.expect {
				t.Errorf("materiallyDifferentIntent(%q, %q) = %v, want %v", tt.raw, tt.interpreted, got, tt.expect)
			}
		})
	}
}

func TestIntentStopWord(t *testing.T) {
	tests := []struct {
		token    string
		expected bool
	}{
		{"the", true},
		{"a", true},
		{"an", true},
		{"please", true},
		{"좀", true},
		{"해주세요", true},
		{"login", false},
		{"feature", false},
		{"add", false},
	}
	for _, tt := range tests {
		got := intentStopWord(tt.token)
		if got != tt.expected {
			t.Errorf("intentStopWord(%q) = %v, want %v", tt.token, got, tt.expected)
		}
	}
}

func TestIntentTokenSet(t *testing.T) {
	got := intentTokenSet("add login feature for users to sign in")
	expected := []string{"add", "login", "feature", "for", "users", "to", "sign", "in"}
	for _, token := range expected {
		if !got[token] {
			t.Errorf("expected token %q in set", token)
		}
	}
	// Stop words should be excluded
	if got["the"] {
		t.Error("stop word 'the' should be excluded")
	}
}

func TestRecordIntentWritesCleanRedactedContract(t *testing.T) {
	written := false
	store := Store{
		Read: func(stateRoot, id string) (model.IssueOpsRecord, error) {
			if stateRoot != "state" || id != "io-1" {
				t.Fatalf("Read(%q, %q)", stateRoot, id)
			}
			return model.IssueOpsRecord{OK: true, ID: id}, nil
		},
		TouchWrite: func(stateRoot string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			written = true
			if stateRoot != "state" {
				t.Fatalf("TouchWrite stateRoot = %q", stateRoot)
			}
			if record.Intent == nil {
				t.Fatal("intent was not recorded")
			}
			if got := record.Intent.SuccessCriteria; !stringSlicesEqual(got, []string{"tests pass", "docs updated"}) {
				t.Fatalf("success criteria = %#v", got)
			}
			if got := record.Intent.Constraints; !stringSlicesEqual(got, []string{"no schema changes"}) {
				t.Fatalf("constraints = %#v", got)
			}
			return record, nil
		},
	}
	record, err := RecordIntent(store, "state", "io-1", model.IssueOpsIntentRecordRequest{
		RawRequest:        "fix flaky quality gate in cmd/issueops",
		InterpretedIntent: "stabilize quality gate coverage workflow for harness commands",
		SuccessCriteria:   []string{" tests pass ", "", "docs updated", "tests pass"},
		Constraints:       []string{"no schema changes"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !written || record.Intent == nil {
		t.Fatalf("record = %#v written=%v", record, written)
	}
}

func TestRecordIntentRejectsWeakContractsBeforeRead(t *testing.T) {
	store := Store{Read: func(string, string) (model.IssueOpsRecord, error) {
		t.Fatal("invalid intent should fail before store read")
		return model.IssueOpsRecord{}, nil
	}}
	cases := []model.IssueOpsIntentRecordRequest{
		{InterpretedIntent: "intent", SuccessCriteria: []string{"pass"}},
		{RawRequest: "raw", SuccessCriteria: []string{"pass"}},
		{RawRequest: "same request", InterpretedIntent: "same request", SuccessCriteria: []string{"pass"}},
		{RawRequest: "add login feature for all users", InterpretedIntent: "add login feature for all users", SuccessCriteria: []string{"pass"}},
		{RawRequest: "raw request with enough detail", InterpretedIntent: "different interpreted outcome"},
	}
	for _, req := range cases {
		if _, err := RecordIntent(store, "state", "io-1", req); err == nil {
			t.Fatalf("RecordIntent(%#v) succeeded", req)
		}
	}
}

func TestRecordDesignReviewWritesApprovedReview(t *testing.T) {
	store := Store{
		Read: func(stateRoot, id string) (model.IssueOpsRecord, error) {
			return model.IssueOpsRecord{OK: true, ID: id, Intent: &model.IssueOpsIntentContract{RawRequest: "raw"}}, nil
		},
		PlanReadiness: func(record model.IssueOpsRecord) model.IssueOpsReadiness {
			if record.Intent == nil {
				return model.IssueOpsReadiness{OK: true, Ready: false, Missing: []string{"intent"}}
			}
			return model.IssueOpsReadiness{OK: true, Ready: true}
		},
		TouchWrite: func(stateRoot string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			if record.DesignReview == nil {
				t.Fatal("design review was not recorded")
			}
			if !record.DesignReview.Approved {
				t.Fatal("design review should be approved")
			}
			if got := record.DesignReview.Verification; !stringSlicesEqual(got, []string{"go test", DesignReviewEvidenceExample}) {
				t.Fatalf("verification = %#v", got)
			}
			return record, nil
		},
	}
	record, err := RecordDesignReview(store, "state", "io-1", model.IssueOpsDesignReviewRequest{
		ProblemSummary: "quality signal is low",
		ProposedDesign: "add focused tests",
		RefactorPlan:   "keep production unchanged",
		Alternatives:   []string{"raise threshold", "ignore package"},
		Risks:          []string{"brittle tests"},
		Verification:   []string{"go test", DesignReviewEvidenceExample},
		Approved:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.DesignReview == nil {
		t.Fatalf("record = %#v", record)
	}
}

func TestRecordDesignReviewRejectsIncompleteApprovedReview(t *testing.T) {
	store := Store{
		Read: func(stateRoot, id string) (model.IssueOpsRecord, error) {
			return model.IssueOpsRecord{OK: true, ID: id, Intent: &model.IssueOpsIntentContract{RawRequest: "raw"}}, nil
		},
		PlanReadiness: func(record model.IssueOpsRecord) model.IssueOpsReadiness {
			return model.IssueOpsReadiness{OK: true, Ready: true}
		},
		TouchWrite: func(string, model.IssueOpsRecord) (model.IssueOpsRecord, error) {
			t.Fatal("invalid design review should fail before write")
			return model.IssueOpsRecord{}, nil
		},
	}
	req := model.IssueOpsDesignReviewRequest{
		ProblemSummary: "quality signal is low",
		ProposedDesign: "add focused tests",
		RefactorPlan:   "keep production unchanged",
		Alternatives:   []string{"raise threshold"},
		Risks:          []string{"brittle tests"},
		Verification:   []string{"go test"},
		Approved:       true,
	}
	if _, err := RecordDesignReview(store, "state", "io-1", req); err == nil {
		t.Fatal("approved design review without design review evidence should fail")
	}
	req.Verification = []string{DesignReviewEvidenceExample}
	req.OpenQuestions = []string{"what next"}
	if _, err := RecordDesignReview(store, "state", "io-1", req); err == nil {
		t.Fatal("approved design review with open questions should fail")
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
