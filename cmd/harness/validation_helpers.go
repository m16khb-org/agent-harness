package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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
		filepath.Join("internal", "core", "docs.go"),
		filepath.Join("internal", "core", "project_docs.go"),
		filepath.Join("internal", "core", "inspect.go"),
		filepath.Join("internal", "core", "policy.go"),
		filepath.Join("internal", "core", "preflight.go"),
		filepath.Join("internal", "core", "state.go"),
		filepath.Join("cmd", "harness", "contract_golden_test.go"),
		filepath.Join("cmd", "harness", "response_contract_golden_test.go"),
		filepath.Join("cmd", "harness", "self_augment_summary_test.go"),
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

func forbiddenNameHits(root string) []string {
	var hits []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				rel = name
			}
			if shouldSkipForbiddenNameScanDir(name, rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if name == ".git" {
			return nil
		}
		if name == "LICENSE" {
			return nil
		}
		info, statErr := d.Info()
		if statErr != nil || info.Size() > 2*1024*1024 {
			return nil
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil || bytes.Contains(b, []byte{0}) {
			return nil
		}
		text := allowCurrentOwnerHandle(string(b))
		for _, needle := range forbiddenLegacyNeedles() {
			if strings.Contains(text, needle) {
				rel, _ := filepath.Rel(root, path)
				hits = append(hits, rel+" contains "+needle)
				break
			}
		}
		return nil
	})
	sort.Strings(hits)
	if len(hits) > 20 {
		return hits[:20]
	}
	return hits
}

func shouldSkipForbiddenNameScanDir(name, rel string) bool {
	switch name {
	case ".git", "bin", "cache", ".cache", ".codex", ".codegraph", ".omc", ".omx", ".antigravitycli":
		return true
	}
	return filepath.ToSlash(rel) == ".claude/hooks/.logs"
}

func forbiddenLegacyNeedles() []string {
	return []string{"m" + "16kh", "m" + "16h", "M" + "16H", "m" + "16"}
}

func currentOwnerHandle() string {
	return "m" + "16khb"
}

func allowCurrentOwnerHandle(text string) string {
	return strings.ReplaceAll(text, currentOwnerHandle(), "$CURRENT_OWNER")
}

func containsForbiddenLegacyOutsideRuntimePaths(text, root string) bool {
	sanitized := allowCurrentOwnerHandle(text)
	replacements := []string{}
	if abs, err := filepath.Abs(root); err == nil {
		replacements = append(replacements, abs)
	}
	if home, err := os.UserHomeDir(); err == nil {
		replacements = append(replacements, home)
	}
	for _, runtimePath := range replacements {
		if runtimePath == "" || runtimePath == string(filepath.Separator) {
			continue
		}
		sanitized = strings.ReplaceAll(sanitized, runtimePath, "$RUNTIME_PATH")
	}
	for _, needle := range forbiddenLegacyNeedles() {
		if strings.Contains(sanitized, needle) {
			return true
		}
	}
	return false
}
