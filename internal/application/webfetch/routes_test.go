package webfetch

import (
	"testing"
	"time"

	webfetchcontract "issueops/internal/contract/webfetch"
)

func TestFinalizeResultPreservesBoundedRouteContract(t *testing.T) {
	result := FinalizeResult(webfetchcontract.Result{Content: "abcdef"}, 3, time.Now())
	if result.Content != "abc" {
		t.Fatalf("content=%q want bounded content", result.Content)
	}
	if result.AttemptedRoutes == nil || result.UntriedRoutes == nil || result.Warnings == nil || result.Metadata == nil {
		t.Fatalf("nil public collections: %+v", result)
	}
	routes := SkippedRoutes("not_needed")
	if len(routes) != 3 || routes[0].ID != "jina_reader" || routes[2].ID != "feed_variant" {
		t.Fatalf("routes=%+v", routes)
	}
}
