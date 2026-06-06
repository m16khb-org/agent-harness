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

type installDryRunSmokeResult struct {
	OK           bool                     `json:"ok"`
	DryRun       bool                     `json:"dry_run"`
	ProjectLocal bool                     `json:"project_local"`
	Hosts        []installDryRunSmokeHost `json:"hosts"`
	Files        []installDryRunSmokeFile `json:"files"`
	Links        []installDryRunSmokeLink `json:"links"`
	SkillNames   []string                 `json:"skill_names"`
	Messages     []string                 `json:"messages"`
}

type installDryRunSmokeHost struct {
	Host   string `json:"host"`
	OK     bool   `json:"ok"`
	DryRun bool   `json:"dry_run"`
}

type installDryRunSmokeFile struct {
	Path       string `json:"path"`
	Written    bool   `json:"written"`
	WouldWrite bool   `json:"would_write"`
}

type installDryRunSmokeLink struct {
	Path        string `json:"path"`
	Created     bool   `json:"created"`
	WouldCreate bool   `json:"would_create"`
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

func installDryRunValidationErrors(result installDryRunSmokeResult, tempHome, tempRoot string, pathExists func(string) bool) []string {
	errs := []string{}
	if !result.OK || !result.DryRun || !result.ProjectLocal {
		errs = append(errs, "install dry-run result flags mismatch")
	}
	if len(result.Hosts) != 2 {
		errs = append(errs, "install dry-run did not cover both hosts")
	}
	for _, host := range result.Hosts {
		if !host.OK || !host.DryRun {
			errs = append(errs, "install dry-run host mismatch:"+host.Host)
		}
	}
	if !containsString(result.SkillNames, skillName) {
		errs = append(errs, "install dry-run did not discover smoke skill")
	}
	plannedWrite := false
	for _, file := range result.Files {
		if file.Written {
			errs = append(errs, "install dry-run reported written file:"+file.Path)
		}
		if file.WouldWrite {
			plannedWrite = true
		}
	}
	plannedLink := false
	for _, link := range result.Links {
		if link.Created {
			errs = append(errs, "install dry-run reported created link:"+link.Path)
		}
		if link.WouldCreate {
			plannedLink = true
		}
	}
	if !plannedWrite || !plannedLink {
		errs = append(errs, "install dry-run did not expose planned writes and links")
	}
	for _, path := range []string{
		filepath.Join(tempHome, ".codex"),
		filepath.Join(tempHome, ".claude"),
		filepath.Join(tempRoot, "configs"),
		filepath.Join(tempRoot, ".mcp.json"),
		filepath.Join(tempRoot, ".claude"),
	} {
		if pathExists(path) {
			errs = append(errs, "install dry-run wrote unexpected path:"+path)
		}
	}
	return errs
}
