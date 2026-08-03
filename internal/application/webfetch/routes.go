package webfetch

import (
	"time"

	webfetchcontract "agent-harness/internal/contract/webfetch"
	webfetchdomain "agent-harness/internal/domain/webfetch"
)

var scheduledRoutes = [...]string{"direct_http", "jina_reader", "mobile_variant", "feed_variant"}

func SkippedRoutes(reason string) []webfetchcontract.RouteRecord {
	routes := make([]webfetchcontract.RouteRecord, 0, len(scheduledRoutes)-1)
	for _, id := range scheduledRoutes[1:] {
		routes = append(routes, webfetchcontract.RouteRecord{ID: id, Status: "skipped", Reason: reason})
	}
	return routes
}

func FinalizeResult(result webfetchcontract.Result, maxChars int, started time.Time) webfetchcontract.Result {
	result.DurationMS = time.Since(started).Milliseconds()
	result.Content = webfetchdomain.TruncateContent(result.Content, maxChars)
	if result.Metadata == nil {
		result.Metadata = map[string]any{}
	}
	if result.AttemptedRoutes == nil {
		result.AttemptedRoutes = []webfetchcontract.RouteRecord{}
	}
	if result.UntriedRoutes == nil {
		result.UntriedRoutes = []webfetchcontract.RouteRecord{}
	}
	if result.Warnings == nil {
		result.Warnings = []string{}
	}
	return result
}
