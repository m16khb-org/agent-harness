package validationcli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func validateHarnessInvariants(root string) StepResult {
	started := time.Now()
	errs := []string{}
	required := []string{
		"AGENTS.md",
		"CLAUDE.md",
		filepath.Join(".agent-harness", "OPERATIONS.md"),
		filepath.Join(".agent-harness", "COMMIT_POLICY.md"),
		filepath.Join("skills", skillName, "SKILL.md"),
		filepath.Join("skills", skillName, "agents", "openai.yaml"),
		filepath.Join("skills", skillName, "scripts", "git_preflight.py"),
		filepath.Join("skills", "self-verify", "SKILL.md"),
		filepath.Join("skills", "self-verify", "CANDIDATES.md"),
		filepath.Join("skills", "project-bootstrap", "SKILL.md"),
		filepath.Join("internal", "core", "docs", "docs.go"),
		filepath.Join("internal", "core", "project_docs_bootstrap.go"),
		filepath.Join("internal", "core", "project_docs_render.go"),
		filepath.Join("internal", "core", "inspect", "inspect.go"),
		filepath.Join("internal", "core", "policy", "policy_evaluate.go"),
		filepath.Join("internal", "core", "policy", "policy_paths.go"),
		filepath.Join("internal", "core", "preflight", "preflight.go"),
		filepath.Join("internal", "core", "preflight_facade.go"),
		filepath.Join("internal", "core", "state", "state_io.go"),
		filepath.Join("internal", "core", "state", "state_migrate.go"),
		filepath.Join("internal", "core", "state", "state_types.go"),
		filepath.Join("cmd", "harness", "contract_golden_test.go"),
		filepath.Join("cmd", "harness", "response_contract_golden_test.go"),
		filepath.Join("cmd", "harness", "selfworkflow", "self_augment_summary_test.go"),
		filepath.Join("cmd", "harness", "testdata", "usage.golden.txt"),
		filepath.Join("cmd", "harness", "testdata", "mcp_tools.golden.json"),
		filepath.Join("cmd", "harness", "testdata", "mcp_resources.golden.json"),
		filepath.Join("cmd", "harness", "testdata", "response_contracts.golden.json"),
		filepath.Join(".mcp.json"),
	}
	for _, rel := range required {
		if !exists(filepath.Join(root, rel)) {
			errs = append(errs, "missing "+rel)
		}
	}
	if err := validateSkillShape(filepath.Join(root, "skills", skillName)); err != nil {
		errs = append(errs, err.Error())
	}
	if hits := forbiddenNameHits(root); len(hits) > 0 {
		errs = append(errs, "forbidden legacy name hits: "+strings.Join(hits, "; "))
	}
	return assertionStep("harness invariants", started, errs)
}

func validateSkillShape(skillDir string) error {
	body, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		return err
	}
	text := string(body)
	if !strings.HasPrefix(text, "---\n") {
		return fmt.Errorf("SKILL.md missing YAML frontmatter")
	}
	front := strings.SplitN(text, "---", 3)
	if len(front) < 3 || !strings.Contains(front[1], "name: "+skillName) || !strings.Contains(front[1], "description:") {
		return fmt.Errorf("SKILL.md frontmatter must include name and description")
	}
	if !exists(filepath.Join(skillDir, "agents", "openai.yaml")) {
		return fmt.Errorf("agents/openai.yaml missing")
	}
	return nil
}
