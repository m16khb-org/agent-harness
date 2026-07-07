package remote

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestIssueOpsRemoteJudgeFileUsesStrictJSONWrapper(t *testing.T) {
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

	result, err := DecodeIssueOpsRemoteJudgeJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.SelectedRelatedIssues) != 1 || len(result.SelectedLabels) != 1 {
		t.Fatalf("expected strict JSON result: %+v", result)
	}
	if result.ExecutionClass != "background_join" || !result.ReadOnly {
		t.Fatalf("expected result to preserve background read-only classification: %+v", result)
	}
}

func TestIssueOpsRemoteJudgeFileParsesFencedJSON(t *testing.T) {
	output := "```json\n" + `{"ok":true,"provider":"github","threshold":0.7,"execution_class":"background_join","read_only":true,"join_before":"remote_artifact_write","selected_related_issues":[],"rejected_related_issues":[],"selected_labels":[],"rejected_labels":[],"apply_instructions":[],"warnings":[]}` + "\n```"

	result, err := DecodeIssueOpsRemoteJudgeJSON([]byte(output))
	if err != nil || !result.OK {
		t.Fatalf("expected fenced JSON result: result=%+v err=%v", result, err)
	}
}

func TestRunIssueOpsRemoteLLMJudgeReturnsRemovedServiceError(t *testing.T) {
	_, err := RunIssueOpsRemoteLLMJudge(IssueOpsRemoteLLMJudgeRequest{
		Request: IssueOpsRemoteScoringRequest{
			Provider: "github",
			Issue:    IssueOpsRemoteArtifact{Title: "IssueOps remote scoring"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "no longer calls external LLM services") {
		t.Fatalf("expected removed service error, got %v", err)
	}
}

func TestIssueOpsRemoteJudgeFileRejectsFencedUnknownField(t *testing.T) {
	output := "```json\n" + `{"ok":true,"provider":"github","threshold":0.7,"selected_related_issues":[],"selected_labels":[],"apply_instructions":[],"unexpected":true}` + "\n```"

	_, err := DecodeIssueOpsRemoteJudgeJSON([]byte(output))
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestIssueOpsRemoteJudgeFileRejectsInvalidLifecycleMetadata(t *testing.T) {
	output := `{"ok":true,"provider":"github","threshold":0.7,"execution_class":"foreground_blocking","read_only":false,"join_before":"never","selected_related_issues":[],"rejected_related_issues":[],"selected_labels":[],"rejected_labels":[],"apply_instructions":[],"warnings":[]}`

	_, err := DecodeIssueOpsRemoteJudgeJSON([]byte(output))
	if err == nil || !strings.Contains(err.Error(), "execution_class") {
		t.Fatalf("expected invalid lifecycle metadata error, got %v", err)
	}
}

func TestIssueOpsRemoteLLMJudgePromptRequiresReadOnlyBackgroundJoin(t *testing.T) {
	prompt, err := buildIssueOpsRemoteLLMJudgePrompt(IssueOpsRemoteScoringRequest{
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
		"Host-Agent Judgement Response Schema",
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
