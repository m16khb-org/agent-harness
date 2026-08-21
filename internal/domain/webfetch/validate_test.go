package webfetch

import (
	"bytes"
	"compress/gzip"
	"strings"
	"testing"

	webfetchcontract "agent-harness/internal/contract/webfetch"
)

// ValidateResponse는 웹 응답을 grid 카테고리로 분류한다. 상태코드/본문
// 신호/콘텐츠 타입별 경계가 정확한 카테고리와 stop reason으로 매핑되는지
// 잠근다. 이 분류는 retry grid의 종료 조건에 직결되므로 회귀가 즉시
// 동작 변화가 된다.
func TestValidateResponseClassifiesStatusCodeBoundaries(t *testing.T) {
	cases := []struct {
		name       string
		statusCode int
		wantCat    string
		wantStop   string
	}{
		{"404 not found", 404, webfetchcontract.CategoryNotFound, webfetchcontract.StopReasonNotFound},
		{"429 rate limited", 429, webfetchcontract.CategoryRateLimited, webfetchcontract.StopReasonRateLimited},
		{"401 auth required", 401, webfetchcontract.CategoryAuthRequired, webfetchcontract.StopReasonGridExhausted},
		{"403 blocked", 403, webfetchcontract.CategoryBlocked, webfetchcontract.StopReasonGridExhausted},
		{"500 blocked", 500, webfetchcontract.CategoryBlocked, webfetchcontract.StopReasonGridExhausted},
		{"302 unknown", 302, webfetchcontract.CategoryUnknown, webfetchcontract.StopReasonGridExhausted},
		{"199 unknown", 199, webfetchcontract.CategoryUnknown, webfetchcontract.StopReasonGridExhausted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidateResponse(webfetchcontract.ResponseValidationInput{StatusCode: tc.statusCode})
			if got.Category != tc.wantCat || got.StopReason != tc.wantStop {
				t.Fatalf("category/stop = %q/%q want %q/%q", got.Category, got.StopReason, tc.wantCat, tc.wantStop)
			}
		})
	}
}

func TestValidateResponseDetectsBodySignalWalls(t *testing.T) {
	long := strings.Repeat("topic ", 200)
	cases := []struct {
		name    string
		body    string
		wantCat string
	}{
		{"captcha challenge", "please complete the recaptcha to continue " + long, webfetchcontract.CategoryChallenge},
		{"turnstile challenge", "cf-turnstile challenge " + long, webfetchcontract.CategoryChallenge},
		{"login wall", "sign in to read the article " + long, webfetchcontract.CategoryAuthRequired},
		{"korean login wall", "로그인 후 이용하실 수 있습니다 " + long, webfetchcontract.CategoryAuthRequired},
		{"paywall", "subscribe to read the full story " + long, webfetchcontract.CategoryPaywalled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidateResponse(webfetchcontract.ResponseValidationInput{StatusCode: 200, Body: []byte(tc.body)})
			if got.Category != tc.wantCat {
				t.Fatalf("category = %q want %q", got.Category, tc.wantCat)
			}
		})
	}
}

func TestValidateResponseContentShapeCategories(t *testing.T) {
	long := strings.Repeat("word ", 200)
	t.Run("valid json is strong", func(t *testing.T) {
		got := ValidateResponse(webfetchcontract.ResponseValidationInput{
			StatusCode: 200,
			Header:     map[string][]string{"Content-Type": {"application/json"}},
			Body:       []byte(`{"ok":true}`),
		})
		if got.Category != webfetchcontract.CategoryStrongOK || !got.Strong || got.Metadata["json_valid"] != true {
			t.Fatalf("json response must be strong ok: %+v", got)
		}
	})
	t.Run("malformed json is suspect", func(t *testing.T) {
		got := ValidateResponse(webfetchcontract.ResponseValidationInput{
			StatusCode: 200,
			Header:     map[string][]string{"Content-Type": {"application/json"}},
			Body:       []byte(`{"ok":`),
		})
		if got.Category != webfetchcontract.CategorySuspectOK {
			t.Fatalf("malformed json must be suspect: %+v", got)
		}
	})
	t.Run("rss with entries is weak ok", func(t *testing.T) {
		got := ValidateResponse(webfetchcontract.ResponseValidationInput{
			StatusCode: 200,
			Header:     map[string][]string{"Content-Type": {"application/rss+xml"}},
			Body:       []byte("<rss><channel><item>a</item><item>b</item></channel></rss>"),
		})
		if got.Category != webfetchcontract.CategoryWeakOK || got.Metadata["entries"] != 2 {
			t.Fatalf("rss with entries must be weak ok with entries=2: %+v", got)
		}
	})
	t.Run("feed without entries falls through", func(t *testing.T) {
		got := ValidateResponse(webfetchcontract.ResponseValidationInput{
			StatusCode: 200,
			Body:       []byte("<feed>nothing here</feed>"),
		})
		if got.Category == webfetchcontract.CategoryWeakOK {
			t.Fatalf("entry-less feed must not be weak ok: %+v", got)
		}
	})
	t.Run("long html is strong ok", func(t *testing.T) {
		got := ValidateResponse(webfetchcontract.ResponseValidationInput{StatusCode: 200, Body: []byte("<html><body>" + long + "</body></html>")})
		if got.Category != webfetchcontract.CategoryStrongOK || !got.Strong {
			t.Fatalf("long html must be strong ok: %+v", got)
		}
	})
	t.Run("medium html is weak ok", func(t *testing.T) {
		got := ValidateResponse(webfetchcontract.ResponseValidationInput{StatusCode: 200, Body: []byte(strings.Repeat("word ", 25))})
		if got.Category != webfetchcontract.CategoryWeakOK || got.Strong {
			t.Fatalf("medium text must be weak ok: %+v", got)
		}
	})
	t.Run("short nonempty is suspect", func(t *testing.T) {
		got := ValidateResponse(webfetchcontract.ResponseValidationInput{StatusCode: 200, Body: []byte("tiny")})
		if got.Category != webfetchcontract.CategorySuspectOK {
			t.Fatalf("short body must be suspect: %+v", got)
		}
	})
	t.Run("empty is unknown with warning", func(t *testing.T) {
		got := ValidateResponse(webfetchcontract.ResponseValidationInput{StatusCode: 200, Body: []byte("  ")})
		if got.Category != webfetchcontract.CategoryUnknown || len(got.Warnings) == 0 || got.Warnings[0] != "empty_content" {
			t.Fatalf("empty body must be unknown/empty_content: %+v", got)
		}
	})
}

func TestDecodeBodyHandlesGzipAndPassthrough(t *testing.T) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	_, _ = writer.Write([]byte("compressed body content"))
	_ = writer.Close()
	got := ValidateResponse(webfetchcontract.ResponseValidationInput{
		StatusCode: 200,
		Header:     map[string][]string{"Content-Encoding": {"gzip"}},
		Body:       buf.Bytes(),
	})
	if !strings.Contains(got.Content, "compressed body content") {
		t.Fatalf("gzip body must be decoded before classification: %+v", got)
	}
	// 손상된 gzip은 원본 바이트로 강등해도 분류 자체는 계속된다.
	broken := decodeBody(map[string][]string{"Content-Encoding": {"gzip"}}, []byte("not-gzip"))
	if string(broken) != "not-gzip" {
		t.Fatalf("corrupt gzip must fall back to the raw body, got %q", broken)
	}
}
