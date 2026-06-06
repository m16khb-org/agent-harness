package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateRedactionAuditWithDepsCoversSuccessSecretAndReadFailure(t *testing.T) {
	root := t.TempDir()
	doc := filepath.Join(root, "AGENTS.md")
	fixture := filepath.Join(root, "cmd", "harness", "testdata", "usage.golden.txt")
	missing := filepath.Join(root, "skills", "self-verify", "SKILL.md")
	deps := docsValidationDeps{
		listDocs: func(string) []string { return []string{doc, doc, missing} },
		glob: func(pattern string) ([]string, error) {
			if strings.Contains(pattern, filepath.Join("cmd", "harness", "testdata")) {
				return []string{fixture}, nil
			}
			return nil, nil
		},
		exists: func(path string) bool { return path != missing },
		readFile: func(path string) ([]byte, error) {
			switch path {
			case doc:
				return []byte("TOKEN=redacted\n"), nil
			case fixture:
				return []byte("OPENAI_API_KEY=sk-123456789012345678901234\n"), nil
			default:
				return nil, errors.New("unexpected read")
			}
		},
		rel: filepath.Rel,
	}

	files := redactionAuditFilesWithDeps(root, deps)
	if len(files) != 2 || files[0] != doc || files[1] != fixture {
		t.Fatalf("expected sorted deduped existing files, got %v", files)
	}
	step := validateRedactionAuditWithDeps(root, deps)
	if step.OK || !strings.Contains(step.Error, "cmd/harness/testdata/usage.golden.txt: line 1 contains openai_token") {
		t.Fatalf("expected secret finding, got %+v", step)
	}

	deps.readFile = func(path string) ([]byte, error) { return nil, errors.New("read blocked") }
	failedRead := validateRedactionAuditWithDeps(root, deps)
	if failedRead.OK || !strings.Contains(failedRead.Error, "read redaction audit file "+doc+": read blocked") {
		t.Fatalf("expected read failure, got %+v", failedRead)
	}
}

func TestRedactionAuditFilesWrapperIncludesDefaultFixtureAndSkillPaths(t *testing.T) {
	root := t.TempDir()
	fixture := filepath.Join(root, "cmd", "harness", "testdata", "usage.golden.txt")
	skill := filepath.Join(root, "skills", "self-verify", "SKILL.md")
	agent := filepath.Join(root, "skills", "self-verify", "agents", "openai.yaml")
	for _, path := range []string{fixture, skill, agent} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("fixture\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	files := redactionAuditFiles(root)

	if len(files) != 3 || files[0] != fixture || files[1] != skill || files[2] != agent {
		t.Fatalf("expected sorted default redaction files, got %v", files)
	}
}

func TestValidateQAGateWithDepsCoversHealthyAndAggregateFailures(t *testing.T) {
	root := t.TempDir()
	selfAugmentDoc := filepath.Join(root, "skills", "self-augment", "SELF_AUGMENTATION.md")
	selfVerifyDoc := filepath.Join(root, "skills", "self-verify", "SKILL.md")
	testingDoc := filepath.Join(root, ".agent-harness", "TESTING.md")
	geniusDoc := filepath.Join(root, "GENIUS_THINK.md")
	atomicSkill := filepath.Join(root, "skills", "atomic-commit-push", "SKILL.md")
	selfAugmentSkill := filepath.Join(root, "skills", "self-augment", "SKILL.md")
	deps := docsValidationDeps{
		readFile: func(path string) ([]byte, error) {
			switch path {
			case geniusDoc:
				return []byte("천재적 사고\n```mermaid\ngraph TD\nA[bad]\n```\nMermaid\n"), nil
			case selfAugmentDoc:
				return []byte("Self-augmentation 95\n"), nil
			case selfVerifyDoc:
				return []byte("---\nname: self-verify\ndescription: verify\n---\nSelf-verification 95\n"), nil
			case testingDoc:
				return []byte("Well-structured tests\nPoorly-structured tests\n"), nil
			case atomicSkill:
				return []byte("---\nname: atomic-commit-push\ndescription: git\n---\n"), nil
			case selfAugmentSkill:
				return []byte("---\nname: self-augment\ndescription: augment\n---\n"), nil
			default:
				return nil, errors.New("unexpected read")
			}
		},
		listSkills: func(string) ([]string, error) { return []string{"atomic-commit-push", "self-augment"}, nil },
		listDocs:   func(string) []string { return []string{geniusDoc} },
		exists:     func(string) bool { return true },
		rel:        filepath.Rel,
	}

	step := validateQAGateWithDeps(root, deps)
	if step.OK || !strings.Contains(step.Error, "GENIUS_THINK.md:4 mermaid node text must start with a quote") {
		t.Fatalf("expected mermaid lint failure, got %+v", step)
	}

	deps.listSkills = func(string) ([]string, error) {
		return []string{"atomic-commit-push", "broken"}, errors.New("skill scan failed")
	}
	deps.exists = func(path string) bool {
		return !strings.Contains(path, filepath.Join("broken", "agents", "openai.yaml"))
	}
	deps.readFile = func(path string) ([]byte, error) {
		if strings.HasSuffix(path, filepath.Join("broken", "SKILL.md")) {
			return []byte("---\nname: broken\n---\n"), nil
		}
		if path == selfAugmentDoc {
			return []byte("Self-augmentation\n"), nil
		}
		if path == atomicSkill {
			return []byte("---\nname: atomic-commit-push\ndescription: git\n---\n"), nil
		}
		return []byte("Self-verification 95\nWell-structured tests\nPoorly-structured tests\n천재적 사고 Mermaid\n"), nil
	}
	aggregate := validateQAGateWithDeps(root, deps)
	for _, want := range []string{
		"list skills: skill scan failed",
		"missing shared skill self-augment",
		"SELF_AUGMENTATION.md missing \"95\"",
		"skill missing description frontmatter broken",
		"skill missing agents/openai.yaml broken",
	} {
		if !strings.Contains(aggregate.Error, want) {
			t.Fatalf("expected %q in aggregate error, got %+v", want, aggregate)
		}
	}
}

func TestValidateMermaidDocsWithDepsCoversReadFailureAndRelFallback(t *testing.T) {
	root := t.TempDir()
	doc := filepath.Join(root, "GENIUS_THINK.md")
	deps := docsValidationDeps{
		listDocs: func(string) []string { return []string{doc} },
		readFile: func(string) ([]byte, error) { return nil, errors.New("cannot read") },
		rel:      func(string, string) (string, error) { return "", errors.New("bad rel") },
	}

	issues := validateMermaidDocsWithDeps(root, deps)
	if len(issues) != 1 || !strings.Contains(issues[0], "read mermaid doc "+doc+": cannot read") {
		t.Fatalf("expected read failure with absolute fallback, got %v", issues)
	}
}
