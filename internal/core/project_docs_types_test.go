package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProjectDocsBootstrapResultJSONContract(t *testing.T) {
	result := ProjectDocsBootstrapResult{
		OK:       true,
		Kind:     "project_docs_bootstrap",
		RepoRoot: "/repo",
		DocsDir:  "/repo/.agent-harness",
		Write:    true,
		Sync:     true,
		DryRun:   false,
		Signals: ProjectSignals{
			Languages:       []string{"go"},
			PackageManagers: []string{"go"},
			Profile: ProjectProfile{
				VCS: ProjectVCSProfile{
					Provider: "git",
					Hosting:  "github",
				},
				Languages: []string{"go"},
			},
		},
		Files: []ProjectDocsPlannedFile{
			{RelPath: ".agent-harness/ARCHITECTURE.md", Action: "write", SHA256: "abc"},
		},
	}

	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal project docs bootstrap result: %v", err)
	}
	text := string(payload)
	for _, want := range []string{
		`"repo_root":"/repo"`,
		`"docs_dir":"/repo/.agent-harness"`,
		`"package_managers":["go"]`,
		`"lifecycle_state"`,
		`"rel_path":".agent-harness/ARCHITECTURE.md"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("JSON payload missing %s: %s", want, text)
		}
	}
}
