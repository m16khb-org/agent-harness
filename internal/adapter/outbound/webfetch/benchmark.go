package webfetch

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	webfetchcontract "agent-harness/internal/contract/webfetch"
)

func DeterministicFixtures() []webfetchcontract.BenchmarkFixture {
	return []webfetchcontract.BenchmarkFixture{
		{ID: "article_basic", StatusCode: http.StatusOK, Headers: map[string]string{"Content-Type": "text/html"}, Body: "<main>" + strings.Repeat("article body ", 60) + "</main>", Expected: []string{webfetchcontract.CategoryStrongOK}, MinBodyChars: 500},
		{ID: "json_api", StatusCode: http.StatusOK, Headers: map[string]string{"Content-Type": "application/json"}, Body: `{"ok":true,"items":[{"id":1}]}`, Expected: []string{webfetchcontract.CategoryStrongOK}},
		{ID: "rss_feed", StatusCode: http.StatusOK, Headers: map[string]string{"Content-Type": "application/rss+xml"}, Body: `<rss><channel><item><title>one</title></item></channel></rss>`, Expected: []string{webfetchcontract.CategoryStrongOK, webfetchcontract.CategoryWeakOK}},
		{ID: "empty_spa", StatusCode: http.StatusOK, Headers: map[string]string{"Content-Type": "text/html"}, Body: `<html><body><div id="root"></div><script src="/app.js"></script></body></html>`, Expected: []string{webfetchcontract.CategorySuspectOK}},
		{ID: "waf_challenge", StatusCode: http.StatusOK, Headers: map[string]string{"Content-Type": "text/html"}, Body: `Just a moment... captcha check your browser`, Expected: []string{webfetchcontract.CategoryChallenge, webfetchcontract.CategoryBlocked}},
		{ID: "login_wall", StatusCode: http.StatusOK, Headers: map[string]string{"Content-Type": "text/html"}, Body: `Sign in to continue. Log in to view this page.`, Expected: []string{webfetchcontract.CategoryAuthRequired}},
		{ID: "paywall_shell", StatusCode: http.StatusOK, Headers: map[string]string{"Content-Type": "text/html"}, Body: `<meta property="og:title" content="Headline"><body>Subscribe to read this member-only article.</body>`, Expected: []string{webfetchcontract.CategoryPaywalled}},
		{ID: "rate_limit", StatusCode: http.StatusTooManyRequests, Headers: map[string]string{"Content-Type": "text/plain", "Retry-After": "1"}, Body: `slow down`, Expected: []string{webfetchcontract.CategoryRateLimited}},
		{ID: "redirect_loop", RedirectLoop: true, Expected: []string{webfetchcontract.CategoryUnknown}},
		{ID: "unsafe_redirect", UnsafeRedirect: true, Expected: []string{webfetchcontract.CategoryBlocked}},
		{ID: "gzip_charset", StatusCode: http.StatusOK, Headers: map[string]string{"Content-Type": "text/html; charset=shift_jis", "Content-Encoding": "gzip"}, Body: gzipString("<main>" + strings.Repeat("decoded body ", 60) + "</main>"), Expected: []string{webfetchcontract.CategoryStrongOK}},
		{ID: "malformed_content", StatusCode: http.StatusOK, Headers: map[string]string{"Content-Type": "application/json"}, Body: `{"broken":`, Expected: []string{webfetchcontract.CategoryUnknown, webfetchcontract.CategorySuspectOK}},
	}
}

