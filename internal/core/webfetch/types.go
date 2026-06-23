package webfetch

import (
	"context"
	"net"
	"net/http"
	"time"
)

const (
	CategoryStrongOK     = "strong_ok"
	CategoryWeakOK       = "weak_ok"
	CategorySuspectOK    = "suspect_ok"
	CategoryChallenge    = "challenge"
	CategoryBlocked      = "blocked"
	CategoryRateLimited  = "rate_limited"
	CategoryAuthRequired = "auth_required"
	CategoryPaywalled    = "paywalled"
	CategoryNotFound     = "not_found"
	CategoryUnknown      = "unknown"
)

const (
	StopReasonAccepted       = "accepted"
	StopReasonSafetyRejected = "safety_rejected"
	StopReasonUnsafeRedirect = "unsafe_redirect"
	StopReasonRedirectLoop   = "redirect_loop"
	StopReasonGridExhausted  = "grid_exhausted"
	StopReasonRateLimited    = "rate_limited"
	StopReasonNotFound       = "not_found"
	StopReasonError          = "error"
)

type Request struct {
	URL                 string
	Timeout             time.Duration
	MaxChars            int
	HTTPClient          *http.Client
	Resolver            Resolver
	Now                 func() time.Time
	AllowPrivateNetwork bool
}

type Resolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

type Result struct {
	OK              bool           `json:"ok"`
	URL             string         `json:"url"`
	FinalURL        string         `json:"final_url"`
	Category        string         `json:"category"`
	StopReason      string         `json:"stop_reason"`
	GridExhausted   bool           `json:"grid_exhausted"`
	AttemptedRoutes []RouteRecord  `json:"attempted_routes"`
	UntriedRoutes   []RouteRecord  `json:"untried_routes"`
	Content         string         `json:"content"`
	Metadata        map[string]any `json:"metadata"`
	Warnings        []string       `json:"warnings"`
	RetrievedAt     string         `json:"retrieved_at"`
	DurationMS      int64          `json:"duration_ms"`
}

type RouteRecord struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type ResponseValidationInput struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	URL        string
}

type ResponseValidation struct {
	Category   string
	StopReason string
	Strong     bool
	Content    string
	Metadata   map[string]any
	Warnings   []string
}

type BenchmarkRequest struct {
	Fixtures            []BenchmarkFixture
	Timeout             time.Duration
	Live                bool
	LiveOptIn           bool
	CompareCommand      string
	AllowPrivateNetwork bool
}

type BenchmarkFixture struct {
	ID             string            `json:"id"`
	URL            string            `json:"url,omitempty"`
	StatusCode     int               `json:"status_code"`
	Headers        map[string]string `json:"headers"`
	Body           string            `json:"body"`
	Expected       []string          `json:"expected"`
	MinBodyChars   int               `json:"min_body_chars"`
	UnsafeRedirect bool              `json:"unsafe_redirect"`
	RedirectLoop   bool              `json:"redirect_loop"`
}

type BenchmarkResult struct {
	OK                  bool                  `json:"ok"`
	Score               float64               `json:"score"`
	FixtureCount        int                   `json:"fixture_count"`
	HardFailures        []string              `json:"hard_failures,omitempty"`
	SafetyPassRate      float64               `json:"safety_pass_rate"`
	FalseStrongOK       int                   `json:"false_strong_ok"`
	FixtureResults      []BenchmarkFixtureRun `json:"fixture_results"`
	LiveParityEvaluated bool                  `json:"live_parity_evaluated"`
	LiveParityStatus    string                `json:"live_parity_status"`
	LiveParityReport    LiveParityReport      `json:"live_parity_report"`
	DimensionScores     map[string]float64    `json:"dimension_scores"`
}

type BenchmarkFixtureRun struct {
	ID         string `json:"id"`
	OK         bool   `json:"ok"`
	Category   string `json:"category"`
	StopReason string `json:"stop_reason"`
	Failure    string `json:"failure,omitempty"`
	LatencyMS  int64  `json:"latency_ms,omitempty"`
}

type LiveParityReport struct {
	SuccessRate          float64  `json:"success_rate"`
	CategoryAgreement    float64  `json:"category_agreement"`
	RouteCount           int      `json:"route_count"`
	LatencyP50MS         int64    `json:"latency_p50_ms"`
	LatencyP95MS         int64    `json:"latency_p95_ms"`
	SafetyFailures       int      `json:"safety_failures"`
	FalseStrongOK        int      `json:"false_strong_ok"`
	BaselineAvailable    bool     `json:"baseline_available"`
	BaselineCommit       string   `json:"baseline_commit,omitempty"`
	BaselineSuccessRate  float64  `json:"baseline_success_rate,omitempty"`
	BaselineLatencyP50MS int64    `json:"baseline_latency_p50_ms,omitempty"`
	BaselineLatencyP95MS int64    `json:"baseline_latency_p95_ms,omitempty"`
	Warnings             []string `json:"warnings"`
}
