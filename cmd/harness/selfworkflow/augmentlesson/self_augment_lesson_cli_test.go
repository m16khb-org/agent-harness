package augmentlesson

import (
	"io"
	"os"
	"strings"
	"testing"

	"agent-harness/cmd/harness/selfworkflow/model"
	"agent-harness/internal/core"
)

func TestRunSelfAugmentLessonSavesStateAndUsesTextDefaults(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())

	out := captureStdout(t, func() error {
		return RunSelfAugmentLesson([]string{
			"--candidate", "candidate-one",
			"--lesson", "Keep coverage slices isolated",
			"--next-action", "Add the next characterization test",
			"--state-key", "lesson-one",
		}, Deps{})
	})

	if !strings.Contains(out, "self-augment lesson saved: candidate=candidate-one key=lesson-one") {
		t.Fatalf("unexpected lesson text output:\n%s", out)
	}
	assertStateRecordContains(t, "lesson-one", model.SelfAugmentationLessonKind)
	assertStateRecordContains(t, "lesson-one", "Keep coverage slices isolated")
}

func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()
	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = writer
	err = fn()
	if closeErr := writer.Close(); closeErr != nil {
		t.Fatalf("close stdout writer: %v", closeErr)
	}
	os.Stdout = oldStdout
	out, readErr := io.ReadAll(reader)
	if readErr != nil {
		t.Fatalf("read stdout: %v", readErr)
	}
	if err != nil {
		t.Fatalf("function returned error: %v", err)
	}
	return string(out)
}

func assertStateRecordContains(t *testing.T, key string, want string) {
	t.Helper()
	state, err := core.StateRead(key)
	if err != nil {
		t.Fatalf("read state %q: %v", key, err)
	}
	if !strings.Contains(state.Record.Content, want) {
		t.Fatalf("state %q content does not contain %q:\n%s", key, want, state.Record.Content)
	}
}
