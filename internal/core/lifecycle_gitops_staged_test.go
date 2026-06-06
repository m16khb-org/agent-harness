package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreToolUseGitOpsKubectlBlocksMutatingCommands(t *testing.T) {
	for _, command := range []string{
		`kubectl apply -f k8s/deployment.yaml`,
		`/usr/local/bin/kubectl delete pod api-0 -n prod`,
		`kubectl get pods && kubectl rollout restart deployment/api -n prod`,
	} {
		got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
			Tool:                 "bash",
			Command:              command,
			EnforceGitOpsKubectl: true,
		})
		if got.Decision != "block" || !strings.Contains(got.Reason, "GitOps") {
			t.Fatalf("expected mutating kubectl command to be blocked: %q -> %+v", command, got)
		}
	}
}

func TestPreToolUseGitOpsKubectlAsksForLiveAccessCommands(t *testing.T) {
	for _, command := range []string{
		`kubectl exec -it pod/api-0 -- sh`,
		`kubectl port-forward svc/api 8080:80 -n prod`,
		`kubectl get pods && kubectl exec deployment/api -- env`,
	} {
		got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
			Tool:                 "bash",
			Command:              command,
			EnforceGitOpsKubectl: true,
		})
		if got.Decision != "ask" || !strings.Contains(got.Reason, "kubectl") || !strings.Contains(got.Reason, "confirm") {
			t.Fatalf("expected live kubectl access to ask for confirmation: %q -> %+v", command, got)
		}
	}
}

func TestPreToolUseGitOpsKubectlAllowsReadOnlyCommands(t *testing.T) {
	for _, command := range []string{
		`kubectl get pods -n prod`,
		`kubectl logs deployment/api -n prod --tail=100`,
		`kubectl diff -f k8s/`,
		`kubectl apply --dry-run=client -f k8s/deployment.yaml`,
		`kubectl apply --dry-run=server -f k8s/deployment.yaml`,
		`kubectl apply --dry-run server -f k8s/deployment.yaml`,
	} {
		got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
			Tool:                 "bash",
			Command:              command,
			EnforceGitOpsKubectl: true,
		})
		if got.Decision != "allow" {
			t.Fatalf("expected read-only kubectl command to be allowed: %q -> %+v", command, got)
		}
	}
}

func TestPreToolUseStagedChecksAsksForBroadBiomeCommands(t *testing.T) {
	for _, command := range []string{
		`npx biome check apps libs`,
		`biome format --check apps libs`,
		`npm run lint:check`,
	} {
		repo := t.TempDir()
		if err := os.WriteFile(filepath.Join(repo, "package.json"), []byte(`{"scripts":{"lint:check":"biome check apps libs"}}`), 0o644); err != nil {
			t.Fatal(err)
		}
		got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
			Repo:                repo,
			Tool:                "bash",
			Command:             command,
			EnforceStagedChecks: true,
		})
		if got.Decision != "ask" || !strings.Contains(got.Reason, "staged") {
			t.Fatalf("expected broad staged check to ask for confirmation: %q -> %+v", command, got)
		}
	}
}

func TestPreToolUseStagedChecksAllowsScopedBiomeCommands(t *testing.T) {
	for _, command := range []string{
		`npx biome check --staged --no-errors-on-unmatched`,
		`biome format --staged`,
		`biome check scripts/check-swagger-rules.js package.json`,
		`npm run lint:check`,
	} {
		repo := t.TempDir()
		if err := os.WriteFile(filepath.Join(repo, "package.json"), []byte(`{"scripts":{"lint:check":"biome check --staged --no-errors-on-unmatched"}}`), 0o644); err != nil {
			t.Fatal(err)
		}
		got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
			Repo:                repo,
			Tool:                "bash",
			Command:             command,
			EnforceStagedChecks: true,
		})
		if got.Decision != "allow" {
			t.Fatalf("expected scoped staged check to be allowed: %q -> %+v", command, got)
		}
	}
}
