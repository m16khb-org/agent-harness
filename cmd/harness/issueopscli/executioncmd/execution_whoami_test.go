package executioncmd

import (
	"os"
	"strings"
	"testing"
)

// 이슈 #90 발견 3: owner가 claim identity(pid/started-at/executable)를 shell
// 확장($$, $(date), $SHELL) 없이 채울 수 있도록, 호출 프로세스의 native
// ancestry receipt를 그대로 노출하는 read-only 표면이 필요하다.
func TestExecutionWhoamiExposesCallerAncestryReceipts(t *testing.T) {
	t.Setenv("CODEX_THREAD_ID", "019fabc8-1c66-73c0-89f5-d9b80914beef")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	var captured any
	deps := Deps{PrintJSON: func(value any) error { captured = value; return nil }}
	if err := Run([]string{"whoami", "--json"}, deps); err != nil {
		t.Fatalf("whoami must not require state or provisioners: %v", err)
	}
	result, ok := captured.(ExecutionWhoamiResult)
	if !ok || !result.OK || len(result.Ancestry) == 0 {
		t.Fatalf("whoami must expose a non-empty ancestry: %#v", captured)
	}
	self := result.Ancestry[0]
	if self.PID != os.Getpid() || self.StartedAt == "" || self.Executable == "" {
		t.Fatalf("first receipt must be the calling process with full identity: %+v", self)
	}
	if result.Host != "codex" || result.SessionID != "019fabc8-1c66-73c0-89f5-d9b80914beef" || result.SessionIDSource != "CODEX_THREAD_ID" {
		t.Fatalf("whoami must expose the verified Codex session identity: %+v", result)
	}
	if len(result.ClaimActorFlags) == 0 ||
		!strings.Contains(result.ClaimActorFlags[0], "--host codex") ||
		!strings.Contains(result.ClaimActorFlags[0], "--session-id '019fabc8-1c66-73c0-89f5-d9b80914beef'") {
		t.Fatalf("whoami must render copy-pasteable claim actor flags: %+v", result.ClaimActorFlags)
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

func TestNativeSessionIdentityFromEnvRejectsAmbiguousHosts(t *testing.T) {
	values := map[string]string{
		"CODEX_THREAD_ID":        "codex-session",
		"CLAUDE_CODE_SESSION_ID": "claude-session",
	}
	_, err := nativeSessionIdentityFromEnv(func(key string) string { return values[key] })
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("both native host identities must fail closed: %v", err)
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
