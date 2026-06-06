package remote

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
