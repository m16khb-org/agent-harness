package commandguard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKubectlFlagTakesValueRecognizesValueFlags(t *testing.T) {
	for _, flag := range []string{"-n", "--namespace", "--context=prod", "--kubeconfig", "-o", "-l"} {
		if !KubectlFlagTakesValue(flag) {
			t.Fatalf("KubectlFlagTakesValue(%q) = false, want true", flag)
		}
	}

	for _, flag := range []string{"--watch", "--dry-run", "--force"} {
		if KubectlFlagTakesValue(flag) {
			t.Fatalf("KubectlFlagTakesValue(%q) = true, want false", flag)
		}
	}
}

func TestGitOpsKubectlDecisionBlocksMutatingCommands(t *testing.T) {
	tests := []struct {
		name        string
		tool        string
		command     string
		wantAction  string
		wantMessage string
	}{
		{
			name:        "direct apply is blocked",
			tool:        "Bash",
			command:     "kubectl apply -f deploy.yaml",
			wantAction:  "block",
			wantMessage: "GitOps is the source of truth",
		},
		{
			name:        "rollout restart is blocked after namespace flag",
			tool:        "Bash",
			command:     "kubectl --namespace prod rollout restart deploy/api",
			wantAction:  "block",
			wantMessage: "GitOps is the source of truth",
		},
		{
			name:        "exec asks for confirmation",
			tool:        "Bash",
			command:     "kubectl exec deploy/api -- printenv",
			wantAction:  "ask",
			wantMessage: "live cluster access",
		},
		{
			name:       "dry run apply is allowed",
			tool:       "Bash",
			command:    "kubectl apply --dry-run=server -f deploy.yaml",
			wantAction: "",
		},
		{
			name:       "non shell tools are ignored",
			tool:       "Read",
			command:    "kubectl delete pod api",
			wantAction: "",
		},
		{
			name:       "read only command is allowed",
			tool:       "Bash",
			command:    "kubectl get pods -n prod",
			wantAction: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action, message := GitOpsKubectlDecision(tt.tool, tt.command)
			if action != tt.wantAction {
				t.Fatalf("action=%q, want %q; message=%q", action, tt.wantAction, message)
			}
			if tt.wantMessage != "" && !strings.Contains(message, tt.wantMessage) {
				t.Fatalf("message=%q, want substring %q", message, tt.wantMessage)
			}
		})
	}
}

func TestKubectlVerbSkipsFlagsAndCapturesSubverb(t *testing.T) {
	verb, subverb := kubectlVerb([]string{"--namespace", "prod", "rollout", "restart", "deploy/api"})
	if verb != "rollout" || subverb != "restart" {
		t.Fatalf("kubectlVerb returned %q/%q, want rollout/restart", verb, subverb)
	}

	verb, subverb = kubectlVerb([]string{"--context=prod", "--watch"})
	if verb != "" || subverb != "" {
		t.Fatalf("kubectlVerb with only flags returned %q/%q, want empty", verb, subverb)
	}
}

func TestGitOpsKubectlDecisionHandlesBoundaryTokens(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		wantAction string
	}{
		{name: "separate dry-run flag allows apply", command: "kubectl apply --dry-run client -f deploy.yaml", wantAction: ""},
		{name: "shell separator stops rollout subverb", command: "kubectl rollout ; restart deploy/api", wantAction: ""},
		{name: "rollout undo is blocked", command: "kubectl rollout undo deploy/api", wantAction: "block"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action, _ := GitOpsKubectlDecision("Bash", tt.command)
			if action != tt.wantAction {
				t.Fatalf("action=%q, want %q", action, tt.wantAction)
			}
		})
	}
}

func TestStagedCheckDecisionWarnsForBroadBiomeCommands(t *testing.T) {
	repo := t.TempDir()
	writeCommandGuardFile(t, filepath.Join(repo, "package.json"), `{"scripts":{"lint":"biome check apps libs","format":"biome format --staged apps"}}`)

	tests := []struct {
		name       string
		tool       string
		command    string
		wantAction string
	}{
		{name: "direct broad biome check asks", tool: "Bash", command: "biome check apps libs", wantAction: "ask"},
		{name: "scoped biome check is allowed", tool: "Bash", command: "biome check --staged apps libs", wantAction: ""},
		{name: "package script expansion asks", tool: "Bash", command: "npm run lint", wantAction: "ask"},
		{name: "scoped package script is allowed", tool: "Bash", command: "npm run format", wantAction: ""},
		{name: "non shell tools are ignored", tool: "Read", command: "biome check apps libs", wantAction: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action, message := StagedCheckDecision(tt.tool, repo, tt.command)
			if action != tt.wantAction {
				t.Fatalf("action=%q, want %q; message=%q", action, tt.wantAction, message)
			}
			if tt.wantAction == "ask" && !strings.Contains(message, "Broad lint/format checks") {
				t.Fatalf("message=%q, want broad-check warning", message)
			}
		})
	}
}

func TestPackageScriptAndBiomeHelpersHandleBoundaries(t *testing.T) {
	repo := t.TempDir()
	writeCommandGuardFile(t, filepath.Join(repo, "package.json"), `{"scripts":{"quoted":"biome check \"apps\" 'libs'","empty":"   "}}`)

	if got := PackageScript(repo, "quoted"); got != `biome check "apps" 'libs'` {
		t.Fatalf("PackageScript returned %q", got)
	}
	if got := PackageScript(repo, "missing"); got != "" {
		t.Fatalf("missing PackageScript returned %q, want empty", got)
	}
	if got := PackageScript("", "quoted"); got != "" {
		t.Fatalf("empty repo PackageScript returned %q, want empty", got)
	}
	if !BroadBiomeCheckCommand(`biome check "apps" 'libs'`) {
		t.Fatalf("quoted broad biome dirs were not detected")
	}
	if BroadBiomeCheckCommand("biome check --since main apps libs") {
		t.Fatalf("scoped biome command should not be broad")
	}
	if BroadBiomeCheckCommand("biome check packages services") {
		t.Fatalf("non-app/lib directories should not count as broad repo dirs")
	}
	if got := PackageScript(repo, "empty"); got != "" {
		t.Fatalf("empty PackageScript returned %q, want empty", got)
	}
}

func writeCommandGuardFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
