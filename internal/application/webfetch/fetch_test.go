package webfetch

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	webfetchcontract "agent-harness/internal/contract/webfetch"
	webfetchport "agent-harness/internal/port/webfetch"
)

func TestFetchOwnsRetryOrderingAndResponseBodyFlow(t *testing.T) {
	client := &sequenceClient{responses: []webfetchport.HTTPResponse{
		{StatusCode: 429, Header: map[string][]string{"Retry-After": {"1"}}, Body: io.NopCloser(strings.NewReader("slow down"))},
		{StatusCode: 200, Header: map[string][]string{"Content-Type": {"text/plain"}}, Body: io.NopCloser(strings.NewReader(strings.Repeat("accepted body ", 50)))},
	}}
	var slept time.Duration
	result, err := Fetch(context.Background(), webfetchcontract.Request{URL: "https://example.test", MaxChars: 100}, Dependencies{
		HTTPClient: client,
		URLPolicy:  allowURLPolicy{},
		Now:        func() time.Time { return time.Date(2026, 8, 4, 1, 2, 3, 0, time.UTC) },
		RetryAfter: func(string) time.Duration { return 25 * time.Millisecond },
		Sleep:      func(_ context.Context, delay time.Duration) { slept = delay },
	})
	if err != nil || !result.OK || !strings.HasPrefix(result.Content, "accepted body") {
		t.Fatalf("Fetch() = %+v, %v", result, err)
	}
	if client.calls != 2 || slept != 25*time.Millisecond || result.Metadata["retry_after_retried"] != true {
		t.Fatalf("calls=%d slept=%s metadata=%v", client.calls, slept, result.Metadata)
	}
}

func TestFetchStopsWhenRequestTimeoutExpires(t *testing.T) {
	started := time.Now()
	result, err := Fetch(context.Background(), webfetchcontract.Request{
		URL:     "https://example.test",
		Timeout: 10 * time.Millisecond,
	}, Dependencies{
		HTTPClient: contextDeadlineClient{},
		URLPolicy:  allowURLPolicy{},
	})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if result.OK || result.StopReason != webfetchcontract.StopReasonError {
		t.Fatalf("Fetch() = %+v, want bounded request error", result)
	}
	if !strings.Contains(strings.Join(result.Warnings, " "), context.DeadlineExceeded.Error()) {
		t.Fatalf("warnings=%v, want deadline exceeded", result.Warnings)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("Fetch() elapsed=%s, want bounded timeout", elapsed)
	}
}

type allowURLPolicy struct{}

func (allowURLPolicy) Validate(context.Context, string, bool) (string, error) {
	return "https://example.test", nil
}

type contextDeadlineClient struct{}

func (contextDeadlineClient) Do(ctx context.Context, _ webfetchport.HTTPRequest) (webfetchport.HTTPResponse, error) {
	<-ctx.Done()
	return webfetchport.HTTPResponse{}, ctx.Err()
}

type sequenceClient struct {
	responses []webfetchport.HTTPResponse
	calls     int
}

func (c *sequenceClient) Do(context.Context, webfetchport.HTTPRequest) (webfetchport.HTTPResponse, error) {
	response := c.responses[c.calls]
	c.calls++
	return response, nil
}
