package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScoreIssueOpsRemoteCandidatesSelectsThresholdMatches(t *testing.T) {
	relatedScore := 0.92
	unrelatedScore := 0.42
	labelScore := 0.88
	docScore := 0.40
	result, err := ScoreIssueOpsRemoteCandidates(IssueOpsRemoteScoringRequest{
		Provider:  "github",
		Threshold: 0.70,
		Issue: IssueOpsRemoteArtifact{
			Title: "IssueOps 이슈 생성 시 관련 이슈와 라벨 점수화",
			Body:  "관련 이슈 링크와 enhancement 라벨을 threshold 기반으로 적용한다.",
		},
		IssueCandidates: []IssueOpsRemoteIssueCandidate{
			{ID: "#9", Title: "IssueOps autoresearch quality gate", URL: "https://example.com/issues/9", Score: &relatedScore},
			{ID: "#7", Title: "Template cleanup", URL: "https://example.com/issues/7", Score: &unrelatedScore},
		},
		LabelCandidates: []IssueOpsRemoteLabelCandidate{
			{Name: "enhancement", Score: &labelScore},
			{Name: "documentation", Score: &docScore},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Provider != "github" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(result.SelectedRelatedIssues) != 1 || result.SelectedRelatedIssues[0].ID != "#9" {
		t.Fatalf("expected only #9 selected: %+v", result.SelectedRelatedIssues)
	}
	if len(result.SelectedLabels) != 1 || result.SelectedLabels[0].Name != "enhancement" {
		t.Fatalf("expected only enhancement selected: %+v", result.SelectedLabels)
	}
	if !containsFold(strings.Join(result.ApplyInstructions, "\n"), "gh issue") {
		t.Fatalf("expected GitHub apply instruction: %+v", result.ApplyInstructions)
	}
	if result.ExecutionClass != "background_join" || !result.ReadOnly {
		t.Fatalf("expected background join read-only classification: %+v", result)
	}
}

func TestScoreIssueOpsRemoteCandidatesSupportsGitLabInstructions(t *testing.T) {
	score := 0.95
	result, err := ScoreIssueOpsRemoteCandidates(IssueOpsRemoteScoringRequest{
		Provider:  "gitlab",
		Threshold: 0.70,
		Issue:     IssueOpsRemoteArtifact{Title: "IssueOps GitLab label support"},
		LabelCandidates: []IssueOpsRemoteLabelCandidate{
			{Name: "enhancement", Score: &score},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.SelectedLabels) != 1 {
		t.Fatalf("expected selected GitLab label: %+v", result)
	}
	if !containsFold(strings.Join(result.ApplyInstructions, "\n"), "glab issue create") {
		t.Fatalf("expected GitLab apply instruction: %+v", result.ApplyInstructions)
	}
}

func TestScoreIssueOpsRemoteCandidatesGitLabRelatedIssuesUseLinkedItems(t *testing.T) {
	relatedScore := 0.92
	result, err := ScoreIssueOpsRemoteCandidates(IssueOpsRemoteScoringRequest{
		Provider:  "gitlab",
		Threshold: 0.70,
		Issue:     IssueOpsRemoteArtifact{Title: "IssueOps GitLab linked item 연결"},
		IssueCandidates: []IssueOpsRemoteIssueCandidate{
			{ID: "#9", Title: "IssueOps autoresearch quality gate", URL: "https://gitlab.example.com/group/repo/-/issues/9", Score: &relatedScore},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.SelectedRelatedIssues) != 1 {
		t.Fatalf("expected one selected GitLab related issue: %+v", result.SelectedRelatedIssues)
	}
	joined := strings.Join(result.ApplyInstructions, "\n")
	if !containsFold(joined, "linked item") || !containsFold(joined, "/links") {
		t.Fatalf("expected GitLab linked-item apply instruction: %+v", result.ApplyInstructions)
	}
	if containsFold(joined, "in the issue body") {
		t.Fatalf("GitLab related issues must not be instructed into the issue body: %+v", result.ApplyInstructions)
	}
	if hint := result.SelectedRelatedIssues[0].ApplyHint; !containsFold(hint, "linked item") {
		t.Fatalf("expected GitLab linked-item apply hint: %q", hint)
	}
}

func TestScoreIssueOpsRemoteCandidatesDoesNotWarnWhenNoCandidatesExist(t *testing.T) {
	result, err := ScoreIssueOpsRemoteCandidates(IssueOpsRemoteScoringRequest{
		Provider: "github",
		Issue:    IssueOpsRemoteArtifact{Title: "IssueOps remote score with no candidate data"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("empty candidate lists should not produce threshold warnings: %+v", result.Warnings)
	}
	b, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, want := range []string{
		`"selected_related_issues":[]`,
		`"rejected_related_issues":[]`,
		`"selected_labels":[]`,
		`"rejected_labels":[]`,
		`"apply_instructions":[]`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected empty array %s in JSON, got %s", want, text)
		}
	}
	if strings.Contains(text, "null") {
		t.Fatalf("remote score JSON must use [] instead of null: %s", text)
	}
}

func TestDecodeIssueOpsRemoteScoringRequestAcceptsCandidateAliases(t *testing.T) {
	req, err := DecodeIssueOpsRemoteScoringRequest([]byte(`{
		"provider": "github",
		"issue": {"title": "IssueOps feedback gate"},
		"related_issues": [{"id": "#1", "title": "IssueOps feedback gate"}],
		"labels": [{"name": "bug", "score": 0.91}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(req.IssueCandidates) != 1 || req.IssueCandidates[0].ID != "#1" {
		t.Fatalf("expected related_issues alias to populate issue candidates: %+v", req.IssueCandidates)
	}
	if len(req.LabelCandidates) != 1 || req.LabelCandidates[0].Name != "bug" {
		t.Fatalf("expected labels alias to populate label candidates: %+v", req.LabelCandidates)
	}
}

func TestDecodeIssueOpsRemoteScoringRequestRejectsDuplicateCandidateFields(t *testing.T) {
	if _, err := DecodeIssueOpsRemoteScoringRequest([]byte(`{
		"issue": {"title": "IssueOps feedback gate"},
		"issue_candidates": [],
		"related_issues": []
	}`)); err == nil || !strings.Contains(err.Error(), "issue_candidates") {
		t.Fatalf("expected duplicate issue candidate field error, got %v", err)
	}
	if _, err := DecodeIssueOpsRemoteScoringRequest([]byte(`{
		"issue": {"title": "IssueOps feedback gate"},
		"label_candidates": [],
		"labels": []
	}`)); err == nil || !strings.Contains(err.Error(), "label_candidates") {
		t.Fatalf("expected duplicate label candidate field error, got %v", err)
	}
}

func TestScoreIssueOpsRemoteCandidatesRequiresExplicitLabelDecisionWhenAllLabelsRejected(t *testing.T) {
	labelScore := 0.40
	result, err := ScoreIssueOpsRemoteCandidates(IssueOpsRemoteScoringRequest{
		Provider:  "gitlab",
		Threshold: 0.70,
		Issue: IssueOpsRemoteArtifact{
			Title: "IssueOps MR 라벨 누락 방지",
			Body:  "원격 이슈와 MR 생성 전에 라벨 결정을 검증한다.",
		},
		LabelCandidates: []IssueOpsRemoteLabelCandidate{
			{Name: "documentation", Score: &labelScore},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.SelectedLabels) != 0 {
		t.Fatalf("expected all labels rejected: %+v", result.SelectedLabels)
	}
	joined := strings.Join(result.ApplyInstructions, "\n")
	if !containsFold(joined, "stop before remote artifact writes") || !containsFold(joined, "manual label") {
		t.Fatalf("expected explicit label-decision instruction before remote writes: %+v", result.ApplyInstructions)
	}
	if !containsFold(strings.Join(result.Warnings, "\n"), "no label candidates met threshold") {
		t.Fatalf("expected no-label warning: %+v", result.Warnings)
	}
}

func TestRunIssueOpsRemoteAgyJudgeUsesStrictJSONWrapper(t *testing.T) {
	dir := t.TempDir()
	fakeAgy := filepath.Join(dir, "fake-agy.sh")
	out := IssueOpsRemoteScoringResult{
		OK:             true,
		Provider:       "github",
		Threshold:      0.70,
		ExecutionClass: "background_join",
		ReadOnly:       true,
		JoinBefore:     "remote_artifact_write",
		SelectedRelatedIssues: []IssueOpsRemoteScoredItem{{
			ID:        "#9",
			Score:     0.91,
			Threshold: 0.70,
			Selected:  true,
			Evidence:  []string{"shared IssueOps workflow"},
			ApplyHint: "link in issue body: #9",
		}},
		SelectedLabels: []IssueOpsRemoteScoredItem{{
			Name:      "enhancement",
			Score:     0.93,
			Threshold: 0.70,
			Selected:  true,
			Evidence:  []string{"feature request"},
			ApplyHint: "apply GitHub label: enhancement",
		}},
		ApplyInstructions: []string{"apply selected labels with gh issue create --label or gh issue edit --add-label: enhancement"},
	}
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fakeAgy, []byte("#!/bin/sh\nif [ \"$1\" != \"--dangerously-skip-permissions\" ] || [ \"$2\" != \"-p\" ]; then echo missing agy flags >&2; exit 2; fi\nprintf '%s' '"+string(b)+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := RunIssueOpsRemoteAgyJudge(IssueOpsRemoteAgyJudgeRequest{
		RepoRoot:   dir,
		AgyCommand: fakeAgy,
		Request: IssueOpsRemoteScoringRequest{
			Provider: "github",
			Issue:    IssueOpsRemoteArtifact{Title: "IssueOps related issue scoring"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.SelectedRelatedIssues) != 1 || len(result.SelectedLabels) != 1 {
		t.Fatalf("expected strict JSON result from fake agy: %+v", result)
	}
	if result.ExecutionClass != "background_join" || !result.ReadOnly {
		t.Fatalf("expected agy result to be normalized with background read-only classification: %+v", result)
	}
}

func TestRunIssueOpsRemoteAgyJudgeParsesFencedJSON(t *testing.T) {
	fakeAgy := filepath.Join(t.TempDir(), "fake-agy.sh")
	output := "```json\n" + `{"ok":true,"provider":"github","threshold":0.7,"execution_class":"background_join","read_only":true,"join_before":"remote_artifact_write","selected_related_issues":[],"rejected_related_issues":[],"selected_labels":[],"rejected_labels":[],"apply_instructions":[],"warnings":[]}` + "\n```"
	if err := os.WriteFile(fakeAgy, []byte("#!/bin/sh\nif [ \"$1\" != \"--dangerously-skip-permissions\" ] || [ \"$2\" != \"-p\" ]; then echo missing agy flags >&2; exit 2; fi\ncat <<'EOF'\n"+output+"\nEOF\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := RunIssueOpsRemoteAgyJudge(IssueOpsRemoteAgyJudgeRequest{
		RepoRoot:   t.TempDir(),
		AgyCommand: fakeAgy,
		Request: IssueOpsRemoteScoringRequest{
			Provider: "github",
			Issue:    IssueOpsRemoteArtifact{Title: "IssueOps remote fenced JSON scoring"},
		},
	})
	if err != nil || !result.OK {
		t.Fatalf("expected fenced JSON result from fake agy: result=%+v err=%v", result, err)
	}
}

func TestRunIssueOpsRemoteAgyJudgeRejectsFencedUnknownField(t *testing.T) {
	fakeAgy := filepath.Join(t.TempDir(), "fake-agy.sh")
	output := "```json\n" + `{"ok":true,"provider":"github","threshold":0.7,"selected_related_issues":[],"selected_labels":[],"apply_instructions":[],"unexpected":true}` + "\n```"
	if err := os.WriteFile(fakeAgy, []byte("#!/bin/sh\ncat <<'EOF'\n"+output+"\nEOF\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := RunIssueOpsRemoteAgyJudge(IssueOpsRemoteAgyJudgeRequest{
		RepoRoot:   t.TempDir(),
		AgyCommand: fakeAgy,
		Attempts:   1,
		Request: IssueOpsRemoteScoringRequest{
			Provider: "github",
			Issue:    IssueOpsRemoteArtifact{Title: "IssueOps remote fenced JSON scoring"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestRunIssueOpsRemoteAgyJudgeRejectsInvalidLifecycleMetadata(t *testing.T) {
	fakeAgy := filepath.Join(t.TempDir(), "fake-agy.sh")
	output := `{"ok":true,"provider":"github","threshold":0.7,"execution_class":"foreground_blocking","read_only":false,"join_before":"never","selected_related_issues":[],"rejected_related_issues":[],"selected_labels":[],"rejected_labels":[],"apply_instructions":[],"warnings":[]}`
	if err := os.WriteFile(fakeAgy, []byte("#!/bin/sh\nprintf '%s' '"+output+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := RunIssueOpsRemoteAgyJudge(IssueOpsRemoteAgyJudgeRequest{
		RepoRoot:   t.TempDir(),
		AgyCommand: fakeAgy,
		Attempts:   1,
		Request: IssueOpsRemoteScoringRequest{
			Provider: "github",
			Issue:    IssueOpsRemoteArtifact{Title: "IssueOps remote lifecycle contract"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "execution_class") {
		t.Fatalf("expected invalid lifecycle metadata error, got %v", err)
	}
}

func TestIssueOpsRemoteAgyJudgePromptRequiresReadOnlyBackgroundJoin(t *testing.T) {
	prompt, err := buildIssueOpsRemoteAgyJudgePrompt(IssueOpsRemoteScoringRequest{
		Provider: "github",
		Issue:    IssueOpsRemoteArtifact{Title: "IssueOps remote scoring prompt hardening"},
	})
	if err != nil {
		t.Fatal(err)
	}
	required := []string{
		"read-only evaluator",
		"background_join",
		"main work may continue",
		"join before creating or editing remote issues, labels, pull requests, or merge requests",
		"Do not create, edit, delete, label, assign, comment on, close, reopen, stage, commit, push",
		"Do not inspect the workspace, run tools, or read files",
		"```json",
		"Response Schema",
		"Field Types",
		"Return exactly this JSON object shape",
		"ok: boolean",
		"selected_related_issues: array of scored item objects",
		"scored item score and threshold: numbers",
		`"selected_related_issues"`,
	}
	for _, want := range required {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt should contain %q:\n%s", want, prompt)
		}
	}
}

func TestRunIssueOpsRemoteAgyJudgeRetriesExternalLLMFailure(t *testing.T) {
	dir := t.TempDir()
	counter := filepath.Join(dir, "counter")
	fakeAgy := filepath.Join(dir, "fake-agy.sh")
	if err := os.WriteFile(fakeAgy, []byte(`#!/bin/sh
if [ "$1" != "--dangerously-skip-permissions" ] || [ "$2" != "-p" ]; then
  echo missing agy flags >&2
  exit 2
fi
count=0
if [ -f "$COUNTER_FILE" ]; then
  count=$(cat "$COUNTER_FILE")
fi
count=$((count + 1))
printf '%s' "$count" > "$COUNTER_FILE"
if [ "$count" -eq 1 ]; then
  echo transient failure >&2
  exit 7
fi
cat <<'EOF'
{"ok":true,"provider":"github","threshold":0.7,"execution_class":"background_join","read_only":true,"join_before":"remote_artifact_write","selected_related_issues":[],"rejected_related_issues":[],"selected_labels":[],"rejected_labels":[],"apply_instructions":[],"warnings":[]}
EOF
`), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("COUNTER_FILE", counter)

	result, err := RunIssueOpsRemoteAgyJudge(IssueOpsRemoteAgyJudgeRequest{
		RepoRoot:   dir,
		AgyCommand: fakeAgy,
		Attempts:   2,
		Request: IssueOpsRemoteScoringRequest{
			Provider: "github",
			Issue:    IssueOpsRemoteArtifact{Title: "IssueOps remote scoring retry"},
		},
	})
	if err != nil || !result.OK {
		t.Fatalf("expected retry to recover external LLM failure: result=%+v err=%v", result, err)
	}
}
