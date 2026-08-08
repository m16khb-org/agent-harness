package liveapproval

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestEvaluateCreatesAndReusesPendingWithoutRawCommand(t *testing.T) {
	store, _ := newTestStore(t)
	req := testRequest()

	first := Evaluate(store, req)
	if !first.Handled || first.Allowed || first.Token != "AH-ABC234" {
		t.Fatalf("unexpected first evaluation: %+v", first)
	}
	second := Evaluate(store, req)
	if second.Token != first.Token {
		t.Fatalf("same pending request rotated token: first=%+v second=%+v", first, second)
	}

	files, err := filepath.Glob(filepath.Join(storeRoot(t, store), "kubectl-live-approval-*.json"))
	if err != nil || len(files) != 1 {
		t.Fatalf("approval state files = %v, err=%v", files, err)
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{req.Command, "grpc-user", "rest-api-gateway"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("state leaked raw command data %q: %s", forbidden, data)
		}
	}
	info, err := os.Stat(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("state mode = %o, want 600", got)
	}
}

func TestEvaluateBindsPendingToEveryRequestField(t *testing.T) {
	base := testRequest()
	tests := []struct {
		name      string
		mutate    func(*Request)
		wantToken bool
	}{
		{name: "host", mutate: func(r *Request) { r.Host = "other-codex" }, wantToken: false},
		{name: "session", mutate: func(r *Request) { r.SessionID = "session-2" }, wantToken: true},
		{name: "repo", mutate: func(r *Request) { r.RepoRoot = t.TempDir() }, wantToken: true},
		{name: "cwd", mutate: func(r *Request) { r.CWD = filepath.Join(r.RepoRoot, "other") }, wantToken: true},
		{name: "tool", mutate: func(r *Request) { r.Tool = "shell" }, wantToken: true},
		{name: "command", mutate: func(r *Request) { r.Command += " --container proxy" }, wantToken: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, _ := newTestStore(t)
			first := Evaluate(store, base)
			changed := base
			tt.mutate(&changed)
			next := Evaluate(store, changed)
			if next.Allowed || next.Token == first.Token || (tt.wantToken && next.Token == "") {
				t.Fatalf("changed %s reused pending grant: first=%+v next=%+v", tt.name, first, next)
			}
		})
	}
}

func TestApproveConsumesPortForwardGrantExactlyOnce(t *testing.T) {
	store, _ := newTestStore(t)
	req := testRequest()
	first := Evaluate(store, req)

	approved := Approve(store, ApprovalRequest{
		Host:      req.Host,
		SessionID: req.SessionID,
		RepoRoot:  req.RepoRoot,
		Prompt:    "승인 " + first.Token,
	})
	if !approved.Handled || approved.Allowed || !strings.Contains(approved.AdditionalContext, "다음 동일 명령 한 번") {
		t.Fatalf("approval prompt did not grant: %+v", approved)
	}
	consumed := Evaluate(store, req)
	if !consumed.Allowed {
		t.Fatalf("granted request was not allowed: %+v", consumed)
	}
	again := Evaluate(store, req)
	if again.Allowed || again.Token == "" || again.Token == first.Token {
		t.Fatalf("grant was reusable: first=%+v again=%+v", first, again)
	}
}

func TestApproveRequiresExactPromptAndMatchingSession(t *testing.T) {
	tests := []struct {
		name    string
		prompt  func(string) string
		session string
		handled bool
	}{
		{name: "wrong token", prompt: func(string) string { return "승인 AH-ZZZZZZ" }, session: "session-1", handled: true},
		{name: "extra prose", prompt: func(token string) string { return "승인 " + token + " 진행해줘" }, session: "session-1", handled: false},
		{name: "lowercase", prompt: func(token string) string { return strings.ToLower("승인 " + token) }, session: "session-1", handled: false},
		{name: "different session", prompt: func(token string) string { return "승인 " + token }, session: "session-2", handled: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, _ := newTestStore(t)
			req := testRequest()
			first := Evaluate(store, req)
			got := Approve(store, ApprovalRequest{
				Host:      req.Host,
				SessionID: tt.session,
				RepoRoot:  req.RepoRoot,
				Prompt:    tt.prompt(first.Token),
			})
			if got.Handled != tt.handled || got.Allowed {
				t.Fatalf("Approve() = %+v, want handled=%v allowed=false", got, tt.handled)
			}
			if consumed := Evaluate(store, req); consumed.Allowed {
				t.Fatalf("invalid approval created a grant: %+v", consumed)
			}
		})
	}
}

