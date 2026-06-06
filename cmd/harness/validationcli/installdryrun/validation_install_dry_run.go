package installdryrun

import (
	"encoding/json"
	"path/filepath"
	"time"

	"agent-harness/cmd/harness/commandstep"
)

const aggregateOutputBudgetBytes = 8 * 1024
const commandOutputBudgetBytes = 32 * 1024
const skillName = "atomic-commit-push"

type StepResult = commandstep.StepResult

func Validate(binary, root string, seed int64) StepResult {
	return validateInstallDryRunSmokeWithDeps(binary, root, seed, installDryRunValidationDeps{})
}

func validateInstallDryRunSmoke(binary, root string, seed int64) StepResult {
	return Validate(binary, root, seed)
}

func validateInstallDryRunSmokeWithDeps(binary, root string, seed int64, deps installDryRunValidationDeps) StepResult {
	deps = deps.withDefaults()
	started := time.Now()
	tempHome, err := deps.makeTempDir("home", seed)
	if err != nil {
		return commandstep.FailedStep("install dry-run smoke", err)
	}
	defer deps.removeAll(tempHome)
	tempRoot, err := deps.makeTempDir("root", seed)
	if err != nil {
		return commandstep.FailedStep("install dry-run smoke", err)
	}
	defer deps.removeAll(tempRoot)
	skillDir := filepath.Join(tempRoot, "skills", skillName)
	if err := deps.makeDirAll(skillDir, 0o755); err != nil {
		return commandstep.FailedStep("install dry-run smoke", err)
	}
	if err := deps.writeFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: "+skillName+"\ndescription: install dry-run smoke\n---\n"), 0o644); err != nil {
		return commandstep.FailedStep("install dry-run smoke", err)
	}
	env := []string{
		"HOME=" + tempHome,
		"CODEX_HOME=" + filepath.Join(tempHome, ".codex"),
		"HARNESS_ROOT=" + tempRoot,
	}
	step := deps.run(root, "install dry-run smoke", 30*time.Second, "", env, binary, "install-native", "--dry-run", "--project-local", "--json")
	if !step.OK {
		return step
	}
	var result installDryRunSmokeResult
	if err := json.Unmarshal([]byte(step.Stdout), &result); err != nil {
		return commandstep.AssertionStepWithOutput("install dry-run smoke", started, []string{err.Error()}, []string{step.Stdout}, []string{step.Command}, aggregateOutputBudgetBytes)
	}
	errs := installDryRunValidationErrors(result, tempHome, tempRoot, deps.exists)
	if len(errs) > 0 {
		return commandstep.AssertionStepWithOutput("install dry-run smoke", started, errs, []string{step.Stdout}, []string{step.Command}, aggregateOutputBudgetBytes)
	}
	step.DurationMS = time.Since(started).Milliseconds()
	return step
}
