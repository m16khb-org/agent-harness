package lifecycle

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var liveApprovalTokenPattern = regexp.MustCompile(`AH-[A-HJ-NP-Z2-9]{6}`)

func TestCodexKubectlReadOnlyExecApprovalAllowsSameScopeRepeatedly(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	req := HookToolUseLifecycleRequest{
		Repo:                 repo,
		CWD:                  repo,
		Host:                 "codex",
		SessionID:            "session-1",
		Tool:                 "Bash",
		Command:              "kubectl --context bc-stgdev -n stg exec deploy/rest-api-gateway -- getent hosts grpc-user",
		EnforceGitOpsKubectl: true,
	}

	first := BuildLifecyclePreToolUseDecision(req)
	token := liveApprovalTokenPattern.FindString(first.Reason)
	if first.Decision != "ask" || token == "" {
		t.Fatalf("first live-access request did not issue approval token: %+v", first)
	}
	repeated := BuildLifecyclePreToolUseDecision(req)
	if got := liveApprovalTokenPattern.FindString(repeated.Reason); got != token {
		t.Fatalf("pending request token changed: first=%q repeated=%q", token, got)
	}

	approval := ApproveCodexKubectlLiveAccess(repo, req.Host, req.SessionID, "승인 "+token)
	if !approval.Handled || approval.AdditionalContext == "" {
		t.Fatalf("approval was not recorded: %+v", approval)
	}
	if allowed := BuildLifecyclePreToolUseDecision(req); allowed.Decision != "allow" {
		t.Fatalf("approved safe request was not allowed: %+v", allowed)
	}
	reused := req
	reused.CWD = filepath.Join(repo, "nested")
	reused.Tool = "shell"
	reused.Command = "kubectl --context bc-stgdev -n stg exec -c linkerd-proxy pod/gateway-2 -- curl -fsS http://localhost:4191/metrics"
	if allowed := BuildLifecyclePreToolUseDecision(reused); allowed.Decision != "allow" {
		t.Fatalf("same-scope safe request was not allowed: %+v", allowed)
	}
}

func TestCodexKubectlReadOnlyExecScopeChangeRequiresApproval(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	req := HookToolUseLifecycleRequest{
		Repo: repo, CWD: repo, Host: "codex", SessionID: "session-1", Tool: "Bash",
		Command:              "kubectl --context bc-stgdev -n stg exec deploy/gateway -- getent hosts grpc-user",
		EnforceGitOpsKubectl: true,
	}
	first := BuildLifecyclePreToolUseDecision(req)
	token := liveApprovalTokenPattern.FindString(first.Reason)
	ApproveCodexKubectlLiveAccess(repo, req.Host, req.SessionID, "승인 "+token)
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("base scope not allowed: %+v", got)
	}

	changed := req
	changed.Command = "kubectl --context bc-stgdev -n prod exec deploy/gateway -- getent hosts grpc-user"
	got := BuildLifecyclePreToolUseDecision(changed)
	changedToken := liveApprovalTokenPattern.FindString(got.Reason)
	if got.Decision != "ask" || changedToken == "" || changedToken == token {
		t.Fatalf("changed scope did not require new approval: %+v", got)
	}
}

func TestCodexKubectlUnsafeExecBlocksWithoutApprovalState(t *testing.T) {
	for _, command := range []string{
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- env",
		"kubectl -n stg exec deploy/gateway -- getent hosts grpc-user",
		"kubectl --context bc-stgdev -n stg exec deploy/gateway -- sh -c 'cat /etc/resolv.conf'",
	} {
		t.Run(command, func(t *testing.T) {
			stateRoot := t.TempDir()
			t.Setenv("HARNESS_STATE_DIR", stateRoot)
			repo := t.TempDir()
			got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
				Repo: repo, CWD: repo, Host: "codex", SessionID: "session-1", Tool: "Bash", Command: command,
				EnforceGitOpsKubectl: true,
			})
			if got.Decision != "block" || strings.Contains(got.Reason, "AH-") || strings.Contains(got.Reason, "bc-stgdev") || strings.Contains(got.Reason, "stg") {
				t.Fatalf("unsafe exec did not fail closed: %+v", got)
			}
			if _, err := os.Stat(filepath.Join(stateRoot, "projects")); !os.IsNotExist(err) {
				t.Fatalf("unsafe exec created approval state: %v", err)
			}
		})
	}
}

func TestCodexKubectlPortForwardApprovalRemainsOneShot(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	req := HookToolUseLifecycleRequest{
		Repo: repo, CWD: repo, Host: "codex", SessionID: "session-1", Tool: "Bash",
		Command:              "kubectl --context bc-stgdev -n stg port-forward svc/api 8080:80",
		EnforceGitOpsKubectl: true,
	}
	first := BuildLifecyclePreToolUseDecision(req)
	token := liveApprovalTokenPattern.FindString(first.Reason)
	if first.Decision != "ask" || token == "" {
		t.Fatalf("port-forward did not issue token: %+v", first)
	}
	ApproveCodexKubectlLiveAccess(repo, req.Host, req.SessionID, "승인 "+token)
	if got := BuildLifecyclePreToolUseDecision(req); got.Decision != "allow" {
		t.Fatalf("approved port-forward not allowed: %+v", got)
	}
	again := BuildLifecyclePreToolUseDecision(req)
	if again.Decision != "ask" || liveApprovalTokenPattern.FindString(again.Reason) == "" || liveApprovalTokenPattern.FindString(again.Reason) == token {
		t.Fatalf("port-forward grant was reusable: first=%q again=%+v", token, again)
	}
}

func TestCodexKubectlLiveApprovalFailsClosedWithoutSession(t *testing.T) {
	t.Setenv("HARNESS_STATE_DIR", t.TempDir())
	repo := t.TempDir()
	got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo:                 repo,
		CWD:                  repo,
		Host:                 "codex",
		Tool:                 "Bash",
		Command:              "kubectl port-forward svc/api 8080:80",
		EnforceGitOpsKubectl: true,
	})
	if got.Decision != "block" || strings.Contains(got.Reason, "AH-") {
		t.Fatalf("missing session did not fail closed: %+v", got)
	}
}

func TestClaudeKubectlLiveAccessKeepsNativeAskWithoutApprovalState(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("HARNESS_STATE_DIR", stateRoot)
	repo := t.TempDir()
	got := BuildLifecyclePreToolUseDecision(HookToolUseLifecycleRequest{
		Repo:                 repo,
		CWD:                  repo,
		Host:                 "claude",
		SessionID:            "session-1",
		Tool:                 "Bash",
		Command:              "kubectl exec deploy/api -- env",
		EnforceGitOpsKubectl: true,
	})
	if got.Decision != "ask" || strings.Contains(got.Reason, "AH-") {
		t.Fatalf("Claude live access did not preserve native ask: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(stateRoot, "projects")); !os.IsNotExist(err) {
		t.Fatalf("Claude native ask created approval state: %v", err)
	}
}
