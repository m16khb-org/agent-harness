package preflightfuzz

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"agent-harness/cmd/harness/commandstep"
	"agent-harness/internal/core"
)

const commandOutputBudgetBytes = 32 * 1024

type StepResult = commandstep.StepResult

func Validate(binary, root string, seed int64) StepResult {
	return validatePreflightFuzzWithDeps(binary, root, seed, preflightFuzzValidationDeps{})
}

func validatePreflightFuzz(binary, root string, seed int64) StepResult {
	return Validate(binary, root, seed)
}

func validatePreflightFuzzWithDeps(binary, root string, seed int64, deps preflightFuzzValidationDeps) StepResult {
	deps = deps.withDefaults()
	started := time.Now()
	tempRepo, err := deps.mkdirTemp("", "agent-harness-preflight-fuzz-*")
	if err != nil {
		return commandstep.FailedStep("preflight fuzz", err)
	}
	defer deps.removeAll(tempRepo)
	if code, _, stderr := deps.git(tempRepo, "init", "-q"); code != 0 {
		return commandstep.FailedStep("preflight fuzz", fmt.Errorf("git init: %s", stderr))
	}
	if err := deps.writeFile(filepath.Join(tempRepo, "file.txt"), []byte("seed="+strconv.FormatInt(seed, 10)+"\n"), 0o644); err != nil {
		return commandstep.FailedStep("preflight fuzz", err)
	}
	if code, _, stderr := deps.git(tempRepo, "add", "file.txt"); code != 0 {
		return commandstep.FailedStep("preflight fuzz", fmt.Errorf("git add: %s", stderr))
	}
	msg := "docs(test): add seeded sample"
	body := "Lore:\n- Intent: Validate seeded preflight fuzz.\n- Why: Self-verification needs deterministic git fixtures.\n- Changes:\n  - Add sample file.\n- Verify: agent-harness self-verify\n- Risk: Low"
	commitArgs := []string{"-c", "user.name=Self Verify", "-c", "user.email=self-verify@example.invalid", "commit", "-q", "-m", msg, "-m", body}
	if code, _, stderr := deps.git(tempRepo, commitArgs...); code != 0 {
		return commandstep.FailedStep("preflight fuzz", fmt.Errorf("git commit: %s", stderr))
	}
	secretName := ".env"
	if seed%2 == 0 {
		secretName = "nested.secret"
	}
	if err := deps.writeFile(filepath.Join(tempRepo, secretName), []byte("TOKEN=redacted\n"), 0o600); err != nil {
		return commandstep.FailedStep("preflight fuzz", err)
	}
	step := deps.run(root, "preflight fuzz", 30*time.Second, "", binary, "preflight", "--json", tempRepo)
	step.DurationMS = time.Since(started).Milliseconds()
	if !step.OK {
		return step
	}
	var preflight core.PreflightResult
	if err := json.Unmarshal([]byte(step.Stdout), &preflight); err != nil {
		step.OK = false
		step.Error = err.Error()
		return step
	}
	errs := preflightFuzzValidationErrors(preflight)
	if len(errs) > 0 {
		step.OK = false
		step.Error = strings.Join(errs, "; ")
	}
	return step
}
