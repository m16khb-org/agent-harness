package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent-harness/internal/core"
)

type docsValidationDeps struct {
	readFile   func(string) ([]byte, error)
	listDocs   func(string) []string
	listSkills func(string) ([]string, error)
	exists     func(string) bool
	glob       func(string) ([]string, error)
	rel        func(string, string) (string, error)
}

func (deps docsValidationDeps) withDefaults() docsValidationDeps {
	if deps.readFile == nil {
		deps.readFile = os.ReadFile
	}
	if deps.listDocs == nil {
		deps.listDocs = core.ListDocs
	}
	if deps.listSkills == nil {
		deps.listSkills = core.ListSkillNames
	}
	if deps.exists == nil {
		deps.exists = exists
	}
	if deps.glob == nil {
		deps.glob = filepath.Glob
	}
	if deps.rel == nil {
		deps.rel = filepath.Rel
	}
	return deps
}

func validateQAGate(root string) StepResult {
	return validateQAGateWithDeps(root, docsValidationDeps{})
}

func validateQAGateWithDeps(root string, deps docsValidationDeps) StepResult {
	deps = deps.withDefaults()
	started := time.Now()
	errs := []string{}
	requiredDocs := map[string][]string{
		filepath.Join(root, "GENIUS_THINK.md"):                                {"천재적 사고", "Mermaid"},
		filepath.Join(root, "skills", "self-augment", "SELF_AUGMENTATION.md"): {"Self-augmentation", "95"},
		filepath.Join(root, "skills", "self-verify", "SKILL.md"):              {"Self-verification", "95"},
		filepath.Join(root, ".agent-harness", "TESTING.md"):                   {"Well-structured tests", "Poorly-structured tests"},
	}
	for path, needles := range requiredDocs {
		b, err := deps.readFile(path)
		if err != nil {
			errs = append(errs, "missing QA doc "+path)
			continue
		}
		text := string(b)
		for _, needle := range needles {
			if !strings.Contains(text, needle) {
				errs = append(errs, fmt.Sprintf("%s missing %q", path, needle))
			}
		}
	}
	skills, err := deps.listSkills(root)
	if err != nil {
		errs = append(errs, "list skills: "+err.Error())
	}
	for _, want := range []string{"atomic-commit-push", "self-augment"} {
		if !containsString(skills, want) {
			errs = append(errs, "missing shared skill "+want)
		}
	}
	for _, skill := range skills {
		skillDir := filepath.Join(root, "skills", skill)
		skillMD := filepath.Join(skillDir, "SKILL.md")
		b, err := deps.readFile(skillMD)
		if err != nil {
			errs = append(errs, "missing skill file "+skillMD)
			continue
		}
		text := string(b)
		if !strings.Contains(text, "\nname:") && !strings.HasPrefix(text, "---\nname:") {
			errs = append(errs, "skill missing name frontmatter "+skill)
		}
		if !strings.Contains(text, "\ndescription:") {
			errs = append(errs, "skill missing description frontmatter "+skill)
		}
		if !deps.exists(filepath.Join(skillDir, "agents", "openai.yaml")) {
			errs = append(errs, "skill missing agents/openai.yaml "+skill)
		}
	}
	errs = append(errs, validateMermaidDocsWithDeps(root, deps)...)
	return assertionStep("QA gate", started, errs)
}
