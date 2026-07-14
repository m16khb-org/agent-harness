package liveapproval

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestReadOnlyExecApprovalReusesScopeAndSlidesExpiry(t *testing.T) {
	store, now := newTestStore(t)
	req := testReadOnlyExecRequest()

	first := EvaluateReadOnlyExec(store, req)
	if !first.Handled || first.Allowed || first.Token == "" {
		t.Fatalf("first evaluation = %+v", first)
	}
	if repeated := EvaluateReadOnlyExec(store, req); repeated.Token != first.Token {
		t.Fatalf("pending token rotated: first=%+v repeated=%+v", first, repeated)
	}

	path := readOnlyExecStatePath(t, store, req.SessionID)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{req.Command, req.RepoRoot, req.Context, req.Namespace, "grpc-user", "rest-api-gateway"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("state leaked %q: %s", forbidden, data)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("state mode = %o", info.Mode().Perm())
	}

	approved := Approve(store, ApprovalRequest{Host: req.Host, SessionID: req.SessionID, RepoRoot: req.RepoRoot, Prompt: "승인 " + first.Token})
	if !approved.Handled || !strings.Contains(approved.AdditionalContext, "30분") {
		t.Fatalf("approval = %+v", approved)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), first.Token) || strings.Contains(string(data), "request_fingerprint") {
		t.Fatalf("granted state retained pending identity: %s", data)
	}

	if allowed := EvaluateReadOnlyExec(store, req); !allowed.Allowed {
		t.Fatalf("first scoped exec was not allowed: %+v", allowed)
	}
	changedSameScope := req
	changedSameScope.CWD = filepath.Join(req.RepoRoot, "nested")
	changedSameScope.Tool = "shell"
	changedSameScope.Command = "kubectl --context bc-stgdev -n stg exec -c linkerd-proxy pod/gateway-2 -- curl -fsS http://localhost:4191/metrics"
	*now = now.Add(29*time.Minute + 59*time.Second)
	if allowed := EvaluateReadOnlyExec(store, changedSameScope); !allowed.Allowed {
		t.Fatalf("same-scope exec did not extend grant: %+v", allowed)
	}
	*now = now.Add(ReadOnlyExecGrantTTL)
	expired := EvaluateReadOnlyExec(store, changedSameScope)
	if expired.Allowed || expired.Token == "" {
		t.Fatalf("idle grant did not expire: %+v", expired)
	}
}

func TestReadOnlyExecApprovalRequiresActivationWithinTenMinutes(t *testing.T) {
	store, now := newTestStore(t)
	req := testReadOnlyExecRequest()
	first := EvaluateReadOnlyExec(store, req)
	Approve(store, ApprovalRequest{Host: req.Host, SessionID: req.SessionID, RepoRoot: req.RepoRoot, Prompt: "승인 " + first.Token})

	*now = now.Add(ApprovalTTL)
	got := EvaluateReadOnlyExec(store, req)
	if got.Allowed || got.Token == "" || got.Token == first.Token {
		t.Fatalf("inactive grant survived: first=%+v got=%+v", first, got)
	}
}

func TestReadOnlyExecPendingIsExactButGrantUsesScope(t *testing.T) {
	store, _ := newTestStore(t)
	req := testReadOnlyExecRequest()
	first := EvaluateReadOnlyExec(store, req)
	changed := req
	changed.Command = "kubectl --context bc-stgdev -n stg exec pod/gateway-2 -- nslookup grpc-user"
	second := EvaluateReadOnlyExec(store, changed)
	if second.Token == "" || second.Token == first.Token {
		t.Fatalf("changed pending request reused token: first=%+v second=%+v", first, second)
	}
	Approve(store, ApprovalRequest{Host: req.Host, SessionID: req.SessionID, RepoRoot: req.RepoRoot, Prompt: "승인 " + second.Token})
	if got := EvaluateReadOnlyExec(store, req); !got.Allowed {
		t.Fatalf("same scope did not allow original command: %+v", got)
	}
}

func TestReadOnlyExecGrantIsolatedByScopeFields(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ReadOnlyExecRequest)
	}{
		{name: "session", mutate: func(r *ReadOnlyExecRequest) { r.SessionID = "session-2" }},
		{name: "repo", mutate: func(r *ReadOnlyExecRequest) { r.RepoRoot = t.TempDir(); r.CWD = r.RepoRoot }},
		{name: "context", mutate: func(r *ReadOnlyExecRequest) { r.Context = "bc-prod" }},
		{name: "namespace", mutate: func(r *ReadOnlyExecRequest) { r.Namespace = "prod" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, _ := newTestStore(t)
			req := testReadOnlyExecRequest()
			first := EvaluateReadOnlyExec(store, req)
			Approve(store, ApprovalRequest{Host: req.Host, SessionID: req.SessionID, RepoRoot: req.RepoRoot, Prompt: "승인 " + first.Token})
			if got := EvaluateReadOnlyExec(store, req); !got.Allowed {
				t.Fatalf("base grant not active: %+v", got)
			}
			changed := req
			tt.mutate(&changed)
			got := EvaluateReadOnlyExec(store, changed)
			if got.Allowed || got.Token == "" {
				t.Fatalf("scope mismatch reused grant: %+v", got)
			}
		})
	}
}

func TestReadOnlyExecDoesNotPromoteLegacyExactGrant(t *testing.T) {
	store, _ := newTestStore(t)
	req := testReadOnlyExecRequest()
	legacy := Request{Host: req.Host, SessionID: req.SessionID, RepoRoot: req.RepoRoot, CWD: req.CWD, Tool: req.Tool, Command: req.Command}
	first := Evaluate(store, legacy)
	Approve(store, ApprovalRequest{Host: req.Host, SessionID: req.SessionID, RepoRoot: req.RepoRoot, Prompt: "승인 " + first.Token})

	got := EvaluateReadOnlyExec(store, req)
	if got.Allowed || got.Token == "" {
		t.Fatalf("legacy exact grant became scope authority: %+v", got)
	}
}