func TestPendingAndGrantedExpireAtTenMinutes(t *testing.T) {
	t.Run("pending", func(t *testing.T) {
		store, now := newTestStore(t)
		req := testRequest()
		first := Evaluate(store, req)
		*now = now.Add(ApprovalTTL)
		next := Evaluate(store, req)
		if next.Allowed || next.Token == first.Token {
			t.Fatalf("expired pending record survived: first=%+v next=%+v", first, next)
		}
	})

	t.Run("granted", func(t *testing.T) {
		store, now := newTestStore(t)
		req := testRequest()
		first := Evaluate(store, req)
		Approve(store, ApprovalRequest{Host: req.Host, SessionID: req.SessionID, RepoRoot: req.RepoRoot, Prompt: "승인 " + first.Token})
		*now = now.Add(ApprovalTTL)
		next := Evaluate(store, req)
		if next.Allowed || next.Token == first.Token {
			t.Fatalf("expired grant survived: first=%+v next=%+v", first, next)
		}
	})
}

func TestConcurrentConsumersAllowExactlyOne(t *testing.T) {
	store, _ := newTestStore(t)
	req := testRequest()
	first := Evaluate(store, req)
	Approve(store, ApprovalRequest{Host: req.Host, SessionID: req.SessionID, RepoRoot: req.RepoRoot, Prompt: "승인 " + first.Token})

	start := make(chan struct{})
	results := make(chan Result, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- Evaluate(store, req)
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	allowed := 0
	for result := range results {
		if result.Allowed {
			allowed++
		}
	}
	if allowed != 1 {
		t.Fatalf("concurrent allowed count = %d, want 1", allowed)
	}
}

func testRequest() Request {
	repo, err := filepath.Abs(filepath.Join(os.TempDir(), "liveapproval-repo"))
	if err != nil {
		panic(err)
	}
	return Request{
		Host:      "codex",
		SessionID: "session-1",
		RepoRoot:  repo,
		CWD:       repo,
		Tool:      "Bash",
		Command:   "kubectl --context bc-stgdev -n stg port-forward svc/api 8080:80",
	}
}

func newTestStore(t *testing.T) (Store, *time.Time) {
	t.Helper()
	root := t.TempDir()
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	tokens := []string{"AH-ABC234", "AH-DEF567", "AH-GHJ789", "AH-KLM234", "AH-NPQ567", "AH-RST789", "AH-UVW234", "AH-XYZ567"}
	tokenIndex := 0
	var mu sync.Mutex
	return Store{
		Resolve: func(repoRoot string) (Namespace, error) {
			dir := filepath.Join(root, sessionSafeName(repoRoot))
			_, err := os.Stat(dir)
			return Namespace{Exists: err == nil, Valid: err == nil, RepoRoot: repoRoot, Dir: dir}, nil
		},
		Init: func(repoRoot string) (Namespace, error) {
			dir := filepath.Join(root, sessionSafeName(repoRoot))
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return Namespace{}, err
			}
			return Namespace{Exists: true, Valid: true, RepoRoot: repoRoot, Dir: dir}, nil
		},
		WithLock: func(_ context.Context, _ string, _ string, fn func(context.Context) error) error {
			mu.Lock()
			defer mu.Unlock()
			return fn(context.Background())
		},
		WriteJSON: func(path string, value any, perm os.FileMode) error {
			data, err := json.Marshal(value)
			if err != nil {
				return err
			}
			return os.WriteFile(path, append(data, '\n'), perm)
		},
		Now: func() time.Time { return now },
		NewToken: func() (string, error) {
			token := tokens[tokenIndex%len(tokens)]
			tokenIndex++
			return token, nil
		},
	}, &now
}

func storeRoot(t *testing.T, store Store) string {
	t.Helper()
	namespace, err := store.Resolve(testRequest().RepoRoot)
	if err != nil {
		t.Fatal(err)
	}
	return namespace.Dir
}
