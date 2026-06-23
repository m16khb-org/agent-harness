package webfetch

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
)

var tagRE = regexp.MustCompile(`(?s)<[^>]+>`)

func ValidateResponse(input ResponseValidationInput) ResponseValidation {
	body := decodeBody(input.Header, input.Body)
	text := strings.TrimSpace(string(body))
	lower := strings.ToLower(text)
	metadata := metadataFromResponse(input.Header, text)
	contentType := strings.ToLower(input.Header.Get("Content-Type"))

	switch input.StatusCode {
	case http.StatusNotFound:
		return ResponseValidation{Category: CategoryNotFound, StopReason: StopReasonNotFound, Metadata: metadata}
	case http.StatusTooManyRequests:
		return ResponseValidation{Category: CategoryRateLimited, StopReason: StopReasonRateLimited, Metadata: metadata}
	case http.StatusUnauthorized:
		return ResponseValidation{Category: CategoryAuthRequired, StopReason: StopReasonGridExhausted, Metadata: metadata}
	case http.StatusForbidden:
		return ResponseValidation{Category: CategoryBlocked, StopReason: StopReasonGridExhausted, Metadata: metadata}
	}
	if input.StatusCode >= 500 {
		return ResponseValidation{Category: CategoryBlocked, StopReason: StopReasonGridExhausted, Metadata: metadata}
	}
	if input.StatusCode < 200 || input.StatusCode >= 300 {
		return ResponseValidation{Category: CategoryUnknown, StopReason: StopReasonGridExhausted, Metadata: metadata}
	}

	if looksLikeChallenge(lower) {
		return ResponseValidation{Category: CategoryChallenge, StopReason: StopReasonGridExhausted, Metadata: metadata}
	}
	if looksLikeLoginWall(lower) {
		return ResponseValidation{Category: CategoryAuthRequired, StopReason: StopReasonGridExhausted, Metadata: metadata}
	}
	if looksLikePaywall(lower) {
		return ResponseValidation{Category: CategoryPaywalled, StopReason: StopReasonGridExhausted, Metadata: metadata}
	}
	if strings.Contains(contentType, "json") {
		var value any
		if err := json.Unmarshal(body, &value); err == nil {
			metadata["json_valid"] = true
			return ResponseValidation{Category: CategoryStrongOK, StopReason: StopReasonAccepted, Strong: true, Content: truncateContent(text, 0), Metadata: metadata}
		}
		return ResponseValidation{Category: CategorySuspectOK, StopReason: StopReasonGridExhausted, Metadata: metadata, Warnings: []string{"malformed_json"}}
	}
	if strings.Contains(contentType, "rss") || strings.Contains(contentType, "atom") || strings.Contains(lower, "<rss") || strings.Contains(lower, "<feed") {
		entries := strings.Count(lower, "<item")
		if entries == 0 {
			entries = strings.Count(lower, "<entry")
		}
		metadata["entries"] = entries
		if entries > 0 {
			return ResponseValidation{Category: CategoryWeakOK, StopReason: StopReasonAccepted, Content: stripTags(text), Metadata: metadata}
		}
	}

	stripped := stripTags(text)
	if looksLikeEmptySPA(lower, stripped) {
		return ResponseValidation{Category: CategorySuspectOK, StopReason: StopReasonGridExhausted, Metadata: metadata, Warnings: []string{"empty_spa_shell"}}
	}
	if len(stripped) >= 500 {
		return ResponseValidation{Category: CategoryStrongOK, StopReason: StopReasonAccepted, Strong: true, Content: stripped, Metadata: metadata}
	}
	if len(stripped) >= 80 {
		return ResponseValidation{Category: CategoryWeakOK, StopReason: StopReasonAccepted, Content: stripped, Metadata: metadata}
	}
	if len(stripped) > 0 {
		return ResponseValidation{Category: CategorySuspectOK, StopReason: StopReasonGridExhausted, Content: stripped, Metadata: metadata}
	}
	return ResponseValidation{Category: CategoryUnknown, StopReason: StopReasonGridExhausted, Metadata: metadata, Warnings: []string{"empty_content"}}
}

func decodeBody(header http.Header, body []byte) []byte {
	if !strings.EqualFold(header.Get("Content-Encoding"), "gzip") {
		return body
	}
	reader, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return body
	}
	defer reader.Close()
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return body
	}
	return decoded
}

func looksLikeChallenge(lower string) bool {
	for _, needle := range []string{"captcha", "recaptcha", "hcaptcha", "cf-turnstile", "just a moment", "checking your browser", "access denied"} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func looksLikeLoginWall(lower string) bool {
	for _, needle := range []string{"sign in to", "log in to", "login to", "로그인"} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func looksLikePaywall(lower string) bool {
	for _, needle := range []string{"subscribe to read", "member-only", "members only", "구독"} {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func looksLikeEmptySPA(lower, stripped string) bool {
	return (strings.Contains(lower, `id="root"`) || strings.Contains(lower, `id='root'`) || strings.Contains(lower, `id="app"`)) && len(stripped) < 100
}

func stripTags(s string) string {
	return strings.Join(strings.Fields(tagRE.ReplaceAllString(s, " ")), " ")
}

func metadataFromResponse(header http.Header, text string) map[string]any {
	metadata := map[string]any{}
	if ct := header.Get("Content-Type"); ct != "" {
		metadata["content_type"] = ct
	}
	if title := extractMetaContent(text, `property="og:title"`); title != "" {
		metadata["og_title"] = title
	}
	return metadata
}

func extractMetaContent(text, marker string) string {
	idx := strings.Index(strings.ToLower(text), strings.ToLower(marker))
	if idx < 0 {
		return ""
	}
	fragment := text[idx:]
	contentIdx := strings.Index(strings.ToLower(fragment), `content=`)
	if contentIdx < 0 {
		return ""
	}
	fragment = fragment[contentIdx+len("content="):]
	if len(fragment) == 0 {
		return ""
	}
	quote := fragment[0]
	if quote != '"' && quote != '\'' {
		return ""
	}
	end := strings.IndexByte(fragment[1:], quote)
	if end < 0 {
		return ""
	}
	return fragment[1 : 1+end]
}

func truncateContent(content string, maxChars int) string {
	if maxChars <= 0 || len(content) <= maxChars {
		return content
	}
	return content[:maxChars]
}
