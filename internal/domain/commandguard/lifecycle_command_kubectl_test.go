package commandguard

import (
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
