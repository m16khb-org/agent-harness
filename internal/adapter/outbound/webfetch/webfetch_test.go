package webfetch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	webfetchcontract "agent-harness/internal/contract/webfetch"
	webfetchdomain "agent-harness/internal/domain/webfetch"
)

func TestFetchRejectsUnsafeURLsBeforeNetwork(t *testing.T) {
	var calls atomic.Int64
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, nil
	})}

	for _, rawURL := range []string{
		"file:///etc/passwd",
		"http://localhost/private",
		"http://127.0.0.1/private",
		"http://169.254.169.254/latest/meta-data",
		"http://192.0.2.1/reserved",
		"http://198.51.100.1/reserved",
		"http://203.0.113.1/reserved",
		"http://240.0.0.1/reserved",
		"http://[2001:db8::1]/reserved",
		"http://[::1]/private",
		"http://user:secret@example.com/private",
	} {
		result, err := FetchWithOptions(context.Background(), webfetchcontract.Request{URL: rawURL, Timeout: time.Second}, Options{HTTPClient: newHTTPClient(client)})
		if err != nil {
			t.Fatalf("Fetch(%q) returned unexpected error: %v", rawURL, err)
		}
		if result.OK {
			t.Fatalf("Fetch(%q) OK=true, want safety rejection", rawURL)
		}
		if result.StopReason != webfetchcontract.StopReasonSafetyRejected {
			t.Fatalf("Fetch(%q) stop_reason=%q, want %q", rawURL, result.StopReason, webfetchcontract.StopReasonSafetyRejected)
		}
		if strings.Contains(result.URL, "secret") {
			t.Fatalf("Fetch(%q) leaked URL userinfo in result URL: %q", rawURL, result.URL)
		}
	}

	if calls.Load() != 0 {
		t.Fatalf("unsafe URLs reached network transport %d times", calls.Load())
	}
}

func TestFetchRejectsUnsafeResolvedHostBeforeNetwork(t *testing.T) {
	var calls atomic.Int64
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, nil
	})}
	resolver := fakeResolver{ips: []string{"169.254.169.254"}}

	result, err := FetchWithOptions(context.Background(), webfetchcontract.Request{
		URL:     "http://public.example/resource",
		Timeout: time.Second,
	}, Options{HTTPClient: newHTTPClient(client), Resolver: resolver})
	if err != nil {
		t.Fatalf("Fetch returned unexpected error: %v", err)
	}
	if result.StopReason != webfetchcontract.StopReasonSafetyRejected {
		t.Fatalf("stop_reason=%q, want %q", result.StopReason, webfetchcontract.StopReasonSafetyRejected)
	}
	if calls.Load() != 0 {
		t.Fatalf("resolved unsafe host reached network transport %d times", calls.Load())
	}
}

func TestFetchStopsBeforeUnsafeRedirectTarget(t *testing.T) {
	var finalTargetFetched atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "http://169.254.169.254/latest/meta-data", http.StatusFound)
			return
		}
		finalTargetFetched.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("should not be fetched"))
	}))
	defer server.Close()

	result, err := Fetch(context.Background(), webfetchcontract.Request{URL: server.URL + "/start", Timeout: time.Second, AllowPrivateNetwork: true})
	if err != nil {
		t.Fatalf("Fetch returned unexpected error: %v", err)
	}
	if result.OK {
		t.Fatalf("unsafe redirect returned OK result: %#v", result)
	}
	if result.StopReason != webfetchcontract.StopReasonUnsafeRedirect {
		t.Fatalf("stop_reason=%q, want %q", result.StopReason, webfetchcontract.StopReasonUnsafeRedirect)
	}
	if finalTargetFetched.Load() != 0 {
		t.Fatalf("unsafe redirect target was fetched %d times", finalTargetFetched.Load())
	}
}

func TestFetchReportsRedirectLoopExplicitly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/loop", http.StatusFound)
	}))
	defer server.Close()

	result, err := Fetch(context.Background(), webfetchcontract.Request{URL: server.URL + "/loop", Timeout: time.Second, AllowPrivateNetwork: true})
	if err != nil {
		t.Fatalf("Fetch returned unexpected error: %v", err)
	}
	if result.StopReason != webfetchcontract.StopReasonRedirectLoop {
		t.Fatalf("stop_reason=%q, want %q", result.StopReason, webfetchcontract.StopReasonRedirectLoop)
	}
	if len(result.AttemptedRoutes) != 1 {
		t.Fatalf("attempted_routes=%v, want one stable direct route", result.AttemptedRoutes)
	}
}