func TestApproveRejectsAmbiguousPendingToken(t *testing.T) {
	store, _ := newTestStore(t)
	store.NewToken = func() (string, error) { return "AH-ABC234", nil }
	portForward := testRequest()
	exec := testReadOnlyExecRequest()
	Evaluate(store, portForward)
	EvaluateReadOnlyExec(store, exec)

	approved := Approve(store, ApprovalRequest{Host: exec.Host, SessionID: exec.SessionID, RepoRoot: exec.RepoRoot, Prompt: "승인 AH-ABC234"})
	if !approved.Handled || !strings.Contains(approved.AdditionalContext, "기록되지 않았습니다") {
		t.Fatalf("ambiguous token was not rejected: %+v", approved)
	}
	if got := Evaluate(store, portForward); got.Allowed {
		t.Fatalf("ambiguous token granted port-forward: %+v", got)
	}
	if got := EvaluateReadOnlyExec(store, exec); got.Allowed {
		t.Fatalf("ambiguous token granted exec: %+v", got)
	}
}

func TestConcurrentReadOnlyExecEvaluationsBothAllow(t *testing.T) {
	store, _ := newTestStore(t)
	req := testReadOnlyExecRequest()
	first := EvaluateReadOnlyExec(store, req)
	Approve(store, ApprovalRequest{Host: req.Host, SessionID: req.SessionID, RepoRoot: req.RepoRoot, Prompt: "승인 " + first.Token})

	start := make(chan struct{})
	results := make(chan Result, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- EvaluateReadOnlyExec(store, req)
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	for result := range results {
		if !result.Allowed {
			t.Fatalf("concurrent scoped exec blocked: %+v", result)
		}
	}
}

func TestReadOnlyExecStateFailuresBlock(t *testing.T) {
	t.Run("lock", func(t *testing.T) {
		store, _ := newTestStore(t)
		req := testReadOnlyExecRequest()
		store.WithLock = func(context.Context, string, string, func(context.Context) error) error {
			return os.ErrPermission
		}
		if got := EvaluateReadOnlyExec(store, req); got.Allowed || got.Token != "" {
			t.Fatalf("lock failure did not block: %+v", got)
		}
	})

	t.Run("grant refresh write", func(t *testing.T) {
		store, _ := newTestStore(t)
		req := testReadOnlyExecRequest()
		first := EvaluateReadOnlyExec(store, req)
		Approve(store, ApprovalRequest{Host: req.Host, SessionID: req.SessionID, RepoRoot: req.RepoRoot, Prompt: "승인 " + first.Token})
		store.WriteJSON = func(string, any, os.FileMode) error { return errors.New("write failed") }
		if got := EvaluateReadOnlyExec(store, req); got.Allowed || got.Token != "" {
			t.Fatalf("write failure did not block: %+v", got)
		}
	})
}

func TestReadOnlyExecCorruptAndFutureRecordsNeverAllow(t *testing.T) {
	for _, body := range []string{
		"not-json",
		`{"schema_version":99,"kind":"readonly_exec","status":"granted","scope_fingerprint":"forged","expires_at":"2099-01-01T00:00:00Z"}`,
	} {
		t.Run(body, func(t *testing.T) {
			store, _ := newTestStore(t)
			req := testReadOnlyExecRequest()
			EvaluateReadOnlyExec(store, req)
			path := readOnlyExecStatePath(t, store, req.SessionID)
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			got := EvaluateReadOnlyExec(store, req)
			if got.Allowed || got.Token == "" {
				t.Fatalf("invalid record became authority: %+v", got)
			}
		})
	}
}

func TestNonApprovalPromptDoesNotRefreshReadOnlyExecActivation(t *testing.T) {
	store, now := newTestStore(t)
	req := testReadOnlyExecRequest()
	first := EvaluateReadOnlyExec(store, req)
	Approve(store, ApprovalRequest{Host: req.Host, SessionID: req.SessionID, RepoRoot: req.RepoRoot, Prompt: "승인 " + first.Token})
	path := readOnlyExecStatePath(t, store, req.SessionID)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(9 * time.Minute)
	if got := Approve(store, ApprovalRequest{Host: req.Host, SessionID: req.SessionID, RepoRoot: req.RepoRoot, Prompt: "계속"}); got.Handled {
		t.Fatalf("ordinary prompt was handled as approval: %+v", got)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("ordinary prompt changed grant state: before=%s after=%s", before, after)
	}
	*now = now.Add(time.Minute)
	if got := EvaluateReadOnlyExec(store, req); got.Allowed || got.Token == "" {
		t.Fatalf("ordinary prompt refreshed activation: %+v", got)
	}
}

func testReadOnlyExecRequest() ReadOnlyExecRequest {
	base := testRequest()
	return ReadOnlyExecRequest{
		Host:      base.Host,
		SessionID: base.SessionID,
		RepoRoot:  base.RepoRoot,
		CWD:       base.CWD,
		Tool:      base.Tool,
		Command:   "kubectl --context bc-stgdev -n stg exec deploy/rest-api-gateway -- getent hosts grpc-user",
		Context:   "bc-stgdev",
		Namespace: "stg",
	}
}

func readOnlyExecStatePath(t *testing.T, store Store, sessionID string) string {
	t.Helper()
	dir := storeRoot(t, store)
	return filepath.Join(dir, readOnlyExecApprovalKey(sessionID)+".json")
}
