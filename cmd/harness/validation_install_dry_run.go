package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type installDryRunCommandRunner func(dir, label string, timeout time.Duration, stdin string, env []string, name string, args ...string) StepResult

type installDryRunValidationDeps struct {
	makeTempDir func(kind string, seed int64) (string, error)
	removeAll   func(path string) error
	makeDirAll  func(path string, perm uint32) error
	writeFile   func(path string, data []byte, perm uint32) error
	exists      func(path string) bool
	run         installDryRunCommandRunner
}

func (deps installDryRunValidationDeps) withDefaults() installDryRunValidationDeps {
	if deps.makeTempDir == nil {
		deps.makeTempDir = func(kind string, seed int64) (string, error) {
			return os.MkdirTemp("", fmt.Sprintf("agent-harness-install-%s-%d-*", kind, seed))
		}
	}
	if deps.removeAll == nil {
		deps.removeAll = os.RemoveAll
	}
	if deps.makeDirAll == nil {
		deps.makeDirAll = func(path string, perm uint32) error {
			return os.MkdirAll(path, os.FileMode(perm))
		}
	}
	if deps.writeFile == nil {
		deps.writeFile = func(path string, data []byte, perm uint32) error {
			return os.WriteFile(path, data, os.FileMode(perm))
		}
	}
	if deps.exists == nil {
		deps.exists = exists
	}
	if deps.run == nil {
		deps.run = runCommandStepEnv
	}
	return deps
}

func validateInstallDryRunSmoke(binary, root string, seed int64) StepResult {
	return validateInstallDryRunSmokeWithDeps(binary, root, seed, installDryRunValidationDeps{})
}

func validateInstallDryRunSmokeWithDeps(binary, root string, seed int64, deps installDryRunValidationDeps) StepResult {
	deps = deps.withDefaults()
	started := time.Now()
	tempHome, err := deps.makeTempDir("home", seed)
	if err != nil {
		return failedStep("install dry-run smoke", err)
	}
	defer deps.removeAll(tempHome)
	tempRoot, err := deps.makeTempDir("root", seed)
	if err != nil {
		return failedStep("install dry-run smoke", err)
	}
	defer deps.removeAll(tempRoot)
	skillDir := filepath.Join(tempRoot, "skills", skillName)
	if err := deps.makeDirAll(skillDir, 0o755); err != nil {
		return failedStep("install dry-run smoke", err)
	}
	if err := deps.writeFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: "+skillName+"\ndescription: install dry-run smoke\n---\n"), 0o644); err != nil {
		return failedStep("install dry-run smoke", err)
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
		return assertionStepWithOutput("install dry-run smoke", started, []string{err.Error()}, []string{step.Stdout}, []string{step.Command})
	}
	errs := installDryRunValidationErrors(result, tempHome, tempRoot, deps.exists)
	if len(errs) > 0 {
		return assertionStepWithOutput("install dry-run smoke", started, errs, []string{step.Stdout}, []string{step.Command})
	}
	step.DurationMS = time.Since(started).Milliseconds()
	return step
}
