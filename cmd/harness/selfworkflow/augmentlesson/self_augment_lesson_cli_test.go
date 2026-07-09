package augmentlesson

import (
	"strings"
	"testing"

	"agent-harness/cmd/harness/selfworkflow/model"
	"agent-harness/internal/core"
	"agent-harness/internal/testsupport"
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
	return testsupport.CaptureStdout(t, fn)
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
