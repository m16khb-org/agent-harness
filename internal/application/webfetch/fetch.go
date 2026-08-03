package webfetch

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	webfetchcontract "agent-harness/internal/contract/webfetch"
	webfetchdomain "agent-harness/internal/domain/webfetch"
	webfetchport "agent-harness/internal/port/webfetch"
)

const maxRedirects = 8

type Dependencies struct {
	HTTPClient webfetchport.HTTPClient
	URLPolicy  webfetchport.URLPolicy
	Now        func() time.Time
	RetryAfter func(value string) time.Duration
	Sleep      func(context.Context, time.Duration)
}

func Fetch(ctx context.Context, request webfetchcontract.Request, dependencies Dependencies) (webfetchcontract.Result, error) {
	started := time.Now()
	now := started
	if dependencies.Now != nil {
		now = dependencies.Now()
	}
	result := webfetchcontract.Result{
		URL:             webfetchdomain.SanitizeURL(request.URL),
		Category:        webfetchcontract.CategoryUnknown,
		StopReason:      webfetchcontract.StopReasonError,
		AttemptedRoutes: []webfetchcontract.RouteRecord{},
		UntriedRoutes:   []webfetchcontract.RouteRecord{},
		Metadata:        map[string]any{},
		Warnings:        []string{},
		RetrievedAt:     now.UTC().Format(time.RFC3339Nano),
	}
	timeout := request.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if dependencies.URLPolicy == nil || dependencies.HTTPClient == nil {
		return result, fmt.Errorf("web fetch dependencies are required")
	}
	current, err := dependencies.URLPolicy.Validate(ctx, request.URL, request.AllowPrivateNetwork)
	if err != nil {
		result.Category = webfetchcontract.CategoryBlocked
		result.StopReason = webfetchcontract.StopReasonSafetyRejected
		result.GridExhausted = true
		result.Warnings = append(result.Warnings, err.Error())
		result.UntriedRoutes = SkippedRoutes("safety_rejected")
		return FinalizeResult(result, request.MaxChars, started), nil
	}
	result.FinalURL = webfetchdomain.SanitizeURL(current)
	result.AttemptedRoutes = append(result.AttemptedRoutes, webfetchcontract.RouteRecord{ID: "direct_http", Status: "attempted"})

	visited := map[string]bool{}
	retriedAfter := false
	for redirects := 0; redirects <= maxRedirects; redirects++ {
		if visited[current] {
			return stopped(result, request.MaxChars, started, webfetchcontract.CategoryUnknown, webfetchcontract.StopReasonRedirectLoop, "redirect_loop"), nil
		}
		visited[current] = true
		response, requestErr := dependencies.HTTPClient.Do(ctx, webfetchport.HTTPRequest{
			URL:    current,
			Header: map[string]string{"User-Agent": "agent-harness-web-fetch/1.0"},
		})
		if requestErr != nil {
			result.Warnings = append(result.Warnings, requestErr.Error())
			return stopped(result, request.MaxChars, started, webfetchcontract.CategoryUnknown, webfetchcontract.StopReasonError, "request_error"), nil
		}

		if webfetchdomain.IsRedirect(response.StatusCode) {
			location := webfetchdomain.HeaderValue(response.Header, "Location")
			_ = response.Body.Close()
			if strings.TrimSpace(location) == "" {
				return stopped(result, request.MaxChars, started, webfetchcontract.CategoryUnknown, webfetchcontract.StopReasonGridExhausted, "empty_redirect_location"), nil
			}
			next, resolveErr := webfetchdomain.ResolveRedirect(current, location)
			if resolveErr != nil {
				result.Warnings = append(result.Warnings, resolveErr.Error())
				return stopped(result, request.MaxChars, started, webfetchcontract.CategoryUnknown, webfetchcontract.StopReasonUnsafeRedirect, "unsafe_redirect"), nil
			}
			validated, validateErr := dependencies.URLPolicy.Validate(ctx, next, request.AllowPrivateNetwork)
			if validateErr != nil {
				result.Warnings = append(result.Warnings, validateErr.Error())
				return stopped(result, request.MaxChars, started, webfetchcontract.CategoryBlocked, webfetchcontract.StopReasonUnsafeRedirect, "unsafe_redirect"), nil
			}
			current = validated
			result.FinalURL = webfetchdomain.SanitizeURL(current)
			continue
		}

		body, readErr := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024))
		_ = response.Body.Close()
		if readErr != nil {
			result.Warnings = append(result.Warnings, readErr.Error())
			return stopped(result, request.MaxChars, started, webfetchcontract.CategoryUnknown, webfetchcontract.StopReasonError, "read_error"), nil
		}
		validation := webfetchdomain.ValidateResponse(webfetchcontract.ResponseValidationInput{
			StatusCode: response.StatusCode,
			Header:     response.Header,
			Body:       body,
			URL:        current,
		})
		if validation.Category == webfetchcontract.CategoryRateLimited && webfetchdomain.HeaderValue(response.Header, "Retry-After") != "" && !retriedAfter {
			retriedAfter = true
			serviceRetryDelay(ctx, dependencies, webfetchdomain.HeaderValue(response.Header, "Retry-After"))
			delete(visited, current)
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
		result.OK = validation.Category == webfetchcontract.CategoryStrongOK || validation.Category == webfetchcontract.CategoryWeakOK || validation.Category == webfetchcontract.CategoryPaywalled
		if validation.StopReason == webfetchcontract.StopReasonAccepted {
			result.GridExhausted = false
			result.UntriedRoutes = SkippedRoutes("not_needed_after_success")
		} else {
			result.GridExhausted = true
			result.UntriedRoutes = SkippedRoutes(validation.Category)
		}
		return FinalizeResult(result, request.MaxChars, started), nil
	}
	return stopped(result, request.MaxChars, started, webfetchcontract.CategoryUnknown, webfetchcontract.StopReasonRedirectLoop, "redirect_loop"), nil
}

func stopped(result webfetchcontract.Result, maxChars int, started time.Time, category, reason, skippedReason string) webfetchcontract.Result {
	result.Category = category
	result.StopReason = reason
	result.GridExhausted = true
	result.UntriedRoutes = SkippedRoutes(skippedReason)
	return FinalizeResult(result, maxChars, started)
}

func serviceRetryDelay(ctx context.Context, dependencies Dependencies, value string) {
	if dependencies.RetryAfter == nil {
		return
	}
	delay := dependencies.RetryAfter(value)
	if delay <= 0 {
		return
	}
	if delay > 100*time.Millisecond {
		delay = 100 * time.Millisecond
	}
	if dependencies.Sleep != nil {
		dependencies.Sleep(ctx, delay)
		return
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}
