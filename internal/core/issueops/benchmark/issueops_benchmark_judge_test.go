package benchmark

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIssueOpsAgyJudgeParsesStrictJSON(t *testing.T) {
	fake := writeFakeIssueOpsAgy(t, `{"ok":true,"fixture_id":"fixture","average_score":100,"minimum_score":100,"dimension_scores":[{"dimension":"intent_understanding","score":100,"evidence":"matches request"}],"deterministic_failures":[],"judge_failures":[],"critical_failures":[],"passed":true}`)
	result, err := RunIssueOpsAgyJudge(IssueOpsAgyJudgeRequest{
		RepoRoot:   t.TempDir(),
		AgyCommand: fake,
		Fixture:    IssueOpsBenchmarkFixture{ID: "fixture"},
		Artifact:   IssueOpsBenchmarkArtifact{ProblemSummary: "summary"},
	})
	if err != nil || !result.OK || len(result.DimensionScores) != 1 {
		t.Fatalf("unexpected judge result err=%v result=%+v", err, result)
	}
}

func TestIssueOpsAgyJudgeParsesFencedJSON(t *testing.T) {
	fake := writeFakeIssueOpsAgy(t, "```json\n"+`{"ok":true,"fixture_id":"fixture","average_score":100,"minimum_score":100,"dimension_scores":[{"dimension":"intent_understanding","score":100,"evidence":"matches request"}],"deterministic_failures":[],"judge_failures":[],"critical_failures":[],"passed":true}`+"\n```")
	result, err := RunIssueOpsAgyJudge(IssueOpsAgyJudgeRequest{
		RepoRoot:   t.TempDir(),
		AgyCommand: fake,
		Fixture:    IssueOpsBenchmarkFixture{ID: "fixture"},
		Artifact:   IssueOpsBenchmarkArtifact{ProblemSummary: "summary"},
	})
	if err != nil || !result.OK || len(result.DimensionScores) != 1 {
		t.Fatalf("expected fenced JSON judge result err=%v result=%+v", err, result)
	}
}

func TestIssueOpsAgyJudgeRejectsNoisyOutput(t *testing.T) {
	fake := writeFakeIssueOpsAgy(t, `I will judge now. {"ok":true}`)
	_, err := RunIssueOpsAgyJudge(IssueOpsAgyJudgeRequest{
		RepoRoot:   t.TempDir(),
		AgyCommand: fake,
		Fixture:    IssueOpsBenchmarkFixture{ID: "fixture"},
	})
	if err == nil {
		t.Fatal("expected strict JSON error")
	}
}

func TestIssueOpsAgyJudgeRetriesEmptyStrictOutput(t *testing.T) {
	dir := t.TempDir()
	counter := filepath.Join(dir, "counter")
	fake := filepath.Join(dir, "fake-agy.sh")
	script := `#!/bin/sh
count=0
if [ -f "$COUNTER_FILE" ]; then
  count=$(cat "$COUNTER_FILE")
fi
count=$((count + 1))
printf '%s' "$count" > "$COUNTER_FILE"
if [ "$count" -eq 1 ]; then
  exit 0
fi
cat <<'EOF'
{"ok":true,"fixture_id":"fixture","average_score":100,"minimum_score":100,"dimension_scores":[{"dimension":"intent_understanding","score":100,"evidence":"retry ok"}],"deterministic_failures":[],"judge_failures":[],"critical_failures":[],"passed":true}
EOF
`
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("COUNTER_FILE", counter)

	result, err := RunIssueOpsAgyJudge(IssueOpsAgyJudgeRequest{
		RepoRoot:   t.TempDir(),
		AgyCommand: fake,
		Attempts:   2,
		Fixture:    IssueOpsBenchmarkFixture{ID: "fixture"},
	})
	if err != nil || !result.OK {
		t.Fatalf("expected retry to recover empty output: result=%+v err=%v", result, err)
	}
}

