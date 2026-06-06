package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"agent-harness/internal/core"
)

type preflightFuzzCommandRunner func(root, label string, timeout time.Duration, input string, command ...string) StepResult
type preflightFuzzGitRunner func(dir string, args ...string) (int, string, string)

type preflightFuzzValidationDeps struct {
	mkdirTemp func(string, string) (string, error)
	removeAll func(string) error
	writeFile func(string, []byte, os.FileMode) error
	git       preflightFuzzGitRunner
	run       preflightFuzzCommandRunner
}

func (deps preflightFuzzValidationDeps) withDefaults() preflightFuzzValidationDeps {
	if deps.mkdirTemp == nil {
		deps.mkdirTemp = os.MkdirTemp
	}
	if deps.removeAll == nil {
		deps.removeAll = os.RemoveAll
	}
	if deps.writeFile == nil {
		deps.writeFile = os.WriteFile
	}
	if deps.git == nil {
		deps.git = core.GitCmd
	}
	if deps.run == nil {
		deps.run = func(root, label string, timeout time.Duration, input string, command ...string) StepResult {
			if len(command) == 0 {
				return failedStep(label, fmt.Errorf("missing command"))
			}
			return runCommandStep(root, label, timeout, input, command[0], command[1:]...)
		}
	}
	return deps
}

func validatePreflightFuzz(binary, root string, seed int64) StepResult {
	return validatePreflightFuzzWithDeps(binary, root, seed, preflightFuzzValidationDeps{})
}

func validatePreflightFuzzWithDeps(binary, root string, seed int64, deps preflightFuzzValidationDeps) StepResult {
	deps = deps.withDefaults()
	started := time.Now()
	tempRepo, err := deps.mkdirTemp("", "agent-harness-preflight-fuzz-*")
	if err != nil {
		return failedStep("preflight fuzz", err)
	}
	defer deps.removeAll(tempRepo)
	if code, _, stderr := deps.git(tempRepo, "init", "-q"); code != 0 {
		return failedStep("preflight fuzz", fmt.Errorf("git init: %s", stderr))
	}
	if err := deps.writeFile(filepath.Join(tempRepo, "file.txt"), []byte("seed="+strconv.FormatInt(seed, 10)+"\n"), 0o644); err != nil {
		return failedStep("preflight fuzz", err)
	}
	if code, _, stderr := deps.git(tempRepo, "add", "file.txt"); code != 0 {
		return failedStep("preflight fuzz", fmt.Errorf("git add: %s", stderr))
	}
	msg := "docs(test): add seeded sample"
	body := "Lore:\n- Intent: Validate seeded preflight fuzz.\n- Why: Self-verification needs deterministic git fixtures.\n- Changes:\n  - Add sample file.\n- Verify: agent-harness self-verify\n- Risk: Low"
	commitArgs := []string{"-c", "user.name=Self Verify", "-c", "user.email=self-verify@example.invalid", "commit", "-q", "-m", msg, "-m", body}
	if code, _, stderr := deps.git(tempRepo, commitArgs...); code != 0 {
		return failedStep("preflight fuzz", fmt.Errorf("git commit: %s", stderr))
	}
	secretName := ".env"
	if seed%2 == 0 {
		secretName = "nested.secret"
	}
	if err := deps.writeFile(filepath.Join(tempRepo, secretName), []byte("TOKEN=redacted\n"), 0o600); err != nil {
		return failedStep("preflight fuzz", err)
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
	errs := []string{}
	if !preflight.OK {
		errs = append(errs, "preflight ok=false")
	}
	if preflight.CommitStyleHints["conventional_subjects"] != float64(1) {
		errs = append(errs, "conventional subject not detected")
	}
	if preflight.CommitStyleHints["lore_bodies"] != float64(1) {
		errs = append(errs, "Lore body not detected")
	}
	if len(preflight.SecretLikePaths) == 0 {
		errs = append(errs, "secret-like path not detected")
	}
	if len(errs) > 0 {
		step.OK = false
		step.Error = strings.Join(errs, "; ")
	}
	return step
}
