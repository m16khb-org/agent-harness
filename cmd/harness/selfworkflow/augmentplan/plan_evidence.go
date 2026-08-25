package augmentplan

import (
	"encoding/json"
	"os/exec"
	"strings"

	"agent-harness/cmd/harness/selfworkflow/model"
)

// selfVerifyLatestKey mirrors stateio's default self-verify summary key. The
// planner reads the persisted summary instead of re-running the verification
// loop, keeping goal scoring cheap while still grounded in a real artifact.
const selfVerifyLatestKey = "self-verify-latest"

// hasImplementationDelta reports whether the repository working tree carries
// uncommitted changes right now. The augmentation contract re-runs the planner
// after implementing the selected candidate, so a non-empty porcelain status is
// a cheap real observation of an implementation delta. It can be true from
// unrelated work in progress, which is why the goal evidence names the exact
// command instead of claiming candidate-specific proof.
func hasImplementationDelta(root string) bool {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

// selfVerificationPassed reports whether the most recent saved self-verify
// summary recorded a passing run (agent-harness self-verify ... --save-state).
func selfVerificationPassed() bool {
	record, err := StateRead(selfVerifyLatestKey)
	if err != nil {
		return false
	}
	var snapshot model.SelfAugmentStateSnapshot
	if err := json.Unmarshal([]byte(record.Record.Content), &snapshot); err != nil {
		return false
	}
	return snapshot.OK
}

// lessonsCaptured reports whether at least one Reflexion-style augmentation
// lesson was persisted, which is the learning-capture observable of the loop.
func lessonsCaptured() bool {
	list, err := StateList()
	if err != nil {
		return false
	}
	for _, key := range list.Keys {
		if strings.HasPrefix(key, selfAugmentLessonKeyPrefix) {
			return true
		}
	}
	return false
}

// Goal evidence names the exact observable behind each score so an agent can
// re-produce or refute it instead of treating the gate as decorative.
func implementationEvidence(root string) []string {
	if hasImplementationDelta(root) {
		return []string{"git status --porcelain non-empty: uncommitted implementation diff observed"}
	}
	return []string{"git status --porcelain empty: no uncommitted diff found; implement the selected candidate first"}
}

func verificationEvidence() []string {
	if selfVerificationPassed() {
		return []string{"self-verify-latest state ok=true (from agent-harness self-verify --save-state)"}
	}
	return []string{"self-verify-latest state absent or failing; run agent-harness self-verify --save-state after verification"}
}

func learningEvidence() []string {
	if lessonsCaptured() {
		return []string{"augmentation lessons present under self-augment-lesson-* state keys"}
	}
	return []string{"no augmentation lessons captured yet; record outcomes via the lesson capture flow"}
}