func TestIssueOpsAgyJudgeRetriesExternalLLMFailure(t *testing.T) {
	dir := t.TempDir()
	counter := filepath.Join(dir, "counter")
	fake := filepath.Join(dir, "fake-agy.sh")
	script := `#!/bin/sh
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
{"ok":true,"fixture_id":"fixture","average_score":100,"minimum_score":100,"dimension_scores":[{"dimension":"intent_understanding","score":100,"evidence":"retry ok"}],"deterministic_failures":[],"judge_failures":[],"critical_failures":[],"passed":true}
EOF
`
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("COUNTER_FILE", counter)

	result, err := RunIssueOpsAgyJudge(IssueOpsAgyJudgeRequest{
		RepoRoot:   t.TempDir(),
		AgyCommand: fake,
		Attempts:   2,
		Fixture:    IssueOpsBenchmarkFixture{ID: "fixture"},
	})
	if err != nil || !result.OK {
		t.Fatalf("expected retry to recover external LLM failure: result=%+v err=%v", result, err)
	}
}

func TestIssueOpsAgyJudgeRejectsDimensionScoreObjectWithOutputEvidence(t *testing.T) {
	output := `{"ok":true,"fixture_id":"fixture","average_score":100,"minimum_score":100,"dimension_scores":{"intent_understanding":{"score":100,"evidence":"object is invalid"}},"deterministic_failures":[],"judge_failures":[],"critical_failures":[],"passed":true}`
	fake := writeFakeIssueOpsAgy(t, output)
	_, err := RunIssueOpsAgyJudge(IssueOpsAgyJudgeRequest{
		RepoRoot:   t.TempDir(),
		AgyCommand: fake,
		Fixture:    IssueOpsBenchmarkFixture{ID: "fixture"},
	})
	if err == nil {
		t.Fatal("expected object-shaped dimension_scores to fail")
	}
	msg := err.Error()
	if !strings.Contains(msg, "dimension_scores") || !strings.Contains(msg, "object is invalid") {
		t.Fatalf("expected decode error to include bounded output evidence, got: %v", err)
	}
}

func TestIssueOpsAgyJudgeRejectsFencedUnknownField(t *testing.T) {
	fake := writeFakeIssueOpsAgy(t, "```json\n"+`{"ok":true,"fixture_id":"fixture","average_score":100,"minimum_score":100,"dimension_scores":[{"dimension":"intent_understanding","score":100,"evidence":"matches request"}],"deterministic_failures":[],"judge_failures":[],"critical_failures":[],"passed":true,"unexpected":true}`+"\n```")
	_, err := RunIssueOpsAgyJudge(IssueOpsAgyJudgeRequest{
		RepoRoot:   t.TempDir(),
		AgyCommand: fake,
		Fixture:    IssueOpsBenchmarkFixture{ID: "fixture"},
		Attempts:   1,
	})
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown field error, got %v", err)
	}
}

func TestIssueOpsAgyJudgePromptRequiresDimensionScoresArray(t *testing.T) {
	prompt, err := buildIssueOpsAgyJudgePrompt(
		IssueOpsBenchmarkFixture{ID: "fixture", Title: "Fixture", UserPrompt: "prompt", RepoContext: "context", CriticalFailures: []string{"failure"}},
		IssueOpsBenchmarkArtifact{ProblemSummary: "summary"},
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"dimension_scores must be a JSON array of objects",
		"Never encode dimension_scores as an object",
		`"dimension_scores":[{"dimension":"intent_understanding","score":100,"evidence":"short evidence"}]`,
		"Every rubric dimension appears exactly once in dimension_scores as an array item",
		"```json",
		"Response Schema",
		"Field Types",
		"Return a raw JSON object",
		"ok: boolean",
		"dimension_scores: array of objects",
		"dimension_scores[].score: number",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func writeFakeIssueOpsAgy(t *testing.T, output string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-agy.sh")
	script := "#!/bin/sh\nif [ \"$1\" != \"--dangerously-skip-permissions\" ] || [ \"$2\" != \"-p\" ]; then echo missing agy flags >&2; exit 2; fi\ncat <<'EOF'\n" + output + "\nEOF\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