func TestFetchReportsNonZeroDurationForDelayedRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Millisecond)
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<main>" + strings.Repeat("article body ", 60) + "</main>"))
	}))
	defer server.Close()

	result, err := Fetch(context.Background(), webfetchcontract.Request{URL: server.URL, Timeout: time.Second, AllowPrivateNetwork: true})
	if err != nil {
		t.Fatalf("Fetch returned unexpected error: %v", err)
	}
	if result.DurationMS <= 0 {
		t.Fatalf("duration_ms=%d, want non-zero elapsed time", result.DurationMS)
	}
}

func TestValidateResponseCategories(t *testing.T) {
	cases := []struct {
		name         string
		status       int
		contentType  string
		body         string
		wantCategory string
		wantStrong   bool
	}{
		{
			name:         "article body is strong",
			status:       http.StatusOK,
			contentType:  "text/html",
			body:         "<html><main>" + strings.Repeat("public article body ", 40) + "</main></html>",
			wantCategory: webfetchcontract.CategoryStrongOK,
			wantStrong:   true,
		},
		{
			name:         "empty SPA shell is not strong",
			status:       http.StatusOK,
			contentType:  "text/html",
			body:         `<html><body><div id="root"></div><script src="/app.js"></script></body></html>`,
			wantCategory: webfetchcontract.CategorySuspectOK,
			wantStrong:   false,
		},
		{
			name:         "captcha is challenge",
			status:       http.StatusOK,
			contentType:  "text/html",
			body:         `<html><title>Just a moment...</title><body>captcha cf-turnstile check your browser</body></html>`,
			wantCategory: webfetchcontract.CategoryChallenge,
			wantStrong:   false,
		},
		{
			name:         "login wall is auth required",
			status:       http.StatusOK,
			contentType:  "text/html",
			body:         `<html><body>Sign in to continue. Log in to view this page.</body></html>`,
			wantCategory: webfetchcontract.CategoryAuthRequired,
			wantStrong:   false,
		},
		{
			name:         "paywall shell is metadata only",
			status:       http.StatusOK,
			contentType:  "text/html",
			body:         `<html><head><meta property="og:title" content="Public headline"></head><body>Subscribe to read this member-only article.</body></html>`,
			wantCategory: webfetchcontract.CategoryPaywalled,
			wantStrong:   false,
		},
		{
			name:         "valid JSON API is strong",
			status:       http.StatusOK,
			contentType:  "application/json",
			body:         `{"ok":true,"items":[{"id":1}]}`,
			wantCategory: webfetchcontract.CategoryStrongOK,
			wantStrong:   true,
		},
		{
			name:         "RSS feed with entries is acceptable",
			status:       http.StatusOK,
			contentType:  "application/rss+xml",
			body:         `<rss><channel><item><title>one</title></item></channel></rss>`,
			wantCategory: webfetchcontract.CategoryWeakOK,
			wantStrong:   false,
		},
		{
			name:         "rate limit is explicit",
			status:       http.StatusTooManyRequests,
			contentType:  "text/html",
			body:         `slow down`,
			wantCategory: webfetchcontract.CategoryRateLimited,
			wantStrong:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := webfetchdomain.ValidateResponse(webfetchcontract.ResponseValidationInput{
				StatusCode: tc.status,
				Header:     http.Header{"Content-Type": []string{tc.contentType}},
				Body:       []byte(tc.body),
			})
			if got.Category != tc.wantCategory {
				t.Fatalf("Category=%q, want %q; validation=%#v", got.Category, tc.wantCategory, got)
			}
			if got.Strong != tc.wantStrong {
				t.Fatalf("Strong=%v, want %v", got.Strong, tc.wantStrong)
			}
		})
	}
}

