package webfetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxRedirects = 8

var scheduledRoutes = []string{"direct_http", "jina_reader", "mobile_variant", "feed_variant"}

func Fetch(ctx context.Context, req Request) (Result, error) {
	started := time.Now()
	now := started
	if req.Now != nil {
		now = req.Now()
	}
	result := Result{
		URL:             sanitizeURL(req.URL),
		Category:        CategoryUnknown,
		StopReason:      StopReasonError,
		AttemptedRoutes: []RouteRecord{},
		UntriedRoutes:   []RouteRecord{},
		Metadata:        map[string]any{},
		Warnings:        []string{},
		RetrievedAt:     now.UTC().Format(time.RFC3339Nano),
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	u, err := validateFetchURL(ctx, req.URL, req.AllowPrivateNetwork, req.Resolver)
	if err != nil {
		result.Category = CategoryBlocked
		result.StopReason = StopReasonSafetyRejected
		result.GridExhausted = true
		result.Warnings = append(result.Warnings, err.Error())
		result.UntriedRoutes = skippedRoutes("safety_rejected")
		return finalizeResult(result, req.MaxChars, started), nil
	}
	result.FinalURL = sanitizeURL(u.String())

	client := req.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	client = cloneNoRedirectClient(client)
	result.AttemptedRoutes = append(result.AttemptedRoutes, RouteRecord{ID: "direct_http", Status: "attempted"})

	current := u
	visited := map[string]bool{}
	retriedAfter := false
	for redirects := 0; redirects <= maxRedirects; redirects++ {
		if visited[current.String()] {
			result.Category = CategoryUnknown
			result.StopReason = StopReasonRedirectLoop
			result.GridExhausted = true
			result.UntriedRoutes = skippedRoutes("redirect_loop")
			return finalizeResult(result, req.MaxChars, started), nil
		}
		visited[current.String()] = true

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, current.String(), nil)
		if err != nil {
			return result, err
		}
		httpReq.Header.Set("User-Agent", "agent-harness-web-fetch/1.0")
		resp, err := client.Do(httpReq)
		if err != nil {
			result.Category = CategoryUnknown
			result.StopReason = StopReasonError
			result.GridExhausted = true
			result.Warnings = append(result.Warnings, err.Error())
			result.UntriedRoutes = skippedRoutes("request_error")
			return finalizeResult(result, req.MaxChars, started), nil
		}

		if isRedirect(resp.StatusCode) {
			location := resp.Header.Get("Location")
			_ = resp.Body.Close()
			if strings.TrimSpace(location) == "" {
				result.Category = CategoryUnknown
				result.StopReason = StopReasonGridExhausted
				result.GridExhausted = true
				result.UntriedRoutes = skippedRoutes("empty_redirect_location")
				return finalizeResult(result, req.MaxChars, started), nil
			}
			next, err := resolveRedirect(current, location)
			if err != nil {
				result.Category = CategoryUnknown
				result.StopReason = StopReasonUnsafeRedirect
				result.GridExhausted = true
				result.Warnings = append(result.Warnings, err.Error())
				result.UntriedRoutes = skippedRoutes("unsafe_redirect")
				return finalizeResult(result, req.MaxChars, started), nil
			}
			if _, err := validateFetchURL(ctx, next.String(), req.AllowPrivateNetwork, req.Resolver); err != nil {
				result.Category = CategoryBlocked
				result.StopReason = StopReasonUnsafeRedirect
				result.GridExhausted = true
				result.Warnings = append(result.Warnings, err.Error())
				result.UntriedRoutes = skippedRoutes("unsafe_redirect")
				return finalizeResult(result, req.MaxChars, started), nil
			}
			current = next
			result.FinalURL = sanitizeURL(current.String())
			continue
		}

		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
		_ = resp.Body.Close()
		if readErr != nil {
			result.Category = CategoryUnknown
			result.StopReason = StopReasonError
			result.GridExhausted = true
			result.Warnings = append(result.Warnings, readErr.Error())
			result.UntriedRoutes = skippedRoutes("read_error")
			return finalizeResult(result, req.MaxChars, started), nil
		}
		validation := ValidateResponse(ResponseValidationInput{
			StatusCode: resp.StatusCode,
			Header:     resp.Header,
			Body:       body,
			URL:        current.String(),
		})
		if validation.Category == CategoryRateLimited && resp.Header.Get("Retry-After") != "" && !retriedAfter {
			retriedAfter = true
			waitRetryAfter(ctx, resp.Header.Get("Retry-After"))
			delete(visited, current.String())
			continue
		}
		result.Category = validation.Category
		result.StopReason = validation.StopReason
		result.Content = validation.Content
		result.Metadata = validation.Metadata
		if retriedAfter {
			result.Metadata["retry_after_retried"] = true
		}
		result.Warnings = append(result.Warnings, validation.Warnings...)
		result.OK = validation.Category == CategoryStrongOK || validation.Category == CategoryWeakOK || validation.Category == CategoryPaywalled
		if validation.StopReason == StopReasonAccepted {
			result.GridExhausted = false
			result.UntriedRoutes = skippedRoutes("not_needed_after_success")
		} else {
			result.GridExhausted = true
			result.UntriedRoutes = skippedRoutes(validation.Category)
		}
		return finalizeResult(result, req.MaxChars, started), nil
	}

	result.Category = CategoryUnknown
	result.StopReason = StopReasonRedirectLoop
	result.GridExhausted = true
	result.UntriedRoutes = skippedRoutes("redirect_loop")
	return finalizeResult(result, req.MaxChars, started), nil
}

func cloneNoRedirectClient(client *http.Client) *http.Client {
	clone := *client
	clone.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &clone
}

func isRedirect(status int) bool {
	return status == http.StatusMovedPermanently || status == http.StatusFound || status == http.StatusSeeOther ||
		status == http.StatusTemporaryRedirect || status == http.StatusPermanentRedirect
}

func resolveRedirect(base *url.URL, location string) (*url.URL, error) {
	ref, err := url.Parse(location)
	if err != nil {
		return nil, fmt.Errorf("invalid redirect location: %w", err)
	}
	return base.ResolveReference(ref), nil
}

func skippedRoutes(reason string) []RouteRecord {
	out := make([]RouteRecord, 0, len(scheduledRoutes)-1)
	for _, id := range scheduledRoutes[1:] {
		out = append(out, RouteRecord{ID: id, Status: "skipped", Reason: reason})
	}
	return out
}

func finalizeResult(result Result, maxChars int, started time.Time) Result {
	result.DurationMS = time.Since(started).Milliseconds()
	result.Content = truncateContent(result.Content, maxChars)
	if result.Metadata == nil {
		result.Metadata = map[string]any{}
	}
	if result.AttemptedRoutes == nil {
		result.AttemptedRoutes = []RouteRecord{}
	}
	if result.UntriedRoutes == nil {
		result.UntriedRoutes = []RouteRecord{}
	}
	if result.Warnings == nil {
		result.Warnings = []string{}
	}
	return result
}

func waitRetryAfter(ctx context.Context, value string) {
	delay := parseRetryAfter(value, time.Now)
	if delay <= 0 {
		return
	}
	if delay > 100*time.Millisecond {
		delay = 100 * time.Millisecond
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}

func parseRetryAfter(value string, now func() time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		return time.Duration(seconds) * time.Second
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0
	}
	return when.Sub(now())
}
