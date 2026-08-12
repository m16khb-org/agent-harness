package executioncmd

import (
	"strings"
	"testing"

	model "agent-harness/internal/contract/issueops"
)

func TestExecutionWhoamiResultAdvertisesTheReusableHostReceiptFirst(t *testing.T) {
	identity := nativeSessionIdentity{
		Host: "codex", SessionID: "019fabc8-1c66-73c0-89f5-d9b80914beef",
		Source: "CODEX_THREAD_ID",
	}
	result, err := executionWhoamiResult(identity, 101, []model.NativeProcessReceipt{
		{PID: 101, StartedAt: "2026-07-30T00:00:00Z", Executable: "agent-harness"},
		{PID: 150, StartedAt: "2026-07-30T00:00:00Z", Executable: "/bin/zsh"},
		{PID: 202, StartedAt: "2026-07-29T00:00:00Z", Executable: "/opt/codex"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.Host != identity.Host || result.SessionID != identity.SessionID ||
		len(result.Ancestry) != 1 || result.Ancestry[0].PID != 202 {
		t.Fatalf("whoami result = %+v", result)
	}
	if len(result.ClaimActorFlags) != 1 ||
		!strings.Contains(result.ClaimActorFlags[0], "--session-pid 202") ||
		!strings.Contains(result.ClaimActorFlags[0], "--session-id '019fabc8-1c66-73c0-89f5-d9b80914beef'") {
		t.Fatalf("whoami claim flags = %+v", result.ClaimActorFlags)
	}
}

func TestReusableNativeProcessAncestryStartsAtTheCodexHost(t *testing.T) {
	ancestry := []model.NativeProcessReceipt{
		{PID: 101, StartedAt: "2026-07-30T00:00:00Z", Executable: "agent-harness"},
		{PID: 150, StartedAt: "2026-07-30T00:00:00Z", Executable: "/bin/zsh"},
		{PID: 202, StartedAt: "2026-07-29T00:00:00Z", Executable: "codex"},
		{PID: 303, StartedAt: "2026-07-28T00:00:00Z", Executable: "node"},
	}
	got, err := reusableNativeProcessAncestry("codex", 101, ancestry)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].PID != 202 || got[1].PID != 303 {
		t.Fatalf("reusable ancestry = %+v", got)
	}
}

func TestReusableNativeProcessAncestryRecognizesOfficialClaudeInstall(t *testing.T) {
	ancestry := []model.NativeProcessReceipt{
		{PID: 101, Executable: "agent-harness"},
		{PID: 202, Executable: "/Users/test/.local/share/claude/versions/2.1.220"},
		{PID: 303, Executable: "/bin/zsh"},
	}
	got, err := reusableNativeProcessAncestry("claude", 101, ancestry)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].PID != 202 {
		t.Fatalf("reusable Claude ancestry = %+v", got)
	}
}

func TestReusableNativeProcessAncestryRecognizesOmoHost(t *testing.T) {
	ancestry := []model.NativeProcessReceipt{
		{PID: 101, Executable: "agent-harness"},
		{PID: 150, Executable: "/bin/zsh"},
		{PID: 202, Executable: "/Users/test/Library/pnpm/bin/omo"},
		{PID: 303, Executable: "node"},
	}
	got, err := reusableNativeProcessAncestry("omo", 101, ancestry)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].PID != 202 || got[1].PID != 303 {
		t.Fatalf("reusable Omo ancestry = %+v", got)
	}
}

func TestReusableNativeProcessAncestryRejectsMissingNativeHost(t *testing.T) {
	_, err := reusableNativeProcessAncestry("codex", 101, []model.NativeProcessReceipt{
		{PID: 101, Executable: "agent-harness"},
		{PID: 202, Executable: "/bin/zsh"},
	})
	if err == nil || !strings.Contains(err.Error(), "codex") {
		t.Fatalf("missing native host error = %v", err)
	}
}

func TestNativeSessionIdentityFromEnvSupportsClaude(t *testing.T) {
	values := map[string]string{
		"CODEX_THREAD_ID":        "",
		"CLAUDE_CODE_SESSION_ID": "5bc74e36-8efc-4b9f-a721-d7a282494dad",
	}
	identity, err := nativeSessionIdentityFromEnv(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("Claude identity must be detected: %v", err)
	}
	if identity.Host != "claude" || identity.SessionID != values["CLAUDE_CODE_SESSION_ID"] ||
		identity.Source != "CLAUDE_CODE_SESSION_ID" {
		t.Fatalf("unexpected Claude identity: %+v", identity)
	}
}

func TestNativeSessionIdentityFromEnvSupportsOmo(t *testing.T) {
	values := map[string]string{
		"CODEX_THREAD_ID":        "",
		"CLAUDE_CODE_SESSION_ID": "",
		"PI_SESSION_ID":          "019ff5b8-7d62-707a-a693-5e7a5e8a3187",
	}
	identity, err := nativeSessionIdentityFromEnv(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("Omo identity must be detected: %v", err)
	}
	if identity.Host != "omo" || identity.SessionID != values["PI_SESSION_ID"] ||
		identity.Source != "PI_SESSION_ID" {
		t.Fatalf("unexpected Omo identity: %+v", identity)
	}
}

func TestNativeSessionIdentityFromEnvRejectsAmbiguousHosts(t *testing.T) {
	values := map[string]string{
		"CODEX_THREAD_ID":        "codex-session",
		"CLAUDE_CODE_SESSION_ID": "claude-session",
		"PI_SESSION_ID":          "omo-session",
	}
	_, err := nativeSessionIdentityFromEnv(func(key string) string { return values[key] })
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("multiple native host identities must fail closed: %v", err)
	}
}

func TestNativeSessionIdentityFromEnvRejectsMissingIdentity(t *testing.T) {
	_, err := nativeSessionIdentityFromEnv(func(string) string { return "" })
	if err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("missing native host identity must fail closed: %v", err)
	}
}

func TestNativeSessionIdentityFromEnvRejectsNonLiteralIdentity(t *testing.T) {
	for _, value := range []string{" session ", "session\nnext"} {
		t.Run(value, func(t *testing.T) {
			_, err := nativeSessionIdentityFromEnv(func(key string) string {
				if key == "CODEX_THREAD_ID" {
					return value
				}
				return ""
			})
			if err == nil {
				t.Fatalf("non-literal native identity must fail closed: %q", value)
			}
		})
	}
}

func TestShellQuoteClaimValuePreservesApostrophe(t *testing.T) {
	if got, want := shellQuoteClaimValue("owner's codex"), `'owner'"'"'s codex'`; got != want {
		t.Fatalf("shellQuoteClaimValue() = %q, want %q", got, want)
	}
}