func TestFetchRecordsDeterministicRouteExhaustion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html><body>Sign in to continue before reading this page.</body></html>`))
	}))
	defer server.Close()

	result, err := Fetch(context.Background(), webfetchcontract.Request{URL: server.URL, Timeout: time.Second, AllowPrivateNetwork: true})
	if err != nil {
		t.Fatalf("Fetch returned unexpected error: %v", err)
	}
	if result.Category != webfetchcontract.CategoryAuthRequired {
		t.Fatalf("category=%q, want %q", result.Category, webfetchcontract.CategoryAuthRequired)
	}
	if !result.GridExhausted {
		t.Fatalf("grid_exhausted=false, want true for boundary stop with skipped reasons")
	}
	if got := routeIDs(result.AttemptedRoutes); strings.Join(got, ",") != "direct_http" {
		t.Fatalf("attempted route ids=%v, want [direct_http]", got)
	}
	if len(result.UntriedRoutes) == 0 {
		t.Fatalf("untried_routes empty, want skipped routes with reasons")
	}
	for _, route := range result.UntriedRoutes {
		if strings.TrimSpace(route.Reason) == "" {
			t.Fatalf("untried route missing skip reason: %#v", route)
		}
	}
}

func TestFetchRetriesRetryAfterOnce(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requests.Add(1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("rate limited request"))
		if count > 2 {
			t.Fatalf("unexpected request count %d; Retry-After should be honored once", count)
		}
	}))
	defer server.Close()

	result, err := Fetch(context.Background(), webfetchcontract.Request{URL: server.URL, Timeout: time.Second, AllowPrivateNetwork: true})
	if err != nil {
		t.Fatalf("Fetch returned unexpected error: %v", err)
	}
	if requests.Load() != 2 {
		t.Fatalf("requests=%d, want exactly one retry after Retry-After", requests.Load())
	}
	if result.Category != webfetchcontract.CategoryRateLimited || result.StopReason != webfetchcontract.StopReasonRateLimited {
		t.Fatalf("unexpected result after retry: %#v", result)
	}
	if retried, _ := result.Metadata["retry_after_retried"].(bool); !retried {
		t.Fatalf("metadata retry_after_retried not recorded: %#v", result.Metadata)
	}
}

func TestRunBenchmarkDeterministicBatteryPassesHardGates(t *testing.T) {
	result, err := RunBenchmark(context.Background(), webfetchcontract.BenchmarkRequest{Fixtures: DeterministicFixtures(), Timeout: time.Second})
	if err != nil {
		t.Fatalf("RunBenchmark returned error: %v", err)
	}
	if !result.OK {
		t.Fatalf("benchmark OK=false: %#v", result)
	}
	if result.Score < 95 {
		t.Fatalf("benchmark score %.1f, want >=95", result.Score)
	}
	if len(result.HardFailures) != 0 {
		t.Fatalf("hard failures: %v", result.HardFailures)
	}
	if result.FixtureCount != 12 {
		t.Fatalf("fixture_count=%d, want 12", result.FixtureCount)
	}
}

func TestRunBenchmarkLiveRequiresExplicitOptIn(t *testing.T) {
	result, err := RunBenchmark(context.Background(), webfetchcontract.BenchmarkRequest{
		Live: true,
		Fixtures: []webfetchcontract.BenchmarkFixture{{
			ID:       "public_article",
			URL:      "https://example.com/article",
			Expected: []string{webfetchcontract.CategoryStrongOK},
		}},
	})
	if err == nil {
		t.Fatalf("RunBenchmark returned nil error, want explicit live opt-in failure")
	}
	if !strings.Contains(err.Error(), "HARNESS_WEBFETCH_LIVE=1") {
		t.Fatalf("error=%v, want HARNESS_WEBFETCH_LIVE=1 guidance", err)
	}
	if result.LiveParityEvaluated {
		t.Fatalf("live parity evaluated without opt-in: %#v", result)
	}
}

func TestRunBenchmarkLiveReportsParityMetrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<main>" + strings.Repeat("live article body ", 60) + "</main>"))
	}))
	defer server.Close()

	result, err := RunBenchmark(context.Background(), webfetchcontract.BenchmarkRequest{
		Live:                true,
		LiveOptIn:           true,
		AllowPrivateNetwork: true,
		Timeout:             time.Second,
		Fixtures: []webfetchcontract.BenchmarkFixture{{
			ID:           "public_article",
			URL:          server.URL,
			Expected:     []string{webfetchcontract.CategoryStrongOK},
			MinBodyChars: 500,
		}},
	})
	if err != nil {
		t.Fatalf("RunBenchmark returned error: %v", err)
	}
	if !result.OK {
		t.Fatalf("live benchmark OK=false: %#v", result)
	}
	if !result.LiveParityEvaluated {
		t.Fatalf("live parity not evaluated: %#v", result)
	}
	if result.LiveParityReport.SuccessRate != 100 {
		t.Fatalf("success_rate=%.1f, want 100", result.LiveParityReport.SuccessRate)
	}
	if result.LiveParityReport.CategoryAgreement != 100 {
		t.Fatalf("category_agreement=%.1f, want 100", result.LiveParityReport.CategoryAgreement)
	}
	if result.LiveParityReport.RouteCount != 1 {
		t.Fatalf("route_count=%d, want 1", result.LiveParityReport.RouteCount)
	}
	if result.LiveParityReport.SafetyFailures != 0 || result.LiveParityReport.FalseStrongOK != 0 {
		t.Fatalf("unexpected live safety report: %#v", result.LiveParityReport)
	}
}

func TestRunBenchmarkLiveRunsBaselineComparatorAsBlackBox(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<main>" + strings.Repeat("live article body ", 60) + "</main>"))
	}))
	defer server.Close()

	comparator := filepath.Join(t.TempDir(), "baseline-fetch")
	if err := os.WriteFile(comparator, []byte("#!/bin/sh\nprintf '%s\\n' '{\"ok\":true,\"category\":\"strong_ok\"}'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := RunBenchmark(context.Background(), webfetchcontract.BenchmarkRequest{
		Live:                true,
		LiveOptIn:           true,
		AllowPrivateNetwork: true,
		CompareCommand:      comparator,
		Timeout:             10 * time.Second,
		Fixtures: []webfetchcontract.BenchmarkFixture{{
			ID:           "public_article",
			URL:          server.URL,
			Expected:     []string{webfetchcontract.CategoryStrongOK},
			MinBodyChars: 500,
		}},
	})
	if err != nil {
		t.Fatalf("RunBenchmark returned error: %v", err)
	}
	if !result.LiveParityReport.BaselineAvailable {
		t.Fatalf("baseline comparator was not marked available: %#v", result.LiveParityReport)
	}
	if result.LiveParityReport.BaselineSuccessRate != 100 {
		t.Fatalf("baseline_success_rate=%.1f, want 100", result.LiveParityReport.BaselineSuccessRate)
	}
}

func TestResultJSONIncludesRequiredFieldsForSafetyRejection(t *testing.T) {
	result, err := Fetch(context.Background(), webfetchcontract.Request{URL: "http://127.0.0.1/private", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"ok", "url", "final_url", "category", "stop_reason", "grid_exhausted", "attempted_routes", "untried_routes", "content", "metadata", "warnings", "retrieved_at", "duration_ms"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("serialized result missing required field %q: %s", key, string(raw))
		}
	}
	if payload["attempted_routes"] == nil {
		t.Fatalf("attempted_routes serialized as null: %s", string(raw))
	}
}

func routeIDs(routes []webfetchcontract.RouteRecord) []string {
	ids := make([]string, 0, len(routes))
	for _, route := range routes {
		ids = append(ids, route.ID)
	}
	return ids
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type fakeResolver struct {
	ips []string
	err error
}

func (f fakeResolver) LookupIPAddr(context.Context, string) ([]string, error) {
	return f.ips, f.err
}

// live 전용 fixture(StatusCode 0)를 오프라인 재생할 때 로컬 서버가
// WriteHeader(0)으로 패닉하지 않는지 잠근다(2026-08-22 실측 회귀).
func TestRunBenchmarkOfflineReplayToleratesUnspecifiedStatus(t *testing.T) {
	liveOnly := []webfetchcontract.BenchmarkFixture{
		{ID: "live-homepage", URL: "https://example.com/", MinBodyChars: 1},
	}
	result, err := RunBenchmark(context.Background(), webfetchcontract.BenchmarkRequest{
		Fixtures: liveOnly, Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("offline replay must not error on live-only fixtures: %v", err)
	}
	if len(result.FixtureResults) != 1 {
		t.Fatalf("expected one result: %+v", result)
	}
	// 오프라인 재생은 빈 본문으로 실패한다 - 패닉이 아니라 예측된 실패.
	if result.FixtureResults[0].OK {
		t.Fatalf("empty replay should fail min_body_chars, not pass: %+v", result.FixtureResults[0])
	}
}
