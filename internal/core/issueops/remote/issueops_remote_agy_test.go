package remote

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agent-harness/internal/core/externalllm"
)

func TestRunIssueOpsRemoteLLMJudgeUsesStrictJSONWrapper(t *testing.T) {
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
	withFakeIssueOpsRemoteZAI(t, remoteFakeZAIResponse{Content: string(b)})

	result, err := RunIssueOpsRemoteLLMJudge(IssueOpsRemoteLLMJudgeRequest{
		RepoRoot: t.TempDir(),
		Request: IssueOpsRemoteScoringRequest{
			Provider: "github",
			Issue:    IssueOpsRemoteArtifact{Title: "IssueOps related issue scoring"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.SelectedRelatedIssues) != 1 || len(result.SelectedLabels) != 1 {
		t.Fatalf("expected strict JSON result from fake Z.AI: %+v", result)
	}
	if result.ExecutionClass != "background_join" || !result.ReadOnly {
		t.Fatalf("expected LLM result to be normalized with background read-only classification: %+v", result)
	}
}

func TestRunIssueOpsRemoteLLMJudgeParsesFencedJSON(t *testing.T) {
	output := "```json\n" + `{"ok":true,"provider":"github","threshold":0.7,"execution_class":"background_join","read_only":true,"join_before":"remote_artifact_write","selected_related_issues":[],"rejected_related_issues":[],"selected_labels":[],"rejected_labels":[],"apply_instructions":[],"warnings":[]}` + "\n```"
	withFakeIssueOpsRemoteZAI(t, remoteFakeZAIResponse{Content: output})

	result, err := RunIssueOpsRemoteLLMJudge(IssueOpsRemoteLLMJudgeRequest{
		RepoRoot: t.TempDir(),
		Request: IssueOpsRemoteScoringRequest{
			Provider: "github",
			Issue:    IssueOpsRemoteArtifact{Title: "IssueOps remote fenced JSON scoring"},
		},
	})
	if err != nil || !result.OK {
		t.Fatalf("expected fenced JSON result from fake Z.AI: result=%+v err=%v", result, err)
	}
}

func TestRunIssueOpsRemoteLLMJudgeRejectsFencedUnknownField(t *testing.T) {
	output := "```json\n" + `{"ok":true,"provider":"github","threshold":0.7,"selected_related_issues":[],"selected_labels":[],"apply_instructions":[],"unexpected":true}` + "\n```"
	withFakeIssueOpsRemoteZAI(t, remoteFakeZAIResponse{Content: output})

	_, err := RunIssueOpsRemoteLLMJudge(IssueOpsRemoteLLMJudgeRequest{
		RepoRoot: t.TempDir(),
		Attempts: 1,
		Request: IssueOpsRemoteScoringRequest{
			Provider: "github",
			Issue:    IssueOpsRemoteArtifact{Title: "IssueOps remote fenced JSON scoring"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestRunIssueOpsRemoteLLMJudgeRejectsInvalidLifecycleMetadata(t *testing.T) {
	output := `{"ok":true,"provider":"github","threshold":0.7,"execution_class":"foreground_blocking","read_only":false,"join_before":"never","selected_related_issues":[],"rejected_related_issues":[],"selected_labels":[],"rejected_labels":[],"apply_instructions":[],"warnings":[]}`
	withFakeIssueOpsRemoteZAI(t, remoteFakeZAIResponse{Content: output})

	_, err := RunIssueOpsRemoteLLMJudge(IssueOpsRemoteLLMJudgeRequest{
		RepoRoot: t.TempDir(),
		Attempts: 1,
		Request: IssueOpsRemoteScoringRequest{
			Provider: "github",
			Issue:    IssueOpsRemoteArtifact{Title: "IssueOps remote lifecycle contract"},
		},
	})
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
		"```json",
		"Response Schema",
		"Field Types",
		"Return a raw JSON object",
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

func TestRunIssueOpsRemoteLLMJudgeRetriesExternalLLMFailure(t *testing.T) {
	withFakeIssueOpsRemoteZAI(t,
		remoteFakeZAIResponse{Status: http.StatusInternalServerError, Body: `{"error":{"message":"transient failure"}}`},
		remoteFakeZAIResponse{Content: `{"ok":true,"provider":"github","threshold":0.7,"execution_class":"background_join","read_only":true,"join_before":"remote_artifact_write","selected_related_issues":[],"rejected_related_issues":[],"selected_labels":[],"rejected_labels":[],"apply_instructions":[],"warnings":[]}`},
	)
	result, err := RunIssueOpsRemoteLLMJudge(IssueOpsRemoteLLMJudgeRequest{
		RepoRoot: t.TempDir(),
		Attempts: 2,
		Request: IssueOpsRemoteScoringRequest{
			Provider: "github",
			Issue:    IssueOpsRemoteArtifact{Title: "IssueOps remote scoring retry"},
		},
	})
	if err != nil || !result.OK {
		t.Fatalf("expected retry to recover external LLM failure: result=%+v err=%v", result, err)
	}
}

type remoteFakeZAIResponse struct {
	Status  int
	Body    string
	Content string
}

func withFakeIssueOpsRemoteZAI(t *testing.T, responses ...remoteFakeZAIResponse) {
	t.Helper()
	if len(responses) == 0 {
		t.Fatal("missing fake Z.AI responses")
	}
	t.Setenv("Z_AI_API_KEY", "test-key")
	index := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		response := responses[index]
		if index < len(responses)-1 {
			index++
		}
		if response.Status != 0 {
			w.WriteHeader(response.Status)
		}
		if response.Body != "" {
			_, _ = w.Write([]byte(response.Body))
			return
		}
		_, _ = fmt.Fprintf(w, `{"choices":[{"message":{"content":%q}}]}`, response.Content)
	}))
	t.Cleanup(server.Close)
	previous := externalllm.SetBaseURL(server.URL)
	t.Cleanup(func() { externalllm.SetBaseURL(previous) })
}