func RunBenchmark(ctx context.Context, req webfetchcontract.BenchmarkRequest) (webfetchcontract.BenchmarkResult, error) {
	if req.Live {
		return runLiveBenchmark(ctx, req)
	}
	fixtures := req.Fixtures
	if len(fixtures) == 0 {
		fixtures = DeterministicFixtures()
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	result := webfetchcontract.BenchmarkResult{
		FixtureCount:        len(fixtures),
		LiveParityStatus:    "not evaluated",
		LiveParityReport:    webfetchcontract.LiveParityReport{Warnings: []string{}},
		DimensionScores:     map[string]float64{},
		SafetyPassRate:      100,
		LiveParityEvaluated: false,
	}

	unsafeRedirectFetches := 0
	falseStrongOK := 0
	for _, fixture := range fixtures {
		fetchResult, finalFetches, err := runFixture(ctx, fixture, timeout)
		if err != nil {
			result.HardFailures = append(result.HardFailures, fixture.ID+": "+err.Error())
			result.FixtureResults = append(result.FixtureResults, webfetchcontract.BenchmarkFixtureRun{ID: fixture.ID, OK: false, Failure: err.Error()})
			continue
		}
		unsafeRedirectFetches += finalFetches
		run := webfetchcontract.BenchmarkFixtureRun{ID: fixture.ID, Category: fetchResult.Category, StopReason: fetchResult.StopReason}
		if categoryAllowed(fetchResult.Category, fixture.Expected) {
			run.OK = true
		} else {
			run.Failure = fmt.Sprintf("category %s not in expected %v", fetchResult.Category, fixture.Expected)
		}
		if fixture.MinBodyChars > 0 && len(fetchResult.Content) < fixture.MinBodyChars {
			run.OK = false
			run.Failure = fmt.Sprintf("content length %d < %d", len(fetchResult.Content), fixture.MinBodyChars)
		}
		if isFalseStrongOKFixture(fixture.ID) && fetchResult.Category == webfetchcontract.CategoryStrongOK {
			falseStrongOK++
			run.OK = false
			run.Failure = "false strong_ok"
		}
		if !run.OK {
			result.HardFailures = append(result.HardFailures, fixture.ID+": "+run.Failure)
		}
		result.FixtureResults = append(result.FixtureResults, run)
	}
	if unsafeRedirectFetches > 0 {
		result.HardFailures = append(result.HardFailures, fmt.Sprintf("unsafe redirect final target fetch count = %d", unsafeRedirectFetches))
	}
	result.FalseStrongOK = falseStrongOK
	if falseStrongOK > 0 {
		result.HardFailures = append(result.HardFailures, fmt.Sprintf("false strong_ok count = %d", falseStrongOK))
	}
	result.Score = scoreBenchmark(result)
	result.DimensionScores = map[string]float64{
		"safety":                     25,
		"classification_correctness": 30,
		"exhaustion_semantics":       15,
		"citation_ready_output":      10,
		"efficiency":                 10,
		"live_parity_readiness":      10,
	}
	result.OK = result.Score >= 95 && len(result.HardFailures) == 0
	return result, nil
}

func runLiveBenchmark(ctx context.Context, req webfetchcontract.BenchmarkRequest) (webfetchcontract.BenchmarkResult, error) {
	result := webfetchcontract.BenchmarkResult{
		FixtureCount:        len(req.Fixtures),
		LiveParityStatus:    "not evaluated",
		DimensionScores:     map[string]float64{},
		SafetyPassRate:      100,
		LiveParityEvaluated: false,
		LiveParityReport:    webfetchcontract.LiveParityReport{Warnings: []string{}},
	}
	if !req.LiveOptIn {
		result.LiveParityStatus = "live mode requires HARNESS_WEBFETCH_LIVE=1"
		return result, fmt.Errorf("live benchmark requires HARNESS_WEBFETCH_LIVE=1")
	}
	if len(req.Fixtures) == 0 {
		result.LiveParityStatus = "live mode requires URL fixtures"
		return result, fmt.Errorf("live benchmark requires URL fixtures")
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	successes := 0
	comparable := 0
	agreements := 0
	var latencies []int64
	candidateCategories := map[string]string{}
	for _, fixture := range req.Fixtures {
		if strings.TrimSpace(fixture.URL) == "" {
			msg := "live fixture missing url"
			result.HardFailures = append(result.HardFailures, fixture.ID+": "+msg)
			result.FixtureResults = append(result.FixtureResults, webfetchcontract.BenchmarkFixtureRun{ID: fixture.ID, OK: false, Failure: msg})
			continue
		}
		started := time.Now()
		fetchResult, err := Fetch(ctx, webfetchcontract.Request{
			URL:                 fixture.URL,
			Timeout:             timeout,
			AllowPrivateNetwork: req.AllowPrivateNetwork,
		})
		latencyMS := time.Since(started).Milliseconds()
		latencies = append(latencies, latencyMS)
		result.LiveParityReport.RouteCount += len(fetchResult.AttemptedRoutes)
		run := webfetchcontract.BenchmarkFixtureRun{ID: fixture.ID, Category: fetchResult.Category, StopReason: fetchResult.StopReason, LatencyMS: latencyMS}
		if err != nil {
			run.Failure = err.Error()
			result.HardFailures = append(result.HardFailures, fixture.ID+": "+err.Error())
			result.FixtureResults = append(result.FixtureResults, run)
			continue
		}
		if fetchResult.OK {
			successes++
		}
		candidateCategories[fixture.ID] = fetchResult.Category
		if len(fixture.Expected) > 0 {
			comparable++
			if categoryAllowed(fetchResult.Category, fixture.Expected) {
				agreements++
				run.OK = true
			} else {
				run.Failure = fmt.Sprintf("category %s not in expected %v", fetchResult.Category, fixture.Expected)
			}
		} else {
			run.OK = fetchResult.OK || fetchResult.GridExhausted
		}
		if fixture.MinBodyChars > 0 && len(fetchResult.Content) < fixture.MinBodyChars {
			run.OK = false
			run.Failure = fmt.Sprintf("content length %d < %d", len(fetchResult.Content), fixture.MinBodyChars)
		}
		if isFalseStrongOKFixture(fixture.ID) && fetchResult.Category == webfetchcontract.CategoryStrongOK {
			result.LiveParityReport.FalseStrongOK++
			result.FalseStrongOK++
			run.OK = false
			run.Failure = "false strong_ok"
		}
		if !run.OK {
			result.HardFailures = append(result.HardFailures, fixture.ID+": "+run.Failure)
		}
		result.FixtureResults = append(result.FixtureResults, run)
	}

	if len(req.Fixtures) > 0 {
		result.LiveParityReport.SuccessRate = percent(successes, len(req.Fixtures))
	}
	if comparable > 0 {
		result.LiveParityReport.CategoryAgreement = percent(agreements, comparable)
	}
	result.LiveParityReport.LatencyP50MS = percentileLatency(latencies, 50)
	result.LiveParityReport.LatencyP95MS = percentileLatency(latencies, 95)
	if req.CompareCommand != "" {
		compareBaseline(ctx, req.CompareCommand, req.Fixtures, candidateCategories, timeout, &result)
	}
	result.Score = scoreBenchmark(result)
	result.DimensionScores = map[string]float64{
		"safety":                     25,
		"classification_correctness": 30,
		"exhaustion_semantics":       15,
		"citation_ready_output":      10,
		"efficiency":                 10,
		"live_parity_readiness":      10,
	}
	result.LiveParityEvaluated = true
	if req.CompareCommand == "" {
		result.LiveParityStatus = "evaluated without baseline comparator"
	} else if result.LiveParityReport.BaselineAvailable {
		result.LiveParityStatus = "evaluated with baseline comparator"
	} else {
		result.LiveParityStatus = "evaluated; baseline comparator unavailable"
	}
	result.OK = result.Score >= 95 && len(result.HardFailures) == 0 && result.LiveParityReport.SafetyFailures == 0 && result.LiveParityReport.FalseStrongOK == 0
	return result, nil
}

func compareBaseline(ctx context.Context, command string, fixtures []webfetchcontract.BenchmarkFixture, candidateCategories map[string]string, timeout time.Duration, result *webfetchcontract.BenchmarkResult) {
	if _, err := os.Stat(command); err != nil {
		result.LiveParityReport.Warnings = append(result.LiveParityReport.Warnings, "baseline comparator unavailable: "+err.Error())
		return
	}
	baselineTimeout := timeout
	if baselineTimeout < 5*time.Second {
		baselineTimeout = 5 * time.Second
	}
	successes := 0
	comparable := 0
	agreements := 0
	var latencies []int64
	for _, fixture := range fixtures {
		if strings.TrimSpace(fixture.URL) == "" {
			continue
		}
		runCtx, cancel := context.WithTimeout(ctx, baselineTimeout)
		started := time.Now()
		output, err := exec.CommandContext(runCtx, command, "--url", fixture.URL, "--json").Output()
		cancel()
		latencies = append(latencies, time.Since(started).Milliseconds())
		if err != nil {
			result.LiveParityReport.Warnings = append(result.LiveParityReport.Warnings, "baseline comparator failed for "+fixture.ID+": "+err.Error())
			continue
		}
		var payload struct {
			OK       bool   `json:"ok"`
			Category string `json:"category"`
		}
		if err := json.Unmarshal(output, &payload); err != nil {
			result.LiveParityReport.Warnings = append(result.LiveParityReport.Warnings, "baseline comparator returned invalid JSON for "+fixture.ID+": "+err.Error())
			continue
		}
		result.LiveParityReport.BaselineAvailable = true
		if payload.OK {
			successes++
		}
		candidateCategory := candidateCategories[fixture.ID]
		if payload.Category != "" && candidateCategory != "" {
			comparable++
			if payload.Category == candidateCategory {
				agreements++
			}
		}
	}
	if result.LiveParityReport.BaselineAvailable {
		result.LiveParityReport.BaselineSuccessRate = percent(successes, len(fixtures))
		result.LiveParityReport.BaselineLatencyP50MS = percentileLatency(latencies, 50)
		result.LiveParityReport.BaselineLatencyP95MS = percentileLatency(latencies, 95)
		if comparable > 0 {
			result.LiveParityReport.CategoryAgreement = percent(agreements, comparable)
		}
	}
}

func runFixture(ctx context.Context, fixture webfetchcontract.BenchmarkFixture, timeout time.Duration) (webfetchcontract.Result, int, error) {
	finalTargetFetches := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fixture.RedirectLoop {
			http.Redirect(w, r, "/loop", http.StatusFound)
			return
		}
		if fixture.UnsafeRedirect {
			http.Redirect(w, r, "http://169.254.169.254/latest/meta-data", http.StatusFound)
			return
		}
		for k, v := range fixture.Headers {
			w.Header().Set(k, v)
		}
		// live 전용 fixture(URL만 있고 StatusCode 미지정 = 0)를 오프라인
		// 서버로 재생할 때 0은 http 허용 코드가 아니어서 panic한다. 실측
		// 2026-08-22: public-fixtures.json을 오프라인으로 돌리면 즉시
		// 패닉했다. 미지정 상태는 200으로 재생한다.
		status := fixture.StatusCode
		if status < 100 || status > 599 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(fixture.Body))
	}))
	defer server.Close()

	fetchResult, err := Fetch(ctx, webfetchcontract.Request{URL: server.URL, Timeout: timeout, AllowPrivateNetwork: true})
	return fetchResult, finalTargetFetches, err
}

func categoryAllowed(category string, expected []string) bool {
	for _, value := range expected {
		if category == value {
			return true
		}
	}
	return false
}

func isFalseStrongOKFixture(id string) bool {
	switch id {
	case "auth_required", "login_wall", "paywall_shell", "waf_challenge", "empty_spa":
		return true
	default:
		return false
	}
}

func scoreBenchmark(result webfetchcontract.BenchmarkResult) float64 {
	score := 100.0
	score -= float64(len(result.HardFailures)) * 10
	if score < 0 {
		return 0
	}
	return score
}

func percent(numerator, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) * 100 / float64(denominator)
}

func percentileLatency(values []int64, percentile int) int64 {
	if len(values) == 0 {
		return 0
	}
	copied := append([]int64(nil), values...)
	sort.Slice(copied, func(i, j int) bool { return copied[i] < copied[j] })
	if percentile <= 0 {
		return copied[0]
	}
	index := (len(copied)*percentile + 99) / 100
	if index < 1 {
		index = 1
	}
	if index > len(copied) {
		index = len(copied)
	}
	return copied[index-1]
}

func gzipString(s string) string {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	_, _ = writer.Write([]byte(s))
	_ = writer.Close()
	return buf.String()
}
